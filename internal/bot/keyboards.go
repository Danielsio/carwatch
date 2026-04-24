package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tgmodels "github.com/go-telegram/bot/models"

	"github.com/dsionov/carwatch/internal/catalog"
	"github.com/dsionov/carwatch/internal/format"
	"github.com/dsionov/carwatch/internal/locale"
)

const maxRecentManufacturers = 4

const (
	cbPrefixSource    = "src:"
	cbPrefixMfr       = "mfr:"
	cbPrefixModel     = "mdl:"
	cbPrefixEngine    = "eng:"
	cbPrefixMaxKm     = "maxkm:"
	cbPrefixMaxHand   = "maxhand:"
	cbConfirm         = "confirm:yes"
	cbEdit            = "confirm:edit"
	cbCancel          = "confirm:cancel"
	cbDeleteSearch    = "del:"
	cbPrefixShareCopy = "share_copy:"
	cbDigestOn        = "digest:on"
	cbDigestOff       = "digest:off"
	cbDigestInterval  = "digest_int:"
	cbMfrPage         = "mfr_pg:"
	cbMfrSearch       = "mfr_search"
	cbMdlPage         = "mdl_pg:"
	cbMdlSearch       = "mdl_search"
	cbAnyModel        = "mdl:0"
	cbHistoryPage     = "hist_pg:"
	cbSourceToggle    = "src_toggle:"
	cbSourceDone      = "src_done"
	cbLangHe          = "lang:he"
	cbLangEn          = "lang:en"
	cbPrefixSave      = "save:"
	cbPrefixHide      = "hide:"
	cbQuickStart      = "quick_start"
	cbHiddenClear     = "hidden_clear"
	cbSavedPage       = "saved_pg:"
	cbHiddenPage      = "hidden_pg:"
	cbSkipKeywords    = "skip_keywords"
	cbSkipExcludeKeys = "skip_exclude_keys"

	pageSize   = 15
	colsPerRow = 3
)

