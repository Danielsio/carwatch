package telegram

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	tgbot "github.com/go-telegram/bot"

	"github.com/dsionov/carwatch/internal/model"
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

	err := n.NotifyRaw(context.Background(), "not-a-number", "hello")
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

func TestSendMessage_APIError(t *testing.T) {
	n := newTestNotifier(t, routingHandler(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		resp, _ := json.Marshal(map[string]any{
			"ok":          false,
			"description": "Forbidden: bot was blocked by the user",
		})
		_, _ = w.Write(resp)
	}))

	err := n.NotifyRaw(context.Background(), "123", "hello")
	if err == nil {
		t.Fatal("expected error for API failure")
	}
	if !strings.Contains(err.Error(), "telegram sendMessage") {
		t.Errorf("expected 'telegram sendMessage' error, got: %v", err)
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
		if c == 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			resp, _ := json.Marshal(map[string]any{
				"ok":          false,
				"description": "Too Many Requests",
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

	err := n.Notify(context.Background(), "123", listings)
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
