// Package webpush implements the notifier.Notifier interface using the
// Web Push protocol (RFC 8030) with VAPID authentication.
package webpush

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	wp "github.com/SherClockHolmes/webpush-go"

	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/storage"
)

// SubscriptionStore abstracts the persistence layer for web push subscriptions.
type SubscriptionStore interface {
	ListPushSubscriptions(ctx context.Context, chatID int64) ([]storage.PushSubscription, error)
	DeletePushSubscription(ctx context.Context, chatID int64, endpoint string) error
}

// pushPayload is the JSON structure delivered to the service worker.
type pushPayload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url,omitempty"`
	Icon  string `json:"icon,omitempty"`
}

// Notifier sends browser push notifications via the Web Push protocol.
type Notifier struct {
	subs       SubscriptionStore
	vapidPub   string
	vapidPriv  string
	vapidEmail string
	logger     *slog.Logger
	// sendFunc is the function used to send push notifications.
	// Defaults to wp.SendNotification; tests override it.
	sendFunc func(msg []byte, s *wp.Subscription, o *wp.Options) (*http.Response, error)
}

// New creates a Notifier that delivers web push notifications.
func New(subs SubscriptionStore, vapidPub, vapidPriv, vapidEmail string, logger *slog.Logger) *Notifier {
	return &Notifier{
		subs:       subs,
		vapidPub:   vapidPub,
		vapidPriv:  vapidPriv,
		vapidEmail: vapidEmail,
		logger:     logger,
		sendFunc:   wp.SendNotification,
	}
}

// Connect is a no-op for web push (stateless protocol).
func (n *Notifier) Connect(_ context.Context) error { return nil }

// Disconnect is a no-op for web push.
func (n *Notifier) Disconnect() error { return nil }

// Notify formats the listings into a push payload and delivers it to all
// subscriptions registered for the given recipient (chat ID).
func (n *Notifier) Notify(ctx context.Context, recipient string, listings []model.Listing, _ locale.Lang) error {
	chatID, err := strconv.ParseInt(recipient, 10, 64)
	if err != nil {
		return fmt.Errorf("webpush: invalid recipient %q: %w", recipient, err)
	}
	n.logger.Debug("delivering web push", "chat_id", chatID, "listings", len(listings))

	subs, err := n.subs.ListPushSubscriptions(ctx, chatID)
	if err != nil {
		return fmt.Errorf("webpush: list subscriptions for %d: %w", chatID, err)
	}
	if len(subs) == 0 {
		return nil // user has not subscribed — not an error
	}

	payload := buildListingPayload(listings)
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("webpush: marshal payload: %w", err)
	}

	return n.deliver(ctx, chatID, subs, data)
}

// NotifyRaw sends a plain text message as a push notification.
func (n *Notifier) NotifyRaw(ctx context.Context, recipient string, message string) error {
	chatID, err := strconv.ParseInt(recipient, 10, 64)
	if err != nil {
		return fmt.Errorf("webpush: invalid recipient %q: %w", recipient, err)
	}

	subs, err := n.subs.ListPushSubscriptions(ctx, chatID)
	if err != nil {
		return fmt.Errorf("webpush: list subscriptions for %d: %w", chatID, err)
	}
	if len(subs) == 0 {
		return nil
	}

	payload := pushPayload{
		Title: "CarWatch",
		Body:  message,
		Icon:  "/icon-192.png",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("webpush: marshal payload: %w", err)
	}

	return n.deliver(ctx, chatID, subs, data)
}

// deliver fans out the push message to every subscription, removing stale
// endpoints (HTTP 410 Gone) and skipping rate-limited ones (HTTP 429).
func (n *Notifier) deliver(ctx context.Context, chatID int64, subs []storage.PushSubscription, data []byte) error {
	var firstErr error
	for _, sub := range subs {
		resp, err := n.sendFunc(data, &wp.Subscription{
			Endpoint: sub.Endpoint,
			Keys: wp.Keys{
				P256dh: sub.P256DH,
				Auth:   sub.Auth,
			},
		}, &wp.Options{
			VAPIDPublicKey:  n.vapidPub,
			VAPIDPrivateKey: n.vapidPriv,
			Subscriber:      n.vapidEmail,
			TTL:             60,
		})
		if err != nil {
			n.logger.Warn("webpush send failed",
				"chat_id", chatID,
				"endpoint", truncateEndpoint(sub.Endpoint),
				"error", err,
			)
			if firstErr == nil {
				firstErr = fmt.Errorf("webpush: send to %d: %w", chatID, err)
			}
			continue
		}
		_ = resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusGone:
			n.logger.Info("removing gone subscription",
				"chat_id", chatID,
				"endpoint", truncateEndpoint(sub.Endpoint),
			)
			if dErr := n.subs.DeletePushSubscription(ctx, chatID, sub.Endpoint); dErr != nil {
				n.logger.Error("failed to delete gone subscription",
					"chat_id", chatID,
					"error", dErr,
				)
			}
		case http.StatusTooManyRequests:
			n.logger.Warn("rate limited by push service, skipping",
				"chat_id", chatID,
				"endpoint", truncateEndpoint(sub.Endpoint),
			)
		default:
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				n.logger.Debug("webpush delivered",
					"chat_id", chatID,
					"endpoint", truncateEndpoint(sub.Endpoint),
					"status", resp.StatusCode,
				)
			} else {
				n.logger.Warn("webpush unexpected status",
					"chat_id", chatID,
					"endpoint", truncateEndpoint(sub.Endpoint),
					"status", resp.StatusCode,
				)
			}
		}
	}
	return firstErr
}

// buildListingPayload creates a push notification payload from one or more listings.
func buildListingPayload(listings []model.Listing) pushPayload {
	if len(listings) == 0 {
		return pushPayload{Title: "CarWatch", Body: "New listings available", Icon: "/icon-192.png"}
	}

	if len(listings) == 1 {
		l := listings[0]
		title := strings.TrimSpace(l.Manufacturer + " " + l.Model)
		if title == "" {
			title = "New listing"
		}
		if l.SubModel != "" {
			title += " " + l.SubModel
		}
		if l.Year > 0 {
			title += fmt.Sprintf(" %d", l.Year)
		}

		var parts []string
		if l.Price > 0 {
			parts = append(parts, fmt.Sprintf("%d NIS", l.Price))
		}
		if l.Km > 0 {
			parts = append(parts, fmt.Sprintf("%d km", l.Km))
		}
		body := strings.Join(parts, " | ")
		if body == "" {
			body = "New listing found"
		}

		url := ""
		if l.Token != "" {
			url = "/listings/" + l.Token
		}

		return pushPayload{
			Title: title,
			Body:  body,
			URL:   url,
			Icon:  "/icon-192.png",
		}
	}

	// Multiple listings — summarize.
	first := listings[0]
	title := fmt.Sprintf("%d new listings", len(listings))
	body := strings.TrimSpace(first.Manufacturer + " " + first.Model)
	if body != "" && len(listings) > 1 {
		body += fmt.Sprintf(" and %d more", len(listings)-1)
	}
	if body == "" {
		body = "New listings found"
	}

	return pushPayload{
		Title: title,
		Body:  body,
		Icon:  "/icon-192.png",
	}
}

// truncateEndpoint returns at most the first 60 chars of an endpoint URL for
// safe logging without leaking full subscription identifiers.
func truncateEndpoint(ep string) string {
	if len(ep) <= 60 {
		return ep
	}
	return ep[:60] + "..."
}
