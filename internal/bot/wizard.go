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
	b.saveWizardState(ctx, chatID, StateAskSource, wd)

	lang := b.getUserLang(ctx, chatID)
	b.sendWithKeyboard(ctx, chatID,
		locale.T(lang, "wizard_source_prompt"),
		sourceKeyboard(selected, lang))
}

func (b *Bot) onLegacySourceSelected(ctx context.Context, chatID int64, source string) {
	unlock := b.lockChat(chatID)
	wd := b.loadWizardData(ctx, chatID)
	wd.Source = source
	b.saveWizardState(ctx, chatID, StateAskSource, wd)
	unlock()

	b.onSourceDone(ctx, chatID)
}

func (b *Bot) onSourceDone(ctx context.Context, chatID int64) {
	unlock := b.lockChat(chatID)
	wd := b.loadWizardData(ctx, chatID)
	lang := b.getUserLang(ctx, chatID)
	if wd.Source == "" {
		unlock()
		b.sendWithKeyboard(ctx, chatID,
			locale.T(lang, "wizard_source_empty"),
			sourceKeyboard("", lang))
		return
	}
	b.logger.Debug("sources selected", "chat_id", chatID, "source", wd.Source)
	b.saveWizardState(ctx, chatID, StateAskManufacturer, wd)
	unlock()

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
	wd := b.loadWizardData(ctx, chatID)
	b.saveWizardState(ctx, chatID, StateSearchManufacturer, wd)
	b.send(ctx, chatID, locale.T(b.getUserLang(ctx, chatID), "wizard_mfr_search"))
}

func (b *Bot) onMdlPage(ctx context.Context, chatID int64, data string) {
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
	wd := b.loadWizardData(ctx, chatID)
	b.saveWizardState(ctx, chatID, StateSearchModel, wd)
	lang := b.getUserLang(ctx, chatID)
	b.send(ctx, chatID, locale.Tf(lang, "wizard_model_search", wd.ManufacturerName))
}

func (b *Bot) onManufacturerSelected(ctx context.Context, chatID int64, data string) {
	idStr := strings.TrimPrefix(data, cbPrefixMfr)
	id, err := strconv.Atoi(idStr)
	if err != nil {
		b.logger.Error("invalid manufacturer ID in callback", "chat_id", chatID, "raw", idStr, "error", err)
		b.send(ctx, chatID, locale.T(b.getUserLang(ctx, chatID), "error_wrong_state"))
		return
	}

	wd := b.loadWizardData(ctx, chatID)
	wd.Manufacturer = id
	wd.ManufacturerName = b.catalog.ManufacturerName(id)
	b.logger.Debug("manufacturer selected", "chat_id", chatID, "id", id, "name", wd.ManufacturerName)
	b.saveWizardState(ctx, chatID, StateAskModel, wd)

	lang := b.getUserLang(ctx, chatID)
	b.sendWithKeyboard(ctx, chatID,
		locale.Tf(lang, "wizard_model_prompt", wd.ManufacturerName),
		b.modelKeyboard(id, 0, lang))
}

func (b *Bot) onModelSelected(ctx context.Context, chatID int64, data string) {
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
	b.saveWizardState(ctx, chatID, StateAskYearMin, wd)

	b.send(ctx, chatID, locale.T(b.getUserLang(ctx, chatID), "wizard_year_min"))
}

func (b *Bot) onEngineSelected(ctx context.Context, chatID int64, data string) {
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
	b.saveWizardState(ctx, chatID, StateAskMaxKm, wd)

	lang := b.getUserLang(ctx, chatID)
	b.sendWithKeyboard(ctx, chatID, locale.T(lang, "wizard_km_prompt"), maxKmKeyboard(lang))
}

func (b *Bot) onMaxKmSelected(ctx context.Context, chatID int64, data string) {
	kmStr := strings.TrimPrefix(data, cbPrefixMaxKm)
	km, err := strconv.Atoi(kmStr)
	if err != nil {
		b.send(ctx, chatID, locale.T(b.getUserLang(ctx, chatID), "error_wrong_state"))
		return
	}

	wd := b.loadWizardData(ctx, chatID)
	wd.MaxKm = km
	b.saveWizardState(ctx, chatID, StateAskMaxHand, wd)

	lang := b.getUserLang(ctx, chatID)
	b.sendWithKeyboard(ctx, chatID, locale.T(lang, "wizard_hand_prompt"), maxHandKeyboard(lang))
}

func (b *Bot) onMaxHandSelected(ctx context.Context, chatID int64, data string) {
	handStr := strings.TrimPrefix(data, cbPrefixMaxHand)
	hand, err := strconv.Atoi(handStr)
	if err != nil {
		b.send(ctx, chatID, locale.T(b.getUserLang(ctx, chatID), "error_wrong_state"))
		return
	}

	wd := b.loadWizardData(ctx, chatID)
	wd.MaxHand = hand
	b.saveWizardState(ctx, chatID, StateAskKeywords, wd)

	lang := b.getUserLang(ctx, chatID)
	b.sendWithKeyboard(ctx, chatID,
		locale.T(lang, "wizard_keywords_prompt"),
		skipKeyboard(cbSkipKeywords, lang))
}

