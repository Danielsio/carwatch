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

// ltrMark (U+200E LEFT-TO-RIGHT MARK) isolates Latin/number runs inside RTL
// (Hebrew) lines so prices, years, and mileage render left-to-right instead of
// being reordered by the bidirectional algorithm.
const ltrMark = string(rune(0x200e))

// num renders a number group-separated and prefixed with an LTR mark so it
// keeps its order inside RTL messages. Callers supply any surrounding symbol
// (e.g. the ₪ sign) via the locale template.
func num(n int) string {
	return ltrMark + format.Number(n)
}

// FormatListing builds the per-listing notification. Field order follows how a
// buyer triages a used car: headline (model + year) → at-a-glance verdict →
// the specs the eye lands on first (mileage, hand, engine) → price and market
// context → seller/location → trust warnings → link.
func FormatListing(l model.Listing, lang locale.Lang) string {
	var b strings.Builder

	b.WriteString(locale.T(lang, "fmt_new_listing"))

	// Headline carries manufacturer, model, sub-model and year — the first
	// things a buyer scans. Year lives here, not buried below the scores.
	b.WriteString("*" + listingTitle(l, lang) + "*\n")

	// At-a-glance verdict (fitness + deal score) with the up/down breakdown
	// beneath, so a user can rank a listing without reading the whole block.
	if verdict := verdictLine(l, lang); verdict != "" {
		b.WriteString(verdict + "\n")
		b.WriteString(formatBreakdown(l.FitnessBreakdown, lang))
	}

	b.WriteString("\n")

	// Eye-first specs, grouped: usage (mileage, hand) then mechanicals
	// (engine, fuel, gearbox, power).
	if t := usageTokens(l, lang); len(t) > 0 {
		b.WriteString(strings.Join(t, " · ") + "\n")
	}
	if t := mechanicalTokens(l, lang); len(t) > 0 {
		b.WriteString(strings.Join(t, " · ") + "\n")
	}

	// Price and market context. The median/cohort is shown once here rather
	// than duplicated by a separate deal-score explanation.
	if l.Price > 0 {
		b.WriteString(locale.Tf(lang, "fmt_price", num(l.Price)))
		if l.DealScore != nil && l.DealScore.MedianPrice > 0 {
			b.WriteString(marketValueLine(lang, l.DealScore, l.Price))
		}
		if l.BasePrice != nil && *l.BasePrice > 0 {
			b.WriteString(basePriceLine(lang, *l.BasePrice, l.Price))
		}
	}

	// Seller type + location on one line.
	if t := sellerLocationTokens(l, lang); len(t) > 0 {
		b.WriteString(strings.Join(t, " · ") + "\n")
	}

	// Trust signal: surface why a listing was flagged suspicious. The data is
	// already computed; without this it is silently dropped on Telegram.
	b.WriteString(suspiciousBlock(l.SuspiciousReasons, lang))

	if l.PageLink != "" {
		b.WriteString(fmt.Sprintf("\n🔗 %s", l.PageLink))
	}

	return b.String()
}

// listingTitle renders the bold headline: "<Manufacturer> <Model> <SubModel> <Year>".
func listingTitle(l model.Listing, lang locale.Lang) string {
	mfr := strings.TrimSpace(l.Manufacturer)
	mdl := strings.TrimSpace(l.Model)
	// Hebrew-first product: prefer the Hebrew names when available so Hebrew
	// users don't get an English car name in an otherwise-Hebrew message.
	if lang == locale.Hebrew {
		if h := strings.TrimSpace(l.ManufacturerNameHe); h != "" {
			mfr = h
		}
		if h := strings.TrimSpace(l.ModelNameHe); h != "" {
			mdl = h
		}
	}
	if mfr == "" && mdl == "" {
		mfr = "Unknown"
	}
	title := format.EscapeMarkdown(strings.TrimSpace(mfr + " " + mdl))
	if l.SubModel != "" {
		title += " " + format.EscapeMarkdown(l.SubModel)
	}
	if l.Year > 0 {
		title += fmt.Sprintf(" %s%d", ltrMark, l.Year)
		if l.Month > 0 {
			title += locale.Tf(lang, "fmt_year_month", l.Month)
		}
	}
	return title
}

// verdictLine combines the fitness and deal scores into a single scannable line.
func verdictLine(l model.Listing, lang locale.Lang) string {
	var parts []string
	if l.FitnessScore > 0 {
		parts = append(parts, locale.Tf(lang, "fmt_fitness_inline", l.FitnessScore))
	}
	if l.DealScore != nil {
		parts = append(parts, locale.Tf(lang, "fmt_deal_inline", l.DealScore.Score))
	}
	return strings.Join(parts, " · ")
}

// usageTokens returns the condition/usage spec tokens (mileage, hand).
func usageTokens(l model.Listing, lang locale.Lang) []string {
	var t []string
	if l.Km > 0 {
		t = append(t, locale.Tf(lang, "fmt_mileage_inline", num(l.Km)))
	} else {
		t = append(t, locale.T(lang, "fmt_mileage_unknown_inline"))
	}
	if l.Hand > 0 {
		t = append(t, locale.Tf(lang, "fmt_hand_inline", l.Hand))
	}
	return t
}

