package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"

	"github.com/dsionov/carwatch/internal/botcore"
	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/storage"
)

func (b *Bot) onSourceToggle(ctx context.Context, chatID int64, data string) {
	unlock := b.lockChat(chatID)
	defer unlock()

	source := strings.TrimPrefix(data, cbSourceToggle)
	wd := b.loadWizardData(ctx, chatID)

	selected := toggleSource(wd.Source, source)
	wd.Source = selected
	if !b.saveWizardStateOrAbort(ctx, chatID, StateAskSource, wd) {
		return
	}

	lang := b.getUserLang(ctx, chatID)
	b.sendWithKeyboard(ctx, chatID,
		locale.T(lang, "wizard_source_prompt"),
		sourceKeyboard(selected, lang))
}

func (b *Bot) onLegacySourceSelected(ctx context.Context, chatID int64, source string) {
	unlock := b.lockChat(chatID)
	defer unlock()

	wd := b.loadWizardData(ctx, chatID)
	wd.Source = source
	if !b.saveWizardStateOrAbort(ctx, chatID, StateAskSource, wd) {
		return
	}

	b.sourceDoneLocked(ctx, chatID)
}

func (b *Bot) onSourceDone(ctx context.Context, chatID int64) {
	unlock := b.lockChat(chatID)
	defer unlock()

	b.sourceDoneLocked(ctx, chatID)
}

func (b *Bot) sourceDoneLocked(ctx context.Context, chatID int64) {
	wd := b.loadWizardData(ctx, chatID)
	lang := b.getUserLang(ctx, chatID)
	if wd.Source == "" {
		b.sendWithKeyboard(ctx, chatID,
			locale.T(lang, "wizard_source_empty"),
			sourceKeyboard("", lang))
		return
	}
	b.logger.Debug("sources selected", "chat_id", chatID, "source", wd.Source)
	if !b.saveWizardStateOrAbort(ctx, chatID, StateAskManufacturer, wd) {
		return
	}

	b.sendWithKeyboard(ctx, chatID,
		locale.T(lang, "wizard_mfr_prompt"),
		b.manufacturerKeyboard(ctx, chatID, 0, lang))
}

func toggleSource(current, toggle string) string {
	return botcore.ToggleSource(current, toggle)
}

func (b *Bot) onMfrPage(ctx context.Context, chatID int64, data string) {
	pageStr := strings.TrimPrefix(data, cbMfrPage)
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		b.logger.Warn("invalid manufacturer page callback", "chat_id", chatID, "raw", pageStr, "error", err)
		b.send(ctx, chatID, locale.T(b.getUserLang(ctx, chatID), "error_generic"))
		return
	}
	lang := b.getUserLang(ctx, chatID)
	b.sendWithKeyboard(ctx, chatID,
		locale.T(lang, "wizard_mfr_prompt"),
		b.manufacturerKeyboard(ctx, chatID, page, lang))
}

func (b *Bot) onMfrSearch(ctx context.Context, chatID int64) {
	unlock := b.lockChat(chatID)
	defer unlock()

	wd := b.loadWizardData(ctx, chatID)
	if !b.saveWizardStateOrAbort(ctx, chatID, StateSearchManufacturer, wd) {
		return
	}
	b.send(ctx, chatID, locale.T(b.getUserLang(ctx, chatID), "wizard_mfr_search"))
}

func (b *Bot) onMdlPage(ctx context.Context, chatID int64, data string) {
	unlock := b.lockChat(chatID)
	defer unlock()

	pageStr := strings.TrimPrefix(data, cbMdlPage)
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		b.logger.Warn("invalid model page callback", "chat_id", chatID, "raw", pageStr, "error", err)
		b.send(ctx, chatID, locale.T(b.getUserLang(ctx, chatID), "error_generic"))
		return
	}
	wd := b.loadWizardData(ctx, chatID)
	lang := b.getUserLang(ctx, chatID)
	b.sendWithKeyboard(ctx, chatID,
		locale.Tf(lang, "wizard_model_prompt", wd.ManufacturerName),
		b.modelKeyboard(wd.Manufacturer, page, lang))
}