func sourceKeyboard(selected string, lang locale.Lang) *tgmodels.InlineKeyboardMarkup {
	yad2Label := "Yad2"
	winwinLabel := "WinWin"
	if strings.Contains(selected, "yad2") {
		yad2Label = "✅ Yad2"
	}
	if strings.Contains(selected, "winwin") {
		winwinLabel = "✅ WinWin"
	}

	rows := [][]tgmodels.InlineKeyboardButton{
		{
			{Text: yad2Label, CallbackData: cbSourceToggle + "yad2"},
			{Text: winwinLabel, CallbackData: cbSourceToggle + "winwin"},
		},
	}
	if selected != "" {
		rows = append(rows, []tgmodels.InlineKeyboardButton{
			{Text: locale.T(lang, "btn_done"), CallbackData: cbSourceDone},
		})
	}

	return &tgmodels.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func (b *Bot) manufacturerKeyboard(ctx context.Context, chatID int64, page int, lang locale.Lang) *tgmodels.InlineKeyboardMarkup {
	mfrs := b.catalog.Manufacturers()
	kb := paginatedKeyboard(mfrs, page, cbPrefixMfr, cbMfrPage, cbMfrSearch, "", lang)

	if page == 0 {
		recent := b.recentManufacturers(ctx, chatID)
		if len(recent) > 0 {
			var recentRows [][]tgmodels.InlineKeyboardButton

			var row []tgmodels.InlineKeyboardButton
			for i, e := range recent {
				row = append(row, tgmodels.InlineKeyboardButton{
					Text:         e.Name,
					CallbackData: cbPrefixMfr + strconv.Itoa(e.ID),
				})
				if len(row) == colsPerRow || i == len(recent)-1 {
					recentRows = append(recentRows, row)
					row = nil
				}
			}
			recentRows = append(recentRows, []tgmodels.InlineKeyboardButton{
				{Text: "───────────", CallbackData: "noop"},
			})

			newRows := make([][]tgmodels.InlineKeyboardButton, 0, len(kb.InlineKeyboard)+len(recentRows))
			newRows = append(newRows, kb.InlineKeyboard[0]) // search button
			newRows = append(newRows, recentRows...)
			newRows = append(newRows, kb.InlineKeyboard[1:]...)
			kb.InlineKeyboard = newRows
		}
	}

	return kb
}

func (b *Bot) recentManufacturers(ctx context.Context, chatID int64) []catalog.Entry {
	searches, err := b.searches.ListSearches(ctx, chatID)
	if err != nil || len(searches) == 0 {
		return nil
	}

	seen := make(map[int]bool)
	var recent []catalog.Entry
	for _, s := range searches {
		if seen[s.Manufacturer] {
			continue
		}
		seen[s.Manufacturer] = true
		name := b.catalog.ManufacturerName(s.Manufacturer)
		if name == "" {
			continue
		}
		recent = append(recent, catalog.Entry{ID: s.Manufacturer, Name: name})
		if len(recent) >= maxRecentManufacturers {
			break
		}
	}
	return recent
}

func (b *Bot) manufacturerSearchResults(query string, lang locale.Lang) *tgmodels.InlineKeyboardMarkup {
	results := b.catalog.SearchManufacturers(query)
	return searchResultKeyboard(results, cbPrefixMfr, cbMfrPage, lang)
}

func (b *Bot) modelKeyboard(manufacturerID int, page int, lang locale.Lang) *tgmodels.InlineKeyboardMarkup {
	models := b.catalog.Models(manufacturerID)
	return paginatedKeyboard(models, page, cbPrefixModel, cbMdlPage, cbMdlSearch, cbAnyModel, lang)
}

func (b *Bot) modelSearchResults(manufacturerID int, query string, lang locale.Lang) *tgmodels.InlineKeyboardMarkup {
	results := b.catalog.SearchModels(manufacturerID, query)
	return searchResultKeyboard(results, cbPrefixModel, cbMdlPage, lang)
}

func paginatedKeyboard(entries []catalog.Entry, page int, selectPrefix, pagePrefix, searchCB, anyCB string, lang locale.Lang) *tgmodels.InlineKeyboardMarkup {
	totalPages := (len(entries) + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}
	page = max(page, 0)
	page = min(page, totalPages-1)

	start := page * pageSize
	end := start + pageSize
	if end > len(entries) {
		end = len(entries)
	}
	pageEntries := entries[start:end]

	var rows [][]tgmodels.InlineKeyboardButton

	rows = append(rows, []tgmodels.InlineKeyboardButton{
		{Text: locale.T(lang, "btn_search"), CallbackData: searchCB},
	})

	if anyCB != "" {
		rows = append(rows, []tgmodels.InlineKeyboardButton{
			{Text: locale.T(lang, "btn_any_model"), CallbackData: anyCB},
		})
	}

	var row []tgmodels.InlineKeyboardButton
	for i, e := range pageEntries {
		row = append(row, tgmodels.InlineKeyboardButton{
			Text:         e.Name,
			CallbackData: selectPrefix + strconv.Itoa(e.ID),
		})
		if len(row) == colsPerRow || i == len(pageEntries)-1 {
			rows = append(rows, row)
			row = nil
		}
	}

	if totalPages > 1 {
		var navRow []tgmodels.InlineKeyboardButton
		if page > 0 {
			navRow = append(navRow, tgmodels.InlineKeyboardButton{
				Text: locale.T(lang, "btn_previous"), CallbackData: pagePrefix + strconv.Itoa(page-1),
			})
		}
		navRow = append(navRow, tgmodels.InlineKeyboardButton{
			Text: fmt.Sprintf("%d/%d", page+1, totalPages), CallbackData: "noop",
		})
		if page < totalPages-1 {
			navRow = append(navRow, tgmodels.InlineKeyboardButton{
				Text: locale.T(lang, "btn_next"), CallbackData: pagePrefix + strconv.Itoa(page+1),
			})
		}
		rows = append(rows, navRow)
	}

	return &tgmodels.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func searchResultKeyboard(results []catalog.Entry, selectPrefix, backPageCB string, lang locale.Lang) *tgmodels.InlineKeyboardMarkup {
	var rows [][]tgmodels.InlineKeyboardButton

	if len(results) == 0 {
		rows = append(rows, []tgmodels.InlineKeyboardButton{
			{Text: locale.T(lang, "btn_no_results"), CallbackData: "noop"},
		})
	} else {
		var row []tgmodels.InlineKeyboardButton
		for i, e := range results {
			row = append(row, tgmodels.InlineKeyboardButton{
				Text:         e.Name,
				CallbackData: selectPrefix + strconv.Itoa(e.ID),
			})
			if len(row) == colsPerRow || i == len(results)-1 {
				rows = append(rows, row)
				row = nil
			}
		}
	}

	rows = append(rows, []tgmodels.InlineKeyboardButton{
		{Text: locale.T(lang, "btn_back"), CallbackData: backPageCB + "0"},
	})

	return &tgmodels.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func engineKeyboard(lang locale.Lang) *tgmodels.InlineKeyboardMarkup {
	return &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: locale.T(lang, "btn_any_engine"), CallbackData: cbPrefixEngine + "0"},
				{Text: "1.5L+", CallbackData: cbPrefixEngine + "1500"},
				{Text: "2.0L+", CallbackData: cbPrefixEngine + "2000"},
			},
			{
				{Text: "2.5L+", CallbackData: cbPrefixEngine + "2500"},
				{Text: "3.0L+", CallbackData: cbPrefixEngine + "3000"},
			},
		},
	}
}

