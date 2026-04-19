package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"

	"github.com/dsionov/carwatch/internal/fetcher/yad2"
	"github.com/dsionov/carwatch/internal/health"
	"github.com/dsionov/carwatch/internal/storage"
)

type Bot struct {
	bot         *tgbot.Bot
	users       storage.UserStore
	searches    storage.SearchStore
	adminChatID int64
	maxSearches int
	botUsername  string
	logger      *slog.Logger
	health      *health.Status
}

type Config struct {
	AdminChatID int64
	MaxSearches int
	BotUsername  string
	Health      *health.Status
}

func New(b *tgbot.Bot, users storage.UserStore, searches storage.SearchStore, cfg Config, logger *slog.Logger) *Bot {
	if cfg.MaxSearches == 0 {
		cfg.MaxSearches = 3
	}
	return &Bot{
		bot:         b,
		users:       users,
		searches:    searches,
		adminChatID: cfg.AdminChatID,
		maxSearches: cfg.MaxSearches,
		botUsername:  cfg.BotUsername,
		logger:      logger,
		health:      cfg.Health,
	}
}

func (b *Bot) SetBot(tg *tgbot.Bot) {
	b.bot = tg
}

func (b *Bot) DefaultHandler() tgbot.HandlerFunc {
	return b.handleDefault
}

func (b *Bot) RegisterHandlers() {
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/start", tgbot.MatchTypePrefix, b.handleStart)
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/watch", tgbot.MatchTypeExact, b.handleWatch)
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/list", tgbot.MatchTypeExact, b.handleList)
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/stop", tgbot.MatchTypePrefix, b.handleStop)
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/pause", tgbot.MatchTypePrefix, b.handlePause)
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/resume", tgbot.MatchTypePrefix, b.handleResume)
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/share", tgbot.MatchTypePrefix, b.handleShare)
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/cancel", tgbot.MatchTypeExact, b.handleCancel)
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/help", tgbot.MatchTypeExact, b.handleHelp)
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/settings", tgbot.MatchTypeExact, b.handleSettings)
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/stats", tgbot.MatchTypeExact, b.handleStats)
	b.bot.RegisterHandler(tgbot.HandlerTypeCallbackQueryData, "", tgbot.MatchTypePrefix, b.handleCallback)
}

func (b *Bot) ensureUser(ctx context.Context, chatID int64, username string) {
	_ = b.users.UpsertUser(ctx, chatID, username)
}

func (b *Bot) send(ctx context.Context, chatID int64, text string) {
	_, _ = b.bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: tgmodels.ParseModeMarkdown,
	})
}

func (b *Bot) sendWithKeyboard(ctx context.Context, chatID int64, text string, kb *tgmodels.InlineKeyboardMarkup) {
	_, _ = b.bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   tgmodels.ParseModeMarkdown,
		ReplyMarkup: kb,
	})
}

// --- Command Handlers ---

func (b *Bot) handleStart(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	username := update.Message.From.Username
	b.ensureUser(ctx, chatID, username)

	// Check for deep-link parameter: /start share_123
	parts := strings.Fields(update.Message.Text)
	if len(parts) == 2 && strings.HasPrefix(parts[1], "share_") {
		b.handleShareStart(ctx, chatID, parts[1])
		return
	}

	b.send(ctx, chatID,
		"Welcome to *CarWatch*! I monitor car listings on Yad2 and WinWin and send you alerts when new matches appear.\n\n"+
			"Use /watch to set up a new car search.\n"+
			"Use /list to see your active searches.\n"+
			"Use /help for all commands.")
}

