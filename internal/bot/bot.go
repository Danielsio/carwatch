package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"

	"github.com/dsionov/carwatch/internal/catalog"
	"github.com/dsionov/carwatch/internal/health"
	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/storage"
)

type PollTrigger interface {
	TriggerPoll()
}

type Bot struct {
	bot          *tgbot.Bot
	msg          messenger
	users        storage.UserStore
	searches     storage.SearchStore
	listings     storage.ListingStore
	digests      storage.DigestStore
	saved        storage.SavedListingStore
	hidden       storage.HiddenListingStore
	dailyDigests storage.DailyDigestStore
	catalog      catalog.Catalog
	adminChatID  int64
	maxSearches  int
	botUsername   string
	pollInterval time.Duration
	logger       *slog.Logger
	health       *health.Status
	chatMu       sync.Map
	pollTrigger  PollTrigger
	rateLimiter  sync.Map
}

type chatMuEntry struct {
	mu       sync.Mutex
	lastUsed atomic.Int64
}

type userRateLimit struct {
	mu       sync.Mutex
	tokens   int
	lastTick time.Time
	lastSeen atomic.Int64
}

const (
	rateLimitBurst    = 10
	rateLimitInterval = time.Second
)

func (b *Bot) isRateLimited(chatID int64) bool {
	v, _ := b.rateLimiter.LoadOrStore(chatID, &userRateLimit{
		tokens:   rateLimitBurst,
		lastTick: time.Now(),
	})
	rl := v.(*userRateLimit)
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastTick)
	refill := int(elapsed / rateLimitInterval)
	if refill > 0 {
		rl.tokens = min(rl.tokens+refill, rateLimitBurst)
		rl.lastTick = rl.lastTick.Add(time.Duration(refill) * rateLimitInterval)
	}

	if rl.tokens <= 0 {
		return true
	}
	rl.tokens--
	rl.lastSeen.Store(now.UnixNano())
	return false
}

type Config struct {
	AdminChatID    int64
	MaxSearches    int
	BotUsername     string
	PollInterval   time.Duration
	Health         *health.Status
	Digests        storage.DigestStore
	Listings       storage.ListingStore
	Saved          storage.SavedListingStore
	Hidden         storage.HiddenListingStore
	DailyDigests   storage.DailyDigestStore
	Catalog        catalog.Catalog
}

func New(b *tgbot.Bot, users storage.UserStore, searches storage.SearchStore, cfg Config, logger *slog.Logger) *Bot {
	if cfg.MaxSearches == 0 {
		cfg.MaxSearches = defaultMaxSearches
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 15 * time.Minute
	}
	cat := cfg.Catalog
	if cat == nil {
		cat = catalog.NewStatic()
	}
	var msg messenger
	if b != nil {
		msg = &telegramMessenger{bot: b}
	}
	return &Bot{
		bot:          b,
		msg:          msg,
		users:        users,
		searches:     searches,
		listings:     cfg.Listings,
		digests:      cfg.Digests,
		saved:        cfg.Saved,
		hidden:       cfg.Hidden,
		dailyDigests: cfg.DailyDigests,
		catalog:      cat,
		adminChatID:  cfg.AdminChatID,
		maxSearches:  cfg.MaxSearches,
		botUsername:   cfg.BotUsername,
		pollInterval: cfg.PollInterval,
		logger:       logger,
		health:       cfg.Health,
	}
}

func (b *Bot) SetPollTrigger(pt PollTrigger) {
	b.pollTrigger = pt
}

func (b *Bot) SetBot(tg *tgbot.Bot) {
	b.bot = tg
	if tg != nil {
		b.msg = &telegramMessenger{bot: tg}
	}
}

func (b *Bot) DefaultHandler() tgbot.HandlerFunc {
	return b.handleDefault
}

