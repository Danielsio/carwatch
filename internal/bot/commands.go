package bot

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"

	"github.com/dsionov/carwatch/internal/format"
	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/storage"
)

// --- Command Handlers ---

func (b *Bot) handleStart(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	username := update.Message.From.Username

	b.ensureUser(ctx, chatID, username)

	parts := strings.Fields(update.Message.Text)
	if len(parts) == 2 && strings.HasPrefix(parts[1], "share_") {
		b.handleShareStart(ctx, chatID, parts[1])
		return
	}
	if len(parts) == 2 && strings.HasPrefix(parts[1], "link_") {
		b.handleLinkStart(ctx, chatID, parts[1])
		return
	}

	lang := b.getUserLang(ctx, chatID)

	kb := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: locale.T(lang, "btn_quick_start"), CallbackData: cbQuickStart},
				{Text: locale.T(lang, "btn_custom"), CallbackData: "watch"},
			},
		},
	}

	if err := b.msg.SendPhoto(ctx, chatID, "carwatch-logo.png",
		locale.T(lang, "onboarding_welcome"), "Markdown", kb); err != nil {
		b.logger.Warn("sendPhoto failed, falling back to text", "error", err)
		b.sendWithKeyboard(ctx, chatID, locale.T(lang, "onboarding_welcome"), kb)
	}
}

func (b *Bot) handleShare(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	b.ensureUser(ctx, chatID, update.Message.From.Username)
	lang := b.getUserLang(ctx, chatID)

	if b.botUsername == "" {
		b.send(ctx, chatID, locale.T(lang, "share_not_configured"))
		return
	}

	parts := strings.Fields(update.Message.Text)
	if len(parts) < 2 {
		b.sendMarkdown(ctx, chatID, locale.T(lang, "share_usage"))
		return
	}

	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		b.send(ctx, chatID, locale.T(lang, "share_invalid"))
		return
	}

	search, err := b.searches.GetSearch(ctx, id)
	if err != nil || search == nil || search.ChatID != chatID {
		b.send(ctx, chatID, locale.T(lang, "share_not_found"))
		return
	}

	link := ShareLink(b.botUsername, search.ShareToken)
	mfr := b.catalog.ManufacturerName(search.Manufacturer)
	mdl := b.modelDisplayName(search.Manufacturer, search.Model)

	b.sendMarkdown(ctx, chatID, locale.Tf(lang, "share_link", mfr, mdl, link))
}

func ShareLink(botUsername string, shareToken string) string {
	return fmt.Sprintf("https://t.me/%s?start=share_%s", botUsername, shareToken)
}

func (b *Bot) handleShareStart(ctx context.Context, chatID int64, param string) {
	lang := b.getUserLang(ctx, chatID)

	token := strings.TrimPrefix(param, "share_")
	if len(token) == 0 || len(token) > 64 {
		b.send(ctx, chatID, locale.T(lang, "share_invalid_link"))
		return
	}

	search, err := b.searches.GetSearchByShareToken(ctx, token)
	if err != nil || search == nil {
		b.send(ctx, chatID, locale.T(lang, "share_search_deleted"))
		return
	}

	mfr := b.catalog.ManufacturerName(search.Manufacturer)
	mdl := b.modelDisplayName(search.Manufacturer, search.Model)

	engineStr := locale.T(lang, "label_any")
	if search.EngineMinCC > 0 {
		engineStr = fmt.Sprintf("%.1fL+", float64(search.EngineMinCC)/1000)
	}

	summary := locale.Tf(lang, "share_summary",
		mfr, mdl, search.YearMin, search.YearMax,
		format.Number(search.PriceMax), engineStr)

	kb := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{{
			{Text: locale.T(lang, "share_copy_btn"), CallbackData: cbPrefixShareCopy + search.ShareToken},
		}},
	}

	b.sendWithKeyboard(ctx, chatID, summary, kb)
}

