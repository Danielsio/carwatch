/**
 * Demo scoring for landing-page Smart Match section only.
 * Production listing scores may use different weights / inputs.
 */

export type DemoSearchCriteria = {
  year_min: number;
  year_max: number;
  price_max: number;
  mileage_max: number;
  hand_max: number;
};

export type DemoListingInput = {
  title: string;
  year: number;
  price: number;
  mileage: number;
  hand: number;
  city: string;
};

export type ScoreBreakdownPct = {
  price: number;
  mileage: number;
  year: number;
  hand: number;
};

function clamp01(x: number): number {
  return Math.max(0, Math.min(1, x));
}

export function scoreListingAgainstSearch(
  listing: DemoListingInput,
  search: DemoSearchCriteria,
): { score: number; breakdown: ScoreBreakdownPct } {
  const priceRatio =
    search.price_max > 0 ? listing.price / search.price_max : 1;
  const priceFactor = priceRatio <= 1 ? 1 : Math.exp(-(priceRatio - 1) * 3);

  const kmRatio =
    search.mileage_max > 0
      ? clamp01(listing.mileage / search.mileage_max)
      : 0;
  const mileageFactor = Math.exp(-kmRatio * 2.2);

  const span = Math.max(1, search.year_max - search.year_min);
  const yearFactor = clamp01((listing.year - search.year_min) / span);

  const handFactor =
    search.hand_max > 0
      ? Math.exp(
          -Math.max(0, listing.hand - 1) / Math.max(1, search.hand_max),
        )
      : listing.hand <= 1
        ? 1
        : 0.55;

  const conditionFactor =
    mileageFactor * 0.45 + yearFactor * 0.35 + handFactor * 0.2;

  const combined =
    0.3 * priceFactor +
    0.25 * mileageFactor +
    0.2 * yearFactor +
    0.15 * handFactor +
    0.1 * conditionFactor;

  const score = clamp01(combined) * 10;

  return {
    score,
    breakdown: {
      price: Math.round(priceFactor * 100),
      mileage: Math.round(mileageFactor * 100),
      year: Math.round(yearFactor * 100),
      hand: Math.round(handFactor * 100),
    },
  };
}

export function scoreColor(score: number): string {
  if (score >= 8.2) return "text-emerald-500";
  if (score >= 6.5) return "text-amber-500";
  if (score >= 4.5) return "text-orange-500";
  return "text-red-500";
}

export function scoreBgColor(score: number): string {
  if (score >= 8.2) return "bg-emerald-500/15 border-emerald-500/40";
  if (score >= 6.5) return "bg-amber-500/15 border-amber-500/40";
  if (score >= 4.5) return "bg-orange-500/15 border-orange-500/35";
  return "bg-red-500/15 border-red-500/35";
}

export function scoreLabel(score: number): string {
  if (score >= 8.5) return "מצוין";
  if (score >= 7) return "טוב מאוד";
  if (score >= 5.5) return "טוב";
  return "חלש";
}

export function scoreBarColor(score: number): string {
  if (score >= 8.2) return "bg-emerald-500";
  if (score >= 6.5) return "bg-amber-500";
  if (score >= 4.5) return "bg-orange-500";
  return "bg-red-500";
}
