package bot

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"

	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/storage"
)

// DefaultQuickStartManufacturer and DefaultQuickStartModel are catalog IDs for the quick-start preset (Toyota Corolla).
// TODO: these should eventually come from config instead of hardcoding.
const (
	DefaultQuickStartManufacturer = 19
	DefaultQuickStartModel        = 8640
)

// --- Callback Handler ---

func (b *Bot) handleCallback(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if update.CallbackQuery == nil {
		b.logger.Debug("handleCallback: nil callback query")
		return
	}

	if update.CallbackQuery.Message.Message == nil {
		b.logger.Warn("handleCallback: unsupported message type in callback")
		return
	}

	chatID := update.CallbackQuery.Message.Message.Chat.ID
	fromID := update.CallbackQuery.From.ID
	data := update.CallbackQuery.Data
	b.logger.Debug("callback received", "chat_id", chatID, "from_id", fromID, "data", data)

	if err := b.msg.AnswerCallback(ctx, update.CallbackQuery.ID); err != nil {
		b.logger.Error("answer callback query failed", "chat_id", chatID, "error", err)
	}

	if chatID != fromID {
		b.logger.Warn("callback from non-owner ignored", "chat_id", chatID, "from_id", fromID)
		return
	}

	b.dispatchCallbackData(ctx, chatID, data)
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
	unlock := b.lockChat(chatID)
	defer unlock()

	lang := b.getUserLang(ctx, chatID)
	token := strings.TrimPrefix(data, cbPrefixShareCopy)
	if len(token) == 0 || len(token) > 64 {
		b.send(ctx, chatID, locale.T(lang, "share_invalid_link"))
		return
	}

	src, err := b.searches.GetSearchByShareToken(ctx, token)
	if err != nil || src == nil {
		b.send(ctx, chatID, locale.T(lang, "share_search_deleted"))
		return
	}

	if b.checkSearchLimit(ctx, chatID, lang, "share_limit") {
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
		PriceMin:     src.PriceMin,
		PriceMax:     src.PriceMax,
		EngineMinCC:  src.EngineMinCC,
		MaxKm:        src.MaxKm,
		MaxHand:      src.MaxHand,
		Keywords:     src.Keywords,
		ExcludeKeys:  src.ExcludeKeys,
		SellerFilter: src.SellerFilter,
		GearBox:      src.GearBox,
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
	unlock := b.lockChat(chatID)
	defer unlock()

	_ = b.users.UpdateUserState(ctx, chatID, StateIdle, "{}")
	b.send(ctx, chatID, locale.T(b.getUserLang(ctx, chatID), "cancel"))
}

func (b *Bot) onEditRestart(ctx context.Context, chatID int64) {
	unlock := b.lockChat(chatID)
	defer unlock()

	lang := b.getUserLang(ctx, chatID)
	wd := b.loadWizardData(ctx, chatID)
	newWd := WizardData{EditSearchID: wd.EditSearchID}
	if !b.saveWizardStateOrAbort(ctx, chatID, StateAskSource, newWd) {
		return
	}
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
	unlock := b.lockChat(chatID)
	defer unlock()

	lang := b.getUserLang(ctx, chatID)

	if b.checkSearchLimit(ctx, chatID, lang, "watch_limit") {
		return
	}

	id, err := b.searches.CreateSearch(ctx, storage.Search{
		ChatID:       chatID,
		Name:         "toyota-corolla",
		Source:       "yad2",
		Manufacturer: DefaultQuickStartManufacturer,
		Model:        DefaultQuickStartModel,
		YearMin:      2018,
		YearMax:      time.Now().Year() + 2,
		PriceMax:     200000,
	})
	if err != nil {
		b.logger.Error("quick start search failed", "error", err)
		b.send(ctx, chatID, locale.T(lang, "wizard_save_failed"))
		return
	}

	b.send(ctx, chatID, locale.Tf(lang, "wizard_search_saved",
		id, "Yad2"))

	if b.pollTrigger != nil {
		b.pollTrigger.TriggerPoll()
	}
}

func (b *Bot) onWatchFromCallback(ctx context.Context, chatID int64) {
	unlock := b.lockChat(chatID)
	defer unlock()

	lang := b.getUserLang(ctx, chatID)

	if b.checkSearchLimit(ctx, chatID, lang, "watch_limit") {
		return
	}

	_ = b.users.UpdateUserState(ctx, chatID, StateAskSource, "{}")
	b.sendWithKeyboard(ctx, chatID,
		locale.T(lang, "wizard_source_prompt"),
		sourceKeyboard("", lang))
}

const (
	maxSavedListings  = 500
	maxHiddenListings = 1000
)

func isValidToken(token string) bool {
	if len(token) < 5 || len(token) > 40 {
		return false
	}
	for _, c := range token {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			continue
		}
		if c == '-' || c == '_' {
			continue
		}
		return false
	}
	return true
}

