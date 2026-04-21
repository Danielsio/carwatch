package bot

import (
	"fmt"
	"strconv"

	tgmodels "github.com/go-telegram/bot/models"

	"github.com/dsionov/carwatch/internal/catalog"
	"github.com/dsionov/carwatch/internal/format"
)

const (
	cbPrefixSource    = "src:"
	cbPrefixMfr       = "mfr:"
	cbPrefixModel     = "mdl:"
	cbPrefixEngine    = "eng:"
	cbConfirm         = "confirm:yes"
	cbEdit            = "confirm:edit"
	cbCancel          = "confirm:cancel"
	cbDeleteSearch    = "del:"
	cbPrefixShareCopy = "share_copy:"
	cbDigestOn        = "digest:on"
	cbDigestOff       = "digest:off"
	cbMfrPage         = "mfr_pg:"
	cbMfrSearch       = "mfr_search"
	cbMdlPage         = "mdl_pg:"
	cbMdlSearch       = "mdl_search"
	cbAnyModel        = "mdl:0"
	cbHistoryPage     = "hist_pg:"

	pageSize = 15
	colsPerRow = 3
)

func sourceKeyboard() *tgmodels.InlineKeyboardMarkup {
	return &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "Yad2", CallbackData: cbPrefixSource + "yad2"},
				{Text: "WinWin", CallbackData: cbPrefixSource + "winwin"},
			},
		},
	}
}

func (b *Bot) manufacturerKeyboard(page int) *tgmodels.InlineKeyboardMarkup {
	mfrs := b.catalog.Manufacturers()
	return paginatedKeyboard(mfrs, page, cbPrefixMfr, cbMfrPage, cbMfrSearch, "")
}

func (b *Bot) manufacturerSearchResults(query string) *tgmodels.InlineKeyboardMarkup {
	results := b.catalog.SearchManufacturers(query)
	return searchResultKeyboard(results, cbPrefixMfr, cbMfrPage)
}

func (b *Bot) modelKeyboard(manufacturerID int, page int) *tgmodels.InlineKeyboardMarkup {
	models := b.catalog.Models(manufacturerID)
	return paginatedKeyboard(models, page, cbPrefixModel, cbMdlPage, cbMdlSearch, cbAnyModel)
}

func (b *Bot) modelSearchResults(manufacturerID int, query string) *tgmodels.InlineKeyboardMarkup {
	results := b.catalog.SearchModels(manufacturerID, query)
	return searchResultKeyboard(results, cbPrefixModel, cbMdlPage)
}

func paginatedKeyboard(entries []catalog.Entry, page int, selectPrefix, pagePrefix, searchCB, anyCB string) *tgmodels.InlineKeyboardMarkup {
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
		{Text: "Search", CallbackData: searchCB},
	})

	if anyCB != "" {
		rows = append(rows, []tgmodels.InlineKeyboardButton{
			{Text: "Any model", CallbackData: anyCB},
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
				Text: "Previous", CallbackData: pagePrefix + strconv.Itoa(page-1),
			})
		}
		navRow = append(navRow, tgmodels.InlineKeyboardButton{
			Text: fmt.Sprintf("%d/%d", page+1, totalPages), CallbackData: "noop",
		})
		if page < totalPages-1 {
			navRow = append(navRow, tgmodels.InlineKeyboardButton{
				Text: "Next", CallbackData: pagePrefix + strconv.Itoa(page+1),
			})
		}
		rows = append(rows, navRow)
	}

	return &tgmodels.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func searchResultKeyboard(results []catalog.Entry, selectPrefix, backPageCB string) *tgmodels.InlineKeyboardMarkup {
	var rows [][]tgmodels.InlineKeyboardButton

	if len(results) == 0 {
		rows = append(rows, []tgmodels.InlineKeyboardButton{
			{Text: "No results found", CallbackData: "noop"},
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
		{Text: "Back to list", CallbackData: backPageCB + "0"},
	})

	return &tgmodels.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func engineKeyboard() *tgmodels.InlineKeyboardMarkup {
	return &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "Any engine", CallbackData: cbPrefixEngine + "0"},
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

func sourceDisplayName(source string) string {
	switch source {
	case "winwin":
		return "WinWin"
	default:
		return "Yad2"
	}
}

func confirmKeyboard(data WizardData) (*tgmodels.InlineKeyboardMarkup, string) {
	engineStr := "Any"
	if data.EngineMinCC > 0 {
		engineStr = fmt.Sprintf("%.1fL+", float64(data.EngineMinCC)/1000)
	}

	source := data.Source
	if source == "" {
		source = "yad2"
	}

	modelDisplay := data.ModelName
	if data.Model == 0 {
		modelDisplay = "Any model"
	}

	summary := fmt.Sprintf(
		"*Your search:*\n"+
			"Source: %s\n"+
			"Car: %s %s\n"+
			"Year: %d–%d\n"+
			"Max price: %s NIS\n"+
			"Engine: %s",
		sourceDisplayName(source),
		data.ManufacturerName, modelDisplay,
		data.YearMin, data.YearMax,
		format.Number(data.PriceMax),
		engineStr,
	)

	kb := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "Confirm", CallbackData: cbConfirm},
				{Text: "Start over", CallbackData: cbEdit},
				{Text: "Cancel", CallbackData: cbCancel},
			},
		},
	}

	return kb, summary
}
