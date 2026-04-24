package bot

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"

	"github.com/dsionov/carwatch/internal/storage"
)

// --- Callback Handler ---

func (b *Bot) handleCallback(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	if update.CallbackQuery == nil {
		b.logger.Debug("handleCallback: nil callback query")
		return
	}

	if update.CallbackQuery.Message.Message == nil {
		b.logger.Warn("handleCallback: unsupported message type in callback")
		return
	}

	chatID := update.CallbackQuery.Message.Message.Chat.ID
	data := update.CallbackQuery.Data
	b.logger.Debug("callback received", "chat_id", chatID, "data", data)

	if err := b.msg.AnswerCallback(ctx, update.CallbackQuery.ID); err != nil {
		b.logger.Error("answer callback query failed", "chat_id", chatID, "error", err)
	}

	switch {
	case strings.HasPrefix(data, cbPrefixSource):
		b.onLegacySourceSelected(ctx, chatID, strings.TrimPrefix(data, cbPrefixSource))
	case strings.HasPrefix(data, cbSourceToggle):
		b.onSourceToggle(ctx, chatID, data)
	case data == cbSourceDone:
		b.onSourceDone(ctx, chatID)
	case strings.HasPrefix(data, cbMfrPage):
		b.onMfrPage(ctx, chatID, data)
	case data == cbMfrSearch:
		b.onMfrSearch(ctx, chatID)
	case strings.HasPrefix(data, cbPrefixMfr):
		b.onManufacturerSelected(ctx, chatID, data)
	case strings.HasPrefix(data, cbMdlPage):
		b.onMdlPage(ctx, chatID, data)
	case data == cbMdlSearch:
		b.onMdlSearch(ctx, chatID)
	case strings.HasPrefix(data, cbPrefixModel):
		b.onModelSelected(ctx, chatID, data)
	case strings.HasPrefix(data, cbPrefixEngine):
		b.onEngineSelected(ctx, chatID, data)
	case strings.HasPrefix(data, cbPrefixMaxKm):
		b.onMaxKmSelected(ctx, chatID, data)
	case strings.HasPrefix(data, cbPrefixMaxHand):
		b.onMaxHandSelected(ctx, chatID, data)
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
	case data == cbDigestOn:
		b.onDigestOn(ctx, chatID)
	case data == cbDigestOff:
		b.onDigestOff(ctx, chatID)
	case strings.HasPrefix(data, cbDigestInterval):
		b.onDigestInterval(ctx, chatID, data)
	case strings.HasPrefix(data, cbHistoryPage):
		b.onHistoryPage(ctx, chatID, data)
	case data == "noop":
		// page indicator button, do nothing
	}
}

func (b *Bot) onDeleteSearch(ctx context.Context, chatID int64, data string) {
	idStr := strings.TrimPrefix(data, cbDeleteSearch)
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		b.logger.Error("invalid search ID in delete callback", "raw", idStr, "error", err)
		b.send(ctx, chatID, "Invalid search ID.")
		return
	}

	if err := b.searches.DeleteSearch(ctx, id, chatID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			b.send(ctx, chatID, "Search not found.")
		} else {
			b.logger.Error("delete search failed", "id", id, "error", err)
			b.send(ctx, chatID, "Failed to delete search. Please try again.")
		}
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

	mfr := b.catalog.ManufacturerName(src.Manufacturer)
	mdl := b.modelDisplayName(src.Manufacturer, src.Model)
	name := fmt.Sprintf("%s-%s", strings.ToLower(mfr), strings.ToLower(mdl))

	newID, err := b.searches.CreateSearch(ctx, storage.Search{
		ChatID:       chatID,
		Name:         name,
		Source:       src.Source,
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

	srcDisplay := sourceDisplayName(src.Source)
	b.send(ctx, chatID, fmt.Sprintf(
		"Search #%d saved! I'll check %s every %s and send you new listings.\n\nUse /list to see your searches.",
		newID, srcDisplay, b.formatInterval()))
}

func (b *Bot) onDigestOn(ctx context.Context, chatID int64) {
	if b.digests == nil {
		return
	}
	if err := b.digests.SetDigestMode(ctx, chatID, "digest", "6h"); err != nil {
		b.send(ctx, chatID, "Failed to update digest mode.")
		return
	}
	b.sendMarkdown(ctx, chatID, "Switched to *digest* mode. Listings will be batched and sent every 6 hours.")
}

func (b *Bot) onDigestOff(ctx context.Context, chatID int64) {
	if b.digests == nil {
		return
	}
	if err := b.digests.SetDigestMode(ctx, chatID, "instant", "6h"); err != nil {
		b.send(ctx, chatID, "Failed to update digest mode.")
		return
	}
	b.sendMarkdown(ctx, chatID, "Switched to *instant* mode. Listings will be sent immediately.")
}

func (b *Bot) onDigestInterval(ctx context.Context, chatID int64, data string) {
	if b.digests == nil {
		return
	}
	interval := strings.TrimPrefix(data, cbDigestInterval)
	switch interval {
	case "2h", "6h", "12h", "24h":
	default:
		b.send(ctx, chatID, "Invalid interval.")
		return
	}
	if err := b.digests.SetDigestMode(ctx, chatID, "digest", interval); err != nil {
		b.send(ctx, chatID, "Failed to update digest interval.")
		return
	}
	b.sendMarkdown(ctx, chatID, fmt.Sprintf("Switched to *digest* mode — listings batched every *%s*.", interval))
}

func (b *Bot) onCancelCallback(ctx context.Context, chatID int64) {
	_ = b.users.UpdateUserState(ctx, chatID, StateIdle, "{}")
	b.send(ctx, chatID, "Search cancelled.")
}

func (b *Bot) onEdit(ctx context.Context, chatID int64) {
	_ = b.users.UpdateUserState(ctx, chatID, StateAskSource, "{}")
	b.sendWithKeyboard(ctx, chatID,
		"Let's start over. Which marketplaces?",
		sourceKeyboard(""))
}
