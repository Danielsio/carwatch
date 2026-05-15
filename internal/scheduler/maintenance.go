package scheduler

import (
	"context"
	"errors"
	"time"

	"github.com/dsionov/carwatch/internal/notifier"
)

func (s *Scheduler) pruneOldData(ctx context.Context) {
	if time.Since(s.lastPruneTime) <= pruneInterval {
		return
	}
	s.cfgMu.RLock()
	pruneAfter := s.cfg.Storage.PruneAfter
	s.cfgMu.RUnlock()
	if pruneAfter > 0 {
		pruned, err := s.stores.Dedup.Prune(ctx, pruneAfter)
		if err != nil {
			s.logger.Error("prune failed", "error", err)
		} else if pruned > 0 {
			s.logger.Info("pruned old listings", "count", pruned)
		}
	}
	if s.stores.Queue != nil {
		pruned, err := s.stores.Queue.PruneNotifications(ctx, notificationPruneAge)
		if err != nil {
			s.logger.Error("prune notifications failed", "error", err)
		} else if pruned > 0 {
			s.logger.Info("pruned expired notifications", "count", pruned)
		}
	}
	if s.stores.Prices != nil {
		pruned, err := s.stores.Prices.PrunePrices(ctx, priceHistoryRetention)
		if err != nil {
			s.logger.Error("prune prices failed", "error", err)
		} else if pruned > 0 {
			s.logger.Info("pruned old price history", "count", pruned)
		}
	}
	if s.stores.Listings != nil {
		pruned, err := s.stores.Listings.PruneListings(ctx, listingHistoryRetention)
		if err != nil {
			s.logger.Error("prune listing history failed", "error", err)
		} else if pruned > 0 {
			s.logger.Info("pruned old listing history", "count", pruned)
		}
	}
	s.lastPruneTime = time.Now()
}

func (s *Scheduler) processExpiredPremium(ctx context.Context) {
	if s.stores.Users == nil {
		return
	}
	expired, err := s.stores.Users.ListExpiredPremium(ctx)
	if err != nil {
		s.logger.Error("list expired premium users failed", "error", err)
		return
	}
	if len(expired) == 0 {
		return
	}
	s.cfgMu.RLock()
	maxSearches := s.cfg.Telegram.MaxSearches
	s.cfgMu.RUnlock()
	for _, u := range expired {
		if err := s.stores.Users.SetUserTier(ctx, u.ChatID, "free", time.Time{}); err != nil {
			s.logger.Error("downgrade expired premium user failed",
				"chat_id", u.ChatID,
				"error", err,
			)
			continue
		}
		s.deactivateExcessSearches(ctx, u.ChatID, maxSearches)
		s.logger.Info("premium subscription expired, user downgraded to free",
			"chat_id", u.ChatID,
		)
	}
}

func (s *Scheduler) retryPending(ctx context.Context) {
	if s.stores.Queue == nil {
		return
	}
	pending, err := s.stores.Queue.PendingNotifications(ctx)
	if err != nil {
		s.logger.Error("failed to load pending notifications", "error", err)
		return
	}
	if len(pending) == 0 {
		return
	}
	s.logger.Info("retrying queued messages", "count", len(pending))
	for _, p := range pending {
		if notifier.IsMalformedMessage(p.Payload) {
			s.logger.Error("purging malformed pending notification",
				"id", p.ID,
				"chat_id", p.Recipient,
				"msg_len", len(p.Payload),
				"msg_preview", truncateStr(p.Payload, 200),
			)
			if err := s.stores.Queue.AckNotification(ctx, p.ID); err != nil {
				s.logger.Error("ack malformed notification failed", "id", p.ID, "error", err)
			}
			continue
		}
		s.logger.Debug("retrying pending notification",
			"id", p.ID,
			"chat_id", p.Recipient,
			"msg_len", len(p.Payload),
			"msg_preview", truncateStr(p.Payload, 100),
		)
		if err := s.notifier.NotifyRaw(ctx, p.Recipient, p.Payload); err != nil {
			if errors.Is(err, notifier.ErrRecipientBlocked) {
				s.logger.Warn("purging notification for unreachable recipient",
					"id", p.ID,
					"chat_id", p.Recipient,
					"error", err,
				)
				if ackErr := s.stores.Queue.AckNotification(ctx, p.ID); ackErr != nil {
					s.logger.Error("ack unreachable notification failed", "id", p.ID, "error", ackErr)
				}
				continue
			}
			s.logger.Error("retry notification failed",
				"id", p.ID,
				"chat_id", p.Recipient,
				"error", err,
			)
			continue
		}
		if err := s.stores.Queue.AckNotification(ctx, p.ID); err != nil {
			s.logger.Error("ack notification failed", "id", p.ID, "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
}
