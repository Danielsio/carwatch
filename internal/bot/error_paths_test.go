package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	tgmodels "github.com/go-telegram/bot/models"

	"github.com/dsionov/carwatch/internal/catalog"
	"github.com/dsionov/carwatch/internal/storage"
)

// errMessenger returns configurable errors from SendMessage and AnswerCallback.
type errMessenger struct {
	mockMessenger
	sendErr     error
	callbackErr error
}

func (m *errMessenger) SendMessage(ctx context.Context, chatID int64, text string, parseMode string, kb *tgmodels.InlineKeyboardMarkup) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	return m.mockMessenger.SendMessage(ctx, chatID, text, parseMode, kb)
}

func (m *errMessenger) AnswerCallback(_ context.Context, _ string) error {
	return m.callbackErr
}

// errUserStore implements UserStore and returns errors.
type errUserStore struct {
	upsertErr      error
	getUserErr     error
	updateStateErr error
	user           *storage.User
}

func (m *errUserStore) UpsertUser(_ context.Context, _ int64, _ string) error {
	return m.upsertErr
}

func (m *errUserStore) GetUser(_ context.Context, _ int64) (*storage.User, error) {
	if m.getUserErr != nil {
		return nil, m.getUserErr
	}
	return m.user, nil
}

func (m *errUserStore) UpdateUserState(_ context.Context, _ int64, _ string, _ string) error {
	return m.updateStateErr
}

func (m *errUserStore) ListActiveUsers(_ context.Context) ([]storage.User, error) { return nil, nil }
func (m *errUserStore) SetUserActive(_ context.Context, _ int64, _ bool) error    { return nil }
func (m *errUserStore) SetUserLanguage(_ context.Context, _ int64, _ string) error { return nil }
func (m *errUserStore) CountUsers(_ context.Context) (int64, error)               { return 0, nil }
func (m *errUserStore) SetUserTier(_ context.Context, _ int64, _ string, _ time.Time) error {
	return nil
}
func (m *errUserStore) GrantTrial(_ context.Context, _ int64, _ time.Duration) error { return nil }
func (m *errUserStore) ListExpiredPremium(_ context.Context) ([]storage.User, error) {
	return nil, nil
}
func (m *errUserStore) GetUserByChannelID(_ context.Context, _, _ string) (*storage.User, error) {
	return nil, nil
}
func (m *errUserStore) UpsertWhatsAppUser(_ context.Context, _ string) (int64, error) {
	return 0, nil
}
func (m *errUserStore) UpdateLastSeenAt(_ context.Context, _ int64) error { return nil }

// errSearchStore implements SearchStore and returns errors.
type errSearchStore struct {
	listErr       error
	createErr     error
	deleteErr     error
	getErr        error
	countErr      error
	searches      []storage.Search
	setActiveErr  error
}

func (m *errSearchStore) CreateSearch(_ context.Context, _ storage.Search) (int64, error) {
	if m.createErr != nil {
		return 0, m.createErr
	}
	return 1, nil
}

func (m *errSearchStore) ListSearches(_ context.Context, _ int64) ([]storage.Search, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.searches, nil
}

func (m *errSearchStore) GetSearch(_ context.Context, _ int64) (*storage.Search, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if len(m.searches) > 0 {
		return &m.searches[0], nil
	}
	return nil, nil
}

func (m *errSearchStore) DeleteSearch(_ context.Context, _ int64, _ int64) error {
	return m.deleteErr
}

func (m *errSearchStore) SetSearchActive(_ context.Context, _, _ int64, _ bool) error {
	return m.setActiveErr
}

func (m *errSearchStore) ListAllActiveSearches(_ context.Context) ([]storage.Search, error) {
	return m.searches, nil
}

func (m *errSearchStore) CountSearches(_ context.Context, _ int64) (int64, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}
	return int64(len(m.searches)), nil
}

func (m *errSearchStore) CountAllSearches(_ context.Context) (int64, error) { return 0, nil }
func (m *errSearchStore) UpdateSearch(_ context.Context, _ storage.Search) error { return nil }

