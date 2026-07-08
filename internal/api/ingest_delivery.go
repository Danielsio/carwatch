package api

import (
	"context"
	"errors"
	"log/slog"

	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/notifier"
	"github.com/dsionov/carwatch/internal/scheduler"
	"github.com/dsionov/carwatch/internal/storage"
)

// deliverIngestMatches notifies the user about newly matched listings that
// arrived via the browser-extension ingest path. It reuses the same mechanics
// as the scheduler so behaviour stays consistent:
//
//   - each token is claimed via the dedup store so a listing is delivered at
//     most once per (user, search) — this is what prevents re-alerting the same
//     cars on every 15-minute ingest cycle;
//   - the user's delivery mode picks the transport (digest → accumulate for the
//     scheduled flush; instant → publish to the Redis alert stream the notifier
//     consumes);
//   - on a delivery failure the claims are released so the listings are retried
//     on the next ingest instead of being lost.
//
// It is best-effort: any error is logged, never surfaced to the extension (the
// ingest itself already succeeded once listings are persisted).
func (s *Server) deliverIngestMatches(ctx context.Context, sr storage.Search, listings []model.Listing, log *slog.Logger) {
	if s.dedup == nil || len(listings) == 0 {
		return
	}
	delivery := s.ingestDeliveryFor(ctx, sr, log)
	if delivery == nil {
		// No transport wired (no Redis publisher and not digest mode) — do not
		// consume dedup claims we cannot act on.
		return
	}
	s.claimAndDeliver(ctx, sr, listings, delivery, log)
}

// claimAndDeliver claims the new tokens, delivers them via the chosen strategy,
// and releases claims on failure. Split out so it can be tested with a fake
// dedup store and delivery strategy.
func (s *Server) claimAndDeliver(ctx context.Context, sr storage.Search, listings []model.Listing, delivery scheduler.DeliveryStrategy, log *slog.Logger) {
	newOnes := make([]model.Listing, 0, len(listings))
	for _, l := range listings {
		isNew, err := s.dedup.ClaimNew(ctx, l.Token, sr.ChatID, sr.ID)
		if err != nil {
			log.ErrorContext(ctx, "ingest delivery: dedup claim failed, skipping listing",
				"token", l.Token, "search_id", sr.ID, "error", err)
			continue
		}
		if isNew {
			newOnes = append(newOnes, l)
		}
	}
	if len(newOnes) == 0 {
		return
	}

	if err := delivery.DeliverBatch(ctx, sr.ChatID, newOnes); err != nil {
		// Permanent failures (malformed message, recipient blocked the bot)
		// will fail identically forever — keep the claims so we don't replay
		// the same batch on every ingest. Only transient failures are retried.
		if errors.Is(err, scheduler.ErrMalformedMessage) || errors.Is(err, notifier.ErrRecipientBlocked) {
			log.ErrorContext(ctx, "ingest delivery: permanent failure, keeping claims (no retry)",
				"count", len(newOnes), "search_id", sr.ID, "error", err)
			return
		}
		log.ErrorContext(ctx, "ingest delivery: transient failure, releasing claims for retry",
			"count", len(newOnes), "search_id", sr.ID, "error", err)
		for _, l := range newOnes {
			if relErr := s.dedup.ReleaseClaim(ctx, l.Token, sr.ChatID, sr.ID); relErr != nil {
				log.ErrorContext(ctx, "ingest delivery: failed to release dedup claim",
					"token", l.Token, "search_id", sr.ID, "error", relErr)
			}
		}
		return
	}
	log.InfoContext(ctx, "ingest delivery: notified user of new listings",
		"count", len(newOnes), "search_id", sr.ID, "search_name", sr.Name)
}

// ingestDeliveryFor mirrors the scheduler's per-user delivery selection.
// Returns nil when instant delivery is requested but no alert publisher is
// wired, so the caller skips (rather than nil-panicking on a missing notifier).
func (s *Server) ingestDeliveryFor(ctx context.Context, sr storage.Search, log *slog.Logger) scheduler.DeliveryStrategy {
	lang := s.userLang(ctx, sr.ChatID)
	if s.digests != nil {
		if mode, _, err := s.digests.GetDigestMode(ctx, sr.ChatID); err == nil && mode == "digest" {
			return scheduler.NewDigestDelivery(s.digests, lang)
		}
	}
	if s.alertPublisher == nil {
		return nil
	}
	return scheduler.NewInstantDelivery(nil, lang,
		scheduler.WithLogger(log),
		scheduler.WithSearchContext(sr.ID, sr.Name),
		scheduler.WithPublisher(s.alertPublisher),
	)
}

// userLang resolves the user's language, defaulting to Hebrew.
func (s *Server) userLang(ctx context.Context, chatID int64) locale.Lang {
	lang := locale.Hebrew
	if s.users != nil {
		if u, err := s.users.GetUser(ctx, chatID); err == nil && u != nil && u.Language != "" {
			lang = locale.Lang(u.Language)
		}
	}
	return lang
}