func (b *Bot) handleShare(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	b.ensureUser(ctx, chatID, update.Message.From.Username)

	if b.botUsername == "" {
		b.send(ctx, chatID, "Sharing is not configured. Bot username is missing.")
		return
	}

	parts := strings.Fields(update.Message.Text)
	if len(parts) < 2 {
		b.send(ctx, chatID, "Usage: /share <search\\_id>\nUse /list to see your search IDs.")
		return
	}

	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		b.send(ctx, chatID, "Invalid search ID. Use /list to see your searches.")
		return
	}

	search, err := b.searches.GetSearch(ctx, id)
	if err != nil || search == nil || search.ChatID != chatID {
		b.send(ctx, chatID, "Search not found. Use /list to see your searches.")
		return
	}

	link := ShareLink(b.botUsername, search.ID)
	mfr := yad2.ManufacturerName(search.Manufacturer)
	mdl := yad2.ModelName(search.Manufacturer, search.Model)

	b.send(ctx, chatID, fmt.Sprintf(
		"Share this link for *%s %s* search:\n\n%s",
		mfr, mdl, link))
}

// ShareLink returns a Telegram deep link for sharing a search.
func ShareLink(botUsername string, searchID int64) string {
	return fmt.Sprintf("https://t.me/%s?start=share_%d", botUsername, searchID)
}

func (b *Bot) handleShareStart(ctx context.Context, chatID int64, param string) {
	idStr := strings.TrimPrefix(param, "share_")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		b.send(ctx, chatID, "Invalid share link.")
		return
	}

	search, err := b.searches.GetSearch(ctx, id)
	if err != nil || search == nil {
		b.send(ctx, chatID, "The shared search was not found. It may have been deleted.")
		return
	}

	mfr := yad2.ManufacturerName(search.Manufacturer)
	mdl := yad2.ModelName(search.Manufacturer, search.Model)

	engineStr := "Any"
	if search.EngineMinCC > 0 {
		engineStr = fmt.Sprintf("%.1fL+", float64(search.EngineMinCC)/1000)
	}

	summary := fmt.Sprintf(
		"*Shared search:*\n"+
			"Car: %s %s\n"+
			"Year: %d\u2013%d\n"+
			"Max price: %s NIS\n"+
			"Engine: %s\n\n"+
			"Copy this search to start receiving alerts?",
		mfr, mdl, search.YearMin, search.YearMax,
		formatNumber(search.PriceMax), engineStr)

	kb := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{{
			{Text: "Copy this search", CallbackData: cbPrefixShareCopy + strconv.FormatInt(id, 10)},
		}},
	}

	b.sendWithKeyboard(ctx, chatID, summary, kb)
}

func (b *Bot) handleWatch(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	b.ensureUser(ctx, chatID, update.Message.From.Username)

	count, _ := b.searches.CountSearches(ctx, chatID)
	if count >= int64(b.maxSearches) {
		b.send(ctx, chatID, fmt.Sprintf(
			"You already have %d active searches (max %d). Use /stop to remove one first.",
			count, b.maxSearches))
		return
	}

	_ = b.users.UpdateUserState(ctx, chatID, StateAskSource, "{}")
	b.sendWithKeyboard(ctx, chatID,
		"Which marketplace do you want to search?",
		sourceKeyboard())
}

func (b *Bot) handleList(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	b.ensureUser(ctx, chatID, update.Message.From.Username)

	searches, err := b.searches.ListSearches(ctx, chatID)
	if err != nil {
		b.send(ctx, chatID, "Failed to load searches. Please try again.")
		return
	}

	if len(searches) == 0 {
		b.send(ctx, chatID, "You have no active searches. Use /watch to create one.")
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("*Your searches (%d):*\n\n", len(searches)))

	var buttons [][]tgmodels.InlineKeyboardButton
	for _, s := range searches {
		prefix := ""
		if !s.Active {
			prefix = "\u23f8 "
		}
		mfr := yad2.ManufacturerName(s.Manufacturer)
		mdl := yad2.ModelName(s.Manufacturer, s.Model)

		status := "active"
		if !s.Active {
			status = "paused"
		}

		src := sourceDisplayName(s.Source)
		sb.WriteString(fmt.Sprintf(
			"%s#%d [%s] %s %s (%d\u2013%d, max %s NIS) [%s]\n",
			prefix, s.ID, src, mfr, mdl, s.YearMin, s.YearMax, formatNumber(s.PriceMax), status))

		buttons = append(buttons, []tgmodels.InlineKeyboardButton{{
			Text:         fmt.Sprintf("Delete #%d", s.ID),
			CallbackData: cbDeleteSearch + strconv.FormatInt(s.ID, 10),
		}})
	}

	kb := &tgmodels.InlineKeyboardMarkup{InlineKeyboard: buttons}
	b.sendWithKeyboard(ctx, chatID, sb.String(), kb)
}

