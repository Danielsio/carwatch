package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tgbot "github.com/go-telegram/bot"

	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/notifier"
)

type discardWriter struct{}

func (d *discardWriter) Write(p []byte) (int, error) { return len(p), nil }

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(&discardWriter{}, &slog.HandlerOptions{Level: slog.LevelError}))
}

func tgResponse(result any) []byte {
	b, _ := json.Marshal(map[string]any{"ok": true, "result": result})
	return b
}

var getMeResponse = tgResponse(map[string]any{
	"id": 12345, "is_bot": true, "first_name": "TestBot", "username": "test_bot",
})

var sendMessageOK = tgResponse(map[string]any{
	"message_id": 1, "chat": map[string]any{"id": 123}, "date": 0,
})

var sendPhotoOK = tgResponse(map[string]any{
	"message_id": 2, "chat": map[string]any{"id": 123}, "date": 0,
})

func routingHandler(sendMessageHandler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "getMe") {
			_, _ = w.Write(getMeResponse)
			return
		}
		sendMessageHandler(w, r)
	}
}

func newTestNotifier(t *testing.T, handler http.Handler) *Notifier {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	n, err := New("test-token", testLogger(),
		tgbot.WithServerURL(srv.URL),
	)
	if err != nil {
		t.Fatalf("create notifier: %v", err)
	}
	// Disable send delay in tests to avoid sleeping.
	n.sendDelay = 0
	return n
}

// --- splitMessage tests ---

func TestSplitMessage_ShortMessage(t *testing.T) {
	chunks := splitMessage("hello", 100)
	if len(chunks) != 1 || chunks[0] != "hello" {
		t.Errorf("expected single chunk 'hello', got %v", chunks)
	}
}

func TestSplitMessage_ExactLimit(t *testing.T) {
	text := strings.Repeat("a", 100)
	chunks := splitMessage(text, 100)
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk for text at exact limit, got %d", len(chunks))
	}
}

func TestSplitMessage_SplitsAtNewline(t *testing.T) {
	line := strings.Repeat("a", 40)
	text := line + "\n" + line + "\n" + line
	chunks := splitMessage(text, 50)

	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}
	for i, chunk := range chunks {
		if len([]rune(chunk)) > 50 {
			t.Errorf("chunk %d exceeds limit: %d runes", i, len([]rune(chunk)))
		}
	}
}

func TestSplitMessage_NoNewline_SplitsAtLimit(t *testing.T) {
	text := strings.Repeat("x", 200)
	chunks := splitMessage(text, 100)

	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if len([]rune(chunks[0])) != 100 {
		t.Errorf("first chunk should be 100 runes, got %d", len([]rune(chunks[0])))
	}
	if len([]rune(chunks[1])) != 100 {
		t.Errorf("second chunk should be 100 runes, got %d", len([]rune(chunks[1])))
	}
}

func TestSplitMessage_Unicode(t *testing.T) {
	text := strings.Repeat("שלום", 30) // 4 runes each = 120 runes
	chunks := splitMessage(text, 50)

	total := 0
	for i, chunk := range chunks {
		runeLen := len([]rune(chunk))
		if runeLen > 50 {
			t.Errorf("chunk %d has %d runes, exceeds limit 50", i, runeLen)
		}
		total += runeLen
	}
	if total != 120 {
		t.Errorf("total runes = %d, want 120", total)
	}
}

func TestSplitMessage_EmptyString(t *testing.T) {
	chunks := splitMessage("", 100)
	if len(chunks) != 1 || chunks[0] != "" {
		t.Errorf("expected single empty chunk, got %v", chunks)
	}
}

func TestSplitMessage_PreservesContent(t *testing.T) {
	text := "line1\nline2\nline3\nline4\nline5"
	chunks := splitMessage(text, 15)

	combined := strings.Join(chunks, "")
	if combined != text {
		t.Errorf("reassembled text differs:\ngot:  %q\nwant: %q", combined, text)
	}
}

// --- lastRuneNewlineBefore tests ---

func TestLastRuneNewlineBefore_Found(t *testing.T) {
	r := []rune("hello\nworld\nfoo")
	idx := lastRuneNewlineBefore(r, 12)
	if idx != 11 {
		t.Errorf("expected 11, got %d", idx)
	}
}

func TestLastRuneNewlineBefore_NotFound(t *testing.T) {
	r := []rune("helloworld")
	idx := lastRuneNewlineBefore(r, 10)
	if idx != -1 {
		t.Errorf("expected -1 when no newline, got %d", idx)
	}
}

