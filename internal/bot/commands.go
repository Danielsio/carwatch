package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"

	"github.com/dsionov/carwatch/internal/format"
)

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

	b.sendMarkdown(ctx, chatID,
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
		b.send(ctx, chatID, "Usage: /share <search_id>\nUse /list to see your search IDs.")
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
	mfr := b.catalog.ManufacturerName(search.Manufacturer)
	mdl := b.modelDisplayName(search.Manufacturer, search.Model)

	b.sendMarkdown(ctx, chatID, fmt.Sprintf(
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

	mfr := b.catalog.ManufacturerName(search.Manufacturer)
	mdl := b.modelDisplayName(search.Manufacturer, search.Model)

	engineStr := "Any"
	if search.EngineMinCC > 0 {
		engineStr = fmt.Sprintf("%.1fL+", float64(search.EngineMinCC)/1000)
	}

	summary := fmt.Sprintf(
		"*Shared search:*\n"+
			"Car: %s %s\n"+
			"Year: %d–%d\n"+
			"Max price: %s NIS\n"+
			"Engine: %s\n\n"+
			"Copy this search to start receiving alerts?",
		mfr, mdl, search.YearMin, search.YearMax,
		format.Number(search.PriceMax), engineStr)

	kb := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{{
			{Text: "Copy this search", CallbackData: cbPrefixShareCopy + strconv.FormatInt(id, 10)},
		}},
	}

	b.sendWithKeyboard(ctx, chatID, summary, kb)
}

func (b *Bot) handleWatch(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	b.logger.Debug("/watch command", "chat_id", chatID, "username", update.Message.From.Username)
	b.ensureUser(ctx, chatID, update.Message.From.Username)

	count, err := b.searches.CountSearches(ctx, chatID)
	if err != nil {
		b.logger.Error("count searches failed", "chat_id", chatID, "error", err)
		b.send(ctx, chatID, "Failed to check search limits. Please try again.")
		return
	}
	if count >= int64(b.maxSearches) {
		b.send(ctx, chatID, fmt.Sprintf(
			"You already have %d active searches (max %d). Use /stop to remove one first.",
			count, b.maxSearches))
		return
	}

	_ = b.users.UpdateUserState(ctx, chatID, StateAskSource, "{}")
	b.sendWithKeyboard(ctx, chatID,
		"Which marketplaces do you want to search? (select one or both)",
		sourceKeyboard(""))
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
			prefix = "⏸ "
		}
		mfr := b.catalog.ManufacturerName(s.Manufacturer)
		mdl := b.modelDisplayName(s.Manufacturer, s.Model)

		status := "active"
		if !s.Active {
			status = "paused"
		}

		src := sourceDisplayName(s.Source)
		sb.WriteString(fmt.Sprintf(
			"%s#%d [%s] %s %s (%d–%d, max %s NIS) [%s]\n",
			prefix, s.ID, src, mfr, mdl, s.YearMin, s.YearMax, format.Number(s.PriceMax), status))

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
	b.sendMarkdown(ctx, update.Message.Chat.ID,
		"*CarWatch Commands:*\n\n"+
			"/watch — Set up a new car search\n"+
			"/list — Show your active searches\n"+
			"/history — View past matched listings\n"+
			"/pause <id> — Pause a search\n"+
			"/resume <id> — Resume a paused search\n"+
			"/stop <id> — Delete a search\n"+
			"/share <id> — Share a search via link\n"+
			"/digest — Toggle notification mode (instant/digest)\n"+
			"/settings — View your current limits\n"+
			"/cancel — Cancel current wizard\n"+
			"/help — Show this message")
}

func (b *Bot) handleSettings(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	b.ensureUser(ctx, chatID, update.Message.From.Username)

	count, _ := b.searches.CountSearches(ctx, chatID)
	b.sendMarkdown(ctx, chatID, fmt.Sprintf(
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

	if b.digests == nil {
		b.send(ctx, chatID, "Digest mode is not available.")
		return
	}

	mode, interval, err := b.digests.GetDigestMode(ctx, chatID)
	if err != nil {
		b.send(ctx, chatID, "Failed to load digest settings.")
		return
	}

	var kb *tgmodels.InlineKeyboardMarkup
	if mode == "digest" {
		kb = &tgmodels.InlineKeyboardMarkup{
			InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
				{
					{Text: "2h", CallbackData: cbDigestInterval + "2h"},
					{Text: "6h", CallbackData: cbDigestInterval + "6h"},
					{Text: "12h", CallbackData: cbDigestInterval + "12h"},
					{Text: "24h", CallbackData: cbDigestInterval + "24h"},
				},
				{
					{Text: "Switch to instant", CallbackData: cbDigestOff},
				},
			},
		}
		b.sendWithKeyboard(ctx, chatID,
			fmt.Sprintf("*Notification mode:* digest (every %s)\n\nChoose interval or switch to instant:", interval), kb)
	} else {
		kb = &tgmodels.InlineKeyboardMarkup{
			InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
				{
					{Text: "Every 2h", CallbackData: cbDigestInterval + "2h"},
					{Text: "Every 6h", CallbackData: cbDigestInterval + "6h"},
				},
				{
					{Text: "Every 12h", CallbackData: cbDigestInterval + "12h"},
					{Text: "Every 24h", CallbackData: cbDigestInterval + "24h"},
				},
			},
		}
		b.sendWithKeyboard(ctx, chatID,
			"*Notification mode:* instant\n\nSwitch to digest mode — choose how often to receive batched listings:", kb)
	}
}
