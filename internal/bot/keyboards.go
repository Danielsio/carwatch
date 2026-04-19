package bot

import (
	"fmt"
	"strconv"

	tgmodels "github.com/go-telegram/bot/models"

	"github.com/dsionov/carwatch/internal/fetcher/yad2"
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

func manufacturerKeyboard() *tgmodels.InlineKeyboardMarkup {
	mfrs := yad2.Manufacturers()
	var rows [][]tgmodels.InlineKeyboardButton
	var row []tgmodels.InlineKeyboardButton

	for i, m := range mfrs {
		row = append(row, tgmodels.InlineKeyboardButton{
			Text:         m.Name,
			CallbackData: cbPrefixMfr + strconv.Itoa(m.ID),
		})
		if len(row) == 3 || i == len(mfrs)-1 {
			rows = append(rows, row)
			row = nil
		}
	}

	return &tgmodels.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func modelKeyboard(manufacturerID int) *tgmodels.InlineKeyboardMarkup {
	models := yad2.Models(manufacturerID)
	var rows [][]tgmodels.InlineKeyboardButton
	var row []tgmodels.InlineKeyboardButton

	for i, m := range models {
		row = append(row, tgmodels.InlineKeyboardButton{
			Text:         m.Name,
			CallbackData: cbPrefixModel + strconv.Itoa(m.ID),
		})
		if len(row) == 3 || i == len(models)-1 {
			rows = append(rows, row)
			row = nil
		}
	}

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

	summary := fmt.Sprintf(
		"*Your search:*\n"+
			"Source: %s\n"+
			"Car: %s %s\n"+
			"Year: %d–%d\n"+
			"Max price: %s NIS\n"+
			"Engine: %s",
		sourceDisplayName(source),
		data.ManufacturerName, data.ModelName,
		data.YearMin, data.YearMax,
		formatNumber(data.PriceMax),
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

func formatNumber(n int) string {
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}
