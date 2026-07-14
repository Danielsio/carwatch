package postgres

import (
	"context"
	"fmt"
	"time"
)

// ClaimNew records that (token, chat_id, search_id) has been seen and reports
// whether this claim is the first one — i.e. whether the user should be alerted
// about this listing for this search.
//
// A re-observation is not a no-op: it refreshes last_seen_at, which is what the
// prune retains the claim by. The extension re-pushes every still-matching
// listing every cycle, so a live listing's claim is renewed for as long as its
// ad is up; only listings that have actually disappeared from the source age
// out of the retention window. (Before this, a claim expired on a fixed timer
// from first sight, and the next push re-alerted the user about a car they had
// already been shown.)
//
// The insert-or-touch is a single statement so it stays atomic under concurrent
// pushes. `xmax = 0` is true only for a freshly inserted row — an ON CONFLICT
// update leaves the previous tuple's xmax behind — which is how we tell a new
// claim from a renewed one now that both affect a row.
func (s *Store) ClaimNew(ctx context.Context, token string, chatID int64, searchID int64) (bool, error) {
	var inserted bool
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO seen_listings (token, chat_id, search_id, first_seen_at, last_seen_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (token, chat_id, search_id) DO UPDATE SET last_seen_at = NOW()
		RETURNING (xmax = 0)`,
		token, chatID, searchID).Scan(&inserted)
	if err != nil {
		return false, fmt.Errorf("claim new: %w", err)
	}
	return inserted, nil
}

func (s *Store) ReleaseClaim(ctx context.Context, token string, chatID int64, searchID int64) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM seen_listings WHERE token = $1 AND chat_id = $2 AND search_id = $3`,
		token, chatID, searchID)
	if err != nil {
		return fmt.Errorf("release claim: %w", err)
	}
	return nil
}

// Prune drops dedup claims for listings nobody has seen for olderThan — i.e.
// listings that have been gone from the source that long. It deliberately does
// NOT prune by first_seen_at: a listing that is still live is still being
// re-claimed every cycle, and expiring its claim would re-alert the user about
// a car they have already been shown.
func (s *Store) Prune(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM seen_listings WHERE last_seen_at < $1`,
		cutoff)
	if err != nil {
		return 0, fmt.Errorf("prune seen listings: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("prune seen listings rows affected: %w", err)
	}
	return rows, nil
}