func (b *Bot) onSkipKeywords(ctx context.Context, chatID int64) {
	wd := b.loadWizardData(ctx, chatID)
	wd.Keywords = ""
	b.saveWizardState(ctx, chatID, StateAskExcludeKeys, wd)

	lang := b.getUserLang(ctx, chatID)
	b.sendWithKeyboard(ctx, chatID,
		locale.T(lang, "wizard_exclude_keys_prompt"),
		skipKeyboard(cbSkipExcludeKeys, lang))
}

func (b *Bot) onSkipExcludeKeys(ctx context.Context, chatID int64) {
	wd := b.loadWizardData(ctx, chatID)
	wd.ExcludeKeys = ""
	b.saveWizardState(ctx, chatID, StateConfirm, wd)

	lang := b.getUserLang(ctx, chatID)
	kb, summary := confirmKeyboard(wd, lang)
	b.sendWithKeyboard(ctx, chatID, summary, kb)
}

func (b *Bot) onConfirm(ctx context.Context, chatID int64) {
	user, err := b.users.GetUser(ctx, chatID)
	if err != nil {
		b.logger.Error("get user failed in onConfirm", "chat_id", chatID, "error", err)
		b.send(ctx, chatID, locale.T(b.getUserLang(ctx, chatID), "error_generic"))
		return
	}
	lang := b.getUserLang(ctx, chatID)
	if user == nil || user.State == StateIdle {
		b.send(ctx, chatID, locale.T(lang, "wizard_session_expired"))
		return
	}

	wd := b.loadWizardData(ctx, chatID)
	b.logger.Debug("confirm clicked", "chat_id", chatID, "wizard_data", wd)

	source := wd.Source
	if source == "" {
		source = "yad2,winwin"
	}

	name := fmt.Sprintf("%s-%s", strings.ToLower(wd.ManufacturerName), strings.ToLower(wd.ModelName))

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
			PriceMax:     wd.PriceMax,
			EngineMinCC:  wd.EngineMinCC,
			MaxKm:        wd.MaxKm,
			MaxHand:      wd.MaxHand,
			Keywords:     wd.Keywords,
			ExcludeKeys:  wd.ExcludeKeys,
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

	b.logger.Debug("creating search", "chat_id", chatID, "name", name, "source", source)
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
		MaxKm:        wd.MaxKm,
		MaxHand:      wd.MaxHand,
		Keywords:     wd.Keywords,
		ExcludeKeys:  wd.ExcludeKeys,
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
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	b.ensureUser(ctx, chatID, update.Message.From.Username)

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
	b.saveWizardState(ctx, chatID, StateAskYearMax, wd)
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
	b.saveWizardState(ctx, chatID, StateAskPriceMax, wd)
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
	b.saveWizardState(ctx, chatID, StateAskEngine, wd)
	b.sendWithKeyboard(ctx, chatID, locale.T(lang, "wizard_engine_prompt"), engineKeyboard(lang))
}

func (b *Bot) handleKeywordsInput(ctx context.Context, chatID int64, text string) {
	wd := b.loadWizardData(ctx, chatID)
	lang := b.getUserLang(ctx, chatID)

	skip := locale.T(lang, "wizard_keywords_skip")
	if strings.EqualFold(text, skip) || strings.EqualFold(text, "skip") || strings.EqualFold(text, "דלג") {
		wd.Keywords = ""
	} else {
		wd.Keywords = normalizeKeywords(text)
	}

	b.saveWizardState(ctx, chatID, StateAskExcludeKeys, wd)
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
		wd.ExcludeKeys = normalizeKeywords(text)
	}

	b.saveWizardState(ctx, chatID, StateConfirm, wd)
	kb, summary := confirmKeyboard(wd, lang)
	b.sendWithKeyboard(ctx, chatID, summary, kb)
}

func normalizeKeywords(input string) string {
	return botcore.NormalizeKeywords(input)
}

func (b *Bot) handleManufacturerSearch(ctx context.Context, chatID int64, query string) {
	wd := b.loadWizardData(ctx, chatID)
	b.saveWizardState(ctx, chatID, StateAskManufacturer, wd)
	lang := b.getUserLang(ctx, chatID)
	b.sendWithKeyboard(ctx, chatID,
		locale.T(lang, "wizard_mfr_results"),
		b.manufacturerSearchResults(query, lang))
}

func (b *Bot) handleModelSearch(ctx context.Context, chatID int64, query string) {
	wd := b.loadWizardData(ctx, chatID)
	b.saveWizardState(ctx, chatID, StateAskModel, wd)
	lang := b.getUserLang(ctx, chatID)
	b.sendWithKeyboard(ctx, chatID,
		locale.T(lang, "wizard_model_results"),
		b.modelSearchResults(wd.Manufacturer, query, lang))
}
