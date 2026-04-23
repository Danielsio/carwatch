package notifier

import (
	"context"
	"errors"

	"github.com/dsionov/carwatch/internal/model"
)

var ErrRecipientBlocked = errors.New("recipient blocked the bot")

type Notifier interface {
	Connect(ctx context.Context) error
	Notify(ctx context.Context, recipient string, listings []model.Listing) error
	NotifyRaw(ctx context.Context, recipient string, message string) error
	Disconnect() error
}
