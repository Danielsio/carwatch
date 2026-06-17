package notifier

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/storage"
)

type MultiNotifier struct {
	notifiers map[string]Notifier
	fallback  string
	userStore storage.UserStore
	logger    *slog.Logger
}

func NewMultiNotifier(userStore storage.UserStore, logger *slog.Logger) *MultiNotifier {
	return &MultiNotifier{
		notifiers: make(map[string]Notifier),
		userStore: userStore,
		logger:    logger,
	}
}

func (m *MultiNotifier) Register(channel string, n Notifier) error {
	if channel == "" {
		return fmt.Errorf("channel must not be empty")
	}
	if n == nil {
		return fmt.Errorf("notifier must not be nil")
	}
	if m.fallback == "" {
		m.fallback = channel
	}
	m.notifiers[channel] = n
	return nil
}

func (m *MultiNotifier) Connect(ctx context.Context) error {
	for ch, n := range m.notifiers {
		if err := n.Connect(ctx); err != nil {
			return fmt.Errorf("connect %s: %w", ch, err)
		}
	}
	return nil
}

func (m *MultiNotifier) Disconnect() error {
	var errs []error
	for ch, n := range m.notifiers {
		if err := n.Disconnect(); err != nil {
			errs = append(errs, fmt.Errorf("disconnect %s: %w", ch, err))
		}
	}
	return errors.Join(errs...)
}

var ErrNoChannelNotifier = fmt.Errorf("no notifier for user channel")

func normalizeChannel(channel string) string {
	switch channel {
	case "web":
		return "webpush"
	default:
		return channel
	}
}

func (m *MultiNotifier) Notify(ctx context.Context, recipient string, listings []model.Listing, lang locale.Lang) error {
	targets := m.resolveAll(ctx, recipient)
	if len(targets) == 0 {
		return ErrNoChannelNotifier
	}

	var errs []error
	var ok int
	for ch, n := range targets {
		if err := n.Notify(ctx, recipient, listings, lang); err != nil {
			m.logger.Warn("fan-out delivery failed", "recipient", recipient, "channel", ch, "error", err)
			errs = append(errs, fmt.Errorf("%s: %w", ch, err))
		} else {
			ok++
		}
	}
	if ok > 0 {
		return nil
	}
	return errors.Join(errs...)
}

func (m *MultiNotifier) NotifyRaw(ctx context.Context, recipient string, message string) error {
	targets := m.resolveAll(ctx, recipient)
	if len(targets) == 0 {
		return ErrNoChannelNotifier
	}

	var errs []error
	var ok int
	for ch, n := range targets {
		if err := n.NotifyRaw(ctx, recipient, message); err != nil {
			m.logger.Warn("fan-out raw delivery failed", "recipient", recipient, "channel", ch, "error", err)
			errs = append(errs, fmt.Errorf("%s: %w", ch, err))
		} else {
			ok++
		}
	}
	if ok > 0 {
		return nil
	}
	return errors.Join(errs...)
}

// resolveAll returns the notifier(s) a recipient should be delivered through,
// based strictly on the user's platform channel (telegram or web). Each user
// record belongs to exactly one channel, so we never fan out to channels the
// user did not opt into — previously web-push was added to every recipient,
// which caused duplicate sends and, worse, let a no-op web-push "success"
// (zero subscriptions) mask a failed Telegram delivery, silently dropping the
// alert. A last-resort fallback is used only when the recipient/user/channel
// cannot be resolved, so we never drop an alert without attempting delivery.
func (m *MultiNotifier) resolveAll(ctx context.Context, recipient string) map[string]Notifier {
	result := make(map[string]Notifier)

	chatID, err := strconv.ParseInt(recipient, 10, 64)
	if err != nil {
		m.addFallback(result)
		return result
	}

	user, uErr := m.userStore.GetUser(ctx, chatID)
	if uErr != nil {
		m.logger.Warn("resolve user failed, using fallback", "recipient", recipient, "error", uErr)
		m.addFallback(result)
		return result
	}

	channel := ""
	if user != nil {
		channel = normalizeChannel(user.Channel)
	}
	if n, ok := m.notifiers[channel]; ok {
		result[channel] = n
		return result
	}

	m.logger.Warn("no notifier for user channel, using fallback",
		"recipient", recipient, "channel", channel)
	m.addFallback(result)
	return result
}

// addFallback adds the first-registered (fallback) notifier to result, if one
// exists. Used only when a recipient's own channel cannot be resolved.
func (m *MultiNotifier) addFallback(result map[string]Notifier) {
	if n := m.notifiers[m.fallback]; n != nil {
		result[m.fallback] = n
	}
}
