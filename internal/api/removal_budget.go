package api

import (
	"sync"
	"time"
)

// A confirmed-404 from Yad2's item endpoint is the extension's proof that a
// listing is gone, and the extension already refuses to trust those 404s until
// a canary listing it KNOWS is live still resolves (see the extension's removal
// tripwire). This is the server-side backstop to that: even a client that has
// been compromised or has a broken canary must not be able to retire an
// unbounded number of a user's listings.
//
// The budget is a per-chat daily counter. It is in-memory on purpose — removal
// is best-effort and a process restart resetting the counter only ever makes it
// MORE permissive for the rest of the day, never less, so it cannot cause a
// wrongful deletion; a durable counter would be cost without safety benefit.
const removalBudgetPerDay = 200

type removalBudget struct {
	mu    sync.Mutex
	day   map[int64]int // chatID -> removals counted today
	epoch int64         // unix day the counts belong to
}

func newRemovalBudget() *removalBudget {
	return &removalBudget{day: make(map[int64]int), epoch: unixDay(time.Now())}
}

func unixDay(t time.Time) int64 { return t.UTC().Unix() / 86400 }

// take reserves up to want removals for a chat against its daily budget and
// returns how many are allowed now. A return less than want means the cap was
// hit — the caller should retire only that many and log the rest as refused,
// because a user legitimately losing 200 cars in a day is not a thing that
// happens; a broken endpoint retiring them by the cycleful is.
func (b *removalBudget) take(chatID int64, want int) (allowed int, capped bool) {
	if want <= 0 {
		return 0, false
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	if today := unixDay(time.Now()); today != b.epoch {
		b.epoch = today
		clear(b.day)
	}

	used := b.day[chatID]
	remaining := removalBudgetPerDay - used
	if remaining <= 0 {
		return 0, true
	}
	if want > remaining {
		b.day[chatID] = removalBudgetPerDay
		return remaining, true
	}
	b.day[chatID] = used + want
	return want, false
}