func maxKmKeyboard(lang locale.Lang) *tgmodels.InlineKeyboardMarkup {
	return &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: locale.T(lang, "btn_any"), CallbackData: cbPrefixMaxKm + "0"},
				{Text: "50,000", CallbackData: cbPrefixMaxKm + "50000"},
				{Text: "100,000", CallbackData: cbPrefixMaxKm + "100000"},
			},
			{
				{Text: "150,000", CallbackData: cbPrefixMaxKm + "150000"},
				{Text: "200,000", CallbackData: cbPrefixMaxKm + "200000"},
			},
		},
	}
}

func maxHandKeyboard(lang locale.Lang) *tgmodels.InlineKeyboardMarkup {
	return &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: locale.T(lang, "btn_any"), CallbackData: cbPrefixMaxHand + "0"},
				{Text: locale.T(lang, "btn_hand_1"), CallbackData: cbPrefixMaxHand + "1"},
				{Text: locale.T(lang, "btn_hand_2"), CallbackData: cbPrefixMaxHand + "2"},
			},
			{
				{Text: locale.T(lang, "btn_hand_3"), CallbackData: cbPrefixMaxHand + "3"},
				{Text: locale.T(lang, "btn_hand_4"), CallbackData: cbPrefixMaxHand + "4"},
			},
		},
	}
}

func skipKeyboard(cbData string, lang locale.Lang) *tgmodels.InlineKeyboardMarkup {
	return &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: locale.T(lang, "btn_skip"), CallbackData: cbData},
			},
		},
	}
}

func sourceDisplayName(source string) string {
	if strings.TrimSpace(source) == "" {
		return "Yad2, WinWin"
	}
	parts := strings.Split(source, ",")
	names := make([]string, 0, len(parts))
	for _, p := range parts {
		switch strings.TrimSpace(p) {
		case "winwin":
			names = append(names, "WinWin")
		case "yad2":
			names = append(names, "Yad2")
		}
	}
	if len(names) == 0 {
		return "Yad2, WinWin"
	}
	return strings.Join(names, ", ")
}

func confirmKeyboard(data WizardData, lang locale.Lang) (*tgmodels.InlineKeyboardMarkup, string) {
	engineStr := locale.T(lang, "label_any")
	if data.EngineMinCC > 0 {
		engineStr = fmt.Sprintf("%.1fL+", float64(data.EngineMinCC)/1000)
	}

	kmStr := locale.T(lang, "label_any")
	if data.MaxKm > 0 {
		kmStr = format.Number(data.MaxKm) + " km"
	}

	handStr := locale.T(lang, "label_any")
	if data.MaxHand > 0 {
		handStr = strconv.Itoa(data.MaxHand)
	}

	source := data.Source
	if source == "" {
		source = "yad2,winwin"
	}

	modelDisplay := data.ModelName
	if data.Model == 0 {
		modelDisplay = locale.T(lang, "btn_any_model")
	}

	summary := locale.Tf(lang, "wizard_confirm_summary",
		sourceDisplayName(source),
		data.ManufacturerName, modelDisplay,
		data.YearMin, data.YearMax,
		format.Number(data.PriceMax),
		engineStr,
		kmStr,
		handStr,
	)

	if data.Keywords != "" {
		summary += locale.Tf(lang, "wizard_confirm_keywords", data.Keywords)
	}
	if data.ExcludeKeys != "" {
		summary += locale.Tf(lang, "wizard_confirm_exclude_keys", data.ExcludeKeys)
	}

	kb := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: locale.T(lang, "btn_confirm"), CallbackData: cbConfirm},
				{Text: locale.T(lang, "btn_start_over"), CallbackData: cbEdit},
				{Text: locale.T(lang, "btn_cancel"), CallbackData: cbCancel},
			},
		},
	}

	return kb, summary
}

func ListingActionKeyboard(token string, lang locale.Lang) *tgmodels.InlineKeyboardMarkup {
	return &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: locale.T(lang, "btn_save"), CallbackData: cbPrefixSave + token},
				{Text: locale.T(lang, "btn_hide"), CallbackData: cbPrefixHide + token},
			},
		},
	}
}