func (m *errSearchStore) GetSearchBySeq(_ context.Context, chatID int64, seq int) (*storage.Search, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for i := range m.searches {
		if m.searches[i].ChatID == chatID && m.searches[i].UserSeq == seq {
			return &m.searches[i], nil
		}
	}
	return nil, nil
}

func (m *errSearchStore) GetSearchByShareToken(_ context.Context, token string) (*storage.Search, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for i := range m.searches {
		if m.searches[i].ShareToken == token {
			return &m.searches[i], nil
		}
	}
	return nil, nil
}

// errDigestStore implements DigestStore and returns errors.
type errDigestStore struct {
	getModeErr error
	setModeErr error
	mode       string
	interval   string
}

func (m *errDigestStore) SetDigestMode(_ context.Context, _ int64, _ string, _ string) error {
	return m.setModeErr
}

func (m *errDigestStore) GetDigestMode(_ context.Context, _ int64) (string, string, error) {
	if m.getModeErr != nil {
		return "", "", m.getModeErr
	}
	return m.mode, m.interval, nil
}

func (m *errDigestStore) AddDigestItem(_ context.Context, _ int64, _ string) error { return nil }
func (m *errDigestStore) PeekDigest(_ context.Context, _ int64) ([]string, time.Time, error) {
	return nil, time.Time{}, nil
}
func (m *errDigestStore) AckDigest(_ context.Context, _ int64, _ time.Time) error { return nil }
func (m *errDigestStore) PendingDigestUsers(_ context.Context) ([]int64, error)    { return nil, nil }

func (m *errDigestStore) DigestLastFlushed(_ context.Context, _ int64) (time.Time, error) {
	return time.Time{}, nil
}

func newErrBot(t *testing.T, msg messenger, users storage.UserStore, searches storage.SearchStore) *Bot {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(&discardWriter{}, &slog.HandlerOptions{Level: slog.LevelError}))
	return &Bot{
		msg:         msg,
		users:       users,
		searches:    searches,
		catalog:     catalog.NewStatic(),
		adminChatID: 999,
		maxSearches: 3,
		botUsername:  "test_bot",
		logger:      logger,
	}
}

// --- Send error tests ---

func TestSend_Error_LogsAndContinues(t *testing.T) {
	msg := &errMessenger{sendErr: errors.New("telegram down")}
	users := &errUserStore{user: &storage.User{ChatID: 100, State: StateIdle, StateData: "{}", Language: "en"}}
	searches := &errSearchStore{}
	b := newErrBot(t, msg, users, searches)

	b.send(context.Background(), 100, "hello")
	// Should not panic — error is logged.
}

func TestSendMarkdown_Error_LogsAndContinues(t *testing.T) {
	msg := &errMessenger{sendErr: errors.New("telegram down")}
	users := &errUserStore{user: &storage.User{ChatID: 100, State: StateIdle, StateData: "{}", Language: "en"}}
	searches := &errSearchStore{}
	b := newErrBot(t, msg, users, searches)

	b.sendMarkdown(context.Background(), 100, "*bold*")
	// Should not panic.
}

func TestSendWithKeyboard_Error_LogsAndContinues(t *testing.T) {
	msg := &errMessenger{sendErr: errors.New("telegram down")}
	users := &errUserStore{user: &storage.User{ChatID: 100, State: StateIdle, StateData: "{}", Language: "en"}}
	searches := &errSearchStore{}
	b := newErrBot(t, msg, users, searches)

	kb := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{{Text: "OK", CallbackData: "ok"}},
		},
	}
	b.sendWithKeyboard(context.Background(), 100, "choose", kb)
	// Should not panic.
}

// --- ensureUser error test ---

func TestEnsureUser_Error_LogsAndContinues(t *testing.T) {
	msg := &mockMessenger{}
	users := &errUserStore{upsertErr: errors.New("db full")}
	searches := &errSearchStore{}
	b := newErrBot(t, msg, users, searches)

	b.ensureUser(context.Background(), 100, "alice")
	// Should not panic.
}