func (b *Bot) onMdlSearch(ctx context.Context, chatID int64) {
	unlock := b.lockChat(chatID)
	defer unlock()

	wd := b.loadWizardData(ctx, chatID)
	if !b.saveWizardStateOrAbort(ctx, chatID, StateSearchModel, wd) {
		return
	}
	lang := b.getUserLang(ctx, chatID)
	b.send(ctx, chatID, locale.Tf(lang, "wizard_model_search", wd.ManufacturerName))
}

func (b *Bot) onManufacturerSelected(ctx context.Context, chatID int64, data string) {
	unlock := b.lockChat(chatID)
	defer unlock()

	if !b.expectState(ctx, chatID, StateAskManufacturer) {
		return
	}
	idStr := strings.TrimPrefix(data, cbPrefixMfr)
	id, err := strconv.Atoi(idStr)
	if err != nil {
		b.logger.Error("invalid manufacturer ID in callback", "chat_id", chatID, "raw", idStr, "error", err)
		b.send(ctx, chatID, locale.T(b.getUserLang(ctx, chatID), "error_wrong_state"))
		return
	}

	lang := b.getUserLang(ctx, chatID)
	if id < 0 {
		b.logger.Warn("negative manufacturer ID in callback", "chat_id", chatID, "id", id)
		b.send(ctx, chatID, locale.T(lang, "error_wrong_state"))
		return
	}

	wd := b.loadWizardData(ctx, chatID)
	wd.Manufacturer = id

	if id == 0 {
		wd.ManufacturerName = locale.T(lang, "btn_any_manufacturer")
		wd.Model = 0
		wd.ModelName = locale.T(lang, "btn_any_model")
		b.logger.Debug("any manufacturer selected, skipping model step", "chat_id", chatID)
		if !b.saveWizardStateOrAbort(ctx, chatID, StateAskYearMin, wd) {
			return
		}
		b.send(ctx, chatID, locale.T(lang, "wizard_year_min"))
		return
	}

	wd.ManufacturerName = b.catalog.ManufacturerName(id)
	b.logger.Debug("manufacturer selected", "chat_id", chatID, "id", id, "name", wd.ManufacturerName)
	if !b.saveWizardStateOrAbort(ctx, chatID, StateAskModel, wd) {
		return
	}

	b.sendWithKeyboard(ctx, chatID,
		locale.Tf(lang, "wizard_model_prompt", wd.ManufacturerName),
		b.modelKeyboard(id, 0, lang))
}

func (b *Bot) onModelSelected(ctx context.Context, chatID int64, data string) {
	unlock := b.lockChat(chatID)
	defer unlock()

	if !b.expectState(ctx, chatID, StateAskModel) {
		return
	}
	idStr := strings.TrimPrefix(data, cbPrefixModel)
	modelID, err := strconv.Atoi(idStr)
	if err != nil {
		b.logger.Error("invalid model ID in callback", "chat_id", chatID, "raw", idStr, "error", err)
		b.send(ctx, chatID, locale.T(b.getUserLang(ctx, chatID), "error_wrong_state"))
		return
	}

	wd := b.loadWizardData(ctx, chatID)
	wd.Model = modelID
	wd.ModelName = b.modelDisplayName(wd.Manufacturer, modelID)
	b.logger.Debug("model selected", "chat_id", chatID, "manufacturer", wd.ManufacturerName, "model_id", modelID, "model_name", wd.ModelName)
	if !b.saveWizardStateOrAbort(ctx, chatID, StateAskYearMin, wd) {
		return
	}

	b.send(ctx, chatID, locale.T(b.getUserLang(ctx, chatID), "wizard_year_min"))
}

