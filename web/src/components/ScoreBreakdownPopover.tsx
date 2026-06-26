"use client";

import type { Listing, ScoreBreakdown } from "@/lib/api";
import { MatchScoreBox } from "@/components/ui/MatchScoreBox";
import {
  Popover,
  PopoverTrigger,
  PopoverContent,
} from "@/components/ui/popover";
import { scoreHsl, scoreHslAlpha } from "@/lib/scoringAlgorithm";
import { cn } from "@/lib/utils";

type ScoreBreakdownPopoverProps = {
  score: number;
  breakdown?: ScoreBreakdown;
  listing: Listing;
  size?: "sm" | "md" | "lg";
  className?: string;
};

const DIMENSIONS = [
  { key: "condition" as const, label: "מצב הרכב", weight: "60%" },
  { key: "value" as const, label: "שווי עסקה", weight: "35%" },
  { key: "engine" as const, label: "מנוע", weight: "5%" },
];

const OWNERSHIP_LABELS: Record<string, string> = {
  private: "פרטי",
  lease: "ליסינג",
  rental: "השכרה",
};

type Insight = {
  text: string;
  sentiment: "positive" | "neutral" | "negative";
};

const EXPECTED_KM_PER_YEAR = 15_000;
const EXPECTED_YEARS_PER_HAND = 5;

function fmtNum(n: number): string {
  return n.toLocaleString("he-IL");
}

function conditionInsights(listing: Listing): Insight[] {
  const insights: Insight[] = [];
  const carAge = new Date().getFullYear() - listing.year;

  if (listing.km > 0 && carAge > 0) {
    const kmPerYear = Math.round(listing.km / carAge);
    const ratio = kmPerYear / EXPECTED_KM_PER_YEAR;

    let tag: string;
    let sentiment: Insight["sentiment"];
    if (ratio <= 0.5) {
      tag = "נמוך מאוד";
      sentiment = "positive";
    } else if (ratio <= 0.85) {
      tag = "נמוך";
      sentiment = "positive";
    } else if (ratio <= 1.15) {
      tag = "ממוצע";
      sentiment = "neutral";
    } else if (ratio <= 1.5) {
      tag = "גבוה";
      sentiment = "negative";
    } else {
      tag = "גבוה מאוד";
      sentiment = "negative";
    }

    insights.push({
      text: `${fmtNum(listing.km)} ק"מ ב-${carAge} שנים = ${fmtNum(kmPerYear)} ק"מ/שנה · ${tag}`,
      sentiment,
    });
  }

  if (listing.hand > 0 && carAge > 0) {
    if (listing.hand === 1) {
      const ownerLabel = listing.original_ownership
        ? OWNERSHIP_LABELS[listing.original_ownership]
        : null;
      const ownershipSentiment: Insight["sentiment"] =
        listing.original_ownership == null || listing.original_ownership === "private"
          ? "positive"
          : listing.original_ownership === "rental"
            ? "negative"
            : "neutral";
      insights.push({
        text: ownerLabel ? `יד ראשונה · ${ownerLabel}` : "יד ראשונה",
        sentiment: ownershipSentiment,
      });
    } else {
      const expectedHands = 1 + carAge / EXPECTED_YEARS_PER_HAND;
      if (listing.hand <= Math.ceil(expectedHands)) {
        insights.push({
          text: `יד ${listing.hand} · סביר לרכב בן ${carAge}`,
          sentiment: "neutral",
        });
      } else {
        insights.push({
          text: `יד ${listing.hand} · גבוה לרכב בן ${carAge}`,
          sentiment: "negative",
        });
      }
    }
  }

  return insights;
}