// --- loadWizardData error tests ---

func TestLoadWizardData_GetUserError(t *testing.T) {
	msg := &mockMessenger{}
	users := &errUserStore{getUserErr: errors.New("db error")}
	searches := &errSearchStore{}
	b := newErrBot(t, msg, users, searches)

	wd := b.loadWizardData(context.Background(), 100)
	if wd != (WizardData{}) {
		t.Errorf("expected empty WizardData on error, got %+v", wd)
	}
}

func TestLoadWizardData_NilUser(t *testing.T) {
	msg := &mockMessenger{}
	users := &errUserStore{user: nil}
	searches := &errSearchStore{}
	b := newErrBot(t, msg, users, searches)

	wd := b.loadWizardData(context.Background(), 100)
	if wd != (WizardData{}) {
		t.Errorf("expected empty WizardData for nil user, got %+v", wd)
	}
}

func TestLoadWizardData_CorruptJSON(t *testing.T) {
	msg := &mockMessenger{}
	users := &errUserStore{
		user: &storage.User{ChatID: 100, State: StateAskYearMin, StateData: "{{not json", Language: "en"},
	}
	searches := &errSearchStore{}
	b := newErrBot(t, msg, users, searches)

	wd := b.loadWizardData(context.Background(), 100)
	if wd != (WizardData{}) {
		t.Errorf("expected empty WizardData for corrupt JSON, got %+v", wd)
	}
}

// --- saveWizardState error tests ---

func TestSaveWizardState_UpdateError(t *testing.T) {
	msg := &mockMessenger{}
	users := &errUserStore{updateStateErr: errors.New("write failed")}
	searches := &errSearchStore{}
	b := newErrBot(t, msg, users, searches)

	b.saveWizardState(context.Background(), 100, StateAskYearMin, WizardData{Manufacturer: 27})
	// Should not panic — error is logged.
}

// --- handleList error test ---

func TestHandleList_StoreError(t *testing.T) {
	msg := &mockMessenger{}
	users := &errUserStore{user: &storage.User{ChatID: 100, State: StateIdle, StateData: "{}", Language: "en"}}
	searches := &errSearchStore{listErr: errors.New("db error")}
	b := newErrBot(t, msg, users, searches)

	update := fakeMessage(100, "/list")
	b.handleList(context.Background(), nil, update)

	last := msg.last()
	if !strings.Contains(last.Text, "Failed to load") {
		t.Errorf("expected failure message, got %q", last.Text)
	}
}

// --- handleWatch CountSearches error test ---

func TestHandleWatch_CountSearchesError(t *testing.T) {
	msg := &mockMessenger{}
	users := &errUserStore{user: &storage.User{ChatID: 100, State: StateIdle, StateData: "{}", Language: "en"}}
	searches := &errSearchStore{countErr: errors.New("db error")}
	b := newErrBot(t, msg, users, searches)

	update := fakeMessage(100, "/watch")
	b.handleWatch(context.Background(), nil, update)

	last := msg.last()
	if !strings.Contains(last.Text, "Failed to check search limits") {
		t.Errorf("expected error message about search limits, got %q", last.Text)
	}
}

// --- onConfirm error test ---

func TestOnConfirm_CreateSearchError(t *testing.T) {
	msg := &mockMessenger{}
	users := &errUserStore{
		user: &storage.User{
			ChatID:    100,
			State:     StateConfirm,
			StateData: `{"manufacturer":27,"manufacturerName":"Mazda","model":10332,"modelName":"3","yearMin":2020,"yearMax":2024,"priceMax":150000,"engineMinCC":2000,"source":"yad2"}`,
			Language:  "en",
		},
	}
	searches := &errSearchStore{createErr: errors.New("db full")}
	b := newErrBot(t, msg, users, searches)

	b.onConfirm(context.Background(), 100)

	last := msg.last()
	if !strings.Contains(last.Text, "Failed to save") {
		t.Errorf("expected failure message, got %q", last.Text)
	}
}