func (b *Bot) handleStop(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	parts := strings.Fields(update.Message.Text)
	if len(parts) < 2 {
		b.send(ctx, chatID, "Usage: /stop <search_id>\nUse /list to see your search IDs.")
		return
	}

	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		b.send(ctx, chatID, "Invalid search ID. Use /list to see your searches.")
		return
	}

	if err := b.searches.DeleteSearch(ctx, id, chatID); err != nil {
		b.send(ctx, chatID, "Failed to delete search.")
		return
	}

	b.send(ctx, chatID, fmt.Sprintf("Search #%d deleted.", id))
}

func (b *Bot) handlePause(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	parts := strings.Fields(update.Message.Text)
	if len(parts) < 2 {
		b.send(ctx, chatID, "Usage: /pause <search_id>\nUse /list to see your search IDs.")
		return
	}

	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		b.send(ctx, chatID, "Invalid search ID. Use /list to see your searches.")
		return
	}

	search, err := b.searches.GetSearch(ctx, id)
	if err != nil || search == nil || search.ChatID != chatID {
		b.send(ctx, chatID, "Search not found. Use /list to see your searches.")
		return
	}

	if !search.Active {
		b.send(ctx, chatID, fmt.Sprintf("Search #%d is already paused.", id))
		return
	}

	if err := b.searches.SetSearchActive(ctx, id, false); err != nil {
		b.send(ctx, chatID, "Failed to pause search.")
		return
	}

	b.send(ctx, chatID, fmt.Sprintf("Search #%d paused. Use /resume %d to resume it.", id, id))
}

func (b *Bot) handleResume(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	parts := strings.Fields(update.Message.Text)
	if len(parts) < 2 {
		b.send(ctx, chatID, "Usage: /resume <search_id>\nUse /list to see your search IDs.")
		return
	}

	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		b.send(ctx, chatID, "Invalid search ID. Use /list to see your searches.")
		return
	}

	search, err := b.searches.GetSearch(ctx, id)
	if err != nil || search == nil || search.ChatID != chatID {
		b.send(ctx, chatID, "Search not found. Use /list to see your searches.")
		return
	}

	if search.Active {
		b.send(ctx, chatID, fmt.Sprintf("Search #%d is already active.", id))
		return
	}

	if err := b.searches.SetSearchActive(ctx, id, true); err != nil {
		b.send(ctx, chatID, "Failed to resume search.")
		return
	}

	b.send(ctx, chatID, fmt.Sprintf("Search #%d resumed.", id))
}

func (b *Bot) handleCancel(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	_ = b.users.UpdateUserState(ctx, chatID, StateIdle, "{}")
	b.send(ctx, chatID, "Cancelled. Use /watch to start a new search.")
}

func (b *Bot) handleHelp(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	b.send(ctx, update.Message.Chat.ID,
		"*CarWatch Commands:*\n\n"+
			"/watch — Set up a new car search\n"+
			"/list — Show your active searches\n"+
			"/pause <id> — Pause a search\n"+
			"/resume <id> — Resume a paused search\n"+
			"/stop <id> — Delete a search\n"+
			"/share <id> — Share a search via link\n"+
			"/settings — View your current limits\n"+
			"/cancel — Cancel current wizard\n"+
			"/help — Show this message")
}