func TestLastRuneNewlineBefore_PosExceedsLen(t *testing.T) {
	r := []rune("a\nb")
	idx := lastRuneNewlineBefore(r, 100)
	if idx != 1 {
		t.Errorf("expected 1, got %d", idx)
	}
}

func TestLastRuneNewlineBefore_Empty(t *testing.T) {
	idx := lastRuneNewlineBefore(nil, 5)
	if idx != -1 {
		t.Errorf("expected -1 for nil slice, got %d", idx)
	}
}

// --- sendMessage tests ---

func TestSendMessage_InvalidChatID(t *testing.T) {
	n := newTestNotifier(t, routingHandler(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(sendMessageOK)
	}))

	err := n.NotifyRaw(context.Background(), "not-a-number", "test message body")
	if err == nil {
		t.Fatal("expected error for invalid chat ID")
	}
	if !strings.Contains(err.Error(), "invalid chat ID") {
		t.Errorf("expected 'invalid chat ID' error, got: %v", err)
	}
}

func TestSendMessage_Success(t *testing.T) {
	var sendCalls atomic.Int32
	n := newTestNotifier(t, routingHandler(func(w http.ResponseWriter, r *http.Request) {
		sendCalls.Add(1)
		_, _ = w.Write(sendMessageOK)
	}))

	err := n.NotifyRaw(context.Background(), "123", "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sendCalls.Load() != 1 {
		t.Errorf("expected 1 sendMessage call, got %d", sendCalls.Load())
	}
}

func TestSendMessage_APIError_Blocked(t *testing.T) {
	n := newTestNotifier(t, routingHandler(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		resp, _ := json.Marshal(map[string]any{
			"ok":          false,
			"description": "Forbidden: bot was blocked by the user",
		})
		_, _ = w.Write(resp)
	}))

	err := n.NotifyRaw(context.Background(), "123", "test message body")
	if err == nil {
		t.Fatal("expected error for API failure")
	}
	if !errors.Is(err, notifier.ErrRecipientBlocked) {
		t.Errorf("expected ErrRecipientBlocked, got: %v", err)
	}
}

func TestSendMessage_LargeMessage_MultipleChunks(t *testing.T) {
	var sendCalls atomic.Int32
	n := newTestNotifier(t, routingHandler(func(w http.ResponseWriter, r *http.Request) {
		sendCalls.Add(1)
		_, _ = w.Write(sendMessageOK)
	}))

	text := strings.Repeat("a\n", maxMessageLen)
	err := n.NotifyRaw(context.Background(), "123", text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sendCalls.Load() < 2 {
		t.Errorf("expected multiple sendMessage calls for large message, got %d", sendCalls.Load())
	}
}

func TestSendMessage_PartialFailure(t *testing.T) {
	var sendCalls atomic.Int32
	n := newTestNotifier(t, routingHandler(func(w http.ResponseWriter, r *http.Request) {
		c := sendCalls.Add(1)
		// Fail on every call from chunk 2 onward (non-retryable error).
		if c >= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			resp, _ := json.Marshal(map[string]any{
				"ok":          false,
				"description": "Internal Server Error",
			})
			_, _ = w.Write(resp)
			return
		}
		_, _ = w.Write(sendMessageOK)
	}))

	text := strings.Repeat("x", maxMessageLen+100)
	err := n.NotifyRaw(context.Background(), "123", text)
	if err == nil {
		t.Fatal("expected error on second chunk failure")
	}
}

// --- Rate limit retry tests ---

func TestSendMessage_429_RetriesAndSucceeds(t *testing.T) {
	var sendCalls atomic.Int32
	n := newTestNotifier(t, routingHandler(func(w http.ResponseWriter, r *http.Request) {
		c := sendCalls.Add(1)
		if c == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			resp, _ := json.Marshal(map[string]any{
				"ok":          false,
				"description": "Too Many Requests: retry after 1",
			})
			_, _ = w.Write(resp)
			return
		}
		_, _ = w.Write(sendMessageOK)
	}))

	err := n.NotifyRaw(context.Background(), "123", "test message body")
	if err != nil {
		t.Fatalf("expected retry to succeed, got: %v", err)
	}
	if sendCalls.Load() != 2 {
		t.Errorf("expected 2 calls (1 fail + 1 retry), got %d", sendCalls.Load())
	}
}

func TestSendMessage_429_RetryAlsoFails(t *testing.T) {
	n := newTestNotifier(t, routingHandler(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		resp, _ := json.Marshal(map[string]any{
			"ok":          false,
			"description": "Too Many Requests: retry after 1",
		})
		_, _ = w.Write(resp)
	}))

	err := n.NotifyRaw(context.Background(), "123", "test message body")
	if err == nil {
		t.Fatal("expected error when retry also fails")
	}
	if !strings.Contains(err.Error(), "telegram sendMessage") {
		t.Errorf("expected 'telegram sendMessage' error, got: %v", err)
	}
}

