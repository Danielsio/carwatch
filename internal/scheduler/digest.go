package scheduler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/notifier"
)

const (
	// digestRetryInterval is the shorter flush interval used after a
	// delivery failure, so retries happen sooner than the next full
	// digest interval.
	digestRetryInterval = 2 * time.Minute

	// maxDigestPending is the maximum number of items allowed to
	// accumulate in a digest queue. When exceeded, the oldest items
	// are acked (discarded) to prevent unbounded growth.
	maxDigestPending = 100
)

func (s *Scheduler) processDigests(ctx context.Context) {
	if s.stores.Digests == nil {
		return
	}

	users, err := s.stores.Digests.PendingDigestUsers(ctx)
	if err != nil {
		s.logger.Error("list pending digest users failed", "error", err)
		return
	}

	for _, chatID := range users {
		mode, intervalStr, err := s.stores.Digests.GetDigestMode(ctx, chatID)
		if err != nil {
			s.logger.Error("get digest mode failed", "chat_id", chatID, "error", err)
			continue
		}
		if mode != "digest" {
			// User switched back to instant; flush and send immediately.
			s.flushAndSendDigest(ctx, chatID)
			continue
		}

		interval, err := time.ParseDuration(intervalStr)
		if err != nil {
			s.logger.Error("parse digest interval failed",
				"chat_id", chatID,
				"interval", intervalStr,
				"error", err,
			)
			interval = 6 * time.Hour
		}

		// If a previous flush failed, use a shorter retry interval.
		if _, failed := s.digestFailures.Load(chatID); failed {
			interval = digestRetryInterval
		}

		lastFlushed, err := s.stores.Digests.DigestLastFlushed(ctx, chatID)
		if err != nil {
			s.logger.Error("get last flushed failed", "chat_id", chatID, "error", err)
			continue
		}

		if time.Since(lastFlushed) >= interval {
			s.flushAndSendDigest(ctx, chatID)
		}
	}
}

func (s *Scheduler) flushAndSendDigest(ctx context.Context, chatID int64) {
	payloads, cutoff, err := s.stores.Digests.PeekDigest(ctx, chatID)
	if err != nil {
		s.logger.Error("peek digest failed", "chat_id", chatID, "error", err)
		return
	}
	if len(payloads) == 0 {
		return
	}

	// Cap accumulated items to prevent unbounded growth.
	if len(payloads) > maxDigestPending {
		overflow := len(payloads) - maxDigestPending
		s.logger.Warn("digest queue exceeded max pending items, discarding oldest",
			"chat_id", chatID,
			"total", len(payloads),
			"discarding", overflow,
			"max", maxDigestPending,
		)
		payloads = payloads[overflow:]
	}

	chatIDStr := fmt.Sprintf("%d", chatID)
	lang := s.userLang(ctx, chatID)
	header := locale.Tf(lang, "fmt_digest_header", len(payloads))
	combined := header + strings.Join(payloads, "\n\n━━━━━━━━━━━━━━━━━━━━\n\n")

	if err := s.notifier.NotifyRaw(ctx, chatIDStr, combined); err != nil {
		s.logger.Error("send digest failed, items preserved for retry",
			"chat_id", chatID,
			"items", len(payloads),
			"error", err,
		)
		// Record the failure so processDigests uses a shorter retry interval.
		s.digestFailures.Store(chatID, time.Now())
		return
	}

	// Delivery succeeded; clear the failure marker.
	s.digestFailures.Delete(chatID)

	if err := s.stores.Digests.AckDigest(ctx, chatID, cutoff); err != nil {
		s.logger.Error("digest ack failed after successful send, items may be resent",
			"chat_id", chatID,
			"cutoff", cutoff,
			"items", len(payloads),
			"error", err,
		)
	}

	s.logger.Info("digest sent",
		"chat_id", chatID,
		"items", len(payloads),
	)
	s.observer.RecordNotificationSent()
}

func (s *Scheduler) processDailyDigests(ctx context.Context) {
	if s.stores.DailyDigests == nil {
		return
	}

	users, err := s.stores.DailyDigests.ListDailyDigestUsers(ctx)
	if err != nil {
		s.logger.Error("list daily digest users failed", "error", err)
		return
	}

	now := time.Now().In(s.loc)

	for _, u := range users {
		targetMinutes := parseTimeOfDayOrZero(u.DigestTime)
		currentMinutes := now.Hour()*60 + now.Minute()

		diff := currentMinutes - targetMinutes
		if diff < 0 {
			diff = -diff
		}
		if diff > 12*60 {
			diff = 24*60 - diff
		}
		if diff > 15 {
			continue
		}

		lastSentLocal := u.LastSent.In(s.loc)
		if lastSentLocal.Year() == now.Year() &&
			lastSentLocal.Month() == now.Month() &&
			lastSentLocal.Day() == now.Day() {
			continue
		}

		s.sendDailyDigest(ctx, u.ChatID)
	}
}

func (s *Scheduler) sendDailyDigest(ctx context.Context, chatID int64) {
	stats, err := s.stores.DailyDigests.DailyStats(ctx, chatID)
	if err != nil {
		s.logger.Error("compute daily stats failed", "chat_id", chatID, "error", err)
		return
	}

	if len(stats) == 0 {
		return
	}

	lang := s.userLang(ctx, chatID)
	msg := notifier.FormatDailyDigest(stats, lang, time.Now().In(s.loc))

	chatIDStr := fmt.Sprintf("%d", chatID)
	if err := s.notifier.NotifyRaw(ctx, chatIDStr, msg); err != nil {
		s.logger.Error("send daily digest failed", "chat_id", chatID, "error", err)
		return
	}

	if err := s.stores.DailyDigests.UpdateDailyDigestLastSent(ctx, chatID); err != nil {
		s.logger.Error("daily digest last-sent update failed after successful send, digest may be resent",
			"chat_id", chatID,
			"error", err,
		)
	}

	s.logger.Info("daily digest sent", "chat_id", chatID, "searches", len(stats))
}