func (b *Bot) handleSettings(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	b.ensureUser(ctx, chatID, update.Message.From.Username)

	count, _ := b.searches.CountSearches(ctx, chatID)
	b.send(ctx, chatID, fmt.Sprintf(
		"*Your settings:*\nActive searches: %d/%d", count, b.maxSearches))
}

func (b *Bot) handleStats(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	if chatID != b.adminChatID {
		b.send(ctx, chatID, "Unknown command. Use /help for available commands.")
		return
	}

	users, _ := b.users.CountUsers(ctx)
	searches, _ := b.searches.CountAllSearches(ctx)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(
		"*CarWatch Stats:*\nUsers: %d\nActive searches: %d", users, searches))

	if b.health != nil {
		snap := b.health.Snapshot()
		sb.WriteString(fmt.Sprintf("\n\n*Health:*\nStatus: %s\nUptime: %s\nCycles: %v\nErrors: %v",
			snap["status"], snap["uptime"], snap["cycles"], snap["errors"]))
		sb.WriteString(fmt.Sprintf("\nListings found: %v\nNotifications sent: %v",
			snap["listings_found"], snap["notifications_sent"]))
	}

	b.send(ctx, chatID, sb.String())
}

// --- Callback Handler ---

func (b *Bot) handleCallback(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	if update.CallbackQuery == nil {
		return
	}

	chatID := update.CallbackQuery.Message.Message.Chat.ID
	data := update.CallbackQuery.Data

	_, _ = b.bot.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
	})

	switch {
	case strings.HasPrefix(data, cbPrefixSource):
		b.onSourceSelected(ctx, chatID, data)
	case strings.HasPrefix(data, cbPrefixMfr):
		b.onManufacturerSelected(ctx, chatID, data)
	case strings.HasPrefix(data, cbPrefixModel):
		b.onModelSelected(ctx, chatID, data)
	case strings.HasPrefix(data, cbPrefixEngine):
		b.onEngineSelected(ctx, chatID, data)
	case data == cbConfirm:
		b.onConfirm(ctx, chatID)
	case data == cbEdit:
		b.onEdit(ctx, chatID)
	case data == cbCancel:
		b.onCancelCallback(ctx, chatID)
	case strings.HasPrefix(data, cbDeleteSearch):
		b.onDeleteSearch(ctx, chatID, data)
	case strings.HasPrefix(data, cbPrefixShareCopy):
		b.onShareCopy(ctx, chatID, data)
	}
}

func (b *Bot) onSourceSelected(ctx context.Context, chatID int64, data string) {
	source := strings.TrimPrefix(data, cbPrefixSource)
	wd := WizardData{Source: source}
	b.saveWizardState(ctx, chatID, StateAskManufacturer, wd)

	b.sendWithKeyboard(ctx, chatID,
		"What manufacturer are you looking for?",
		manufacturerKeyboard())
}

func (b *Bot) onManufacturerSelected(ctx context.Context, chatID int64, data string) {
	idStr := strings.TrimPrefix(data, cbPrefixMfr)
	id, _ := strconv.Atoi(idStr)

	wd := b.loadWizardData(ctx, chatID)
	wd.Manufacturer = id
	wd.ManufacturerName = yad2.ManufacturerName(id)
	b.saveWizardState(ctx, chatID, StateAskModel, wd)

	models := yad2.Models(id)
	if len(models) == 0 {
		b.send(ctx, chatID, "No models found for this manufacturer. Use /cancel to start over.")
		return
	}

	b.sendWithKeyboard(ctx, chatID,
		fmt.Sprintf("Which %s model?", wd.ManufacturerName),
		modelKeyboard(id))
}

func (b *Bot) onModelSelected(ctx context.Context, chatID int64, data string) {
	idStr := strings.TrimPrefix(data, cbPrefixModel)
	modelID, _ := strconv.Atoi(idStr)

	wd := b.loadWizardData(ctx, chatID)
	wd.Model = modelID
	wd.ModelName = yad2.ModelName(wd.Manufacturer, modelID)
	b.saveWizardState(ctx, chatID, StateAskYearMin, wd)

	b.send(ctx, chatID, "From which year? (e.g. 2018)")
}

