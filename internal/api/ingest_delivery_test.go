package api

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/scheduler"
	"github.com/dsionov/carwatch/internal/storage"
)

// fakeDigest implements storage.DigestStore; only GetDigestMode is meaningful.
type fakeDigest struct{ mode string }

func (f *fakeDigest) GetDigestMode(context.Context, int64) (string, string, error) {
	return f.mode, "", nil
}
func (f *fakeDigest) SetDigestMode(context.Context, int64, string, string) error { return nil }
func (f *fakeDigest) AddDigestItem(context.Context, int64, string, []string) error {
	return nil
}
func (f *fakeDigest) PeekDigest(context.Context, int64) ([]string, time.Time, error) {
	return nil, time.Time{}, nil
}
func (f *fakeDigest) AckDigest(context.Context, int64, time.Time) error   { return nil }
func (f *fakeDigest) PendingDigestUsers(context.Context) ([]int64, error) { return nil, nil }
func (f *fakeDigest) DigestLastFlushed(context.Context, int64) (time.Time, error) {
	return time.Time{}, nil
}

// fakeDedup implements storage.DedupStore; `seen` holds already-claimed tokens.
type fakeDedup struct {
	seen     map[string]bool
	released []string
	claimErr error
}

func (f *fakeDedup) ClaimNew(_ context.Context, token string, _, _ int64) (bool, error) {
	if f.claimErr != nil {
		return false, f.claimErr
	}
	if f.seen == nil {
		f.seen = map[string]bool{}
	}
	if f.seen[token] {
		return false, nil
	}
	f.seen[token] = true
	return true, nil
}

func (f *fakeDedup) ReleaseClaim(_ context.Context, token string, _, _ int64) error {
	f.released = append(f.released, token)
	return nil
}

func (f *fakeDedup) Prune(context.Context, time.Duration) (int64, error) { return 0, nil }

// fakeDelivery captures delivered listings and can force an error.
type fakeDelivery struct {
	delivered []model.Listing
	err       error
}

func (f *fakeDelivery) DeliverBatch(_ context.Context, _ int64, listings []model.Listing) error {
	if f.err != nil {
		return f.err
	}
	f.delivered = append(f.delivered, listings...)
	return nil
}

func (f *fakeDelivery) DeliverRaw(context.Context, int64, string) error { return nil }

func listingsWithTokens(tokens ...string) []model.Listing {
	out := make([]model.Listing, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, model.Listing{RawListing: model.RawListing{Token: t}})
	}
	return out
}

func deliveredTokens(ls []model.Listing) []string {
	out := make([]string, 0, len(ls))
	for _, l := range ls {
		out = append(out, l.Token)
	}
	return out
}

// Only listings not already claimed are delivered — this is what stops the
// extension re-alerting the same cars on every 15-minute ingest cycle.
func TestClaimAndDeliver_OnlyNewListings(t *testing.T) {
	dedup := &fakeDedup{seen: map[string]bool{"b": true}} // b already seen
	del := &fakeDelivery{}
	s := &Server{dedup: dedup}
	sr := storage.Search{ID: 1, ChatID: 7, Name: "test"}

	s.claimAndDeliver(context.Background(), sr, listingsWithTokens("a", "b", "c"), del, slog.Default())

	got := deliveredTokens(del.delivered)
	if len(got) != 2 || got[0] != "a" || got[1] != "c" {
		t.Fatalf("expected only new listings [a c] delivered, got %v", got)
	}
	// A second ingest of the same tokens delivers nothing new.
	del.delivered = nil
	s.claimAndDeliver(context.Background(), sr, listingsWithTokens("a", "c"), del, slog.Default())
	if len(del.delivered) != 0 {
		t.Fatalf("expected no re-delivery of already-claimed listings, got %v", deliveredTokens(del.delivered))
	}
}

// On delivery failure the claims are released so the listings retry next cycle.
func TestClaimAndDeliver_ReleaseClaimsOnFailure(t *testing.T) {
	dedup := &fakeDedup{}
	del := &fakeDelivery{err: errors.New("publish failed")}
	s := &Server{dedup: dedup}
	sr := storage.Search{ID: 1, ChatID: 7}

	s.claimAndDeliver(context.Background(), sr, listingsWithTokens("a", "b"), del, slog.Default())

	if len(dedup.released) != 2 {
		t.Fatalf("expected both claims released on delivery failure, got %v", dedup.released)
	}
}

// Without a publisher (and not digest mode) there is no transport, so the
// selection returns nil and the caller skips — never nil-panicking on a
// missing notifier, and never consuming dedup claims it cannot act on.
func TestIngestDeliveryFor_NilWithoutPublisher(t *testing.T) {
	s := &Server{} // no digests, no alertPublisher, no users
	got := s.ingestDeliveryFor(context.Background(), storage.Search{ChatID: 1}, slog.Default())
	if got != nil {
		t.Fatalf("expected nil delivery strategy without a publisher, got %T", got)
	}
}

// A digest-mode user gets a DigestDelivery even without an alert publisher —
// digest items are accumulated in the store, not published to the stream.
func TestIngestDeliveryFor_DigestMode(t *testing.T) {
	s := &Server{digests: &fakeDigest{mode: "digest"}} // no publisher
	got := s.ingestDeliveryFor(context.Background(), storage.Search{ChatID: 1}, slog.Default())
	if _, ok := got.(*scheduler.DigestDelivery); !ok {
		t.Fatalf("expected *scheduler.DigestDelivery for a digest-mode user, got %T", got)
	}
}
