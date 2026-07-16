package api

import (
	"context"
	"sync"
	"time"
)

// Every authenticated request resolves the caller's Firebase UID to a chatID by
// calling UpsertWebUser, which — for a user that already exists, i.e. all but
// the very first request — opens a transaction and runs a SELECT that returns
// the same chatID every time. On the 1 GB VM that redundant round trip is paid
// on every poll: the web UI hits /scheduler/status every 30s per tab,
// /notifications every 60s, and so on, all before the per-user rate limiter can
// even see the request.
//
// uidCache memoizes the UID→chatID mapping so the steady state performs zero
// user-table round trips. It is safe because UpsertWebUser does not mutate an
// existing user (it does not update the email on a returning user), so the
// mapping is stable for the life of the account; a short TTL bounds how long a
// deleted account could still resolve, and admin deletion clears the cache
// outright.
const (
	uidCacheTTL     = 15 * time.Minute
	uidCacheMaxSize = 10000 // bound memory; a full cache just re-populates
)

type uidCacheEntry struct {
	chatID  int64
	expires time.Time
}

type uidCache struct {
	mu sync.RWMutex
	m  map[string]uidCacheEntry
}

func newUIDCache() *uidCache {
	return &uidCache{m: make(map[string]uidCacheEntry)}
}

// get returns the cached chatID for a UID when present and unexpired.
func (c *uidCache) get(uid string) (int64, bool) {
	c.mu.RLock()
	e, ok := c.m[uid]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expires) {
		return 0, false
	}
	return e.chatID, true
}

// put records a UID→chatID mapping. When the cache is full it is cleared rather
// than evicting one entry at a time: at this scale a full cache is unusual, and
// a clear simply re-populates from the DB on the next requests.
func (c *uidCache) put(uid string, chatID int64) {
	c.mu.Lock()
	if len(c.m) >= uidCacheMaxSize {
		c.m = make(map[string]uidCacheEntry, uidCacheMaxSize)
	}
	c.m[uid] = uidCacheEntry{chatID: chatID, expires: time.Now().Add(uidCacheTTL)}
	c.mu.Unlock()
}

// clear drops every entry. Called when a user is deleted or the data is reset,
// so a stale mapping cannot outlive the account it points at.
func (c *uidCache) clear() {
	c.mu.Lock()
	c.m = make(map[string]uidCacheEntry)
	c.mu.Unlock()
}

// resolveWebUser maps a Firebase UID to its chatID, using the cache to avoid a
// user-table round trip in the common case. On a miss it upserts (which creates
// the user on first sight) and caches the result.
func (s *Server) resolveWebUser(ctx context.Context, uid, email string) (int64, error) {
	if s.uidCache != nil {
		if id, ok := s.uidCache.get(uid); ok {
			return id, nil
		}
	}
	id, err := s.users.UpsertWebUser(ctx, uid, email)
	if err != nil {
		return 0, err
	}
	if s.uidCache != nil {
		s.uidCache.put(uid, id)
	}
	return id, nil
}