// --- onDeleteSearch error tests ---

func TestOnDeleteSearch_NotFoundError(t *testing.T) {
	msg := &mockMessenger{}
	users := &errUserStore{user: &storage.User{ChatID: 100, State: StateIdle, StateData: "{}", Language: "en"}}
	searches := &errSearchStore{deleteErr: storage.ErrNotFound}
	b := newErrBot(t, msg, users, searches)

	b.onDeleteSearch(context.Background(), 100, cbDeleteSearch+"1")

	last := msg.last()
	if !strings.Contains(last.Text, "not found") {
		t.Errorf("expected 'not found' message, got %q", last.Text)
	}
}

func TestOnDeleteSearch_GenericError(t *testing.T) {
	msg := &mockMessenger{}
	users := &errUserStore{user: &storage.User{ChatID: 100, State: StateIdle, StateData: "{}", Language: "en"}}
	searches := &errSearchStore{deleteErr: errors.New("db locked")}
	b := newErrBot(t, msg, users, searches)

	b.onDeleteSearch(context.Background(), 100, cbDeleteSearch+"1")

	last := msg.last()
	if !strings.Contains(last.Text, "Failed to delete") {
		t.Errorf("expected failure message, got %q", last.Text)
	}
}

func TestOnDeleteSearch_InvalidID(t *testing.T) {
	msg := &mockMessenger{}
	users := &errUserStore{user: &storage.User{ChatID: 100, State: StateIdle, StateData: "{}", Language: "en"}}
	searches := &errSearchStore{}
	b := newErrBot(t, msg, users, searches)

	b.onDeleteSearch(context.Background(), 100, cbDeleteSearch+"notanumber")

	last := msg.last()
	if !strings.Contains(last.Text, "Invalid") {
		t.Errorf("expected invalid ID message, got %q", last.Text)
	}
}

// --- handleDigest error tests ---

func TestHandleDigest_GetModeError(t *testing.T) {
	msg := &mockMessenger{}
	users := &errUserStore{user: &storage.User{ChatID: 100, State: StateIdle, StateData: "{}", Language: "en"}}
	searches := &errSearchStore{}
	ds := &errDigestStore{getModeErr: errors.New("db error")}
	b := newErrBot(t, msg, users, searches)
	b.digests = ds

	update := fakeMessage(100, "/digest")
	b.handleDigest(context.Background(), nil, update)

	last := msg.last()
	if !strings.Contains(last.Text, "Failed to load") {
		t.Errorf("expected failure message, got %q", last.Text)
	}
}

func TestOnDigestOn_SetModeError(t *testing.T) {
	msg := &mockMessenger{}
	users := &errUserStore{user: &storage.User{ChatID: 100, State: StateIdle, StateData: "{}", Language: "en"}}
	searches := &errSearchStore{}
	ds := &errDigestStore{setModeErr: errors.New("db error")}
	b := newErrBot(t, msg, users, searches)
	b.digests = ds

	b.onDigestOn(context.Background(), 100)

	last := msg.last()
	if !strings.Contains(last.Text, "Failed to update") {
		t.Errorf("expected failure message, got %q", last.Text)
	}
}

func TestOnDigestOff_SetModeError(t *testing.T) {
	msg := &mockMessenger{}
	users := &errUserStore{user: &storage.User{ChatID: 100, State: StateIdle, StateData: "{}", Language: "en"}}
	searches := &errSearchStore{}
	ds := &errDigestStore{setModeErr: errors.New("db error")}
	b := newErrBot(t, msg, users, searches)
	b.digests = ds

	b.onDigestOff(context.Background(), 100)

	last := msg.last()
	if !strings.Contains(last.Text, "Failed to update") {
		t.Errorf("expected failure message, got %q", last.Text)
	}
}

