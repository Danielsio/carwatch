package notifier

import (
	"context"
	"errors"
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
	return nil
}

func (f *fakeNotifier) NotifyRaw(_ context.Context, recipient string, _ string) error {
	f.rawCalls = append(f.rawCalls, recipient)
	return nil
}

type fakeUserStore struct {
	users map[int64]*storage.User
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

func (f *fakeUserStore) GetLinkedTelegramUser(_ context.Context, _ int64) (*storage.User, error) {
	return nil, nil
}

func TestMultiNotifier_RoutesToCorrectChannel(t *testing.T) {
	tg := &fakeNotifier{name: "telegram"}
	wa := &fakeNotifier{name: "whatsapp"}

	users := &fakeUserStore{users: map[int64]*storage.User{
		100: {ChatID: 100, Channel: "telegram"},
		200: {ChatID: 200, Channel: "whatsapp"},
	}}

	mn := NewMultiNotifier(users, slog.Default())
	_ = mn.Register("telegram", tg)
	_ = mn.Register("whatsapp", wa)

	ctx := context.Background()
	if err := mn.NotifyRaw(ctx, "100", "hello telegram"); err != nil {
		t.Fatalf("notify 100: %v", err)
	}
	if err := mn.NotifyRaw(ctx, "200", "hello whatsapp"); err != nil {
		t.Fatalf("notify 200: %v", err)
	}

	if len(tg.rawCalls) != 1 || tg.rawCalls[0] != "100" {
		t.Errorf("telegram got %v, want [100]", tg.rawCalls)
	}
	if len(wa.rawCalls) != 1 || wa.rawCalls[0] != "200" {
		t.Errorf("whatsapp got %v, want [200]", wa.rawCalls)
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

func TestMultiNotifier_NotifyRoutesCorrectly(t *testing.T) {
	tg := &fakeNotifier{name: "telegram"}
	wa := &fakeNotifier{name: "whatsapp"}

	users := &fakeUserStore{users: map[int64]*storage.User{
		300: {ChatID: 300, Channel: "whatsapp"},
	}}

	mn := NewMultiNotifier(users, slog.Default())
	_ = mn.Register("telegram", tg)
	_ = mn.Register("whatsapp", wa)

	ctx := context.Background()
	if err := mn.Notify(ctx, "300", nil, locale.Hebrew); err != nil {
		t.Fatalf("notify: %v", err)
	}

	if len(wa.calls) != 1 {
		t.Errorf("whatsapp got %d Notify calls, want 1", len(wa.calls))
	}
	if len(tg.calls) != 0 {
		t.Errorf("telegram got %d Notify calls, want 0", len(tg.calls))
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
	if err != errNoNotifier {
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
	wa := &fakeNotifier{name: "whatsapp"}
	base := &fakeUserStore{
		users: map[int64]*storage.User{
			55: {ChatID: 55, Channel: "whatsapp"},
		},
	}
	usersBroken := &fakeUserGetErrStore{fakeUserStore: base, err: errors.New("db unavailable")}

	mn := NewMultiNotifier(usersBroken, slog.Default())
	_ = mn.Register("telegram", tg)
	_ = mn.Register("whatsapp", wa)

	ctx := context.Background()
	if err := mn.NotifyRaw(ctx, "55", "hi"); err != nil {
		t.Fatalf("notify: %v", err)
	}
	if len(tg.rawCalls) != 1 {
		t.Errorf("fallback should use first registered notifier (telegram), calls=%v", tg.rawCalls)
	}
	if len(wa.rawCalls) != 0 {
		t.Errorf("whatsapp should not receive when falling back, got %v", wa.rawCalls)
	}
}

type fakeUserGetErrStore struct {
	*fakeUserStore
	err error
}

func (f *fakeUserGetErrStore) GetUser(_ context.Context, _ int64) (*storage.User, error) {
	return nil, f.err
}