func (b *Bot) rateLimited(next tgbot.HandlerFunc) tgbot.HandlerFunc {
	return func(ctx context.Context, bot *tgbot.Bot, update *tgmodels.Update) {
		var chatID int64
		if update.Message != nil {
			chatID = update.Message.Chat.ID
		} else if update.CallbackQuery != nil && update.CallbackQuery.Message.Message != nil {
			chatID = update.CallbackQuery.Message.Message.Chat.ID
		}
		if chatID != 0 && b.isRateLimited(chatID) {
			b.logger.Warn("rate limited", "chat_id", chatID)
			if update.CallbackQuery != nil {
				if err := b.msg.AnswerCallback(ctx, update.CallbackQuery.ID); err != nil {
					b.logger.Error("answer callback query failed", "chat_id", chatID, "error", err)
				}
			}
			return
		}
		next(ctx, bot, update)
	}
}

func (b *Bot) RegisterHandlers() {
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/start", tgbot.MatchTypePrefix, b.rateLimited(b.handleStart))
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/watch", tgbot.MatchTypeExact, b.rateLimited(b.handleWatch))
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/list", tgbot.MatchTypeExact, b.rateLimited(b.handleList))
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/stop", tgbot.MatchTypePrefix, b.rateLimited(b.handleStop))
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/pause", tgbot.MatchTypePrefix, b.rateLimited(b.handlePause))
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/resume", tgbot.MatchTypePrefix, b.rateLimited(b.handleResume))
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/share", tgbot.MatchTypePrefix, b.rateLimited(b.handleShare))
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/cancel", tgbot.MatchTypeExact, b.rateLimited(b.handleCancel))
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/help", tgbot.MatchTypeExact, b.rateLimited(b.handleHelp))
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/history", tgbot.MatchTypeExact, b.rateLimited(b.handleHistory))
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/digest", tgbot.MatchTypeExact, b.rateLimited(b.handleDigest))
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/settings", tgbot.MatchTypeExact, b.rateLimited(b.handleSettings))
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/stats", tgbot.MatchTypeExact, b.rateLimited(b.handleStats))
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/language", tgbot.MatchTypeExact, b.rateLimited(b.handleLanguage))
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/edit", tgbot.MatchTypePrefix, b.rateLimited(b.handleEdit))
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/saved", tgbot.MatchTypeExact, b.rateLimited(b.handleSaved))
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/hidden", tgbot.MatchTypeExact, b.rateLimited(b.handleHidden))
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/upgrade", tgbot.MatchTypeExact, b.rateLimited(b.handleUpgrade))
	b.bot.RegisterHandler(tgbot.HandlerTypeCallbackQueryData, "", tgbot.MatchTypePrefix, b.rateLimited(b.handleCallback))
}

func (b *Bot) ensureUser(ctx context.Context, chatID int64, username string) {
	if err := b.users.UpsertUser(ctx, chatID, username); err != nil {
		b.logger.Error("upsert user failed", "chat_id", chatID, "username", username, "error", err)
	}
}

func (b *Bot) send(ctx context.Context, chatID int64, text string) {
	b.logger.Debug("sending message", "chat_id", chatID, "text_len", len(text))
	if err := b.msg.SendMessage(ctx, chatID, text, "", nil); err != nil {
		b.logger.Error("send message failed", "chat_id", chatID, "error", err)
	}
}

func (b *Bot) sendMarkdown(ctx context.Context, chatID int64, text string) {
	b.logger.Debug("sending markdown message", "chat_id", chatID, "text_len", len(text))
	if err := b.msg.SendMessage(ctx, chatID, text, "Markdown", nil); err != nil {
		b.logger.Error("send markdown message failed", "chat_id", chatID, "error", err)
	}
}

func (b *Bot) sendWithKeyboard(ctx context.Context, chatID int64, text string, kb *tgmodels.InlineKeyboardMarkup) {
	buttonCount := 0
	for _, row := range kb.InlineKeyboard {
		buttonCount += len(row)
	}
	b.logger.Debug("sending message with keyboard", "chat_id", chatID, "text_len", len(text), "buttons", buttonCount)
	if err := b.msg.SendMessage(ctx, chatID, text, "Markdown", kb); err != nil {
		b.logger.Error("send message with keyboard failed", "chat_id", chatID, "buttons", buttonCount, "error", err)
	}
}

func (b *Bot) lockChat(chatID int64) func() {
	v, _ := b.chatMu.LoadOrStore(chatID, &chatMuEntry{})
	entry := v.(*chatMuEntry)
	entry.mu.Lock()
	entry.lastUsed.Store(time.Now().UnixNano())
	return entry.mu.Unlock
}