func TestOnDigestInterval_SetModeError(t *testing.T) {
	msg := &mockMessenger{}
	users := &errUserStore{user: &storage.User{ChatID: 100, State: StateIdle, StateData: "{}", Language: "en"}}
	searches := &errSearchStore{}
	ds := &errDigestStore{setModeErr: errors.New("db error")}
	b := newErrBot(t, msg, users, searches)
	b.digests = ds

	b.onDigestInterval(context.Background(), 100, cbDigestInterval+"6h")

	last := msg.last()
	if !strings.Contains(last.Text, "Failed to update") {
		t.Errorf("expected failure message, got %q", last.Text)
	}
}

// --- handleCallback edge cases ---

func TestHandleCallback_InaccessibleMessage(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()

	update := &tgmodels.Update{
		CallbackQuery: &tgmodels.CallbackQuery{
			ID:   "cb-1",
			Data: cbConfirm,
			From: tgmodels.User{Username: "alice"},
			Message: tgmodels.MaybeInaccessibleMessage{
				Message: nil,
			},
		},
	}

	tb.bot.handleCallback(ctx, nil, update)
	// Should return early without panicking.
}

func TestHandleCallback_AnswerCallbackError(t *testing.T) {
	msg := &errMessenger{callbackErr: errors.New("callback ack failed")}
	users := &errUserStore{user: &storage.User{ChatID: 100, State: StateIdle, StateData: "{}", Language: "en"}}
	searches := &errSearchStore{}
	b := newErrBot(t, msg, users, searches)

	update := fakeCallback(100, cbConfirm)
	b.handleCallback(context.Background(), nil, update)
	// Should continue despite callback ack failure.
}

// --- handleDefault with GetUser error ---

func TestHandleDefault_GetUserError(t *testing.T) {
	msg := &mockMessenger{}
	users := &errUserStore{getUserErr: errors.New("db error")}
	searches := &errSearchStore{}
	b := newErrBot(t, msg, users, searches)

	update := fakeMessage(100, "some text")
	b.handleDefault(context.Background(), nil, update)
	// Should not panic, no messages sent.
	if len(msg.messages) != 0 {
		t.Errorf("expected no messages on GetUser error, got %d", len(msg.messages))
	}
}

// --- handleStop delete error ---

func TestHandleStop_DeleteError(t *testing.T) {
	msg := &mockMessenger{}
	users := &errUserStore{user: &storage.User{ChatID: 100, State: StateIdle, StateData: "{}", Language: "en"}}
	searches := &errSearchStore{deleteErr: errors.New("db error")}
	b := newErrBot(t, msg, users, searches)

	update := fakeMessage(100, "/stop 1")
	b.handleStop(context.Background(), nil, update)

	last := msg.last()
	if !strings.Contains(last.Text, "Failed to delete") {
		t.Errorf("expected failure message, got %q", last.Text)
	}
}

// --- handlePause SetSearchActive error ---

func TestHandlePause_SetActiveError(t *testing.T) {
	msg := &mockMessenger{}
	users := &errUserStore{user: &storage.User{ChatID: 100, State: StateIdle, StateData: "{}", Language: "en"}}
	searches := &errSearchStore{
		searches:     []storage.Search{{ID: 1, ChatID: 100, Active: true}},
		setActiveErr: errors.New("db error"),
	}
	b := newErrBot(t, msg, users, searches)

	update := fakeMessage(100, "/pause 1")
	b.handlePause(context.Background(), nil, update)

	last := msg.last()
	if !strings.Contains(last.Text, "Failed to pause") {
		t.Errorf("expected failure message, got %q", last.Text)
	}
}

// --- handleResume SetSearchActive error ---

func TestHandleResume_SetActiveError(t *testing.T) {
	msg := &mockMessenger{}
	users := &errUserStore{user: &storage.User{ChatID: 100, State: StateIdle, StateData: "{}", Language: "en"}}
	searches := &errSearchStore{
		searches:     []storage.Search{{ID: 1, ChatID: 100, Active: false}},
		setActiveErr: errors.New("db error"),
	}
	b := newErrBot(t, msg, users, searches)

	update := fakeMessage(100, "/resume 1")
	b.handleResume(context.Background(), nil, update)

	last := msg.last()
	if !strings.Contains(last.Text, "Failed to resume") {
		t.Errorf("expected failure message, got %q", last.Text)
	}
}