func (b *Bot) handleLinkStart(ctx context.Context, telegramChatID int64, param string) {
	if b.linkTokens == nil {
		b.logger.Warn("link deep link: link token store not configured")
		b.send(ctx, telegramChatID, "❌ הקישור פג תוקף. נסה שוב מהאתר.")
		return
	}

	token := strings.TrimPrefix(param, "link_")
	if len(token) == 0 || len(token) > 64 {
		b.send(ctx, telegramChatID, "❌ הקישור פג תוקף. נסה שוב מהאתר.")
		return
	}

	webChatID, err := b.linkTokens.ConsumeLinkToken(ctx, token)
	if err != nil {
		if errors.Is(err, storage.ErrLinkTokenNotFound) || errors.Is(err, storage.ErrLinkTokenExpired) || errors.Is(err, storage.ErrLinkTokenUsed) {
			b.send(ctx, telegramChatID, "❌ הקישור פג תוקף. נסה שוב מהאתר.")
			return
		}
		b.logger.Error("consume link token", "error", err)
		b.send(ctx, telegramChatID, "❌ הקישור פג תוקף. נסה שוב מהאתר.")
		return
	}

	if err := b.users.LinkTelegramToWeb(ctx, telegramChatID, webChatID); err != nil {
		b.logger.Error("link telegram to web", "telegram_chat_id", telegramChatID, "web_chat_id", webChatID, "error", err)
		b.send(ctx, telegramChatID, "❌ הקישור פג תוקף. נסה שוב מהאתר.")
		return
	}

	b.send(ctx, telegramChatID, "✅ חשבון הטלגרם חובר בהצלחה!")
}

func (b *Bot) handleWatch(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	b.logger.Debug("/watch command", "chat_id", chatID, "username", update.Message.From.Username)
	b.ensureUser(ctx, chatID, update.Message.From.Username)
	lang := b.getUserLang(ctx, chatID)

	if b.checkSearchLimit(ctx, chatID, lang, "watch_limit") {
		return
	}

	_ = b.users.UpdateUserState(ctx, chatID, StateAskSource, "{}")
	b.sendWithKeyboard(ctx, chatID,
		locale.T(lang, "wizard_source_prompt"),
		sourceKeyboard("", lang))
}

func (b *Bot) handleList(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	b.ensureUser(ctx, chatID, update.Message.From.Username)
	lang := b.getUserLang(ctx, chatID)

	searches, err := b.searches.ListSearches(ctx, chatID)
	if err != nil {
		b.send(ctx, chatID, locale.T(lang, "list_load_error"))
		return
	}

	if len(searches) == 0 {
		b.send(ctx, chatID, locale.T(lang, "list_empty"))
		return
	}

	var sb strings.Builder
	sb.WriteString(locale.Tf(lang, "list_header", len(searches)))

	var buttons [][]tgmodels.InlineKeyboardButton
	for _, s := range searches {
		prefix := ""
		if !s.Active {
			prefix = "⏸ "
		}
		mfr := b.catalog.ManufacturerName(s.Manufacturer)
		mdl := b.modelDisplayName(s.Manufacturer, s.Model)

		status := locale.T(lang, "label_active")
		if !s.Active {
			status = locale.T(lang, "label_paused")
		}

		src := sourceDisplayName(s.Source)
		sb.WriteString(fmt.Sprintf(
			"%s#%d [%s] %s %s (%d–%d, max %s NIS) [%s]\n",
			prefix, s.ID, src, mfr, mdl, s.YearMin, s.YearMax, format.Number(s.PriceMax), status))

		buttons = append(buttons, []tgmodels.InlineKeyboardButton{{
			Text:         locale.Tf(lang, "list_delete_btn", s.ID),
			CallbackData: cbDeleteSearch + strconv.FormatInt(s.ID, 10),
		}})
	}

	kb := &tgmodels.InlineKeyboardMarkup{InlineKeyboard: buttons}
	b.sendWithKeyboard(ctx, chatID, sb.String(), kb)
}

func (b *Bot) handleStop(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	lang := b.getUserLang(ctx, chatID)
	parts := strings.Fields(update.Message.Text)
	if len(parts) < 2 {
		b.sendMarkdown(ctx, chatID, locale.T(lang, "stop_usage"))
		return
	}

	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		b.send(ctx, chatID, locale.T(lang, "stop_invalid"))
		return
	}

	if err := b.searches.DeleteSearch(ctx, id, chatID); err != nil {
		b.send(ctx, chatID, locale.T(lang, "stop_failed"))
		return
	}

	b.send(ctx, chatID, locale.Tf(lang, "stop_success", id))
}

