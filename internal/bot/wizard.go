package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"

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

	b.sendWithKeyboard(ctx, chatID,
		"Which marketplaces do you want to search? (select one or both)",
		sourceKeyboard(selected))
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
	if wd.Source == "" {
		unlock()
		b.sendWithKeyboard(ctx, chatID,
			"Please select at least one marketplace.",
			sourceKeyboard(""))
		return
	}
	b.logger.Debug("sources selected", "chat_id", chatID, "source", wd.Source)
	b.saveWizardState(ctx, chatID, StateAskManufacturer, wd)
	unlock()

	b.sendWithKeyboard(ctx, chatID,
		"What manufacturer are you looking for?",
		b.manufacturerKeyboard(ctx, chatID, 0))
}

func toggleSource(current, toggle string) string {
	sources := make(map[string]bool)
	if current != "" {
		for _, s := range strings.Split(current, ",") {
			sources[s] = true
		}
	}
	if sources[toggle] {
		delete(sources, toggle)
	} else {
		sources[toggle] = true
	}
	var result []string
	for _, s := range []string{"yad2", "winwin"} {
		if sources[s] {
			result = append(result, s)
		}
	}
	return strings.Join(result, ",")
}

func (b *Bot) onMfrPage(ctx context.Context, chatID int64, data string) {
	pageStr := strings.TrimPrefix(data, cbMfrPage)
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		b.logger.Warn("invalid manufacturer page callback", "chat_id", chatID, "raw", pageStr, "error", err)
		b.send(ctx, chatID, "Something went wrong. Please try again.")
		return
	}
	b.sendWithKeyboard(ctx, chatID,
		"What manufacturer are you looking for?",
		b.manufacturerKeyboard(ctx, chatID, page))
}

func (b *Bot) onMfrSearch(ctx context.Context, chatID int64) {
	wd := b.loadWizardData(ctx, chatID)
	b.saveWizardState(ctx, chatID, StateSearchManufacturer, wd)
	b.send(ctx, chatID, "Type the manufacturer name:")
}

func (b *Bot) onMdlPage(ctx context.Context, chatID int64, data string) {
	pageStr := strings.TrimPrefix(data, cbMdlPage)
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		b.logger.Warn("invalid model page callback", "chat_id", chatID, "raw", pageStr, "error", err)
		b.send(ctx, chatID, "Something went wrong. Please try again.")
		return
	}
	wd := b.loadWizardData(ctx, chatID)
	b.sendWithKeyboard(ctx, chatID,
		fmt.Sprintf("Which %s model?", wd.ManufacturerName),
		b.modelKeyboard(wd.Manufacturer, page))
}

func (b *Bot) onMdlSearch(ctx context.Context, chatID int64) {
	wd := b.loadWizardData(ctx, chatID)
	b.saveWizardState(ctx, chatID, StateSearchModel, wd)
	b.send(ctx, chatID, fmt.Sprintf("Type the %s model name:", wd.ManufacturerName))
}

func (b *Bot) onManufacturerSelected(ctx context.Context, chatID int64, data string) {
	idStr := strings.TrimPrefix(data, cbPrefixMfr)
	id, err := strconv.Atoi(idStr)
	if err != nil {
		b.logger.Error("invalid manufacturer ID in callback", "chat_id", chatID, "raw", idStr, "error", err)
		b.send(ctx, chatID, "Something went wrong. Use /cancel and try again.")
		return
	}

	wd := b.loadWizardData(ctx, chatID)
	wd.Manufacturer = id
	wd.ManufacturerName = b.catalog.ManufacturerName(id)
	b.logger.Debug("manufacturer selected", "chat_id", chatID, "id", id, "name", wd.ManufacturerName)
	b.saveWizardState(ctx, chatID, StateAskModel, wd)

	b.sendWithKeyboard(ctx, chatID,
		fmt.Sprintf("Which %s model?", wd.ManufacturerName),
		b.modelKeyboard(id, 0))
}

