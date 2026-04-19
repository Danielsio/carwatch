package notifier

import (
	"fmt"
	"strings"

	"github.com/dsionov/carwatch/internal/model"
)

func FormatListing(l model.Listing) string {
	var b strings.Builder

	b.WriteString("🚗 *New Car Listing*\n\n")

	title := strings.TrimSpace(l.Manufacturer + " " + l.Model)
	if l.SubModel != "" {
		title += " " + l.SubModel
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
			b.WriteString(", " + l.GearBox)
		}
		b.WriteString("\n")
	}

	if l.HorsePower > 0 {
		b.WriteString(fmt.Sprintf("🐴 Power: %d HP\n", l.HorsePower))
	}

	if l.Km > 0 {
		b.WriteString(fmt.Sprintf("🛣️ Mileage: %s km\n", formatNumber(l.Km)))
	}

	if l.Hand > 0 {
		b.WriteString(fmt.Sprintf("✋ Hand: %d\n", l.Hand))
	}

	if l.City != "" {
		location := l.City
		if l.Area != "" {
			location += ", " + l.Area
		}
		b.WriteString(fmt.Sprintf("📍 Location: %s\n", location))
	}

	if l.Price > 0 {
		b.WriteString(fmt.Sprintf("💰 Price: ₪%s\n", formatNumber(l.Price)))
	}

	if l.PageLink != "" {
		b.WriteString(fmt.Sprintf("\n🔗 %s", l.PageLink))
	}

	return b.String()
}

func FormatPriceDrop(l model.Listing, oldPrice int) string {
	var b strings.Builder

	title := strings.TrimSpace(l.Manufacturer + " " + l.Model)
	if l.SubModel != "" {
		title += " " + l.SubModel
	}
	if l.Year > 0 {
		title += fmt.Sprintf(" %d", l.Year)
	}

	drop := oldPrice - l.Price
	b.WriteString(fmt.Sprintf("💰 *Price Drop!* %s: ₪%s → ₪%s (-₪%s)\n",
		title,
		formatNumber(oldPrice),
		formatNumber(l.Price),
		formatNumber(drop),
	))

	if l.Km > 0 {
		b.WriteString(fmt.Sprintf("🛣️ %s km", formatNumber(l.Km)))
		if l.Hand > 0 {
			b.WriteString(fmt.Sprintf(" · ✋ Hand %d", l.Hand))
		}
		b.WriteString("\n")
	}

	if l.PageLink != "" {
		b.WriteString(fmt.Sprintf("🔗 %s", l.PageLink))
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

func formatNumber(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	var result strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result.WriteRune(',')
		}
		result.WriteRune(c)
	}
	return result.String()
}