func (b *Bot) onEngineSelected(ctx context.Context, chatID int64, data string) {
	unlock := b.lockChat(chatID)
	defer unlock()

	if !b.expectState(ctx, chatID, StateAskEngine) {
		return
	}
	ccStr := strings.TrimPrefix(data, cbPrefixEngine)
	cc, err := strconv.Atoi(ccStr)
	if err != nil {
		b.logger.Error("invalid engine CC in callback", "chat_id", chatID, "raw", ccStr, "error", err)
		b.send(ctx, chatID, locale.T(b.getUserLang(ctx, chatID), "error_wrong_state"))
		return
	}

	wd := b.loadWizardData(ctx, chatID)
	wd.EngineMinCC = cc
	b.logger.Debug("engine selected", "chat_id", chatID, "engine_min_cc", cc)
	if !b.saveWizardStateOrAbort(ctx, chatID, StateAskMaxKm, wd) {
		return
	}

	lang := b.getUserLang(ctx, chatID)
	b.sendWithKeyboard(ctx, chatID, locale.T(lang, "wizard_km_prompt"), maxKmKeyboard(lang))
}

func (b *Bot) onMaxKmSelected(ctx context.Context, chatID int64, data string) {
	unlock := b.lockChat(chatID)
	defer unlock()

	if !b.expectState(ctx, chatID, StateAskMaxKm) {
		return
	}
	kmStr := strings.TrimPrefix(data, cbPrefixMaxKm)
	km, err := strconv.Atoi(kmStr)
	if err != nil {
		b.send(ctx, chatID, locale.T(b.getUserLang(ctx, chatID), "error_wrong_state"))
		return
	}

	wd := b.loadWizardData(ctx, chatID)
	wd.MaxKm = km
	if !b.saveWizardStateOrAbort(ctx, chatID, StateAskMaxHand, wd) {
		return
	}

	lang := b.getUserLang(ctx, chatID)
	b.sendWithKeyboard(ctx, chatID, locale.T(lang, "wizard_hand_prompt"), maxHandKeyboard(lang))
}

func (b *Bot) onMaxHandSelected(ctx context.Context, chatID int64, data string) {
	unlock := b.lockChat(chatID)
	defer unlock()

	if !b.expectState(ctx, chatID, StateAskMaxHand) {
		return
	}
	handStr := strings.TrimPrefix(data, cbPrefixMaxHand)
	hand, err := strconv.Atoi(handStr)
	if err != nil {
		b.send(ctx, chatID, locale.T(b.getUserLang(ctx, chatID), "error_wrong_state"))
		return
	}

	wd := b.loadWizardData(ctx, chatID)
	wd.MaxHand = hand
	if !b.saveWizardStateOrAbort(ctx, chatID, StateAskKeywords, wd) {
		return
	}

	lang := b.getUserLang(ctx, chatID)
	b.sendWithKeyboard(ctx, chatID,
		locale.T(lang, "wizard_keywords_prompt"),
		skipKeyboard(cbSkipKeywords, lang))
}

func (b *Bot) onSkipKeywords(ctx context.Context, chatID int64) {
	unlock := b.lockChat(chatID)
	defer unlock()

	if !b.expectState(ctx, chatID, StateAskKeywords) {
		return
	}
	wd := b.loadWizardData(ctx, chatID)
	wd.Keywords = ""
	if !b.saveWizardStateOrAbort(ctx, chatID, StateAskExcludeKeys, wd) {
		return
	}

	lang := b.getUserLang(ctx, chatID)
	b.sendWithKeyboard(ctx, chatID,
		locale.T(lang, "wizard_exclude_keys_prompt"),
		skipKeyboard(cbSkipExcludeKeys, lang))
}

func (b *Bot) onSkipExcludeKeys(ctx context.Context, chatID int64) {
	unlock := b.lockChat(chatID)
	defer unlock()

	if !b.expectState(ctx, chatID, StateAskExcludeKeys) {
		return
	}
	wd := b.loadWizardData(ctx, chatID)
	wd.ExcludeKeys = ""
	if !b.saveWizardStateOrAbort(ctx, chatID, StateConfirm, wd) {
		return
	}

	lang := b.getUserLang(ctx, chatID)
	kb, summary := confirmKeyboard(wd, lang)
	b.sendWithKeyboard(ctx, chatID, summary, kb)
}

