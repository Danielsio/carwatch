package logstream

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestTeeHandler_FiltersByComponent(t *testing.T) {
	hub := NewHub(100)
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	handler := NewTeeHandler(inner, hub, "yad2", "scheduler")

	logger := slog.New(handler)

	// This should be captured (component=yad2)
	logger.With("component", "yad2").Info("fetching")

	// This should NOT be captured (component=bot)
	logger.With("component", "bot").Info("handling")

	// This should be captured (component=scheduler)
	logger.With("component", "scheduler").Warn("retry")

	entries := hub.Recent(0)
	if len(entries) != 2 {
		t.Fatalf("expected 2 captured entries, got %d", len(entries))
	}
	if entries[0].Message != "fetching" || entries[0].Component != "yad2" {
		t.Errorf("entry 0: got %+v", entries[0])
	}
	if entries[1].Message != "retry" || entries[1].Component != "scheduler" {
		t.Errorf("entry 1: got %+v", entries[1])
	}
}

func TestTeeHandler_ForwardsAllToInner(t *testing.T) {
	hub := NewHub(100)
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	handler := NewTeeHandler(inner, hub, "yad2")

	logger := slog.New(handler)
	logger.With("component", "bot").Info("should forward")

	// Inner should have received it even though it's not captured
	if !bytes.Contains(buf.Bytes(), []byte("should forward")) {
		t.Fatalf("inner handler did not receive the record: %s", buf.String())
	}
	// Hub should not have captured it
	if len(hub.Recent(0)) != 0 {
		t.Fatal("hub should not have captured non-matching component")
	}
}

func TestTeeHandler_RecordAttrs(t *testing.T) {
	hub := NewHub(100)
	inner := slog.NewTextHandler(&bytes.Buffer{}, nil)
	handler := NewTeeHandler(inner, hub, "yad2")

	logger := slog.New(handler).With("component", "yad2")
	logger.Info("fetched listings", "count", 42, "page", 1)

	entries := hub.Recent(0)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Attrs["count"] != "42" {
		t.Errorf("expected count=42, got %q", entries[0].Attrs["count"])
	}
	if entries[0].Attrs["page"] != "1" {
		t.Errorf("expected page=1, got %q", entries[0].Attrs["page"])
	}
}

func TestTeeHandler_Enabled(t *testing.T) {
	hub := NewHub(100)
	inner := slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelWarn})
	handler := NewTeeHandler(inner, hub, "yad2")

	if handler.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("should not be enabled for debug when inner is warn")
	}
	if !handler.Enabled(context.Background(), slog.LevelWarn) {
		t.Fatal("should be enabled for warn")
	}
}

func TestTeeHandler_LevelCaptured(t *testing.T) {
	hub := NewHub(100)
	inner := slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelDebug})
	handler := NewTeeHandler(inner, hub, "yad2")

	logger := slog.New(handler).With("component", "yad2")
	logger.Warn("something bad")

	entries := hub.Recent(0)
	if len(entries) != 1 {
		t.Fatalf("expected 1, got %d", len(entries))
	}
	if entries[0].Level != "WARN" {
		t.Errorf("expected WARN, got %q", entries[0].Level)
	}
}

func TestTeeHandler_SubscriberReceivesLive(t *testing.T) {
	hub := NewHub(100)
	inner := slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelDebug})
	handler := NewTeeHandler(inner, hub, "scheduler")

	ch, unsub := hub.Subscribe()
	defer unsub()

	logger := slog.New(handler).With("component", "scheduler")
	logger.Info("cycle start")

	select {
	case e := <-ch:
		if e.Message != "cycle start" {
			t.Fatalf("unexpected message %q", e.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for live entry")
	}
}
