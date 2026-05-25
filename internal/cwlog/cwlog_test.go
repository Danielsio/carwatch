package cwlog

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/dsionov/carwatch/internal/logstream"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/storage"
)

func parseJSON(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("failed to parse JSON log output: %v\nraw: %s", err, buf.String())
	}
	return m
}

func TestContextHandler_ExtractsAllKeys(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(NewContextHandler(inner))

	ctx := context.Background()
	ctx = WithCycleID(ctx, 42)
	ctx = WithSearchID(ctx, 100)
	ctx = WithChatID(ctx, 999)
	ctx = WithRequestID(ctx, "abc-123")
	ctx = WithComponent(ctx, "scheduler")

	logger.InfoContext(ctx, "test message")

	m := parseJSON(t, &buf)

	if m["cycle_id"] != float64(42) {
		t.Errorf("cycle_id = %v, want 42", m["cycle_id"])
	}
	if m["search_id"] != float64(100) {
		t.Errorf("search_id = %v, want 100", m["search_id"])
	}
	if m["chat_id"] != float64(999) {
		t.Errorf("chat_id = %v, want 999", m["chat_id"])
	}
	if m["request_id"] != "abc-123" {
		t.Errorf("request_id = %v, want abc-123", m["request_id"])
	}
	if m["component"] != "scheduler" {
		t.Errorf("component = %v, want scheduler", m["component"])
	}
	if m["msg"] != "test message" {
		t.Errorf("msg = %v, want 'test message'", m["msg"])
	}
}

func TestContextHandler_OmitsZeroValues(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(NewContextHandler(inner))

	logger.InfoContext(context.Background(), "no context")

	m := parseJSON(t, &buf)

	for _, key := range []string{"cycle_id", "search_id", "chat_id", "request_id", "component"} {
		if _, exists := m[key]; exists {
			t.Errorf("key %q should not appear when context is empty, got %v", key, m[key])
		}
	}
}

func TestContextHandler_PartialContext(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(NewContextHandler(inner))

	ctx := WithCycleID(context.Background(), 7)
	ctx = WithChatID(ctx, 555)

	logger.InfoContext(ctx, "partial")

	m := parseJSON(t, &buf)

	if m["cycle_id"] != float64(7) {
		t.Errorf("cycle_id = %v, want 7", m["cycle_id"])
	}
	if m["chat_id"] != float64(555) {
		t.Errorf("chat_id = %v, want 555", m["chat_id"])
	}
	for _, key := range []string{"search_id", "request_id", "component"} {
		if _, exists := m[key]; exists {
			t.Errorf("key %q should not appear, got %v", key, m[key])
		}
	}
}

func TestWithSearch_SetsBothKeys(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(NewContextHandler(inner))

	ctx := WithSearch(context.Background(), 10, 20)
	logger.InfoContext(ctx, "both")

	m := parseJSON(t, &buf)

	if m["search_id"] != float64(10) {
		t.Errorf("search_id = %v, want 10", m["search_id"])
	}
	if m["chat_id"] != float64(20) {
		t.Errorf("chat_id = %v, want 20", m["chat_id"])
	}
}

func TestContextHandler_WithAttrsPreservation(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(NewContextHandler(inner))

	scoped := logger.With("component", "yad2", "extra", "val")
	ctx := WithCycleID(context.Background(), 3)

	scoped.InfoContext(ctx, "scoped")

	m := parseJSON(t, &buf)

	if m["component"] != "yad2" {
		t.Errorf("component = %v, want yad2", m["component"])
	}
	if m["extra"] != "val" {
		t.Errorf("extra = %v, want val", m["extra"])
	}
	if m["cycle_id"] != float64(3) {
		t.Errorf("cycle_id = %v, want 3", m["cycle_id"])
	}
}

func TestContextHandler_WithGroup(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(NewContextHandler(inner))

	grouped := logger.WithGroup("details").With("key", "value")
	ctx := WithSearchID(context.Background(), 77)

	grouped.InfoContext(ctx, "grouped")

	m := parseJSON(t, &buf)

	// slog nests both pre-set attrs and record-level attrs (including
	// context-injected ones) under the active group.
	details, ok := m["details"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'details' group in output, got %v", m)
	}
	if details["key"] != "value" {
		t.Errorf("details.key = %v, want value", details["key"])
	}
	if details["search_id"] != float64(77) {
		t.Errorf("details.search_id = %v, want 77", details["search_id"])
	}
}

