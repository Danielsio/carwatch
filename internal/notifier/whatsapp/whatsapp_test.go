package whatsapp

import (
	"context"
	"log/slog"
	"testing"

	"github.com/dsionov/carwatch/internal/model"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(&discardWriter{}, &slog.HandlerOptions{Level: slog.LevelError}))
}

type discardWriter struct{}

func (d *discardWriter) Write(p []byte) (int, error) { return len(p), nil }

func TestWhatsAppNotifier_Connect(t *testing.T) {
	n := New("./test.db", testLogger())
	if err := n.Connect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
}

func TestWhatsAppNotifier_Notify(t *testing.T) {
	n := New("./test.db", testLogger())
	listings := []model.Listing{
		{RawListing: model.RawListing{Token: "a", Manufacturer: "Mazda", Model: "3", Price: 95000}},
	}
	if err := n.Notify(context.Background(), "+972111", listings); err != nil {
		t.Fatalf("notify: %v", err)
	}
}

func TestWhatsAppNotifier_NotifyRaw(t *testing.T) {
	n := New("./test.db", testLogger())
	if err := n.NotifyRaw(context.Background(), "+972111", "test message"); err != nil {
		t.Fatalf("notify raw: %v", err)
	}
}

func TestWhatsAppNotifier_Disconnect(t *testing.T) {
	n := New("./test.db", testLogger())
	if err := n.Disconnect(); err != nil {
		t.Fatalf("disconnect: %v", err)
	}
}