func (b *Bot) onModelSelected(ctx context.Context, chatID int64, data string) {
	idStr := strings.TrimPrefix(data, cbPrefixModel)
	modelID, err := strconv.Atoi(idStr)
	if err != nil {
		b.logger.Error("invalid model ID in callback", "chat_id", chatID, "raw", idStr, "error", err)
		b.send(ctx, chatID, "Something went wrong. Use /cancel and try again.")
		return
	}

	wd := b.loadWizardData(ctx, chatID)
	wd.Model = modelID
	wd.ModelName = b.modelDisplayName(wd.Manufacturer, modelID)
	b.logger.Debug("model selected", "chat_id", chatID, "manufacturer", wd.ManufacturerName, "model_id", modelID, "model_name", wd.ModelName)
	b.saveWizardState(ctx, chatID, StateAskYearMin, wd)

	b.send(ctx, chatID, "From which year? (e.g. 2018)")
}

func (b *Bot) onEngineSelected(ctx context.Context, chatID int64, data string) {
	ccStr := strings.TrimPrefix(data, cbPrefixEngine)
	cc, err := strconv.Atoi(ccStr)
	if err != nil {
		b.logger.Error("invalid engine CC in callback", "chat_id", chatID, "raw", ccStr, "error", err)
		b.send(ctx, chatID, "Something went wrong. Use /cancel and try again.")
		return
	}

	wd := b.loadWizardData(ctx, chatID)
	wd.EngineMinCC = cc
	b.logger.Debug("engine selected", "chat_id", chatID, "engine_min_cc", cc)
	b.saveWizardState(ctx, chatID, StateAskMaxKm, wd)

	b.sendWithKeyboard(ctx, chatID, "Maximum kilometers?", maxKmKeyboard())
}

func (b *Bot) onMaxKmSelected(ctx context.Context, chatID int64, data string) {
	kmStr := strings.TrimPrefix(data, cbPrefixMaxKm)
	km, err := strconv.Atoi(kmStr)
	if err != nil {
		b.send(ctx, chatID, "Something went wrong. Use /cancel and try again.")
		return
	}

	wd := b.loadWizardData(ctx, chatID)
	wd.MaxKm = km
	b.saveWizardState(ctx, chatID, StateAskMaxHand, wd)

	b.sendWithKeyboard(ctx, chatID, "Maximum ownership hand?", maxHandKeyboard())
}

func (b *Bot) onMaxHandSelected(ctx context.Context, chatID int64, data string) {
	handStr := strings.TrimPrefix(data, cbPrefixMaxHand)
	hand, err := strconv.Atoi(handStr)
	if err != nil {
		b.send(ctx, chatID, "Something went wrong. Use /cancel and try again.")
		return
	}

	wd := b.loadWizardData(ctx, chatID)
	wd.MaxHand = hand
	b.saveWizardState(ctx, chatID, StateConfirm, wd)

	kb, summary := confirmKeyboard(wd)
	b.sendWithKeyboard(ctx, chatID, summary, kb)
}

func (b *Bot) onConfirm(ctx context.Context, chatID int64) {
	// Verify the user's wizard session is still active.
	user, err := b.users.GetUser(ctx, chatID)
	if err != nil {
		b.logger.Error("get user failed in onConfirm", "chat_id", chatID, "error", err)
		b.send(ctx, chatID, "Something went wrong. Please try again.")
		return
	}
	if user == nil || user.State == StateIdle {
		b.send(ctx, chatID, "Session expired. Use /watch to start a new search.")
		return
	}

	wd := b.loadWizardData(ctx, chatID)
	b.logger.Debug("confirm clicked", "chat_id", chatID, "wizard_data", wd)

	source := wd.Source
	if source == "" {
		source = "yad2,winwin"
	}

	name := fmt.Sprintf("%s-%s", strings.ToLower(wd.ManufacturerName), strings.ToLower(wd.ModelName))
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
	})
	if err != nil {
		b.logger.Error("create search failed", "error", err)
		b.send(ctx, chatID, "Failed to save search. Please try again.")
		return
	}

	_ = b.users.UpdateUserState(ctx, chatID, StateIdle, "{}")
	b.send(ctx, chatID, fmt.Sprintf(
		"Search #%d saved! Checking %s now...\n\nUse /list to see your searches.",
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
	if b.isRateLimited(chatID) {
		b.logger.Warn("rate limited", "chat_id", chatID)
		return
	}
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
	default:
		if text != "" && !strings.HasPrefix(text, "/") {
			b.send(ctx, chatID, "I didn't understand that. Use /help for available commands.")
		}
	}
}

