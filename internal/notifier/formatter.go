package notifier

import (
	"fmt"
	"strings"

	"github.com/dsionov/carwatch/internal/format"
	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/model"
)

func FormatListing(l model.Listing, lang locale.Lang) string {
	var b strings.Builder

	b.WriteString(locale.T(lang, "fmt_new_listing"))

	title := format.EscapeMarkdown(strings.TrimSpace(l.Manufacturer + " " + l.Model))
	if l.SubModel != "" {
		title += " " + format.EscapeMarkdown(l.SubModel)
	}
	b.WriteString("*" + title + "*\n\n")

	if l.Year > 0 {
		b.WriteString(locale.Tf(lang, "fmt_year", l.Year))
		if l.Month > 0 {
			b.WriteString(locale.Tf(lang, "fmt_year_month", l.Month))
		}
		b.WriteString("\n")
	}

	if l.EngineVolume > 0 {
		b.WriteString(locale.Tf(lang, "fmt_engine", l.EngineVolume/1000))
		if l.GearBox != "" {
			b.WriteString(", " + format.EscapeMarkdown(l.GearBox))
		}
		b.WriteString("\n")
	}

	if l.HorsePower > 0 {
		b.WriteString(locale.Tf(lang, "fmt_power", l.HorsePower))
	}

	if l.Km > 0 {
		b.WriteString(locale.Tf(lang, "fmt_mileage", format.Number(l.Km)))
	}

	if l.Hand > 0 {
		b.WriteString(locale.Tf(lang, "fmt_hand", l.Hand))
	}

	if l.City != "" {
		location := format.EscapeMarkdown(l.City)
		if l.Area != "" {
			location += ", " + format.EscapeMarkdown(l.Area)
		}
		b.WriteString(locale.Tf(lang, "fmt_location", location))
	}

	if l.Price > 0 {
		b.WriteString(locale.Tf(lang, "fmt_price", format.Number(l.Price)))
	}

	if l.PageLink != "" {
		b.WriteString(fmt.Sprintf("\n🔗 %s", format.EscapeMarkdown(l.PageLink)))
	}

	return b.String()
}

func FormatPriceDrop(l model.Listing, oldPrice int, lang locale.Lang) string {
	var b strings.Builder

	title := format.EscapeMarkdown(strings.TrimSpace(l.Manufacturer + " " + l.Model))
	if l.SubModel != "" {
		title += " " + format.EscapeMarkdown(l.SubModel)
	}
	if l.Year > 0 {
		title += fmt.Sprintf(" %d", l.Year)
	}

	drop := oldPrice - l.Price
	b.WriteString(locale.Tf(lang, "fmt_price_drop",
		title,
		format.Number(oldPrice),
		format.Number(l.Price),
		format.Number(drop),
	))

	if l.Km > 0 {
		b.WriteString(fmt.Sprintf("🛣️ %s km", format.Number(l.Km)))
		if l.Hand > 0 {
			b.WriteString(fmt.Sprintf(" · ✋ Hand %d", l.Hand))
		}
		b.WriteString("\n")
	}

	if l.PageLink != "" {
		b.WriteString(fmt.Sprintf("🔗 %s", format.EscapeMarkdown(l.PageLink)))
	}

	return b.String()
}

func FormatBatch(listings []model.Listing, lang locale.Lang) string {
	if len(listings) == 1 {
		return FormatListing(listings[0], lang)
	}

	var b strings.Builder
	b.WriteString(locale.Tf(lang, "fmt_batch_header", len(listings)))

	for i, l := range listings {
		b.WriteString("\n━━━━━━━━━━━━━━━━━━━━\n")
		b.WriteString(locale.Tf(lang, "fmt_batch_item", i+1, len(listings)))
		b.WriteString(FormatListing(l, lang))
	}

	return b.String()
}