func (b *Bot) onConfirm(ctx context.Context, chatID int64) {
	unlock := b.lockChat(chatID)
	defer unlock()

	user, err := b.users.GetUser(ctx, chatID)
	if err != nil {
		b.logger.Error("get user failed in onConfirm", "chat_id", chatID, "error", err)
		b.send(ctx, chatID, locale.T(b.getUserLang(ctx, chatID), "error_generic"))
		return
	}
	lang := b.getUserLang(ctx, chatID)
	if user == nil || user.State != StateConfirm {
		b.send(ctx, chatID, locale.T(lang, "wizard_session_expired"))
		return
	}

	wd := b.loadWizardData(ctx, chatID)
	b.logger.Debug("confirm clicked", "chat_id", chatID, "wizard_data", wd)

	source := wd.Source
	if source == "" {
		source = "yad2"
	}

	var name string
	switch {
	case wd.Manufacturer == 0:
		name = "all-cars"
	case wd.Model == 0:
		name = strings.ToLower(wd.ManufacturerName) + "-all"
	default:
		name = fmt.Sprintf("%s-%s", strings.ToLower(wd.ManufacturerName), strings.ToLower(wd.ModelName))
	}

	if wd.EditSearchID > 0 {
		err := b.searches.UpdateSearch(ctx, storage.Search{
			ID:           wd.EditSearchID,
			ChatID:       chatID,
			Name:         name,
			Source:       source,
			Manufacturer: wd.Manufacturer,
			Model:        wd.Model,
			YearMin:      wd.YearMin,
			YearMax:      wd.YearMax,
			PriceMin:     wd.PriceMin,
			PriceMax:     wd.PriceMax,
			EngineMinCC:  wd.EngineMinCC,
			MaxKm:        wd.MaxKm,
			MaxHand:      wd.MaxHand,
			Keywords:     wd.Keywords,
			ExcludeKeys:  wd.ExcludeKeys,
			SellerFilter: wd.SellerFilter,
			GearBox:      wd.GearBox,
			PriceOnly:    wd.PriceOnly,
			PhotoOnly:    wd.PhotoOnly,
		})
		if err != nil {
			b.logger.Error("update search failed", "error", err)
			b.send(ctx, chatID, locale.T(lang, "wizard_save_failed"))
			return
		}

		_ = b.users.UpdateUserState(ctx, chatID, StateIdle, "{}")
		b.send(ctx, chatID, locale.Tf(lang, "wizard_search_updated", wd.EditSearchID))

		if b.pollTrigger != nil {
			b.pollTrigger.TriggerPoll()
		}
		return
	}

	if b.checkSearchLimit(ctx, chatID, lang, "watch_limit") {
		return
	}

	b.logger.Debug("creating search", "chat_id", chatID, "name", name, "source", source)
	id, err := b.searches.CreateSearch(ctx, storage.Search{
		ChatID:       chatID,
		Name:         name,
		Source:       source,
		Manufacturer: wd.Manufacturer,
		Model:        wd.Model,
		YearMin:      wd.YearMin,
		YearMax:      wd.YearMax,
		PriceMin:     wd.PriceMin,
		PriceMax:     wd.PriceMax,
		EngineMinCC:  wd.EngineMinCC,
		MaxKm:        wd.MaxKm,
		MaxHand:      wd.MaxHand,
		Keywords:     wd.Keywords,
		ExcludeKeys:  wd.ExcludeKeys,
		SellerFilter: wd.SellerFilter,
		GearBox:      wd.GearBox,
		PriceOnly:    wd.PriceOnly,
		PhotoOnly:    wd.PhotoOnly,
	})
	if err != nil {
		b.logger.Error("create search failed", "error", err)
		b.send(ctx, chatID, locale.T(lang, "wizard_save_failed"))
		return
	}

	_ = b.users.UpdateUserState(ctx, chatID, StateIdle, "{}")
	b.send(ctx, chatID, locale.Tf(lang, "wizard_search_saved",
		id, sourceDisplayName(source)))

	if b.pollTrigger != nil {
		b.pollTrigger.TriggerPoll()
	}
}