function valueInsights(listing: Listing): Insight[] {
  const insights: Insight[] = [];

  if (listing.median_price && listing.median_price > 0 && listing.price > 0) {
    const diffPct = Math.round(
      ((listing.median_price - listing.price) / listing.median_price) * 100,
    );

    insights.push({
      text: `${fmtNum(listing.price)} ₪ מול חציון ${fmtNum(listing.median_price)} ₪`,
      sentiment: "neutral",
    });

    if (diffPct > 5) {
      insights.push({ text: `${diffPct}% מתחת לשוק`, sentiment: "positive" });
    } else if (diffPct >= -5) {
      insights.push({ text: "במחיר שוק", sentiment: "neutral" });
    } else {
      insights.push({
        text: `${-diffPct}% מעל השוק`,
        sentiment: "negative",
      });
    }
  } else {
    insights.push({
      text: "אין מספיק נתוני שוק להשוואה",
      sentiment: "neutral",
    });
  }

  return insights;
}

function engineInsights(listing: Listing): Insight[] {
  const parts: string[] = [];
  if (listing.engine_volume) {
    parts.push(`${(listing.engine_volume / 1000).toFixed(1)}L`);
  }
  if (listing.engine_type) {
    parts.push(listing.engine_type);
  }
  if (parts.length === 0) return [];
  return [{ text: parts.join(" · "), sentiment: "neutral" }];
}

const INSIGHTS_BY_DIM: Record<
  "condition" | "value" | "engine",
  (l: Listing) => Insight[]
> = {
  condition: conditionInsights,
  value: valueInsights,
  engine: engineInsights,
};

const SENTIMENT_CLASSES: Record<Insight["sentiment"], string> = {
  positive: "text-emerald-400",
  neutral: "text-muted-foreground",
  negative: "text-orange-400",
};

export function ScoreBreakdownPopover({
  score,
  breakdown,
  listing,
  size = "md",
  className,
}: ScoreBreakdownPopoverProps) {
  if (!breakdown) {
    return <MatchScoreBox score={score} size={size} className={className} />;
  }

  return (
    <Popover>
      <PopoverTrigger
        className={cn("cursor-pointer", className)}
        aria-label="הצג פירוט ציון"
      >
        <MatchScoreBox score={score} size={size} />
      </PopoverTrigger>
      <PopoverContent side="bottom" align="start" className="w-[300px] p-0">
        <div className="p-3 space-y-3">
          {/* Header */}
          <div className="flex items-center justify-between">
            <span className="text-sm font-semibold">פירוט ציון</span>
            <span
              className="text-lg font-extrabold tabular-nums"
              style={{ color: scoreHsl(score) }}
            >
              {score.toFixed(1)}
            </span>
          </div>

          {/* Dimension rows */}
          <div className="space-y-3">
            {DIMENSIONS.map((dim) => {
              const value = breakdown[dim.key];
              const barColor = scoreHsl(value * 10);
              const barBg = scoreHslAlpha(value * 10, 0.15);
              const insights = INSIGHTS_BY_DIM[dim.key](listing);
              return (
                <div key={dim.key} className="space-y-1">
                  <div className="flex items-center justify-between text-xs">
                    <div className="flex items-center gap-1.5">
                      <span className="font-medium text-foreground">
                        {dim.label}
                      </span>
                      <span className="rounded bg-muted px-1.5 py-0.5 text-[10px] font-semibold text-muted-foreground">
                        {dim.weight}
                      </span>
                    </div>
                    <span
                      className="font-bold tabular-nums"
                      style={{ color: barColor }}
                    >
                      {Math.round(value * 100)}%
                    </span>
                  </div>
                  <div
                    className="h-1.5 w-full overflow-hidden rounded-full"
                    style={{ backgroundColor: barBg }}
                  >
                    <div
                      className="h-full rounded-full transition-all duration-300"
                      style={{
                        width: `${Math.round(value * 100)}%`,
                        backgroundColor: barColor,
                      }}
                    />
                  </div>
                  {/* Insights */}
                  {insights.length > 0 && (
                    <div className="space-y-0.5 pt-0.5">
                      {insights.map((ins, i) => (
                        <p
                          key={i}
                          className={cn(
                            "text-[11px] leading-tight",
                            SENTIMENT_CLASSES[ins.sentiment],
                          )}
                        >
                          {ins.text}
                        </p>
                      ))}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      </PopoverContent>
    </Popover>
  );
}
