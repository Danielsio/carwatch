package scheduler

import (
	"context"
	"errors"
	"fmt"

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
}

func NewInstantDelivery(n notifier.Notifier, q storage.NotificationQueue) *InstantDelivery {
	return &InstantDelivery{notifier: n, queue: q}
}

func (d *InstantDelivery) DeliverBatch(ctx context.Context, chatID int64, listings []model.Listing) error {
	chatIDStr := fmt.Sprintf("%d", chatID)
	err := d.notifier.Notify(ctx, chatIDStr, listings)
	if err == nil {
		return nil
	}
	if errors.Is(err, notifier.ErrRecipientBlocked) {
		return err
	}

	// Use background context so the enqueue succeeds even during shutdown.
	if d.queue != nil {
		msg := notifier.FormatBatch(listings)
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
}

func NewDigestDelivery(s storage.DigestStore) *DigestDelivery {
	return &DigestDelivery{store: s}
}

func (d *DigestDelivery) DeliverBatch(ctx context.Context, chatID int64, listings []model.Listing) error {
	msg := notifier.FormatBatch(listings)
	return d.store.AddDigestItem(ctx, chatID, msg)
}

func (d *DigestDelivery) DeliverRaw(ctx context.Context, chatID int64, message string) error {
	return d.store.AddDigestItem(ctx, chatID, message)
}
