package notifier

import (
	"fmt"
	"strings"

	"github.com/dsionov/carwatch/internal/format"
	"github.com/dsionov/carwatch/internal/model"
)

func FormatListing(l model.Listing) string {
	var b strings.Builder

	b.WriteString("🚗 *New Car Listing*\n\n")

	title := format.EscapeMarkdown(strings.TrimSpace(l.Manufacturer + " " + l.Model))
	if l.SubModel != "" {
		title += " " + format.EscapeMarkdown(l.SubModel)
	}
	b.WriteString("*" + title + "*\n\n")

	if l.Year > 0 {
		b.WriteString(fmt.Sprintf("📅 Year: %d", l.Year))
		if l.Month > 0 {
			b.WriteString(fmt.Sprintf("/%02d", l.Month))
		}
		b.WriteString("\n")
	}

	if l.EngineVolume > 0 {
		b.WriteString(fmt.Sprintf("⚙️ Engine: %.1fL", l.EngineVolume/1000))
		if l.GearBox != "" {
			b.WriteString(", " + format.EscapeMarkdown(l.GearBox))
		}
		b.WriteString("\n")
	}

	if l.HorsePower > 0 {
		b.WriteString(fmt.Sprintf("🐴 Power: %d HP\n", l.HorsePower))
	}

	if l.Km > 0 {
		b.WriteString(fmt.Sprintf("🛣️ Mileage: %s km\n", format.Number(l.Km)))
	}

	if l.Hand > 0 {
		b.WriteString(fmt.Sprintf("✋ Hand: %d\n", l.Hand))
	}

	if l.City != "" {
		location := format.EscapeMarkdown(l.City)
		if l.Area != "" {
			location += ", " + format.EscapeMarkdown(l.Area)
		}
		b.WriteString(fmt.Sprintf("📍 Location: %s\n", location))
	}

	if l.Price > 0 {
		b.WriteString(fmt.Sprintf("💰 Price: ₪%s\n", format.Number(l.Price)))
	}

	if l.PageLink != "" {
		b.WriteString(fmt.Sprintf("\n🔗 %s", format.EscapeMarkdown(l.PageLink)))
	}

	return b.String()
}

func FormatPriceDrop(l model.Listing, oldPrice int) string {
	var b strings.Builder

	title := format.EscapeMarkdown(strings.TrimSpace(l.Manufacturer + " " + l.Model))
	if l.SubModel != "" {
		title += " " + format.EscapeMarkdown(l.SubModel)
	}
	if l.Year > 0 {
		title += fmt.Sprintf(" %d", l.Year)
	}

	drop := oldPrice - l.Price
	b.WriteString(fmt.Sprintf("💰 *Price Drop!* %s: ₪%s → ₪%s (-₪%s)\n",
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

func FormatBatch(listings []model.Listing) string {
	if len(listings) == 1 {
		return FormatListing(listings[0])
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("🚗 *%d New Listings Found*\n", len(listings)))

	for i, l := range listings {
		b.WriteString("\n━━━━━━━━━━━━━━━━━━━━\n")
		b.WriteString(fmt.Sprintf("*[%d/%d]*\n", i+1, len(listings)))
		b.WriteString(FormatListing(l))
	}

	return b.String()
}