// --- onShareCopy CreateSearch error ---

func TestOnShareCopy_CreateSearchError(t *testing.T) {
	msg := &mockMessenger{}
	expires := time.Now().Add(30 * 24 * time.Hour)
	users := &errUserStore{user: &storage.User{ChatID: 200, State: StateIdle, StateData: "{}", Language: "en",
		Tier: TierPremium, TierExpires: expires}}
	searches := &errSearchStore{
		searches:  []storage.Search{{ID: 1, ChatID: 100, Manufacturer: 27, Model: 10332, Source: "yad2", ShareToken: "abc123"}},
		createErr: errors.New("db full"),
	}
	b := newErrBot(t, msg, users, searches)

	b.onShareCopy(context.Background(), 200, cbPrefixShareCopy+"abc123")

	last := msg.last()
	if !strings.Contains(last.Text, "Failed to copy") {
		t.Errorf("expected failure message, got %q", last.Text)
	}
}

// --- SetBot and DefaultHandler coverage ---

func TestSetBot_Nil(t *testing.T) {
	tb := newTestBot(t)
	tb.bot.SetBot(nil)
	if tb.bot.bot != nil {
		t.Error("SetBot(nil) should set bot to nil")
	}
}

func TestDefaultHandler_ReturnsFunction(t *testing.T) {
	tb := newTestBot(t)
	handler := tb.bot.DefaultHandler()
	if handler == nil {
		t.Error("DefaultHandler should return non-nil")
	}

	update := fakeMessage(100, "test")
	tb.createUser(context.Background(), t, 100, "alice")
	handler(context.Background(), nil, update)
	// Should handle without panic.
}

// --- ShareLink function test ---

func TestShareLink_Format(t *testing.T) {
	link := ShareLink("mybot", "abc123def456")
	expected := "https://t.me/mybot?start=share_abc123def456"
	if link != expected {
		t.Errorf("ShareLink = %q, want %q", link, expected)
	}
}

// --- handleShareStart invalid/unknown token ---

func TestHandleShareStart_UnknownToken(t *testing.T) {
	msg := &mockMessenger{}
	users := &errUserStore{user: &storage.User{ChatID: 100, State: StateIdle, StateData: "{}", Language: "en"}}
	searches := &errSearchStore{}
	b := newErrBot(t, msg, users, searches)

	b.handleShareStart(context.Background(), 100, "share_nonexistenttoken")

	last := msg.last()
	if !strings.Contains(last.Text, "not found") {
		t.Errorf("expected 'not found' message, got %q", last.Text)
	}
}

func TestHandleShareStart_ErrorFetchingSearch(t *testing.T) {
	msg := &mockMessenger{}
	users := &errUserStore{user: &storage.User{ChatID: 100, State: StateIdle, StateData: "{}", Language: "en"}}
	searches := &errSearchStore{getErr: errors.New("db error")}
	b := newErrBot(t, msg, users, searches)

	b.handleShareStart(context.Background(), 100, "share_sometoken123")

	last := msg.last()
	if !strings.Contains(last.Text, "not found") {
		t.Errorf("expected 'not found' message, got %q", last.Text)
	}
}

func TestHandleShareStart_WithEngine(t *testing.T) {
	msg := &mockMessenger{}
	users := &errUserStore{user: &storage.User{ChatID: 100, State: StateIdle, StateData: "{}", Language: "en"}}
	searches := &errSearchStore{
		searches: []storage.Search{{
			ID: 1, ChatID: 200, Manufacturer: 27, Model: 10332,
			YearMin: 2020, YearMax: 2024, PriceMax: 100000, EngineMinCC: 2000,
			ShareToken: "testtoken123",
		}},
	}
	b := newErrBot(t, msg, users, searches)

	b.handleShareStart(context.Background(), 100, "share_testtoken123")

	last := msg.last()
	if !strings.Contains(last.Text, "2.0L+") {
		t.Errorf("expected engine size in summary, got %q", last.Text)
	}
	if !last.HasKB {
		t.Error("expected copy button")
	}
}

