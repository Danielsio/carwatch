package notifier

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/dsionov/carwatch/internal/format"
	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/storage"
)

func FormatListing(l model.Listing, lang locale.Lang) string {
	var b strings.Builder

	b.WriteString(locale.T(lang, "fmt_new_listing"))

	title := format.EscapeMarkdown(strings.TrimSpace(l.Manufacturer + " " + l.Model))
	if l.SubModel != "" {
		title += " " + format.EscapeMarkdown(l.SubModel)
	}
	b.WriteString("*" + title + "*\n\n")

	if l.Price > 0 {
		b.WriteString(locale.Tf(lang, "fmt_price", format.Number(l.Price)))
	}

	if l.Km > 0 {
		b.WriteString(locale.Tf(lang, "fmt_mileage", format.Number(l.Km)))
	} else {
		b.WriteString(locale.T(lang, "fmt_mileage_unknown"))
	}

	if l.FitnessScore > 0 {
		b.WriteString(locale.Tf(lang, "fmt_fitness_score", l.FitnessScore))
		b.WriteString(formatBreakdown(l.FitnessBreakdown, lang))
	}

	if l.DealScore != nil {
		b.WriteString(locale.Tf(lang, "fmt_deal_score", l.DealScore.Score))
		b.WriteString(dealExplanation(lang, l.DealScore, l.Price))
		b.WriteString("\n")
	}

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
	} else {
		b.WriteString(locale.T(lang, "fmt_mileage_unknown_inline"))
	}
	if l.Hand > 0 {
		b.WriteString(fmt.Sprintf(" · ✋ Hand %d", l.Hand))
	}
	if l.FitnessScore > 0 {
		b.WriteString(fmt.Sprintf(" · 🎯 %.1f", l.FitnessScore))
	}
	b.WriteString("\n")

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

var dimKeys = map[string]string{
	"price":  "dim_price",
	"km":     "dim_km",
	"hand":   "dim_hand",
	"year":   "dim_year",
	"engine": "dim_engine",
}

func formatBreakdown(dims []model.FitnessDim, lang locale.Lang) string {
	var good, bad []string
	for _, d := range dims {
		name := locale.T(lang, dimKeys[d.Name])
		if d.Score >= 0.7 {
			good = append(good, name)
		} else if d.Score < 0.4 {
			bad = append(bad, name)
		}
	}
	if len(good) > 0 && len(bad) > 0 {
		return locale.Tf(lang, "fmt_fitness_up_down", strings.Join(good, ", "), strings.Join(bad, ", "))
	}
	if len(good) > 0 {
		return locale.Tf(lang, "fmt_fitness_up_only", strings.Join(good, ", "))
	}
	if len(bad) > 0 {
		return locale.Tf(lang, "fmt_fitness_down_only", strings.Join(bad, ", "))
	}
	return ""
}

func dealExplanation(lang locale.Lang, score *model.ScoreInfo, price int) string {
	medianStr := format.Number(score.MedianPrice)
	pctBelow := int(math.Round(100.0 * (1.0 - float64(price)/float64(score.MedianPrice))))
	if pctBelow > 5 {
		return locale.Tf(lang, "fmt_deal_below_market", pctBelow, medianStr, score.CohortSize)
	}
	if pctBelow >= -5 {
		return locale.Tf(lang, "fmt_deal_near_market", medianStr, score.CohortSize)
	}
	return locale.Tf(lang, "fmt_deal_above_market", medianStr, score.CohortSize)
}

func FormatDailyDigest(stats []storage.DailySearchStats, lang locale.Lang, now time.Time) string {
	var b strings.Builder

	dateStr := now.Format("02/01/2006")
	b.WriteString(locale.Tf(lang, "fmt_market_digest_header", dateStr))

	for _, s := range stats {
		b.WriteString(locale.Tf(lang, "fmt_market_digest_search", format.EscapeMarkdown(s.SearchName)))
		b.WriteString(locale.Tf(lang, "fmt_market_digest_new", s.NewCount))
		b.WriteString(locale.Tf(lang, "fmt_market_digest_avg", format.Number(s.AvgPrice)))
		b.WriteString(locale.Tf(lang, "fmt_market_digest_best", format.Number(s.BestPrice)))

		if s.BestPriceLink != "" {
			b.WriteString(locale.Tf(lang, "fmt_market_digest_best_link", format.EscapeMarkdown(s.BestPriceLink)))
		}

		if s.PriceTrend > 1.0 {
			b.WriteString(locale.Tf(lang, "fmt_market_digest_trend_up", s.PriceTrend))
		} else if s.PriceTrend < -1.0 {
			b.WriteString(locale.Tf(lang, "fmt_market_digest_trend_down", -s.PriceTrend))
		} else {
			b.WriteString(locale.T(lang, "fmt_market_digest_trend_flat"))
		}

		b.WriteString("\n")
	}

	return b.String()
}
