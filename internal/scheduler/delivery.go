package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/dsionov/carwatch/internal/broker"
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
	notifier   notifier.Notifier
	lang       locale.Lang
	logger     *slog.Logger
	publisher  *broker.Publisher
	searchID   int64
	searchName string
}

func NewInstantDelivery(n notifier.Notifier, lang locale.Lang, opts ...func(*InstantDelivery)) *InstantDelivery {
	d := &InstantDelivery{notifier: n, lang: lang, logger: slog.Default()}
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

// WithPublisher configures the delivery to publish alerts to Redis
// instead of calling the notifier directly.
func WithPublisher(p *broker.Publisher) func(*InstantDelivery) {
	return func(d *InstantDelivery) {
		d.publisher = p
	}
}

// WithSearchContext sets the search ID and name so that published alerts
// carry traceability metadata.
func WithSearchContext(id int64, name string) func(*InstantDelivery) {
	return func(d *InstantDelivery) {
		d.searchID = id
		d.searchName = name
	}
}

func (d *InstantDelivery) DeliverBatch(ctx context.Context, chatID int64, listings []model.Listing) error {
	if d.publisher != nil {
		msg := notifier.FormatBatch(listings, d.lang)
		if notifier.IsMalformedMessage(msg) {
			d.logger.ErrorContext(ctx, "blocked delivery of malformed batch message, skipping to prevent Telegram API errors",
				"chat_id", chatID, "msg_len", len(msg))
			return errMalformedMessage
		}
		tokens := make([]string, 0, len(listings))
		for _, l := range listings {
			tokens = append(tokens, l.Token)
		}
		alert := broker.Alert{
			ChatID:     chatID,
			SearchID:   d.searchID,
			SearchName: d.searchName,
			Tokens:     tokens,
			Message:    msg,
			Language:   string(d.lang),
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
		}
		if err := d.publisher.Publish(ctx, alert); err != nil {
			d.logger.ErrorContext(ctx, "failed to publish alert to Redis queue for notification delivery",
				"chat_id", chatID, "error", err)
			return err
		}
		return nil
	}

	chatIDStr := fmt.Sprintf("%d", chatID)
	err := d.notifier.Notify(ctx, chatIDStr, listings, d.lang)
	if err == nil {
		return nil
	}
	if errors.Is(err, notifier.ErrRecipientBlocked) {
		return err
	}
	d.logger.ErrorContext(ctx, "direct notification delivery failed for user",
		"chat_id", chatID, "error", err)
	return err
}

func (d *InstantDelivery) DeliverRaw(ctx context.Context, chatID int64, message string) error {
	if notifier.IsMalformedMessage(message) {
		d.logger.ErrorContext(ctx, "blocked delivery of malformed raw message, skipping to prevent Telegram API errors",
			"chat_id", chatID, "msg_len", len(message),
			"msg_preview", truncateStr(message, 200))
		return errMalformedMessage
	}

	if d.publisher != nil {
		alert := broker.Alert{
			ChatID:     chatID,
			SearchID:   d.searchID,
			SearchName: d.searchName,
			Message:    message,
			Language:   string(d.lang),
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
		}
		if err := d.publisher.Publish(ctx, alert); err != nil {
			d.logger.ErrorContext(ctx, "failed to publish raw alert to Redis queue for notification delivery",
				"chat_id", chatID, "error", err)
			return err
		}
		return nil
	}

	chatIDStr := fmt.Sprintf("%d", chatID)
	err := d.notifier.NotifyRaw(ctx, chatIDStr, message)
	if err != nil && !errors.Is(err, notifier.ErrRecipientBlocked) {
		d.logger.ErrorContext(ctx, "direct raw notification delivery failed for user",
			"chat_id", chatID, "error", err)
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
