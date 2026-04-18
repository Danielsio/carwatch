package notifier

import (
	"context"

	"github.com/dsionov/carwatch/internal/model"
)

type Notifier interface {
	Connect(ctx context.Context) error
	Notify(ctx context.Context, recipient string, listings []model.Listing) error
	Disconnect() error
}