const (
	cleanupInterval = 1 * time.Hour
	staleThreshold  = 1 * time.Hour
)

func (b *Bot) StartCleanup(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				b.sweepStaleMaps()
			}
		}
	}()
}

func (b *Bot) sweepStaleMaps() {
	cutoff := time.Now().Add(-staleThreshold).UnixNano()

	b.rateLimiter.Range(func(key, value any) bool {
		rl := value.(*userRateLimit)
		seen := rl.lastSeen.Load()
		if seen > 0 && seen < cutoff {
			b.rateLimiter.Delete(key)
		}
		return true
	})

	b.chatMu.Range(func(key, value any) bool {
		entry := value.(*chatMuEntry)
		used := entry.lastUsed.Load()
		if used > 0 && used < cutoff {
			b.chatMu.Delete(key)
		}
		return true
	})
}

func (b *Bot) formatInterval() string {
	if b.pollInterval < time.Minute {
		return b.pollInterval.Round(time.Second).String()
	}
	m := int(b.pollInterval.Minutes())
	if m < 60 {
		return fmt.Sprintf("%d minutes", m)
	}
	h := m / 60
	if m%60 == 0 {
		if h == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", h)
	}
	return b.pollInterval.String()
}

func (b *Bot) modelDisplayName(manufacturerID, modelID int) string {
	if modelID == 0 {
		return "Any model"
	}
	return b.catalog.ModelName(manufacturerID, modelID)
}

func (b *Bot) getUserLang(ctx context.Context, chatID int64) locale.Lang {
	user, err := b.users.GetUser(ctx, chatID)
	if err != nil || user == nil || user.Language == "" {
		return locale.Hebrew
	}
	return locale.Lang(user.Language)
}

const (
	TierFree    = "free"
	TierPremium = "premium"

	defaultMaxSearches = 10
)

func (b *Bot) maxSearchesForUser(_ context.Context, _ int64) int {
	if b.maxSearches > 0 {
		return b.maxSearches
	}
	return defaultMaxSearches
}

func (b *Bot) checkSearchLimit(ctx context.Context, chatID int64, lang locale.Lang, limitKey string) bool {
	count, err := b.searches.CountSearches(ctx, chatID)
	if err != nil {
		b.logger.Error("count searches failed", "chat_id", chatID, "error", err)
		b.send(ctx, chatID, locale.T(lang, limitKey+"_error"))
		return true
	}
	limit := b.maxSearchesForUser(ctx, chatID)
	if count >= int64(limit) {
		b.send(ctx, chatID, locale.Tf(lang, limitKey+"_reached", count, limit))
		return true
	}
	return false
}

// --- Wizard State Helpers ---

func (b *Bot) loadWizardData(ctx context.Context, chatID int64) WizardData {
	user, err := b.users.GetUser(ctx, chatID)
	if err != nil {
		b.logger.Error("load wizard data: get user failed", "chat_id", chatID, "error", err)
		return WizardData{}
	}
	if user == nil {
		b.logger.Warn("load wizard data: user not found", "chat_id", chatID)
		return WizardData{}
	}

	var wd WizardData
	if err := json.Unmarshal([]byte(user.StateData), &wd); err != nil {
		b.logger.Error("load wizard data: unmarshal failed", "chat_id", chatID, "state_data", user.StateData, "error", err)
		return WizardData{}
	}
	b.logger.Debug("loaded wizard data", "chat_id", chatID, "state", user.State, "data", wd)
	return wd
}

func (b *Bot) saveWizardState(ctx context.Context, chatID int64, state string, wd WizardData) {
	data, err := json.Marshal(wd)
	if err != nil {
		b.logger.Error("save wizard state: marshal failed", "chat_id", chatID, "error", err)
		return
	}
	b.logger.Debug("saving wizard state", "chat_id", chatID, "state", state, "data", string(data))
	if err := b.users.UpdateUserState(ctx, chatID, state, string(data)); err != nil {
		b.logger.Error("save wizard state: update failed", "chat_id", chatID, "state", state, "error", err)
	}
}