// --- isRateLimited tests ---

func TestIsRateLimited(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"unrelated error", errors.New("connection refused"), false},
		{"too many requests", errors.New("Too Many Requests: retry after 5"), true},
		{"retry_after in json", errors.New(`"retry_after":10`), true},
		{"blocked user", errors.New("Forbidden: bot was blocked by the user"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRateLimited(tt.err)
			if got != tt.want {
				t.Errorf("isRateLimited(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// --- parseRetryAfter tests ---

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want time.Duration
	}{
		{"nil error", nil, retryAfterDefault},
		{"retry after 10", fmt.Errorf("Too Many Requests: retry after 10"), 10 * time.Second},
		{"retry after 1", fmt.Errorf("retry after 1"), 1 * time.Second},
		{"retry_after json", fmt.Errorf(`"retry_after":30`), 30 * time.Second},
		{"no number", fmt.Errorf("retry after abc"), retryAfterDefault},
		{"unrelated error", fmt.Errorf("connection refused"), retryAfterDefault},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRetryAfter(tt.err)
			if got != tt.want {
				t.Errorf("parseRetryAfter(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// --- Notify tests ---

func TestNotify_FormatsAndSends(t *testing.T) {
	var mu sync.Mutex
	var bodies []string

	n := newTestNotifier(t, routingHandler(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(body))
		mu.Unlock()
		_, _ = w.Write(sendMessageOK)
	}))

	listings := []model.Listing{
		{
			RawListing: model.RawListing{
				Token: "abc", Manufacturer: "Toyota", Model: "Corolla",
				Year: 2021, Price: 120000, Km: 30000,
			},
			SearchName: "test",
		},
	}

	err := n.Notify(context.Background(), "123", listings, locale.English)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(bodies) == 0 {
		t.Fatal("expected at least one sendMessage call")
	}
	allBodies := strings.Join(bodies, " ")
	if !strings.Contains(allBodies, "Toyota") {
		t.Errorf("message should contain manufacturer name, got: %s", allBodies)
	}
}

// --- Connect tests ---

func TestConnect_Success(t *testing.T) {
	n := newTestNotifier(t, routingHandler(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(sendMessageOK)
	}))

	err := n.Connect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConnect_Failure(t *testing.T) {
	var getMeCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "getMe") {
			c := getMeCalls.Add(1)
			if c == 1 {
				_, _ = w.Write(getMeResponse)
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
			resp, _ := json.Marshal(map[string]any{
				"ok":          false,
				"description": "Unauthorized",
			})
			_, _ = w.Write(resp)
			return
		}
		_, _ = w.Write(sendMessageOK)
	}))
	t.Cleanup(srv.Close)

	n, err := New("test-token", testLogger(), tgbot.WithServerURL(srv.URL))
	if err != nil {
		t.Fatalf("create notifier: %v", err)
	}

	err = n.Connect(context.Background())
	if err == nil {
		t.Fatal("expected error for unauthorized getMe")
	}
	if !strings.Contains(err.Error(), "telegram getMe") {
		t.Errorf("expected 'telegram getMe' error, got: %v", err)
	}
}

// --- Disconnect test ---

func TestDisconnect_ReturnsNil(t *testing.T) {
	n := newTestNotifier(t, routingHandler(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(sendMessageOK)
	}))

	if err := n.Disconnect(); err != nil {
		t.Errorf("Disconnect should return nil, got: %v", err)
	}
}

// --- Bot accessor test ---

func TestBot_ReturnsInstance(t *testing.T) {
	n := newTestNotifier(t, routingHandler(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(sendMessageOK)
	}))

	if n.Bot() == nil {
		t.Error("Bot() should return non-nil")
	}
}

// --- New error test ---

func TestNew_EmptyToken(t *testing.T) {
	_, err := New("", testLogger())
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

// --- SendPhoto tests ---

func TestNotify_SingleListingWithPhoto(t *testing.T) {
	var mu sync.Mutex
	var endpoints []string

	n := newTestNotifier(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "getMe") {
			_, _ = w.Write(getMeResponse)
			return
		}
		mu.Lock()
		endpoints = append(endpoints, r.URL.Path)
		mu.Unlock()
		if strings.Contains(r.URL.Path, "sendPhoto") {
			_, _ = w.Write(sendPhotoOK)
		} else {
			_, _ = w.Write(sendMessageOK)
		}
	}))

	listings := []model.Listing{
		{
			RawListing: model.RawListing{
				Token: "abc", Manufacturer: "Toyota", Model: "Corolla",
				Year: 2021, Price: 120000, ImageURL: "https://example.com/photo.jpg",
			},
			SearchName: "test",
		},
	}

	err := n.Notify(context.Background(), "123", listings, locale.English)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	hasPhoto := false
	for _, ep := range endpoints {
		if strings.Contains(ep, "sendPhoto") {
			hasPhoto = true
		}
	}
	if !hasPhoto {
		t.Error("expected sendPhoto call for listing with image")
	}
}