func (b *Bot) onEngineSelected(ctx context.Context, chatID int64, data string) {
	ccStr := strings.TrimPrefix(data, cbPrefixEngine)
	cc, _ := strconv.Atoi(ccStr)

	wd := b.loadWizardData(ctx, chatID)
	wd.EngineMinCC = cc
	b.saveWizardState(ctx, chatID, StateConfirm, wd)

	kb, summary := confirmKeyboard(wd)
	b.sendWithKeyboard(ctx, chatID, summary, kb)
}

func (b *Bot) onConfirm(ctx context.Context, chatID int64) {
	wd := b.loadWizardData(ctx, chatID)

	source := wd.Source
	if source == "" {
		source = "yad2"
	}

	name := fmt.Sprintf("%s-%s", strings.ToLower(wd.ManufacturerName), strings.ToLower(wd.ModelName))
	id, err := b.searches.CreateSearch(ctx, storage.Search{
		ChatID:       chatID,
		Name:         name,
		Source:       source,
		Manufacturer: wd.Manufacturer,
		Model:        wd.Model,
		YearMin:      wd.YearMin,
		YearMax:      wd.YearMax,
		PriceMax:     wd.PriceMax,
		EngineMinCC:  wd.EngineMinCC,
	})
	if err != nil {
		b.logger.Error("create search failed", "error", err)
		b.send(ctx, chatID, "Failed to save search. Please try again.")
		return
	}

	_ = b.users.UpdateUserState(ctx, chatID, StateIdle, "{}")
	b.send(ctx, chatID, fmt.Sprintf(
		"Search #%d saved! I'll check %s every 15 minutes and send you new listings.\n\nUse /list to see your searches.",
		id, sourceDisplayName(source)))
}

func (b *Bot) onEdit(ctx context.Context, chatID int64) {
	_ = b.users.UpdateUserState(ctx, chatID, StateAskSource, "{}")
	b.sendWithKeyboard(ctx, chatID,
		"Let's start over. Which marketplace?",
		sourceKeyboard())
}

func (b *Bot) onCancelCallback(ctx context.Context, chatID int64) {
	_ = b.users.UpdateUserState(ctx, chatID, StateIdle, "{}")
	b.send(ctx, chatID, "Search cancelled.")
}

func (b *Bot) onDeleteSearch(ctx context.Context, chatID int64, data string) {
	idStr := strings.TrimPrefix(data, cbDeleteSearch)
	id, _ := strconv.ParseInt(idStr, 10, 64)

	if err := b.searches.DeleteSearch(ctx, id, chatID); err != nil {
		b.send(ctx, chatID, "Failed to delete search.")
		return
	}
	b.send(ctx, chatID, fmt.Sprintf("Search #%d deleted.", id))
}

func (b *Bot) onShareCopy(ctx context.Context, chatID int64, data string) {
	idStr := strings.TrimPrefix(data, cbPrefixShareCopy)
	srcID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		b.send(ctx, chatID, "Invalid share link.")
		return
	}

	src, err := b.searches.GetSearch(ctx, srcID)
	if err != nil || src == nil {
		b.send(ctx, chatID, "The shared search was not found. It may have been deleted.")
		return
	}

	// Enforce per-user search limit.
	count, _ := b.searches.CountSearches(ctx, chatID)
	if count >= int64(b.maxSearches) {
		b.send(ctx, chatID, fmt.Sprintf(
			"You already have %d active searches (max %d). Use /stop to remove one first.",
			count, b.maxSearches))
		return
	}

	mfr := yad2.ManufacturerName(src.Manufacturer)
	mdl := yad2.ModelName(src.Manufacturer, src.Model)
	name := fmt.Sprintf("%s-%s", strings.ToLower(mfr), strings.ToLower(mdl))

	newID, err := b.searches.CreateSearch(ctx, storage.Search{
		ChatID:       chatID,
		Name:         name,
		Manufacturer: src.Manufacturer,
		Model:        src.Model,
		YearMin:      src.YearMin,
		YearMax:      src.YearMax,
		PriceMax:     src.PriceMax,
		EngineMinCC:  src.EngineMinCC,
		MaxKm:        src.MaxKm,
		MaxHand:      src.MaxHand,
	})
	if err != nil {
		b.logger.Error("clone search failed", "error", err)
		b.send(ctx, chatID, "Failed to copy search. Please try again.")
		return
	}

	b.send(ctx, chatID, fmt.Sprintf(
		"Search #%d saved! I'll check Yad2 every 15 minutes and send you new listings.\n\nUse /list to see your searches.",
		newID))
}