// --- Default Handler (free text during wizard) ---

func (b *Bot) handleDefault(ctx context.Context, _ *tgbot.Bot, update *tgmodels.Update) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	if !b.ensureUser(ctx, chatID, update.Message.From.Username) {
		return
	}

	unlock := b.lockChat(chatID)
	defer unlock()

	user, err := b.users.GetUser(ctx, chatID)
	if err != nil {
		b.logger.Error("get user failed in default handler", "chat_id", chatID, "error", err)
		return
	}
	if user == nil {
		b.logger.Debug("no user found in default handler", "chat_id", chatID)
		return
	}

	text := strings.TrimSpace(update.Message.Text)
	b.logger.Debug("default handler", "chat_id", chatID, "state", user.State, "text", text)

	// Auto-cancel stale wizard sessions.
	if user.State != StateIdle {
		wd := b.loadWizardData(ctx, chatID)
		if wd.UpdatedAt > 0 && b.now().Unix()-wd.UpdatedAt > int64(wizardTimeout.Seconds()) {
			b.logger.Info("auto-cancelling stale wizard session", "chat_id", chatID, "state", user.State, "age_sec", b.now().Unix()-wd.UpdatedAt)
			_ = b.users.UpdateUserState(ctx, chatID, StateIdle, "{}")
			lang := b.getUserLang(ctx, chatID)
			b.send(ctx, chatID, locale.T(lang, "wizard_timeout"))
			return
		}
	}

	switch user.State {
	case StateSearchManufacturer:
		b.handleManufacturerSearch(ctx, chatID, text)
	case StateSearchModel:
		b.handleModelSearch(ctx, chatID, text)
	case StateAskYearMin:
		b.handleYearMin(ctx, chatID, text)
	case StateAskYearMax:
		b.handleYearMax(ctx, chatID, text)
	case StateAskPriceMax:
		b.handlePriceMax(ctx, chatID, text)
	case StateAskPriceMin:
		b.handlePriceMin(ctx, chatID, text)
	case StateAskKeywords:
		b.handleKeywordsInput(ctx, chatID, text)
	case StateAskExcludeKeys:
		b.handleExcludeKeysInput(ctx, chatID, text)
	default:
		if text != "" && !strings.HasPrefix(text, "/") {
			lang := b.getUserLang(ctx, chatID)
			b.send(ctx, chatID, locale.T(lang, "unknown_command"))
		}
	}
}

func (b *Bot) handleYearMin(ctx context.Context, chatID int64, text string) {
	b.logger.Debug("handleYearMin", "chat_id", chatID, "input", text)
	lang := b.getUserLang(ctx, chatID)
	maxYear := time.Now().Year() + 2
	year, err := strconv.Atoi(text)
	if err != nil || year < 1990 || year > maxYear {
		b.logger.Debug("invalid year min", "chat_id", chatID, "input", text, "error", err)
		b.send(ctx, chatID, locale.Tf(lang, "wizard_year_invalid", 1990, maxYear))
		return
	}

	wd := b.loadWizardData(ctx, chatID)
	wd.YearMin = year
	b.logger.Debug("year min set", "chat_id", chatID, "year_min", year)
	if !b.saveWizardStateOrAbort(ctx, chatID, StateAskYearMax, wd) {
		return
	}
	b.send(ctx, chatID, locale.T(lang, "wizard_year_max"))
}

