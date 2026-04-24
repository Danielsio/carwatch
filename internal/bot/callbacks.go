package bot

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"

	"github.com/dsionov/carwatch/internal/locale"
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
	case data == cbSkipKeywords:
		b.onSkipKeywords(ctx, chatID)
	case data == cbSkipExcludeKeys:
		b.onSkipExcludeKeys(ctx, chatID)
	case data == cbConfirm:
		b.onConfirm(ctx, chatID)
	case data == cbEdit:
		b.onEditRestart(ctx, chatID)
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
	case data == cbLangHe:
		b.onLanguageSwitch(ctx, chatID, locale.Hebrew)
	case data == cbLangEn:
		b.onLanguageSwitch(ctx, chatID, locale.English)
	case data == cbQuickStart:
		b.onQuickStart(ctx, chatID)
	case strings.HasPrefix(data, cbPrefixSave):
		b.onSaveListing(ctx, chatID, data)
	case strings.HasPrefix(data, cbPrefixHide):
		b.onHideListing(ctx, chatID, data)
	case data == cbHiddenClear:
		b.onClearHidden(ctx, chatID)
	case data == "watch":
		b.onWatchFromCallback(ctx, chatID)
	case data == "noop":
		// page indicator button, do nothing
	}
}

func (b *Bot) onDeleteSearch(ctx context.Context, chatID int64, data string) {
	lang := b.getUserLang(ctx, chatID)
	idStr := strings.TrimPrefix(data, cbDeleteSearch)
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		b.logger.Error("invalid search ID in delete callback", "raw", idStr, "error", err)
		b.send(ctx, chatID, locale.T(lang, "error_invalid_id"))
		return
	}

	if err := b.searches.DeleteSearch(ctx, id, chatID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			b.send(ctx, chatID, locale.T(lang, "pause_not_found"))
		} else {
			b.logger.Error("delete search failed", "id", id, "error", err)
			b.send(ctx, chatID, locale.T(lang, "stop_failed"))
		}
		return
	}
	b.send(ctx, chatID, locale.Tf(lang, "stop_success", id))
}

func (b *Bot) onShareCopy(ctx context.Context, chatID int64, data string) {
	lang := b.getUserLang(ctx, chatID)
	idStr := strings.TrimPrefix(data, cbPrefixShareCopy)
	srcID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		b.send(ctx, chatID, locale.T(lang, "share_invalid_link"))
		return
	}

	src, err := b.searches.GetSearch(ctx, srcID)
	if err != nil || src == nil {
		b.send(ctx, chatID, locale.T(lang, "share_search_deleted"))
		return
	}

	count, err := b.searches.CountSearches(ctx, chatID)
	if err != nil {
		b.logger.Error("count searches failed", "chat_id", chatID, "error", err)
		b.send(ctx, chatID, locale.T(lang, "share_limit_error"))
		return
	}
	if count >= int64(b.maxSearches) {
		b.send(ctx, chatID, locale.Tf(lang, "share_limit_reached", count, b.maxSearches))
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
		Keywords:     src.Keywords,
		ExcludeKeys:  src.ExcludeKeys,
	})
	if err != nil {
		b.logger.Error("clone search failed", "error", err)
		b.send(ctx, chatID, locale.T(lang, "share_copy_failed"))
		return
	}

	srcDisplay := sourceDisplayName(src.Source)
	b.send(ctx, chatID, locale.Tf(lang, "share_copy_success",
		newID, srcDisplay, b.formatInterval()))
}

func (b *Bot) onDigestOn(ctx context.Context, chatID int64) {
	if b.digests == nil {
		return
	}
	lang := b.getUserLang(ctx, chatID)
	if err := b.digests.SetDigestMode(ctx, chatID, "digest", "6h"); err != nil {
		b.send(ctx, chatID, locale.T(lang, "digest_update_failed"))
		return
	}
	b.sendMarkdown(ctx, chatID, locale.Tf(lang, "digest_switched_digest", "6h"))
}

func (b *Bot) onDigestOff(ctx context.Context, chatID int64) {
	if b.digests == nil {
		return
	}
	lang := b.getUserLang(ctx, chatID)
	if err := b.digests.SetDigestMode(ctx, chatID, "instant", "6h"); err != nil {
		b.send(ctx, chatID, locale.T(lang, "digest_update_failed"))
		return
	}
	b.sendMarkdown(ctx, chatID, locale.T(lang, "digest_switched_instant"))
}

func (b *Bot) onDigestInterval(ctx context.Context, chatID int64, data string) {
	if b.digests == nil {
		return
	}
	lang := b.getUserLang(ctx, chatID)
	interval := strings.TrimPrefix(data, cbDigestInterval)
	switch interval {
	case "2h", "6h", "12h", "24h":
	default:
		b.send(ctx, chatID, locale.T(lang, "digest_invalid_interval"))
		return
	}
	if err := b.digests.SetDigestMode(ctx, chatID, "digest", interval); err != nil {
		b.send(ctx, chatID, locale.T(lang, "digest_update_failed"))
		return
	}
	b.sendMarkdown(ctx, chatID, locale.Tf(lang, "digest_switched_digest", interval))
}