func TestContextHandler_ComposesWithTeeHandler(t *testing.T) {
	hub := logstream.NewHub(100)
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	tee := logstream.NewTeeHandler(base, hub, "scheduler")
	logger := slog.New(NewContextHandler(tee))

	ctx := WithCycleID(context.Background(), 5)
	ctx = WithSearchID(ctx, 42)

	scoped := logger.With("component", "scheduler")
	scoped.InfoContext(ctx, "matched listing")

	entries := hub.Recent(0)
	if len(entries) != 1 {
		t.Fatalf("expected 1 captured entry, got %d", len(entries))
	}
	entry := entries[0]
	if entry.Component != "scheduler" {
		t.Errorf("component = %q, want scheduler", entry.Component)
	}
	if entry.Attrs["cycle_id"] != "5" {
		t.Errorf("attrs[cycle_id] = %q, want 5", entry.Attrs["cycle_id"])
	}
	if entry.Attrs["search_id"] != "42" {
		t.Errorf("attrs[search_id] = %q, want 42", entry.Attrs["search_id"])
	}
}

func TestListingAttrs_FullListing(t *testing.T) {
	l := model.RawListing{
		Token:        "tok-123",
		Manufacturer: "Toyota",
		Model:        "Corolla",
		Year:         2020,
		Price:        85000,
		Km:           45000,
		SubModel:     "GLi",
		SubModelID:   99,
	}
	attrs := ListingAttrs(l)

	expected := map[string]any{
		"token":        "tok-123",
		"manufacturer": "Toyota",
		"model":        "Corolla",
		"year":         int64(2020),
		"price":        int64(85000),
		"km":           int64(45000),
		"sub_model":    "GLi",
		"sub_model_id": int64(99),
	}

	got := make(map[string]any)
	for _, a := range attrs {
		got[a.Key] = a.Value.Any()
	}
	for k, v := range expected {
		if got[k] != v {
			t.Errorf("ListingAttrs[%q] = %v, want %v", k, got[k], v)
		}
	}
}

func TestListingAttrs_OmitsZeroOptionals(t *testing.T) {
	l := model.RawListing{
		Token:        "tok-456",
		Manufacturer: "Honda",
		Model:        "Civic",
		Year:         2019,
		Price:        70000,
	}
	attrs := ListingAttrs(l)

	keys := make(map[string]bool)
	for _, a := range attrs {
		keys[a.Key] = true
	}
	for _, key := range []string{"km", "sub_model", "sub_model_id"} {
		if keys[key] {
			t.Errorf("ListingAttrs should omit zero-value %q", key)
		}
	}
}

func TestSearchAttrs(t *testing.T) {
	s := storage.Search{
		ID:     10,
		ChatID: 20,
		Name:   "my search",
		Source: "yad2",
	}
	attrs := SearchAttrs(s)

	got := make(map[string]any)
	for _, a := range attrs {
		got[a.Key] = a.Value.Any()
	}
	if got["search_id"] != int64(10) {
		t.Errorf("search_id = %v, want 10", got["search_id"])
	}
	if got["chat_id"] != int64(20) {
		t.Errorf("chat_id = %v, want 20", got["chat_id"])
	}
	if got["search_name"] != "my search" {
		t.Errorf("search_name = %v, want 'my search'", got["search_name"])
	}
}

func TestErrorEvent(t *testing.T) {
	err := &testError{msg: "connection refused"}
	attrs := ErrorEvent(err, "listings will not persist", "releasing dedup claims")

	got := make(map[string]string)
	for _, a := range attrs {
		got[a.Key] = a.Value.String()
	}
	if got["error"] != "connection refused" {
		t.Errorf("error = %q, want 'connection refused'", got["error"])
	}
	if got["impact"] != "listings will not persist" {
		t.Errorf("impact = %q", got["impact"])
	}
	if got["action_taken"] != "releasing dedup claims" {
		t.Errorf("action_taken = %q", got["action_taken"])
	}
}

func TestPriceDropAttrs(t *testing.T) {
	attrs := PriceDropAttrs("tok-789", 100000, 90000)

	got := make(map[string]any)
	for _, a := range attrs {
		got[a.Key] = a.Value.Any()
	}
	if got["token"] != "tok-789" {
		t.Errorf("token = %v", got["token"])
	}
	if got["old_price"] != int64(100000) {
		t.Errorf("old_price = %v", got["old_price"])
	}
	if got["new_price"] != int64(90000) {
		t.Errorf("new_price = %v", got["new_price"])
	}
	if got["price_change"] != int64(-10000) {
		t.Errorf("price_change = %v, want -10000", got["price_change"])
	}
}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
