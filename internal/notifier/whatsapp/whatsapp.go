package whatsapp

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/notifier"
)

// WhatsAppNotifier sends listing alerts via WhatsApp using whatsmeow.
//
// To use this notifier, uncomment the whatsmeow imports and implementation below,
// then run: go get go.mau.fi/whatsmeow@latest
//
// For now, this is a stub that logs messages to stdout, allowing the rest of
// the system to be developed and tested without the WhatsApp dependency.
type WhatsAppNotifier struct {
	dbPath string
	logger *slog.Logger
}

func New(dbPath string, logger *slog.Logger) *WhatsAppNotifier {
	return &WhatsAppNotifier{
		dbPath: dbPath,
		logger: logger,
	}
}

func (w *WhatsAppNotifier) Connect(ctx context.Context) error {
	w.logger.Info("whatsapp notifier: connect (stub mode - messages will be logged to stdout)")
	w.logger.Info("to enable real WhatsApp messaging, implement whatsmeow integration")
	return nil
}

func (w *WhatsAppNotifier) Notify(ctx context.Context, recipient string, listings []model.Listing) error {
	msg := notifier.FormatBatch(listings)
	w.logger.Info("would send WhatsApp message",
		"recipient", recipient,
		"listing_count", len(listings),
	)
	fmt.Println("--- WhatsApp Message Preview ---")
	fmt.Println("To:", recipient)
	fmt.Println(msg)
	fmt.Println("--- End Preview ---")
	return nil
}

func (w *WhatsAppNotifier) Disconnect() error {
	w.logger.Info("whatsapp notifier: disconnect")
	return nil
}

/*
// Full whatsmeow implementation — uncomment when ready to integrate.
// Requires: go get go.mau.fi/whatsmeow@latest

import (
	"os"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"

	_ "github.com/mattn/go-sqlite3"
)

type WhatsAppNotifier struct {
	dbPath string
	client *whatsmeow.Client
	logger *slog.Logger
}

func New(dbPath string, logger *slog.Logger) *WhatsAppNotifier {
	return &WhatsAppNotifier{dbPath: dbPath, logger: logger}
}

func (w *WhatsAppNotifier) Connect(ctx context.Context) error {
	container, err := sqlstore.New("sqlite3",
		"file:"+w.dbPath+"?_foreign_keys=on",
		waLog.Noop,
	)
	if err != nil {
		return fmt.Errorf("open whatsmeow store: %w", err)
	}

	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		return fmt.Errorf("get device: %w", err)
	}

	w.client = whatsmeow.NewClient(deviceStore, waLog.Noop)

	if w.client.Store.ID == nil {
		qrChan, _ := w.client.GetQRChannel(ctx)
		if err := w.client.Connect(); err != nil {
			return fmt.Errorf("connect for QR: %w", err)
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				fmt.Println("Scan this QR code with WhatsApp:")
				fmt.Println(evt.Code)
			} else {
				w.logger.Info("QR event", "event", evt.Event)
			}
		}
	} else {
		if err := w.client.Connect(); err != nil {
			return fmt.Errorf("connect: %w", err)
		}
	}

	w.logger.Info("whatsapp connected")
	return nil
}

func (w *WhatsAppNotifier) Notify(ctx context.Context, recipient string, listings []model.Listing) error {
	jid, err := types.ParseJID(recipient + "@s.whatsapp.net")
	if err != nil {
		return fmt.Errorf("parse JID: %w", err)
	}

	msg := notifier.FormatBatch(listings)

	_, err = w.client.SendMessage(ctx, jid, &waE2E.Message{
		Conversation: proto.String(msg),
	})
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}

	w.logger.Info("sent WhatsApp message", "recipient", recipient, "listings", len(listings))
	return nil
}

func (w *WhatsAppNotifier) Disconnect() error {
	if w.client != nil {
		w.client.Disconnect()
	}
	return nil
}
*/