func (b *Bot) onCancelCallback(ctx context.Context, chatID int64) {
	_ = b.users.UpdateUserState(ctx, chatID, StateIdle, "{}")
	b.send(ctx, chatID, locale.T(b.getUserLang(ctx, chatID), "cancel"))
}

func (b *Bot) onEditRestart(ctx context.Context, chatID int64) {
	lang := b.getUserLang(ctx, chatID)
	_ = b.users.UpdateUserState(ctx, chatID, StateAskSource, "{}")
	b.sendWithKeyboard(ctx, chatID,
		locale.T(lang, "wizard_start_over"),
		sourceKeyboard("", lang))
}

func (b *Bot) onLanguageSwitch(ctx context.Context, chatID int64, lang locale.Lang) {
	if err := b.users.SetUserLanguage(ctx, chatID, string(lang)); err != nil {
		b.logger.Error("set user language failed", "chat_id", chatID, "error", err)
		b.send(ctx, chatID, locale.T(lang, "error_generic"))
		return
	}
	b.send(ctx, chatID, locale.T(lang, "language_switched"))
}

func (b *Bot) onQuickStart(ctx context.Context, chatID int64) {
	lang := b.getUserLang(ctx, chatID)

	count, err := b.searches.CountSearches(ctx, chatID)
	if err != nil {
		b.send(ctx, chatID, locale.T(lang, "watch_limit_error"))
		return
	}
	if count >= int64(b.maxSearches) {
		b.send(ctx, chatID, locale.Tf(lang, "watch_limit_reached", count, b.maxSearches))
		return
	}

	id, err := b.searches.CreateSearch(ctx, storage.Search{
		ChatID:       chatID,
		Name:         "toyota-corolla",
		Source:       "yad2,winwin",
		Manufacturer: 19,
		Model:        8640,
		YearMin:      2018,
		YearMax:      2026,
		PriceMax:     200000,
	})
	if err != nil {
		b.logger.Error("quick start search failed", "error", err)
		b.send(ctx, chatID, locale.T(lang, "wizard_save_failed"))
		return
	}

	b.send(ctx, chatID, locale.Tf(lang, "wizard_search_saved",
		id, "Yad2, WinWin"))

	if b.pollTrigger != nil {
		b.pollTrigger.TriggerPoll()
	}
}

func (b *Bot) onWatchFromCallback(ctx context.Context, chatID int64) {
	lang := b.getUserLang(ctx, chatID)

	count, err := b.searches.CountSearches(ctx, chatID)
	if err != nil {
		b.send(ctx, chatID, locale.T(lang, "watch_limit_error"))
		return
	}
	if count >= int64(b.maxSearches) {
		b.send(ctx, chatID, locale.Tf(lang, "watch_limit_reached", count, b.maxSearches))
		return
	}

	_ = b.users.UpdateUserState(ctx, chatID, StateAskSource, "{}")
	b.sendWithKeyboard(ctx, chatID,
		locale.T(lang, "wizard_source_prompt"),
		sourceKeyboard("", lang))
}

func (b *Bot) onSaveListing(ctx context.Context, chatID int64, data string) {
	lang := b.getUserLang(ctx, chatID)
	token := strings.TrimPrefix(data, cbPrefixSave)
	if b.saved == nil {
		return
	}
	if err := b.saved.SaveBookmark(ctx, chatID, token); err != nil {
		b.logger.Error("save listing failed", "chat_id", chatID, "token", token, "error", err)
		return
	}
	b.send(ctx, chatID, locale.T(lang, "listing_saved"))
}

func (b *Bot) onHideListing(ctx context.Context, chatID int64, data string) {
	lang := b.getUserLang(ctx, chatID)
	token := strings.TrimPrefix(data, cbPrefixHide)
	if b.hidden == nil {
		return
	}
	if err := b.hidden.HideListing(ctx, chatID, token); err != nil {
		b.logger.Error("hide listing failed", "chat_id", chatID, "token", token, "error", err)
		return
	}
	b.send(ctx, chatID, locale.T(lang, "listing_hidden"))
}

func (b *Bot) onClearHidden(ctx context.Context, chatID int64) {
	lang := b.getUserLang(ctx, chatID)
	if b.hidden == nil {
		return
	}
	if err := b.hidden.ClearHidden(ctx, chatID); err != nil {
		b.logger.Error("clear hidden failed", "chat_id", chatID, "error", err)
		b.send(ctx, chatID, locale.T(lang, "error_generic"))
		return
	}
	b.send(ctx, chatID, locale.T(lang, "hidden_cleared"))
}