// mechanicalTokens returns the mechanical spec tokens (engine, fuel, gearbox, power).
func mechanicalTokens(l model.Listing, lang locale.Lang) []string {
	var t []string
	if l.EngineVolume > 0 {
		t = append(t, locale.Tf(lang, "fmt_engine_inline", l.EngineVolume/1000.0))
	}
	if et := strings.TrimSpace(l.EngineType); et != "" {
		t = append(t, locale.Tf(lang, "fmt_fuel_inline", format.EscapeMarkdown(et)))
	}
	if gb := strings.TrimSpace(l.GearBox); gb != "" {
		t = append(t, format.EscapeMarkdown(gb))
	}
	if l.HorsePower > 0 {
		t = append(t, locale.Tf(lang, "fmt_power_inline", l.HorsePower))
	}
	return t
}

// sellerLocationTokens returns the seller type and location tokens.
func sellerLocationTokens(l model.Listing, lang locale.Lang) []string {
	var t []string
	if l.Commercial != nil {
		if *l.Commercial {
			t = append(t, locale.T(lang, "fmt_seller_dealer"))
		} else {
			t = append(t, locale.T(lang, "fmt_seller_private"))
		}
	}
	if l.City != "" {
		loc := format.EscapeMarkdown(l.City)
		if l.Area != "" {
			loc += ", " + format.EscapeMarkdown(l.Area)
		}
		t = append(t, locale.Tf(lang, "fmt_location_inline", loc))
	}
	return t
}

// suspiciousReasonKeys maps suspicious-reason codes (from scoring.DetectSuspicious)
// to their localized message keys.
var suspiciousReasonKeys = map[string]string{
	"price_below_market": "fmt_suspicious_price_below_market",
	"no_photo_low_price": "fmt_suspicious_no_photo_low_price",
}

// suspiciousBlock renders the trust warning (header + one line per reason).
func suspiciousBlock(reasons []string, lang locale.Lang) string {
	if len(reasons) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(locale.T(lang, "fmt_suspicious_header"))
	for _, r := range reasons {
		if key, ok := suspiciousReasonKeys[r]; ok {
			b.WriteString(locale.T(lang, key))
		} else {
			b.WriteString("• " + format.EscapeMarkdown(r) + "\n")
		}
	}
	return b.String()
}

func FormatPriceDrop(l model.Listing, oldPrice int, lang locale.Lang) string {
	var b strings.Builder

	mfr := strings.TrimSpace(l.Manufacturer)
	mdl := strings.TrimSpace(l.Model)
	if mfr == "" && mdl == "" {
		mfr = "Unknown"
	}
	title := format.EscapeMarkdown(strings.TrimSpace(mfr + " " + mdl))
	if l.SubModel != "" {
		title += " " + format.EscapeMarkdown(l.SubModel)
	}
	if l.Year > 0 {
		title += fmt.Sprintf(" %d", l.Year)
	}

	drop := oldPrice - l.Price
	b.WriteString(locale.Tf(lang, "fmt_price_drop",
		title,
		num(oldPrice),
		num(l.Price),
		num(drop),
	))

	var inlineParts []string
	if l.Km > 0 {
		inlineParts = append(inlineParts, locale.Tf(lang, "fmt_mileage_inline", num(l.Km)))
	} else {
		inlineParts = append(inlineParts, locale.T(lang, "fmt_mileage_unknown_inline"))
	}
	if l.Hand > 0 {
		inlineParts = append(inlineParts, locale.Tf(lang, "fmt_hand_inline", l.Hand))
	}
	if l.FitnessScore > 0 {
		inlineParts = append(inlineParts, fmt.Sprintf("🎯 %.1f", l.FitnessScore))
	}
	if len(inlineParts) > 0 {
		b.WriteString(strings.Join(inlineParts, " · "))
	}
	b.WriteString("\n")

	if l.PageLink != "" {
		b.WriteString(fmt.Sprintf("🔗 %s", l.PageLink))
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
	"condition": "dim_condition",
	"value":     "dim_value",
	"engine":    "dim_engine",
}

func formatBreakdown(dims []model.FitnessDim, lang locale.Lang) string {
	var good, bad []string
	for _, d := range dims {
		key, ok := dimKeys[d.Name]
		if !ok {
			key = d.Name
		}
		name := locale.T(lang, key)
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

func basePriceLine(lang locale.Lang, basePrice, price int) string {
	if basePrice <= 0 {
		return ""
	}
	bpStr := num(basePrice)
	pctDiff := int(math.Round(100.0 * (1.0 - float64(price)/float64(basePrice))))
	if pctDiff > 5 {
		return locale.Tf(lang, "fmt_base_price_below", bpStr, pctDiff)
	}
	if pctDiff >= -5 {
		return locale.Tf(lang, "fmt_base_price_near", bpStr)
	}
	return locale.Tf(lang, "fmt_base_price_above", bpStr, -pctDiff)
}

func marketValueLine(lang locale.Lang, score *model.ScoreInfo, price int) string {
	medianStr := num(score.MedianPrice)
	pctDiff := int(math.Round(100.0 * (1.0 - float64(price)/float64(score.MedianPrice))))
	if pctDiff > 5 {
		return locale.Tf(lang, "fmt_market_value_below", medianStr, pctDiff, score.CohortSize)
	}
	if pctDiff >= -5 {
		return locale.Tf(lang, "fmt_market_value_near", medianStr, score.CohortSize)
	}
	abovePct := -pctDiff
	return locale.Tf(lang, "fmt_market_value_above", medianStr, abovePct, score.CohortSize)
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
			b.WriteString(locale.Tf(lang, "fmt_market_digest_best_link", s.BestPriceLink))
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