func (b *Bot) handleYearMax(ctx context.Context, chatID int64, text string) {
	b.logger.Debug("handleYearMax", "chat_id", chatID, "input", text)
	lang := b.getUserLang(ctx, chatID)
	maxYear := time.Now().Year() + 2
	year, err := strconv.Atoi(text)
	if err != nil || year < 1990 || year > maxYear {
		b.logger.Debug("invalid year max", "chat_id", chatID, "input", text, "error", err)
		b.send(ctx, chatID, locale.Tf(lang, "wizard_year_invalid", 1990, maxYear))
		return
	}

	wd := b.loadWizardData(ctx, chatID)
	if year < wd.YearMin {
		b.send(ctx, chatID, locale.Tf(lang, "wizard_year_min_error", wd.YearMin))
		return
	}
	wd.YearMax = year
	b.logger.Debug("year max set", "chat_id", chatID, "year_max", year)
	if !b.saveWizardStateOrAbort(ctx, chatID, StateAskPriceMax, wd) {
		return
	}
	b.send(ctx, chatID, locale.T(lang, "wizard_price_prompt"))
}

func (b *Bot) handlePriceMax(ctx context.Context, chatID int64, text string) {
	b.logger.Debug("handlePriceMax", "chat_id", chatID, "input", text)
	lang := b.getUserLang(ctx, chatID)
	text = strings.ReplaceAll(text, ",", "")
	price, err := strconv.Atoi(text)
	if err != nil || price < 1000 || price > 10000000 {
		b.logger.Debug("invalid price", "chat_id", chatID, "input", text, "error", err)
		b.send(ctx, chatID, locale.T(lang, "wizard_price_invalid"))
		return
	}

	wd := b.loadWizardData(ctx, chatID)
	wd.PriceMax = price
	b.logger.Debug("price max set", "chat_id", chatID, "price_max", price)
	if !b.saveWizardStateOrAbort(ctx, chatID, StateAskPriceMin, wd) {
		return
	}
	b.sendWithKeyboard(ctx, chatID,
		locale.T(lang, "wizard_price_min_prompt"),
		skipKeyboard(cbSkipPriceMin, lang))
}

func (b *Bot) handlePriceMin(ctx context.Context, chatID int64, text string) {
	b.logger.Debug("handlePriceMin", "chat_id", chatID, "input", text)
	lang := b.getUserLang(ctx, chatID)
	wd := b.loadWizardData(ctx, chatID)

	skip := locale.T(lang, "wizard_keywords_skip")
	if strings.EqualFold(text, skip) || strings.EqualFold(text, "skip") || strings.EqualFold(text, "דלג") || text == "0" {
		wd.PriceMin = 0
	} else {
		text = strings.ReplaceAll(text, ",", "")
		price, err := strconv.Atoi(text)
		if err != nil || price < 0 || price > 10000000 {
			b.send(ctx, chatID, locale.T(lang, "wizard_price_min_invalid"))
			return
		}
		if wd.PriceMax > 0 && price > wd.PriceMax {
			b.send(ctx, chatID, locale.T(lang, "wizard_price_min_exceeds_max"))
			return
		}
		wd.PriceMin = price
	}

	b.logger.Debug("price min set", "chat_id", chatID, "price_min", wd.PriceMin)
	if !b.saveWizardStateOrAbort(ctx, chatID, StateAskGearBox, wd) {
		return
	}
	b.sendWithKeyboard(ctx, chatID, locale.T(lang, "wizard_gearbox_prompt"), gearBoxKeyboard(lang))
}

func (b *Bot) onSkipPriceMin(ctx context.Context, chatID int64) {
	unlock := b.lockChat(chatID)
	defer unlock()

	if !b.expectState(ctx, chatID, StateAskPriceMin) {
		return
	}
	wd := b.loadWizardData(ctx, chatID)
	wd.PriceMin = 0
	if !b.saveWizardStateOrAbort(ctx, chatID, StateAskGearBox, wd) {
		return
	}

	lang := b.getUserLang(ctx, chatID)
	b.sendWithKeyboard(ctx, chatID, locale.T(lang, "wizard_gearbox_prompt"), gearBoxKeyboard(lang))
}

