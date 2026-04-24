package scheduler

import (
	"context"
	"errors"
	"fmt"

	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/notifier"
	"github.com/dsionov/carwatch/internal/storage"
)

type DeliveryStrategy interface {
	DeliverBatch(ctx context.Context, chatID int64, listings []model.Listing) error
	DeliverRaw(ctx context.Context, chatID int64, message string) error
}

type InstantDelivery struct {
	notifier notifier.Notifier
	queue    storage.NotificationQueue
	lang     locale.Lang
}

func NewInstantDelivery(n notifier.Notifier, q storage.NotificationQueue, lang locale.Lang) *InstantDelivery {
	return &InstantDelivery{notifier: n, queue: q, lang: lang}
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
		if qErr := d.queue.EnqueueNotification(context.Background(), chatIDStr, "", msg); qErr == nil {
			return nil
		}
	}

	return err
}

func (d *InstantDelivery) DeliverRaw(ctx context.Context, chatID int64, message string) error {
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
	return d.store.AddDigestItem(ctx, chatID, msg)
}

func (d *DigestDelivery) DeliverRaw(ctx context.Context, chatID int64, message string) error {
	return d.store.AddDigestItem(ctx, chatID, message)
}