func (b *Bot) onSaveListing(ctx context.Context, chatID int64, data string) {
	unlock := b.lockChat(chatID)
	defer unlock()

	lang := b.getUserLang(ctx, chatID)
	token := strings.TrimPrefix(data, cbPrefixSave)
	if b.saved == nil {
		return
	}
	if !isValidToken(token) {
		b.logger.Warn("invalid save token", "chat_id", chatID, "token", token)
		b.send(ctx, chatID, locale.T(lang, "error_generic"))
		return
	}
	count, err := b.saved.CountSaved(ctx, chatID)
	if err != nil {
		b.logger.Error("count saved failed", "chat_id", chatID, "error", err)
		b.send(ctx, chatID, locale.T(lang, "error_generic"))
		return
	}
	if count >= maxSavedListings {
		b.send(ctx, chatID, locale.Tf(lang, "saved_limit_reached", maxSavedListings))
		return
	}
	if err := b.saved.SaveBookmark(ctx, chatID, token); err != nil {
		b.logger.Error("save listing failed", "chat_id", chatID, "token", token, "error", err)
		b.send(ctx, chatID, locale.T(lang, "error_generic"))
		return
	}
	b.send(ctx, chatID, locale.T(lang, "listing_saved"))
}

func (b *Bot) onHideListing(ctx context.Context, chatID int64, data string) {
	unlock := b.lockChat(chatID)
	defer unlock()

	lang := b.getUserLang(ctx, chatID)
	token := strings.TrimPrefix(data, cbPrefixHide)
	if b.hidden == nil {
		return
	}
	if !isValidToken(token) {
		b.logger.Warn("invalid hide token", "chat_id", chatID, "token", token)
		b.send(ctx, chatID, locale.T(lang, "error_generic"))
		return
	}
	count, err := b.hidden.CountHidden(ctx, chatID)
	if err != nil {
		b.logger.Error("count hidden failed", "chat_id", chatID, "error", err)
		b.send(ctx, chatID, locale.T(lang, "error_generic"))
		return
	}
	if count >= maxHiddenListings {
		b.send(ctx, chatID, locale.Tf(lang, "hidden_limit_reached", maxHiddenListings))
		return
	}
	if err := b.hidden.HideListing(ctx, chatID, token); err != nil {
		b.logger.Error("hide listing failed", "chat_id", chatID, "token", token, "error", err)
		b.send(ctx, chatID, locale.T(lang, "error_generic"))
		return
	}
	b.send(ctx, chatID, locale.T(lang, "listing_hidden"))
}

func (b *Bot) onClearHidden(ctx context.Context, chatID int64) {
	lang := b.getUserLang(ctx, chatID)
	if b.hidden == nil {
		return
	}
	count, err := b.hidden.CountHidden(ctx, chatID)
	if err != nil {
		b.logger.Error("count hidden failed", "chat_id", chatID, "error", err)
		b.send(ctx, chatID, locale.T(lang, "error_generic"))
		return
	}
	if count == 0 {
		b.send(ctx, chatID, locale.T(lang, "hidden_empty"))
		return
	}
	kb := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: locale.T(lang, "btn_confirm"), CallbackData: cbHiddenClearConfirm},
				{Text: locale.T(lang, "btn_cancel"), CallbackData: "noop"},
			},
		},
	}
	b.sendWithKeyboard(ctx, chatID, locale.Tf(lang, "hidden_clear_confirm", count), kb)
}

func (b *Bot) onClearHiddenConfirm(ctx context.Context, chatID int64) {
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

func (b *Bot) onDailyDigestOn(ctx context.Context, chatID int64) {
	if b.dailyDigests == nil {
		return
	}
	lang := b.getUserLang(ctx, chatID)
	_, digestTime, _, err := b.dailyDigests.GetDailyDigest(ctx, chatID)
	if err != nil {
		b.send(ctx, chatID, locale.T(lang, "error_generic"))
		return
	}
	if digestTime == "" {
		digestTime = "09:00"
	}
	if err := b.dailyDigests.SetDailyDigest(ctx, chatID, true, digestTime); err != nil {
		b.send(ctx, chatID, locale.T(lang, "error_generic"))
		return
	}
	b.sendMarkdown(ctx, chatID, locale.Tf(lang, "daily_digest_enabled", digestTime))
}

func (b *Bot) onDailyDigestOff(ctx context.Context, chatID int64) {
	if b.dailyDigests == nil {
		return
	}
	lang := b.getUserLang(ctx, chatID)
	_, digestTime, _, err := b.dailyDigests.GetDailyDigest(ctx, chatID)
	if err != nil {
		b.send(ctx, chatID, locale.T(lang, "error_generic"))
		return
	}
	if digestTime == "" {
		digestTime = "09:00"
	}
	if err := b.dailyDigests.SetDailyDigest(ctx, chatID, false, digestTime); err != nil {
		b.send(ctx, chatID, locale.T(lang, "error_generic"))
		return
	}
	b.sendMarkdown(ctx, chatID, locale.T(lang, "daily_digest_disabled"))
}
