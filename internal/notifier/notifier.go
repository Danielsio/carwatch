package notifier

import (
	"context"
	"errors"
	"strings"

	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/model"
)

var ErrRecipientBlocked = errors.New("recipient blocked the bot")

// MinSafeMessageLen is the minimum length a valid formatted message should
// have. Anything shorter is almost certainly a bug (raw template syntax,
// partial format string, or empty payload).
const MinSafeMessageLen = 10

type Notifier interface {
	Connect(ctx context.Context) error
	Notify(ctx context.Context, recipient string, listings []model.Listing, lang locale.Lang) error
	NotifyRaw(ctx context.Context, recipient string, message string) error
	Disconnect() error
}

// IsMalformedMessage returns true when text looks like a corrupted or
// template-leaked payload that should never be sent to a user.
func IsMalformedMessage(text string) bool {
	trimmed := strings.TrimSpace(text)
	if len(trimmed) < MinSafeMessageLen {
		return true
	}
	if strings.Contains(trimmed, "{{") && strings.Contains(trimmed, "}}") {
		return true
	}
	if strings.HasPrefix(trimmed, "%!") {
		return true
	}
	return false
}