func (b *Bot) handlePause(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	lang := b.getUserLang(ctx, chatID)
	parts := strings.Fields(update.Message.Text)
	if len(parts) < 2 {
		b.sendMarkdown(ctx, chatID, locale.T(lang, "pause_usage"))
		return
	}

	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		b.send(ctx, chatID, locale.T(lang, "pause_invalid"))
		return
	}

	search, err := b.searches.GetSearch(ctx, id)
	if err != nil || search == nil || search.ChatID != chatID {
		b.send(ctx, chatID, locale.T(lang, "pause_not_found"))
		return
	}

	if !search.Active {
		b.send(ctx, chatID, locale.Tf(lang, "pause_already_paused", id))
		return
	}

	if err := b.searches.SetSearchActive(ctx, id, chatID, false); err != nil {
		b.send(ctx, chatID, locale.T(lang, "pause_failed"))
		return
	}

	b.send(ctx, chatID, locale.Tf(lang, "pause_success", id, id))
}

func (b *Bot) handleResume(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	lang := b.getUserLang(ctx, chatID)
	parts := strings.Fields(update.Message.Text)
	if len(parts) < 2 {
		b.sendMarkdown(ctx, chatID, locale.T(lang, "resume_usage"))
		return
	}

	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		b.send(ctx, chatID, locale.T(lang, "resume_invalid"))
		return
	}

	search, err := b.searches.GetSearch(ctx, id)
	if err != nil || search == nil || search.ChatID != chatID {
		b.send(ctx, chatID, locale.T(lang, "resume_not_found"))
		return
	}

	if search.Active {
		b.send(ctx, chatID, locale.Tf(lang, "resume_already_active", id))
		return
	}

	if err := b.searches.SetSearchActive(ctx, id, chatID, true); err != nil {
		b.send(ctx, chatID, locale.T(lang, "resume_failed"))
		return
	}

	b.send(ctx, chatID, locale.Tf(lang, "resume_success", id))
}

func (b *Bot) handleCancel(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	lang := b.getUserLang(ctx, chatID)
	_ = b.users.UpdateUserState(ctx, chatID, StateIdle, "{}")
	b.send(ctx, chatID, locale.T(lang, "cancel"))
}

func (b *Bot) handleHelp(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	lang := b.getUserLang(ctx, update.Message.Chat.ID)
	b.sendMarkdown(ctx, update.Message.Chat.ID, locale.T(lang, "help"))
}

func (b *Bot) handleSettings(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	b.ensureUser(ctx, chatID, update.Message.From.Username)
	lang := b.getUserLang(ctx, chatID)

	count, err := b.searches.CountSearches(ctx, chatID)
	if err != nil {
		b.logger.Error("count searches failed", "chat_id", chatID, "error", err)
		b.send(ctx, chatID, locale.T(lang, "error_generic"))
		return
	}
	limit := b.maxSearchesForUser(ctx, chatID)
	msg := locale.Tf(lang, "settings", count, limit)

	b.sendMarkdown(ctx, chatID, msg)
}

func (b *Bot) handleStats(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	if chatID != b.adminChatID {
		lang := b.getUserLang(ctx, chatID)
		b.send(ctx, chatID, locale.T(lang, "unknown_command"))
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

		if sources, ok := snap["sources"].(map[string]any); ok {
			for name, data := range sources {
				if m, ok := data.(map[string]any); ok {
					sb.WriteString(fmt.Sprintf("\n\n*%s:*\nFetches: %v (success: %v, errors: %v)\nAvg latency: %vms",
						name, m["fetches"], m["successes"], m["errors"], m["avg_latency_ms"]))
				}
			}
		}
	}

	b.sendMarkdown(ctx, chatID, sb.String())
}

