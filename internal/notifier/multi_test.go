package notifier

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/storage"
)

type fakeNotifier struct {
	name          string
	calls         []string
	rawCalls      []string
	connectErr    error
	disconnectErr error
	notifyErr     error
}

func (f *fakeNotifier) Connect(_ context.Context) error {
	if f.connectErr != nil {
		return f.connectErr
	}
	return nil
}

func (f *fakeNotifier) Disconnect() error {
	if f.disconnectErr != nil {
		return f.disconnectErr
	}
	return nil
}

func (f *fakeNotifier) Notify(_ context.Context, recipient string, _ []model.Listing, _ locale.Lang) error {
	f.calls = append(f.calls, recipient)
	return f.notifyErr
}

func (f *fakeNotifier) NotifyRaw(_ context.Context, recipient string, _ string) error {
	f.rawCalls = append(f.rawCalls, recipient)
	return f.notifyErr
}

type fakeUserStore struct {
	users         map[int64]*storage.User
	linkedTgUsers map[int64]*storage.User
}

func (f *fakeUserStore) UpsertUser(_ context.Context, _ int64, _ string) error         { return nil }
func (f *fakeUserStore) UpdateUserState(_ context.Context, _ int64, _, _ string) error { return nil }
func (f *fakeUserStore) ListActiveUsers(_ context.Context) ([]storage.User, error)     { return nil, nil }
func (f *fakeUserStore) SetUserActive(_ context.Context, _ int64, _ bool) error        { return nil }
func (f *fakeUserStore) SetUserLanguage(_ context.Context, _ int64, _ string) error    { return nil }
func (f *fakeUserStore) CountUsers(_ context.Context) (int64, error)                   { return 0, nil }
func (f *fakeUserStore) SetUserTier(_ context.Context, _ int64, _ string, _ time.Time) error {
	return nil
}
func (f *fakeUserStore) GrantTrial(_ context.Context, _ int64, _ time.Duration) error { return nil }
func (f *fakeUserStore) ListExpiredPremium(_ context.Context) ([]storage.User, error) {
	return nil, nil
}
func (f *fakeUserStore) GetUserByChannelID(_ context.Context, _, _ string) (*storage.User, error) {
	return nil, nil
}
func (f *fakeUserStore) UpsertWhatsAppUser(_ context.Context, _ string) (int64, error) { return 0, nil }
func (f *fakeUserStore) UpsertWebUser(_ context.Context, _, _ string) (int64, error) {
	return 0, nil
}
func (f *fakeUserStore) UpdateLastSeenAt(_ context.Context, _ int64) error { return nil }