func TestHandleShareStart_WithoutEngine(t *testing.T) {
	msg := &mockMessenger{}
	users := &errUserStore{user: &storage.User{ChatID: 100, State: StateIdle, StateData: "{}", Language: "en"}}
	searches := &errSearchStore{
		searches: []storage.Search{{
			ID: 1, ChatID: 200, Manufacturer: 27, Model: 10332,
			YearMin: 2020, YearMax: 2024, PriceMax: 100000, EngineMinCC: 0,
			ShareToken: "testtoken456",
		}},
	}
	b := newErrBot(t, msg, users, searches)

	b.handleShareStart(context.Background(), 100, "share_testtoken456")

	last := msg.last()
	if !strings.Contains(last.Text, "Any") {
		t.Errorf("expected 'Any' for zero engine, got %q", last.Text)
	}
}

// --- handleShare GetSearch error ---

func TestHandleShare_GetSearchError(t *testing.T) {
	msg := &mockMessenger{}
	users := &errUserStore{user: &storage.User{ChatID: 100, State: StateIdle, StateData: "{}", Language: "en"}}
	searches := &errSearchStore{getErr: errors.New("db error")}
	b := newErrBot(t, msg, users, searches)

	update := fakeMessage(100, "/share 1")
	b.handleShare(context.Background(), nil, update)

	last := msg.last()
	if !strings.Contains(last.Text, "not found") {
		t.Errorf("expected 'not found' message on GetSearch error, got %q", last.Text)
	}
}

// --- handlePause/Resume GetSearch error ---

func TestHandlePause_GetSearchError(t *testing.T) {
	msg := &mockMessenger{}
	users := &errUserStore{user: &storage.User{ChatID: 100, State: StateIdle, StateData: "{}", Language: "en"}}
	searches := &errSearchStore{getErr: errors.New("db error")}
	b := newErrBot(t, msg, users, searches)

	update := fakeMessage(100, "/pause 1")
	b.handlePause(context.Background(), nil, update)

	last := msg.last()
	if !strings.Contains(last.Text, "not found") {
		t.Errorf("expected 'not found' message, got %q", last.Text)
	}
}

func TestHandleResume_GetSearchError(t *testing.T) {
	msg := &mockMessenger{}
	users := &errUserStore{user: &storage.User{ChatID: 100, State: StateIdle, StateData: "{}", Language: "en"}}
	searches := &errSearchStore{getErr: errors.New("db error")}
	b := newErrBot(t, msg, users, searches)

	update := fakeMessage(100, "/resume 1")
	b.handleResume(context.Background(), nil, update)

	last := msg.last()
	if !strings.Contains(last.Text, "not found") {
		t.Errorf("expected 'not found' message, got %q", last.Text)
	}
}

// --- New constructor coverage ---

func TestNew_DefaultCatalog(t *testing.T) {
	b := New(nil, nil, nil, Config{}, slog.Default())
	if b.catalog == nil {
		t.Error("catalog should default to static when nil")
	}
	if b.maxSearches != 3 {
		t.Errorf("maxSearches = %d, want 3", b.maxSearches)
	}
}

func TestNew_WithCatalog(t *testing.T) {
	cat := catalog.NewStatic()
	b := New(nil, nil, nil, Config{Catalog: cat}, slog.Default())
	if b.catalog != cat {
		t.Error("catalog should use provided instance")
	}
}

// --- handleStats with CountUsers/CountSearches errors ---

func TestHandleStats_Admin_CountErrors(t *testing.T) {
	msg := &mockMessenger{}
	users := &errUserStore{user: &storage.User{ChatID: 999, State: StateIdle, StateData: "{}", Language: "en"}}
	searches := &errSearchStore{countErr: errors.New("db error")}
	b := newErrBot(t, msg, users, searches)

	update := fakeMessage(999, "/stats")
	b.handleStats(context.Background(), nil, update)

	last := msg.last()
	if !strings.Contains(last.Text, "Stats") {
		t.Errorf("should still display stats despite count errors, got %q", last.Text)
	}
}

