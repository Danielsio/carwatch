package api

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/storage"
)

// countingUserStore implements just the UpsertWebUser method resolveWebUser
// touches, counting the calls. The embedded nil interface makes any other
// method panic — which is the point: this test asserts resolveWebUser hits the
// store at most once per UID.
type countingUserStore struct {
	storage.UserStore
	mu    sync.Mutex
	byUID map[string]int64
	calls atomic.Int64
	next  int64
}

func newCountingUserStore() *countingUserStore {
	return &countingUserStore{byUID: make(map[string]int64)}
}

func (c *countingUserStore) UpsertWebUser(_ context.Context, uid, _ string) (int64, error) {
	c.calls.Add(1)
	c.mu.Lock()
	defer c.mu.Unlock()
	if id, ok := c.byUID[uid]; ok {
		return id, nil
	}
	c.next++
	c.byUID[uid] = c.next
	return c.next, nil
}

func newCacheTestServer(users storage.UserStore) *Server {
	return &Server{users: users, uidCache: newUIDCache(), logger: slog.Default()}
}

// The whole point: after the first resolution, repeated requests for the same
// UID perform zero user-table round trips.
func TestResolveWebUser_CachesAfterFirstLookup(t *testing.T) {
	store := newCountingUserStore()
	srv := newCacheTestServer(store)
	ctx := context.Background()

	first, err := srv.resolveWebUser(ctx, "uid-1", "a@x.com")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		id, err := srv.resolveWebUser(ctx, "uid-1", "a@x.com")
		if err != nil {
			t.Fatal(err)
		}
		if id != first {
			t.Fatalf("cached chatID drifted: got %d, want %d", id, first)
		}
	}

	if n := store.calls.Load(); n != 1 {
		t.Fatalf("expected exactly 1 user-table round trip for 51 requests, got %d", n)
	}
}

// Different users each get resolved once and keep their own chatID.
func TestResolveWebUser_DistinctPerUID(t *testing.T) {
	store := newCountingUserStore()
	srv := newCacheTestServer(store)
	ctx := context.Background()

	a, _ := srv.resolveWebUser(ctx, "uid-a", "a@x.com")
	b, _ := srv.resolveWebUser(ctx, "uid-b", "b@x.com")
	// Repeat both from cache.
	_, _ = srv.resolveWebUser(ctx, "uid-a", "a@x.com")
	_, _ = srv.resolveWebUser(ctx, "uid-b", "b@x.com")

	if a == b {
		t.Fatalf("two UIDs resolved to the same chatID (%d)", a)
	}
	if n := store.calls.Load(); n != 2 {
		t.Fatalf("expected 2 round trips (one per UID), got %d", n)
	}
}

// A cleared cache re-hits the store — this is what makes admin deletion take
// effect promptly instead of a deleted account resolving for the whole TTL.
func TestResolveWebUser_ClearForcesRelookup(t *testing.T) {
	store := newCountingUserStore()
	srv := newCacheTestServer(store)
	ctx := context.Background()

	_, _ = srv.resolveWebUser(ctx, "uid-1", "a@x.com")
	srv.uidCache.clear()
	_, _ = srv.resolveWebUser(ctx, "uid-1", "a@x.com")

	if n := store.calls.Load(); n != 2 {
		t.Fatalf("expected a re-lookup after clear (2 round trips), got %d", n)
	}
}

func TestUIDCache_TTLExpiry(t *testing.T) {
	c := newUIDCache()
	c.put("uid-1", 42)

	if id, ok := c.get("uid-1"); !ok || id != 42 {
		t.Fatalf("fresh entry not returned: id=%d ok=%v", id, ok)
	}

	// Force expiry.
	c.mu.Lock()
	e := c.m["uid-1"]
	e.expires = time.Now().Add(-time.Minute)
	c.m["uid-1"] = e
	c.mu.Unlock()

	if _, ok := c.get("uid-1"); ok {
		t.Fatal("expired entry was returned")
	}
}

func TestUIDCache_ClearedWhenFull(t *testing.T) {
	c := newUIDCache()
	// Fill to the cap, then one more triggers the clear-and-repopulate.
	for i := 0; i < uidCacheMaxSize; i++ {
		c.put("uid-"+strconv.Itoa(i), int64(i))
	}
	c.put("overflow", 999999)

	// After the clear, the cache holds only the overflow entry.
	c.mu.RLock()
	size := len(c.m)
	c.mu.RUnlock()
	if size != 1 {
		t.Fatalf("expected the cache to reset to just the overflow entry, size=%d", size)
	}
	if id, ok := c.get("overflow"); !ok || id != 999999 {
		t.Fatalf("overflow entry missing after reset: id=%d ok=%v", id, ok)
	}
}
