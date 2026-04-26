package notifier

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/storage"
)

type fakeNotifier struct {
	name     string
	calls    []string
	rawCalls []string
}

func (f *fakeNotifier) Connect(_ context.Context) error    { return nil }
func (f *fakeNotifier) Disconnect() error                  { return nil }

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

func (f *fakeUserStore) UpsertUser(_ context.Context, _ int64, _ string) error           { return nil }
func (f *fakeUserStore) UpdateUserState(_ context.Context, _ int64, _, _ string) error   { return nil }
func (f *fakeUserStore) ListActiveUsers(_ context.Context) ([]storage.User, error)       { return nil, nil }
func (f *fakeUserStore) SetUserActive(_ context.Context, _ int64, _ bool) error          { return nil }
func (f *fakeUserStore) SetUserLanguage(_ context.Context, _ int64, _ string) error      { return nil }
func (f *fakeUserStore) CountUsers(_ context.Context) (int64, error)                     { return 0, nil }
func (f *fakeUserStore) SetUserTier(_ context.Context, _ int64, _ string, _ time.Time) error {
	return nil
}
func (f *fakeUserStore) GrantTrial(_ context.Context, _ int64, _ time.Duration) error    { return nil }
func (f *fakeUserStore) ListExpiredPremium(_ context.Context) ([]storage.User, error)    { return nil, nil }
func (f *fakeUserStore) GetUserByChannelID(_ context.Context, _, _ string) (*storage.User, error) {
	return nil, nil
}

func (f *fakeUserStore) GetUser(_ context.Context, chatID int64) (*storage.User, error) {
	u, ok := f.users[chatID]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func TestMultiNotifier_RoutesToCorrectChannel(t *testing.T) {
	tg := &fakeNotifier{name: "telegram"}
	wa := &fakeNotifier{name: "whatsapp"}

	users := &fakeUserStore{users: map[int64]*storage.User{
		100: {ChatID: 100, Channel: "telegram"},
		200: {ChatID: 200, Channel: "whatsapp"},
	}}

	mn := NewMultiNotifier(users, slog.Default())
	mn.Register("telegram", tg)
	mn.Register("whatsapp", wa)

	ctx := context.Background()
	_ = mn.NotifyRaw(ctx, "100", "hello telegram")
	_ = mn.NotifyRaw(ctx, "200", "hello whatsapp")

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
	mn.Register("telegram", tg)

	ctx := context.Background()
	_ = mn.NotifyRaw(ctx, "999", "unknown user")

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
	mn.Register("telegram", tg)
	mn.Register("whatsapp", wa)

	ctx := context.Background()
	_ = mn.Notify(ctx, "300", nil, locale.Hebrew)

	if len(wa.calls) != 1 {
		t.Errorf("whatsapp got %d Notify calls, want 1", len(wa.calls))
	}
	if len(tg.calls) != 0 {
		t.Errorf("telegram got %d Notify calls, want 0", len(tg.calls))
	}
}