func (f *fakeUserStore) GetUser(_ context.Context, chatID int64) (*storage.User, error) {
	u, ok := f.users[chatID]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func (f *fakeUserStore) LinkTelegramToWeb(_ context.Context, _, _ int64) error { return nil }

func (f *fakeUserStore) GetLinkedTelegramUser(_ context.Context, webChatID int64) (*storage.User, error) {
	if f.linkedTgUsers == nil {
		return nil, nil
	}
	u, ok := f.linkedTgUsers[webChatID]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func TestMultiNotifier_FanOutToAllChannels(t *testing.T) {
	tg := &fakeNotifier{name: "telegram"}
	wp := &fakeNotifier{name: "webpush"}

	users := &fakeUserStore{users: map[int64]*storage.User{
		100: {ChatID: 100, Channel: "telegram"},
	}}

	mn := NewMultiNotifier(users, slog.Default())
	_ = mn.Register("telegram", tg)
	_ = mn.Register("webpush", wp)

	ctx := context.Background()
	if err := mn.NotifyRaw(ctx, "100", "hello"); err != nil {
		t.Fatalf("notify: %v", err)
	}

	if len(tg.rawCalls) != 1 {
		t.Errorf("telegram got %d calls, want 1", len(tg.rawCalls))
	}
	if len(wp.rawCalls) != 1 {
		t.Errorf("webpush got %d calls, want 1 (fan-out)", len(wp.rawCalls))
	}
}

func TestMultiNotifier_PartialFailureSucceeds(t *testing.T) {
	tg := &fakeNotifier{name: "telegram", notifyErr: fmt.Errorf("telegram down")}
	wp := &fakeNotifier{name: "webpush"}

	users := &fakeUserStore{users: map[int64]*storage.User{
		100: {ChatID: 100, Channel: "telegram"},
	}}

	mn := NewMultiNotifier(users, slog.Default())
	_ = mn.Register("telegram", tg)
	_ = mn.Register("webpush", wp)

	ctx := context.Background()
	err := mn.NotifyRaw(ctx, "100", "hello")
	if err != nil {
		t.Fatalf("partial failure should succeed, got: %v", err)
	}
}

func TestMultiNotifier_TotalFailureReturnsError(t *testing.T) {
	tg := &fakeNotifier{name: "telegram", notifyErr: fmt.Errorf("telegram down")}
	wp := &fakeNotifier{name: "webpush", notifyErr: fmt.Errorf("webpush down")}

	users := &fakeUserStore{users: map[int64]*storage.User{
		100: {ChatID: 100, Channel: "telegram"},
	}}

	mn := NewMultiNotifier(users, slog.Default())
	_ = mn.Register("telegram", tg)
	_ = mn.Register("webpush", wp)

	ctx := context.Background()
	err := mn.NotifyRaw(ctx, "100", "hello")
	if err == nil {
		t.Fatal("total failure should return error")
	}
}

func TestMultiNotifier_FallsBackToFirst(t *testing.T) {
	tg := &fakeNotifier{name: "telegram"}

	users := &fakeUserStore{users: map[int64]*storage.User{}}

	mn := NewMultiNotifier(users, slog.Default())
	_ = mn.Register("telegram", tg)

	ctx := context.Background()
	if err := mn.NotifyRaw(ctx, "999", "unknown user"); err != nil {
		t.Fatalf("fallback notify: %v", err)
	}

	if len(tg.rawCalls) != 1 {
		t.Errorf("fallback: telegram got %d calls, want 1", len(tg.rawCalls))
	}
}

func TestMultiNotifier_NoRegisteredNotifier(t *testing.T) {
	users := &fakeUserStore{users: map[int64]*storage.User{}}
	mn := NewMultiNotifier(users, slog.Default())

	ctx := context.Background()
	err := mn.NotifyRaw(ctx, "100", "hello")
	if err == nil {
		t.Fatal("expected error when no notifiers registered")
	}
	if !errors.Is(err, errNoNotifier) {
		t.Errorf("expected errNoNotifier, got %v", err)
	}
}

func TestMultiNotifier_RegisterValidation(t *testing.T) {
	users := &fakeUserStore{users: map[int64]*storage.User{}}
	mn := NewMultiNotifier(users, slog.Default())

	if err := mn.Register("", &fakeNotifier{}); err == nil {
		t.Error("expected error for empty channel")
	}
	if err := mn.Register("telegram", nil); err == nil {
		t.Error("expected error for nil notifier")
	}
	if err := mn.Register("telegram", &fakeNotifier{}); err != nil {
		t.Errorf("valid register: %v", err)
	}
}

func TestMultiNotifier_ConnectAllAndDisconnect(t *testing.T) {
	a := &fakeNotifier{name: "a"}
	b := &fakeNotifier{name: "b"}
	users := &fakeUserStore{users: map[int64]*storage.User{}}
	mn := NewMultiNotifier(users, slog.Default())
	_ = mn.Register("telegram", a)
	_ = mn.Register("whatsapp", b)

	ctx := context.Background()
	if err := mn.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if err := mn.Disconnect(); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
}

func TestMultiNotifier_ConnectStopsOnFirstError(t *testing.T) {
	boom := errors.New("connect failed")
	a := &fakeNotifier{name: "bad", connectErr: boom}
	b := &fakeNotifier{name: "after"}
	users := &fakeUserStore{users: map[int64]*storage.User{}}
	mn := NewMultiNotifier(users, slog.Default())
	_ = mn.Register("a", a)
	_ = mn.Register("b", b)

	err := mn.Connect(context.Background())
	if err == nil {
		t.Fatal("expected Connect error")
	}
	if !errors.Is(err, boom) {
		t.Errorf("expected wrapped boom, got %v", err)
	}
}

func TestMultiNotifier_DisconnectJoinsErrors(t *testing.T) {
	e1 := errors.New("d1")
	e2 := errors.New("d2")
	a := &fakeNotifier{name: "a", disconnectErr: e1}
	b := &fakeNotifier{name: "b", disconnectErr: e2}
	users := &fakeUserStore{users: map[int64]*storage.User{}}
	mn := NewMultiNotifier(users, slog.Default())
	_ = mn.Register("a", a)
	_ = mn.Register("b", b)

	err := mn.Disconnect()
	if err == nil {
		t.Fatal("expected Disconnect errors")
	}
	if !errors.Is(err, e1) || !errors.Is(err, e2) {
		t.Errorf("expected both disconnect errors joined, got %v", err)
	}
}

func TestMultiNotifier_NotifyRaw_UserLookupErrorFallsBack(t *testing.T) {
	tg := &fakeNotifier{name: "telegram"}
	base := &fakeUserStore{users: map[int64]*storage.User{}}
	usersBroken := &fakeUserGetErrStore{fakeUserStore: base, err: errors.New("db unavailable")}

	mn := NewMultiNotifier(usersBroken, slog.Default())
	_ = mn.Register("telegram", tg)

	ctx := context.Background()
	if err := mn.NotifyRaw(ctx, "55", "hi"); err != nil {
		t.Fatalf("notify: %v", err)
	}
	if len(tg.rawCalls) != 1 {
		t.Errorf("fallback should use first registered notifier, calls=%v", tg.rawCalls)
	}
}

func TestMultiNotifier_WebUserWithoutLinkedTelegram(t *testing.T) {
	tg := &fakeNotifier{name: "telegram"}
	webpush := &fakeNotifier{name: "webpush"}
	users := &fakeUserStore{users: map[int64]*storage.User{
		777: {ChatID: 777, Channel: "web"},
	}}

	mn := NewMultiNotifier(users, slog.Default())
	_ = mn.Register("telegram", tg)
	_ = mn.Register("webpush", webpush)

	if err := mn.NotifyRaw(context.Background(), "777", "hello web user"); err != nil {
		t.Fatalf("notify web user: %v", err)
	}
	if len(webpush.rawCalls) != 1 {
		t.Errorf("webpush got %d calls, want 1", len(webpush.rawCalls))
	}
	if len(tg.rawCalls) != 0 {
		t.Errorf("telegram got %d calls, want 0 (web user without linked telegram)", len(tg.rawCalls))
	}
}

func TestMultiNotifier_WebUserWithLinkedTelegram_NoFanOut(t *testing.T) {
	tg := &fakeNotifier{name: "telegram"}
	webpush := &fakeNotifier{name: "webpush"}
	users := &fakeUserStore{
		users: map[int64]*storage.User{
			777: {ChatID: 777, Channel: "web"},
		},
		linkedTgUsers: map[int64]*storage.User{
			777: {ChatID: 123456, Channel: "telegram"},
		},
	}

	mn := NewMultiNotifier(users, slog.Default())
	_ = mn.Register("telegram", tg)
	_ = mn.Register("webpush", webpush)

	if err := mn.NotifyRaw(context.Background(), "777", "hello linked user"); err != nil {
		t.Fatalf("notify linked user: %v", err)
	}
	if len(webpush.rawCalls) != 1 {
		t.Errorf("webpush got %d calls, want 1", len(webpush.rawCalls))
	}
	// Telegram fan-out is NOT attempted for linked web users because
	// resolveAll passes the web chat ID (777) to the telegram notifier,
	// not the linked telegram chat ID (123456). Until recipient-override
	// support is added, linked telegram delivery is intentionally skipped.
	if len(tg.rawCalls) != 0 {
		t.Errorf("telegram got %d calls, want 0 (linked telegram fan-out not yet supported)", len(tg.rawCalls))
	}
}

func TestMultiNotifier_TelegramUserAlwaysGetsTelegram(t *testing.T) {
	tg := &fakeNotifier{name: "telegram"}
	users := &fakeUserStore{users: map[int64]*storage.User{
		100: {ChatID: 100, Channel: "telegram"},
	}}

	mn := NewMultiNotifier(users, slog.Default())
	_ = mn.Register("telegram", tg)

	if err := mn.NotifyRaw(context.Background(), "100", "hello tg user"); err != nil {
		t.Fatalf("notify telegram user: %v", err)
	}
	if len(tg.rawCalls) != 1 {
		t.Errorf("telegram got %d calls, want 1", len(tg.rawCalls))
	}
}

type fakeUserGetErrStore struct {
	*fakeUserStore
	err error
}

func (f *fakeUserGetErrStore) GetUser(_ context.Context, _ int64) (*storage.User, error) {
	return nil, f.err
}
