package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/notifier"
	"github.com/dsionov/carwatch/internal/storage"
)

var errMalformedMessage = errors.New("blocked malformed message")

type DeliveryStrategy interface {
	DeliverBatch(ctx context.Context, chatID int64, listings []model.Listing) error
	DeliverRaw(ctx context.Context, chatID int64, message string) error
}

type InstantDelivery struct {
	notifier notifier.Notifier
	queue    storage.NotificationQueue
	lang     locale.Lang
	logger   *slog.Logger
}

func NewInstantDelivery(n notifier.Notifier, q storage.NotificationQueue, lang locale.Lang, opts ...func(*InstantDelivery)) *InstantDelivery {
	d := &InstantDelivery{notifier: n, queue: q, lang: lang, logger: slog.Default()}
	for _, o := range opts {
		o(d)
	}
	return d
}

func WithLogger(l *slog.Logger) func(*InstantDelivery) {
	return func(d *InstantDelivery) {
		if l != nil {
			d.logger = l
		}
	}
}

func (d *InstantDelivery) DeliverBatch(ctx context.Context, chatID int64, listings []model.Listing) error {
	chatIDStr := fmt.Sprintf("%d", chatID)
	err := d.notifier.Notify(ctx, chatIDStr, listings, d.lang)
	if err == nil {
		return nil
	}
	if errors.Is(err, notifier.ErrRecipientBlocked) {
		return err
	}

	if d.queue != nil {
		msg := notifier.FormatBatch(listings, d.lang)
		if notifier.IsMalformedMessage(msg) {
			d.logger.Error("blocked malformed batch message before enqueue",
				"chat_id", chatID, "msg_len", len(msg))
			return errMalformedMessage
		}
		d.logger.Debug("enqueueing batch notification after send failure",
			"chat_id", chatID, "msg_len", len(msg))
		if qErr := d.queue.EnqueueNotification(context.Background(), chatIDStr, "", msg); qErr == nil {
			return nil
		}
	}

	return err
}

func (d *InstantDelivery) DeliverRaw(ctx context.Context, chatID int64, message string) error {
	if notifier.IsMalformedMessage(message) {
		d.logger.Error("blocked malformed raw message",
			"chat_id", chatID, "msg_len", len(message),
			"msg_preview", truncateStr(message, 200))
		return errMalformedMessage
	}
	chatIDStr := fmt.Sprintf("%d", chatID)
	err := d.notifier.NotifyRaw(ctx, chatIDStr, message)
	if err == nil || d.queue == nil {
		return err
	}
	if errors.Is(err, notifier.ErrRecipientBlocked) {
		return err
	}
	if qErr := d.queue.EnqueueNotification(context.Background(), chatIDStr, "", message); qErr == nil {
		return nil
	}
	return err
}

type DigestDelivery struct {
	store storage.DigestStore
	lang  locale.Lang
}

func NewDigestDelivery(s storage.DigestStore, lang locale.Lang) *DigestDelivery {
	return &DigestDelivery{store: s, lang: lang}
}

func (d *DigestDelivery) DeliverBatch(ctx context.Context, chatID int64, listings []model.Listing) error {
	msg := notifier.FormatBatch(listings, d.lang)
	if notifier.IsMalformedMessage(msg) {
		return errMalformedMessage
	}
	return d.store.AddDigestItem(ctx, chatID, msg)
}

func (d *DigestDelivery) DeliverRaw(ctx context.Context, chatID int64, message string) error {
	if notifier.IsMalformedMessage(message) {
		return errMalformedMessage
	}
	return d.store.AddDigestItem(ctx, chatID, message)
}
