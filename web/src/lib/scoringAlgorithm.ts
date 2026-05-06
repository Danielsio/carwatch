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

const W_PRICE = 0.35;
const W_KM = 0.25;
const W_HAND = 0.20;
const W_YEAR = 0.15;
const W_ENGINE = 0.05;

const AVG_KM_PER_YEAR = 15000;
const KM_AGE_EXPONENT = 1.2;
const KM_CAP_EXPONENT = 1.5;
const AGE_ADJUST_KM_BLEND = 0.6;
const HAND_CURVE_EXPONENT = 0.6;
const HAND_AGE_BONUS_MAX = 0.15;
const HAND_AGE_BONUS_YEARS = 15;
const YEAR_SCORE_FLOOR = 0.3;

function clamp01(x: number): number {
  return Math.max(0, Math.min(1, x));
}

export function scoreListingAgainstSearch(
  listing: DemoListingInput,
  search: DemoSearchCriteria,
): { score: number; breakdown: ScoreBreakdownPct } {
  // Price: cheaper within budget = better (sqrt curve).
  // Omit dimension when price is unknown (matches Go NaN guard).
  let priceFactor: number;
  if (listing.price <= 0 || search.price_max <= 0) {
    priceFactor = listing.price <= 0 ? NaN : 0.5;
  } else {
    const ratio = listing.price / search.price_max;
    priceFactor = ratio >= 1 ? 0 : Math.sqrt(1 - ratio);
  }

  // Km: blend age-adjusted expectations with cap-based score.
  // When mileage is missing/zero, mark as NaN to omit the dimension (matches backend).
  const now = new Date().getFullYear();
  const carAge = Math.max(1, now - listing.year);
  let mileageFactor: number;
  if (listing.mileage <= 0) {
    mileageFactor = NaN;
  } else {
    const expectedKm = carAge * AVG_KM_PER_YEAR;
    const ageScore = clamp01(1 - Math.pow(clamp01(listing.mileage / expectedKm), KM_AGE_EXPONENT));
    if (search.mileage_max > 0) {
      const capScore = clamp01(1 - Math.pow(clamp01(listing.mileage / search.mileage_max), KM_CAP_EXPONENT));
      mileageFactor = AGE_ADJUST_KM_BLEND * ageScore + (1 - AGE_ADJUST_KM_BLEND) * capScore;
    } else {
      mileageFactor = ageScore;
    }
  }

  // Year: position in range with floor and sqrt curve.
  // When range is invalid/single-year, give full marks (matches Go guard).
  let yearFactor: number;
  if (search.year_min <= 0 || search.year_max <= 0 || search.year_max <= search.year_min) {
    yearFactor = 1.0;
  } else {
    const pos = clamp01((listing.year - search.year_min) / (search.year_max - search.year_min));
    yearFactor = YEAR_SCORE_FLOOR + (1 - YEAR_SCORE_FLOOR) * Math.sqrt(pos);
  }

  // Hand: ladder with age bonus for older cars.
  // When hand is unknown/zero, use neutral 0.5 (matches Go).
  let handFactor: number;
  if (listing.hand <= 0) {
    handFactor = 0.5;
  } else {
    let handBase: number;
    if (search.hand_max > 0) {
      const ratio = Math.max(0, listing.hand - 1) / search.hand_max;
      handBase = clamp01(1 - Math.pow(clamp01(ratio), HAND_CURVE_EXPONENT));
    } else {
      handBase = listing.hand <= 1 ? 1 : listing.hand === 2 ? 0.7 : listing.hand === 3 ? 0.4 : 0.1;
    }
    if (listing.hand > 1) {
      const bonus = clamp01(carAge / HAND_AGE_BONUS_YEARS) * HAND_AGE_BONUS_MAX;
      handBase = clamp01(handBase + bonus);
    }
    handFactor = handBase;
  }

  const dims: [number, number][] = [
    [W_PRICE, priceFactor],
    [W_KM, mileageFactor],
    [W_YEAR, yearFactor],
    [W_HAND, handFactor],
    [W_ENGINE, 1.0], // engine (always 1.0 in demo)
  ];

  let totalWeight = 0;
  let weighted = 0;
  for (const [w, s] of dims) {
    if (Number.isNaN(s)) continue;
    totalWeight += w;
    weighted += w * s;
  }
  const combined = totalWeight > 0 ? weighted / totalWeight : 0.5;
  const score = Math.round(clamp01(combined) * 100) / 10;

  return {
    score,
    breakdown: {
      price: Number.isNaN(priceFactor) ? 0 : Math.round(priceFactor * 100),
      mileage: Number.isNaN(mileageFactor) ? 0 : Math.round(mileageFactor * 100),
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