func TestNotify_SingleListingNoPhoto_FallsBackToText(t *testing.T) {
	var mu sync.Mutex
	var endpoints []string

	n := newTestNotifier(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "getMe") {
			_, _ = w.Write(getMeResponse)
			return
		}
		mu.Lock()
		endpoints = append(endpoints, r.URL.Path)
		mu.Unlock()
		_, _ = w.Write(sendMessageOK)
	}))

	listings := []model.Listing{
		{
			RawListing: model.RawListing{
				Token: "abc", Manufacturer: "Toyota", Model: "Corolla",
				Year: 2021, Price: 120000,
			},
			SearchName: "test",
		},
	}

	err := n.Notify(context.Background(), "123", listings, locale.English)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	for _, ep := range endpoints {
		if strings.Contains(ep, "sendPhoto") {
			t.Error("should not call sendPhoto when no image URL")
		}
	}
}

func TestNotify_PhotoFails_FallsBackToText(t *testing.T) {
	var sendMsgCalls atomic.Int32

	n := newTestNotifier(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "getMe") {
			_, _ = w.Write(getMeResponse)
			return
		}
		if strings.Contains(r.URL.Path, "sendPhoto") {
			w.WriteHeader(http.StatusBadRequest)
			resp, _ := json.Marshal(map[string]any{"ok": false, "description": "Bad Request: wrong photo"})
			_, _ = w.Write(resp)
			return
		}
		sendMsgCalls.Add(1)
		_, _ = w.Write(sendMessageOK)
	}))

	listings := []model.Listing{
		{
			RawListing: model.RawListing{
				Token: "abc", Manufacturer: "Toyota", Model: "Corolla",
				Year: 2021, Price: 120000, ImageURL: "https://example.com/bad.jpg",
			},
			SearchName: "test",
		},
	}

	err := n.Notify(context.Background(), "123", listings, locale.English)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sendMsgCalls.Load() == 0 {
		t.Error("expected fallback to sendMessage when sendPhoto fails")
	}
}

func TestNotify_BatchListings_UsesText(t *testing.T) {
	var mu sync.Mutex
	var endpoints []string

	n := newTestNotifier(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "getMe") {
			_, _ = w.Write(getMeResponse)
			return
		}
		mu.Lock()
		endpoints = append(endpoints, r.URL.Path)
		mu.Unlock()
		_, _ = w.Write(sendMessageOK)
	}))

	listings := []model.Listing{
		{RawListing: model.RawListing{Token: "a", Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000, ImageURL: "https://example.com/1.jpg"}, SearchName: "test"},
		{RawListing: model.RawListing{Token: "b", Manufacturer: "Honda", Model: "Civic", Year: 2022, Price: 110000, ImageURL: "https://example.com/2.jpg"}, SearchName: "test"},
	}

	err := n.Notify(context.Background(), "123", listings, locale.English)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	for _, ep := range endpoints {
		if strings.Contains(ep, "sendPhoto") {
			t.Error("batch listings should use sendMessage, not sendPhoto")
		}
	}
}

func TestNotifyRaw_BlocksMalformedMessage(t *testing.T) {
	n := newTestNotifier(t, routingHandler(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach API for malformed message")
	}))

	tests := []struct {
		name string
		msg  string
	}{
		{"template syntax", "{{.}}"},
		{"short message", "hi"},
		{"empty", ""},
		{"sprintf error", "%!s(MISSING)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := n.NotifyRaw(context.Background(), "123", tt.msg)
			if err == nil {
				t.Error("expected error for malformed message")
			}
			if !strings.Contains(err.Error(), "blocked malformed") {
				t.Errorf("expected 'blocked malformed' error, got: %v", err)
			}
		})
	}
}

func TestNotifyRaw_AllowsValidMessage(t *testing.T) {
	var called atomic.Int32
	n := newTestNotifier(t, routingHandler(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		_, _ = w.Write(sendMessageOK)
	}))

	err := n.NotifyRaw(context.Background(), "123", "This is a valid notification message")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called.Load() == 0 {
		t.Error("API should have been called for valid message")
	}
}