func (b *Bot) handleYearMin(ctx context.Context, chatID int64, text string) {
	b.logger.Debug("handleYearMin", "chat_id", chatID, "input", text)
	maxYear := time.Now().Year() + 2
	year, err := strconv.Atoi(text)
	if err != nil || year < 1990 || year > maxYear {
		b.logger.Debug("invalid year min", "chat_id", chatID, "input", text, "error", err)
		b.send(ctx, chatID, fmt.Sprintf("Please enter a valid year (1990–%d).", maxYear))
		return
	}

	wd := b.loadWizardData(ctx, chatID)
	wd.YearMin = year
	b.logger.Debug("year min set", "chat_id", chatID, "year_min", year)
	b.saveWizardState(ctx, chatID, StateAskYearMax, wd)
	b.send(ctx, chatID, "Until which year? (e.g. 2024)")
}

func (b *Bot) handleYearMax(ctx context.Context, chatID int64, text string) {
	b.logger.Debug("handleYearMax", "chat_id", chatID, "input", text)
	maxYear := time.Now().Year() + 2
	year, err := strconv.Atoi(text)
	if err != nil || year < 1990 || year > maxYear {
		b.logger.Debug("invalid year max", "chat_id", chatID, "input", text, "error", err)
		b.send(ctx, chatID, fmt.Sprintf("Please enter a valid year (1990–%d).", maxYear))
		return
	}

	wd := b.loadWizardData(ctx, chatID)
	if year < wd.YearMin {
		b.send(ctx, chatID, fmt.Sprintf("Must be >= %d. Try again.", wd.YearMin))
		return
	}
	wd.YearMax = year
	b.logger.Debug("year max set", "chat_id", chatID, "year_max", year)
	b.saveWizardState(ctx, chatID, StateAskPriceMax, wd)
	b.send(ctx, chatID, "Max price in NIS? (e.g. 150000)")
}

func (b *Bot) handlePriceMax(ctx context.Context, chatID int64, text string) {
	b.logger.Debug("handlePriceMax", "chat_id", chatID, "input", text)
	text = strings.ReplaceAll(text, ",", "")
	price, err := strconv.Atoi(text)
	if err != nil || price < 1000 || price > 10000000 {
		b.logger.Debug("invalid price", "chat_id", chatID, "input", text, "error", err)
		b.send(ctx, chatID, "Please enter a valid price (1,000–10,000,000).")
		return
	}

	wd := b.loadWizardData(ctx, chatID)
	wd.PriceMax = price
	b.logger.Debug("price max set", "chat_id", chatID, "price_max", price)
	b.saveWizardState(ctx, chatID, StateAskEngine, wd)
	b.sendWithKeyboard(ctx, chatID, "Minimum engine size?", engineKeyboard())
}

func (b *Bot) handleManufacturerSearch(ctx context.Context, chatID int64, query string) {
	wd := b.loadWizardData(ctx, chatID)
	b.saveWizardState(ctx, chatID, StateAskManufacturer, wd)
	b.sendWithKeyboard(ctx, chatID,
		"Search results:",
		b.manufacturerSearchResults(query))
}

func (b *Bot) handleModelSearch(ctx context.Context, chatID int64, query string) {
	wd := b.loadWizardData(ctx, chatID)
	b.saveWizardState(ctx, chatID, StateAskModel, wd)
	b.sendWithKeyboard(ctx, chatID,
		"Search results:",
		b.modelSearchResults(wd.Manufacturer, query))
}