func (b *Bot) handleDigest(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	b.ensureUser(ctx, chatID, update.Message.From.Username)
	lang := b.getUserLang(ctx, chatID)

	if b.digests == nil {
		b.send(ctx, chatID, locale.T(lang, "digest_unavailable"))
		return
	}

	mode, interval, err := b.digests.GetDigestMode(ctx, chatID)
	if err != nil {
		b.send(ctx, chatID, locale.T(lang, "digest_load_error"))
		return
	}

	var rows [][]tgmodels.InlineKeyboardButton
	var msgText string
	if mode == "digest" {
		rows = append(rows, []tgmodels.InlineKeyboardButton{
			{Text: "2h", CallbackData: cbDigestInterval + "2h"},
			{Text: "6h", CallbackData: cbDigestInterval + "6h"},
			{Text: "12h", CallbackData: cbDigestInterval + "12h"},
			{Text: "24h", CallbackData: cbDigestInterval + "24h"},
		})
		rows = append(rows, []tgmodels.InlineKeyboardButton{
			{Text: locale.T(lang, "btn_switch_instant"), CallbackData: cbDigestOff},
		})
		msgText = locale.Tf(lang, "digest_mode_digest", interval)
	} else {
		rows = append(rows, []tgmodels.InlineKeyboardButton{
			{Text: "2h", CallbackData: cbDigestInterval + "2h"},
			{Text: "6h", CallbackData: cbDigestInterval + "6h"},
		})
		rows = append(rows, []tgmodels.InlineKeyboardButton{
			{Text: "12h", CallbackData: cbDigestInterval + "12h"},
			{Text: "24h", CallbackData: cbDigestInterval + "24h"},
		})
		msgText = locale.T(lang, "digest_mode_instant")
	}

	if b.dailyDigests != nil {
		enabled, digestTime, _, ddErr := b.dailyDigests.GetDailyDigest(ctx, chatID)
		if ddErr == nil {
			if enabled {
				rows = append(rows, []tgmodels.InlineKeyboardButton{
					{Text: locale.T(lang, "btn_daily_digest_off"), CallbackData: cbDailyDigestOff},
				})
				msgText += "\n\n" + locale.Tf(lang, "daily_digest_enabled", digestTime)
			} else {
				rows = append(rows, []tgmodels.InlineKeyboardButton{
					{Text: locale.T(lang, "btn_daily_digest_on"), CallbackData: cbDailyDigestOn},
				})
			}
		}
	}

	kb := &tgmodels.InlineKeyboardMarkup{InlineKeyboard: rows}
	b.sendWithKeyboard(ctx, chatID, msgText, kb)
}

func (b *Bot) handleLanguage(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	b.ensureUser(ctx, chatID, update.Message.From.Username)
	lang := b.getUserLang(ctx, chatID)

	kb := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "עברית", CallbackData: cbLangHe},
				{Text: "English", CallbackData: cbLangEn},
			},
		},
	}
	b.sendWithKeyboard(ctx, chatID, locale.T(lang, "language_current"), kb)
}

func (b *Bot) handleEdit(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	b.ensureUser(ctx, chatID, update.Message.From.Username)
	lang := b.getUserLang(ctx, chatID)

	parts := strings.Fields(update.Message.Text)
	if len(parts) < 2 {
		b.sendMarkdown(ctx, chatID, locale.T(lang, "edit_usage"))
		return
	}

	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		b.send(ctx, chatID, locale.T(lang, "edit_invalid"))
		return
	}

	search, err := b.searches.GetSearch(ctx, id)
	if err != nil || search == nil || search.ChatID != chatID {
		b.send(ctx, chatID, locale.T(lang, "edit_not_found"))
		return
	}

	mfr := b.catalog.ManufacturerName(search.Manufacturer)
	mdl := b.modelDisplayName(search.Manufacturer, search.Model)

	wd := WizardData{
		Source:           search.Source,
		Manufacturer:     search.Manufacturer,
		ManufacturerName: mfr,
		Model:            search.Model,
		ModelName:        mdl,
		YearMin:          search.YearMin,
		YearMax:          search.YearMax,
		PriceMax:         search.PriceMax,
		EngineMinCC:      search.EngineMinCC,
		MaxKm:            search.MaxKm,
		MaxHand:          search.MaxHand,
		Keywords:         search.Keywords,
		ExcludeKeys:      search.ExcludeKeys,
		EditSearchID:     search.ID,
	}
	b.saveWizardState(ctx, chatID, StateAskSource, wd)
	b.sendWithKeyboard(ctx, chatID,
		locale.T(lang, "wizard_source_prompt"),
		sourceKeyboard(wd.Source, lang))
}

func (b *Bot) handleUpgrade(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	b.ensureUser(ctx, chatID, update.Message.From.Username)
	lang := b.getUserLang(ctx, chatID)
	b.sendMarkdown(ctx, chatID, locale.T(lang, "upgrade_disabled"))
}