// --- handleOnSourceSelected coverage (quick smoke) ---

func TestOnSourceSelected_WinWin(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/watch")
	tb.msg.reset()

	tb.simulateCallback(ctx, chatID, cbSourceToggle+"winwin")
	tb.simulateCallback(ctx, chatID, cbSourceDone)

	user, _ := tb.store.GetUser(ctx, chatID)
	if user.State != StateAskManufacturer {
		t.Errorf("state = %q, want %q", user.State, StateAskManufacturer)
	}

	last := tb.msg.last()
	if !last.HasKB {
		t.Error("expected manufacturer keyboard")
	}
}

// --- Wizard with WinWin source creates correct search ---

func TestWizardFlow_WinWin(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	tb.simulateCommand(ctx, chatID, "/watch")
	tb.simulateCallback(ctx, chatID, cbSourceToggle+"winwin")
	tb.simulateCallback(ctx, chatID, cbSourceDone)
	tb.simulateCallback(ctx, chatID, cbPrefixMfr+"27")
	tb.simulateCallback(ctx, chatID, cbPrefixModel+"10332")
	tb.simulateText(ctx, chatID, "2018")
	tb.simulateText(ctx, chatID, "2024")
	tb.simulateText(ctx, chatID, "150000")
	tb.simulateCallback(ctx, chatID, cbPrefixEngine+"2000")
	tb.simulateCallback(ctx, chatID, cbConfirm)

	searches, _ := tb.store.ListSearches(ctx, chatID)
	if len(searches) != 1 {
		t.Fatalf("expected 1 search, got %d", len(searches))
	}
	if searches[0].Source != "winwin" {
		t.Errorf("source = %q, want winwin", searches[0].Source)
	}

	last := tb.msg.last()
	if !strings.Contains(last.Text, "WinWin") {
		t.Errorf("confirm message should mention WinWin, got %q", last.Text)
	}
}

// --- Confirm with empty source defaults to yad2 ---

func TestOnConfirm_EmptySourceDefaultsToYad2(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")

	wd := WizardData{
		Manufacturer: 27, ManufacturerName: "Mazda",
		Model: 10332, ModelName: "3",
		YearMin: 2020, YearMax: 2024, PriceMax: 100000,
	}
	tb.bot.saveWizardState(ctx, chatID, StateConfirm, wd)
	tb.msg.reset()

	tb.simulateCallback(ctx, chatID, cbConfirm)

	searches, _ := tb.store.ListSearches(ctx, chatID)
	if len(searches) != 1 {
		t.Fatalf("expected 1 search, got %d", len(searches))
	}
	if searches[0].Source != "yad2,winwin" {
		t.Errorf("empty source should default to yad2,winwin, got %q", searches[0].Source)
	}
}

// --- Multiple delete callbacks ---

func TestDeleteSearch_TwiceYields404(t *testing.T) {
	tb := newTestBot(t)
	ctx := context.Background()
	const chatID int64 = 100

	tb.createUser(ctx, t, chatID, "alice")
	id, _ := tb.store.CreateSearch(ctx, newFakeSearch(chatID, 27))
	tb.msg.reset()

	tb.simulateCallback(ctx, chatID, cbDeleteSearch+fmt.Sprintf("%d", id))
	msg := tb.msg.last()
	if !strings.Contains(msg.Text, "deleted") {
		t.Fatalf("first delete should succeed, got %q", msg.Text)
	}

	tb.msg.reset()
	tb.simulateCallback(ctx, chatID, cbDeleteSearch+fmt.Sprintf("%d", id))
	msg = tb.msg.last()
	if !strings.Contains(msg.Text, "not found") {
		t.Errorf("second delete should say 'not found', got %q", msg.Text)
	}
}
