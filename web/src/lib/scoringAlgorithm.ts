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
  // Price: cheaper within budget = better (sqrt curve).
  let priceFactor: number;
  if (search.price_max <= 0) {
    priceFactor = 0.5;
  } else {
    const ratio = listing.price / search.price_max;
    priceFactor = ratio >= 1 ? 0 : Math.sqrt(1 - ratio);
  }

  // Km: blend age-adjusted expectations with cap-based score.
  const AVG_KM_PER_YEAR = 15000;
  const now = new Date().getFullYear();
  const carAge = Math.max(1, now - listing.year);
  const expectedKm = carAge * AVG_KM_PER_YEAR;
  const ageScore = clamp01(1 - Math.pow(clamp01(listing.mileage / expectedKm), 1.2));
  let mileageFactor: number;
  if (search.mileage_max > 0) {
    const capScore = clamp01(1 - Math.pow(clamp01(listing.mileage / search.mileage_max), 1.5));
    mileageFactor = 0.6 * ageScore + 0.4 * capScore;
  } else {
    mileageFactor = ageScore;
  }

  // Year: position in range with floor at 0.3 and sqrt curve.
  const YEAR_FLOOR = 0.3;
  const span = Math.max(1, search.year_max - search.year_min);
  const pos = clamp01((listing.year - search.year_min) / span);
  const yearFactor = YEAR_FLOOR + (1 - YEAR_FLOOR) * Math.sqrt(pos);

  // Hand: ladder with age bonus for older cars.
  let handBase: number;
  if (search.hand_max > 0) {
    const ratio = Math.max(0, listing.hand - 1) / search.hand_max;
    handBase = clamp01(1 - Math.pow(clamp01(ratio), 0.6));
  } else {
    handBase = listing.hand <= 1 ? 1 : listing.hand === 2 ? 0.7 : listing.hand === 3 ? 0.4 : 0.1;
  }
  if (listing.hand > 1) {
    const bonus = clamp01(carAge / 15) * 0.15;
    handBase = clamp01(handBase + bonus);
  }
  const handFactor = handBase;

  const combined =
    0.35 * priceFactor +
    0.25 * mileageFactor +
    0.15 * yearFactor +
    0.20 * handFactor +
    0.05 * 1.0; // engine (always 1.0 in demo)

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

/**
 * Continuous HSL gradient: red (0) → amber (5) → emerald-green (10).
 * Returns raw HSL components so consumers can build color / bg / border.
 */
function scoreHue(score: number): number {
  if (!Number.isFinite(score)) return 0;
  const t = Math.max(0, Math.min(10, score)) / 10;
  return Math.round(t * 160);
}

export function scoreHsl(score: number): string {
  const hue = scoreHue(score);
  return `hsl(${hue} 72% 55%)`;
}

export function scoreHslAlpha(score: number, alpha: number): string {
  const hue = scoreHue(score);
  const a = Math.max(0, Math.min(1, alpha));
  return `hsl(${hue} 72% 55% / ${a})`;
}

export function scoreLabel(score: number): string {
  if (score >= 8.5) return "מצוין";
  if (score >= 7) return "טוב מאוד";
  if (score >= 5.5) return "טוב";
  return "חלש";
}
