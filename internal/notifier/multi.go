package notifier

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/storage"
)

type MultiNotifier struct {
	notifiers  map[string]Notifier
	fallback   string
	userStore  storage.UserStore
	logger     *slog.Logger
}

func NewMultiNotifier(userStore storage.UserStore, logger *slog.Logger) *MultiNotifier {
	return &MultiNotifier{
		notifiers: make(map[string]Notifier),
		userStore: userStore,
		logger:    logger,
	}
}

func (m *MultiNotifier) Register(channel string, n Notifier) {
	if m.fallback == "" {
		m.fallback = channel
	}
	m.notifiers[channel] = n
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
	for _, n := range m.notifiers {
		_ = n.Disconnect()
	}
	return nil
}

func (m *MultiNotifier) Notify(ctx context.Context, recipient string, listings []model.Listing, lang locale.Lang) error {
	n := m.resolve(ctx, recipient)
	return n.Notify(ctx, recipient, listings, lang)
}

func (m *MultiNotifier) NotifyRaw(ctx context.Context, recipient string, message string) error {
	n := m.resolve(ctx, recipient)
	return n.NotifyRaw(ctx, recipient, message)
}

func (m *MultiNotifier) resolve(ctx context.Context, recipient string) Notifier {
	chatID, err := strconv.ParseInt(recipient, 10, 64)
	if err == nil {
		user, err := m.userStore.GetUser(ctx, chatID)
		if err == nil && user != nil && user.Channel != "" {
			if n, ok := m.notifiers[user.Channel]; ok {
				return n
			}
		}
	}
	return m.notifiers[m.fallback]
}