// --- Default Handler (free text during wizard) ---

func (b *Bot) handleDefault(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	b.ensureUser(ctx, chatID, update.Message.From.Username)

	user, err := b.users.GetUser(ctx, chatID)
	if err != nil || user == nil {
		return
	}

	text := strings.TrimSpace(update.Message.Text)

	switch user.State {
	case StateAskYearMin:
		b.handleYearMin(ctx, chatID, text)
	case StateAskYearMax:
		b.handleYearMax(ctx, chatID, text)
	case StateAskPriceMax:
		b.handlePriceMax(ctx, chatID, text)
	default:
		if text != "" && !strings.HasPrefix(text, "/") {
			b.send(ctx, chatID, "I didn't understand that. Use /help for available commands.")
		}
	}
}

func (b *Bot) handleYearMin(ctx context.Context, chatID int64, text string) {
	year, err := strconv.Atoi(text)
	if err != nil || year < 1990 || year > 2030 {
		b.send(ctx, chatID, "Please enter a valid year (1990–2030).")
		return
	}

	wd := b.loadWizardData(ctx, chatID)
	wd.YearMin = year
	b.saveWizardState(ctx, chatID, StateAskYearMax, wd)
	b.send(ctx, chatID, "Until which year? (e.g. 2024)")
}

func (b *Bot) handleYearMax(ctx context.Context, chatID int64, text string) {
	year, err := strconv.Atoi(text)
	if err != nil || year < 1990 || year > 2030 {
		b.send(ctx, chatID, "Please enter a valid year (1990–2030).")
		return
	}

	wd := b.loadWizardData(ctx, chatID)
	if year < wd.YearMin {
		b.send(ctx, chatID, fmt.Sprintf("Must be >= %d. Try again.", wd.YearMin))
		return
	}
	wd.YearMax = year
	b.saveWizardState(ctx, chatID, StateAskPriceMax, wd)
	b.send(ctx, chatID, "Max price in NIS? (e.g. 150000)")
}

func (b *Bot) handlePriceMax(ctx context.Context, chatID int64, text string) {
	text = strings.ReplaceAll(text, ",", "")
	price, err := strconv.Atoi(text)
	if err != nil || price < 1000 || price > 10000000 {
		b.send(ctx, chatID, "Please enter a valid price (1,000–10,000,000).")
		return
	}

	wd := b.loadWizardData(ctx, chatID)
	wd.PriceMax = price
	b.saveWizardState(ctx, chatID, StateAskEngine, wd)
	b.sendWithKeyboard(ctx, chatID, "Minimum engine size?", engineKeyboard())
}

// --- Wizard State Helpers ---

func (b *Bot) loadWizardData(ctx context.Context, chatID int64) WizardData {
	user, err := b.users.GetUser(ctx, chatID)
	if err != nil || user == nil {
		return WizardData{}
	}

	var wd WizardData
	_ = json.Unmarshal([]byte(user.StateData), &wd)
	return wd
}

func (b *Bot) saveWizardState(ctx context.Context, chatID int64, state string, wd WizardData) {
	data, _ := json.Marshal(wd)
	_ = b.users.UpdateUserState(ctx, chatID, state, string(data))
}