func (b *Bot) onGearBoxSelected(ctx context.Context, chatID int64, data string) {
	unlock := b.lockChat(chatID)
	defer unlock()

	if !b.expectState(ctx, chatID, StateAskGearBox) {
		return
	}
	gearbox := strings.TrimPrefix(data, cbPrefixGearBox)

	wd := b.loadWizardData(ctx, chatID)
	if gearbox == "any" {
		wd.GearBox = ""
	} else {
		wd.GearBox = gearbox
	}
	b.logger.Debug("gearbox selected", "chat_id", chatID, "gear_box", wd.GearBox)
	if !b.saveWizardStateOrAbort(ctx, chatID, StateAskEngine, wd) {
		return
	}

	lang := b.getUserLang(ctx, chatID)
	b.sendWithKeyboard(ctx, chatID, locale.T(lang, "wizard_engine_prompt"), engineKeyboard(lang))
}

const maxKeywordsLen = 500

func (b *Bot) handleKeywordsInput(ctx context.Context, chatID int64, text string) {
	wd := b.loadWizardData(ctx, chatID)
	lang := b.getUserLang(ctx, chatID)

	skip := locale.T(lang, "wizard_keywords_skip")
	if strings.EqualFold(text, skip) || strings.EqualFold(text, "skip") || strings.EqualFold(text, "דלג") {
		wd.Keywords = ""
	} else {
		if len(text) > maxKeywordsLen {
			b.send(ctx, chatID, locale.T(lang, "error_generic"))
			return
		}
		wd.Keywords = normalizeKeywords(text)
	}

	if !b.saveWizardStateOrAbort(ctx, chatID, StateAskExcludeKeys, wd) {
		return
	}
	b.sendWithKeyboard(ctx, chatID,
		locale.T(lang, "wizard_exclude_keys_prompt"),
		skipKeyboard(cbSkipExcludeKeys, lang))
}

func (b *Bot) handleExcludeKeysInput(ctx context.Context, chatID int64, text string) {
	wd := b.loadWizardData(ctx, chatID)
	lang := b.getUserLang(ctx, chatID)

	skip := locale.T(lang, "wizard_keywords_skip")
	if strings.EqualFold(text, skip) || strings.EqualFold(text, "skip") || strings.EqualFold(text, "דלג") {
		wd.ExcludeKeys = ""
	} else {
		if len(text) > maxKeywordsLen {
			b.send(ctx, chatID, locale.T(lang, "error_generic"))
			return
		}
		wd.ExcludeKeys = normalizeKeywords(text)
	}

	if !b.saveWizardStateOrAbort(ctx, chatID, StateConfirm, wd) {
		return
	}
	kb, summary := confirmKeyboard(wd, lang)
	b.sendWithKeyboard(ctx, chatID, summary, kb)
}

func normalizeKeywords(input string) string {
	return botcore.NormalizeKeywords(input)
}

func (b *Bot) handleManufacturerSearch(ctx context.Context, chatID int64, query string) {
	wd := b.loadWizardData(ctx, chatID)
	if !b.saveWizardStateOrAbort(ctx, chatID, StateAskManufacturer, wd) {
		return
	}
	lang := b.getUserLang(ctx, chatID)
	b.sendWithKeyboard(ctx, chatID,
		locale.T(lang, "wizard_mfr_results"),
		b.manufacturerSearchResults(query, lang))
}

func (b *Bot) handleModelSearch(ctx context.Context, chatID int64, query string) {
	wd := b.loadWizardData(ctx, chatID)
	if !b.saveWizardStateOrAbort(ctx, chatID, StateAskModel, wd) {
		return
	}
	lang := b.getUserLang(ctx, chatID)
	b.sendWithKeyboard(ctx, chatID,
		locale.T(lang, "wizard_model_results"),
		b.modelSearchResults(wd.Manufacturer, query, lang))
}
