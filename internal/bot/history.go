package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"

	"github.com/dsionov/carwatch/internal/format"
	"github.com/dsionov/carwatch/internal/locale"
)

const historyPageSize = 5

func (b *Bot) handleHistory(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	b.ensureUser(ctx, chatID, update.Message.From.Username)
	b.sendHistoryPage(ctx, chatID, 0)
}

func (b *Bot) sendHistoryPage(ctx context.Context, chatID int64, page int) {
	lang := b.getUserLang(ctx, chatID)

	if b.listings == nil {
		b.send(ctx, chatID, locale.T(lang, "history_unavailable"))
		return
	}

	total, err := b.listings.CountUserListings(ctx, chatID)
	if err != nil {
		b.logger.Error("count user listings failed", "chat_id", chatID, "error", err)
		b.send(ctx, chatID, locale.T(lang, "history_load_error"))
		return
	}

	if total == 0 {
		b.send(ctx, chatID, locale.T(lang, "history_empty"))
		return
	}

	totalPages := int((total + int64(historyPageSize) - 1) / int64(historyPageSize))
	if page < 0 || page >= totalPages {
		b.send(ctx, chatID, locale.T(lang, "history_page_invalid"))
		return
	}

	offset := page * historyPageSize
	listings, err := b.listings.ListUserListings(ctx, chatID, historyPageSize, offset)
	if err != nil {
		b.logger.Error("list user listings failed", "chat_id", chatID, "error", err)
		b.send(ctx, chatID, locale.T(lang, "history_load_error"))
		return
	}

	var sb strings.Builder
	sb.WriteString(locale.Tf(lang, "history_header", total))

	for _, l := range listings {
		title := format.EscapeMarkdown(strings.TrimSpace(l.Manufacturer + " " + l.Model))
		if l.Year > 0 {
			title += fmt.Sprintf(" %d", l.Year)
		}
		sb.WriteString("\n━━━━━━━━━━━━━━━━━━━━\n")
		sb.WriteString(fmt.Sprintf("*%s*\n", title))
		if l.Price > 0 {
			sb.WriteString(fmt.Sprintf("💰 ₪%s", format.Number(l.Price)))
		}
		if l.Km > 0 {
			sb.WriteString(fmt.Sprintf(" · 🛣️ %s km", format.Number(l.Km)))
		}
		if l.Hand > 0 {
			sb.WriteString(fmt.Sprintf(" · ✋ %d", l.Hand))
		}
		sb.WriteString("\n")
		if l.City != "" {
			sb.WriteString(fmt.Sprintf("📍 %s\n", format.EscapeMarkdown(l.City)))
		}
		sb.WriteString(locale.Tf(lang, "history_found", l.FirstSeenAt.Format("02 Jan 2006 15:04")))
		if l.PageLink != "" {
			sb.WriteString(fmt.Sprintf("🔗 %s\n", format.EscapeMarkdown(l.PageLink)))
		}
	}

	if totalPages <= 1 {
		b.sendMarkdown(ctx, chatID, sb.String())
		return
	}

	var navRow []tgmodels.InlineKeyboardButton
	if page > 0 {
		navRow = append(navRow, tgmodels.InlineKeyboardButton{
			Text: locale.T(lang, "history_newer"), CallbackData: cbHistoryPage + strconv.Itoa(page-1),
		})
	}
	navRow = append(navRow, tgmodels.InlineKeyboardButton{
		Text: fmt.Sprintf("%d/%d", page+1, totalPages), CallbackData: "noop",
	})
	if page < totalPages-1 {
		navRow = append(navRow, tgmodels.InlineKeyboardButton{
			Text: locale.T(lang, "history_older"), CallbackData: cbHistoryPage + strconv.Itoa(page+1),
		})
	}

	kb := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{navRow},
	}
	b.sendWithKeyboard(ctx, chatID, sb.String(), kb)
}

func (b *Bot) onHistoryPage(ctx context.Context, chatID int64, data string) {
	pageStr := strings.TrimPrefix(data, cbHistoryPage)
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		b.logger.Warn("invalid history page callback", "chat_id", chatID, "raw", pageStr, "error", err)
		b.send(ctx, chatID, locale.T(b.getUserLang(ctx, chatID), "error_generic"))
		return
	}
	b.sendHistoryPage(ctx, chatID, page)
}

// --- /saved command ---

func (b *Bot) handleSaved(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	b.ensureUser(ctx, chatID, update.Message.From.Username)
	lang := b.getUserLang(ctx, chatID)

	if b.saved == nil {
		b.send(ctx, chatID, locale.T(lang, "saved_empty"))
		return
	}

	total, err := b.saved.CountSaved(ctx, chatID)
	if err != nil {
		b.send(ctx, chatID, locale.T(lang, "saved_load_error"))
		return
	}
	if total == 0 {
		b.send(ctx, chatID, locale.T(lang, "saved_empty"))
		return
	}

	listings, err := b.saved.ListSaved(ctx, chatID, historyPageSize, 0)
	if err != nil {
		b.send(ctx, chatID, locale.T(lang, "saved_load_error"))
		return
	}

	var sb strings.Builder
	sb.WriteString(locale.Tf(lang, "saved_header", total))

	for _, l := range listings {
		title := format.EscapeMarkdown(strings.TrimSpace(l.Manufacturer + " " + l.Model))
		if l.Year > 0 {
			title += fmt.Sprintf(" %d", l.Year)
		}
		sb.WriteString("\n━━━━━━━━━━━━━━━━━━━━\n")
		sb.WriteString(fmt.Sprintf("*%s*\n", title))
		if l.Price > 0 {
			sb.WriteString(fmt.Sprintf("💰 ₪%s", format.Number(l.Price)))
		}
		if l.Km > 0 {
			sb.WriteString(fmt.Sprintf(" · 🛣️ %s km", format.Number(l.Km)))
		}
		sb.WriteString("\n")
		if l.PageLink != "" {
			sb.WriteString(fmt.Sprintf("🔗 %s\n", format.EscapeMarkdown(l.PageLink)))
		}
	}

	b.sendMarkdown(ctx, chatID, sb.String())
}

// --- /hidden command ---

func (b *Bot) handleHidden(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	chatID := update.Message.Chat.ID
	b.ensureUser(ctx, chatID, update.Message.From.Username)
	lang := b.getUserLang(ctx, chatID)

	if b.hidden == nil {
		b.send(ctx, chatID, locale.T(lang, "hidden_empty"))
		return
	}

	total, err := b.hidden.CountHidden(ctx, chatID)
	if err != nil {
		b.send(ctx, chatID, locale.T(lang, "error_generic"))
		return
	}
	if total == 0 {
		b.send(ctx, chatID, locale.T(lang, "hidden_empty"))
		return
	}

	kb := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: locale.T(lang, "hidden_clear_btn"), CallbackData: cbHiddenClear},
			},
		},
	}
	b.sendWithKeyboard(ctx, chatID,
		locale.Tf(lang, "hidden_header", total), kb)
}
