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

  /* Weights match the “מה נכנס לחישוב?” copy on the landing page (30/30/20/20). */
  const combined =
    0.3 * priceFactor +
    0.3 * mileageFactor +
    0.2 * yearFactor +
    0.2 * handFactor;

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

/** Tier colors aligned with landing Smart Match mock (gold / orange / red). */
export function scoreColor(score: number): string {
  if (score >= 8) return "text-amber-400";
  if (score >= 5) return "text-orange-500";
  return "text-red-500";
}

export function scoreBgColor(score: number): string {
  if (score >= 8) return "bg-amber-400/12 border-amber-400/50";
  if (score >= 5) return "bg-orange-500/12 border-orange-500/45";
  return "bg-red-500/12 border-red-500/42";
}

export function scoreLabel(score: number): string {
  if (score >= 8.5) return "מצוין";
  if (score >= 7) return "טוב מאוד";
  if (score >= 5.5) return "טוב";
  return "חלש";
}

export function scoreBarColor(score: number): string {
  if (score >= 8) return "bg-amber-400";
  if (score >= 5) return "bg-orange-500";
  return "bg-red-500";
}
