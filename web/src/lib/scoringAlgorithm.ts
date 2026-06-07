/**
 * Demo scoring for landing-page Smart Match section only.
 * BES v2 demo: additive baseline model — final = clamp(5 + condDelta + valDelta, 0, 10).
 * Production uses market medians for value scoring; demo uses budget fallback.
 */

export type DemoSearchCriteria = {
  year_min: number;
  year_max: number;
  price_max: number;
  mileage_max: number;
  hand_max: number;
  median_price?: number;
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
  condition: number;
  value: number;
};

const BASELINE = 5.0;
const AVG_KM_PER_YEAR = 15000;
const KM_LOW_THRESHOLD = 0.85;
const KM_LOW_BONUS_SCALE = 4.2;
const KM_DEAD_ZONE_PENALTY = 0.3;
const KM_EXCESS_STEEPNESS = 4.7;
const KM_EXCESS_CAP = 1.5;
const KM_EXCESS_EXPONENT = 0.7;

const HAND1_BONUS_BASE = 0.5;
const HAND1_AGE_BONUS_RATE = 0.10;
const HAND1_AGE_BONUS_CAP = 0.8;
const HAND_EXPECTED_RATE = 5.0;
const HAND_EXCESS_PENALTY = 1.1;
const HAND_BELOW_RATE = 0.3;
const HAND_BELOW_CAP = 0.3;

const COND_DELTA_MIN = -5.0;
const COND_DELTA_MAX = 4.5;

const BUDGET_HIGH_HEADROOM = 0.30;
const BUDGET_HIGH_BONUS = 2.0;
const BUDGET_LOW_HEADROOM = 0.10;
const BUDGET_LOW_PENALTY = 0.5;
const BUDGET_OVER_PENALTY = 1.0;

const VAL_DELTA_MIN = -3.5;
const VAL_DELTA_MAX = 2.5;

function clamp(x: number, lo: number, hi: number): number {
  return Math.max(lo, Math.min(hi, x));
}

function clamp01(x: number): number {
  return clamp(x, 0, 1);
}

function computeKmDelta(km: number, carAge: number): { delta: number; score01: number } {
  if (km <= 0) return { delta: 0, score01: 0.5 };

  const expectedKm = carAge * AVG_KM_PER_YEAR;
  const kmRatio = km / expectedKm;
  let delta: number;

  if (kmRatio <= KM_LOW_THRESHOLD) {
    delta = KM_LOW_BONUS_SCALE * Math.sqrt((KM_LOW_THRESHOLD - kmRatio) / KM_LOW_THRESHOLD);
  } else if (kmRatio <= 1.0) {
    delta = -KM_DEAD_ZONE_PENALTY * (kmRatio - KM_LOW_THRESHOLD) / (1.0 - KM_LOW_THRESHOLD);
  } else {
    const excess = Math.min(kmRatio - 1.0, KM_EXCESS_CAP);
    delta = -KM_DEAD_ZONE_PENALTY - KM_EXCESS_STEEPNESS * Math.pow(excess, KM_EXCESS_EXPONENT);
  }

  const score01 = kmRatio <= 1.0
    ? 1.0 / (1.0 + Math.exp(4.0 * (kmRatio - 0.7)))
    : 1.0 / (1.0 + Math.exp(5.0 * (kmRatio - 1.0)));

  return { delta, score01 };
}

function computeHandDelta(hand: number, carAge: number): { delta: number; score01: number } {
  if (hand <= 0) return { delta: 0, score01: 0.5 };

  if (hand === 1) {
    const ageBonus = Math.min(HAND1_AGE_BONUS_CAP, carAge * HAND1_AGE_BONUS_RATE);
    return { delta: HAND1_BONUS_BASE + ageBonus, score01: 1.0 };
  }

  const expectedHands = 1.0 + carAge / HAND_EXPECTED_RATE;
  const handExcess = hand - expectedHands;

  if (handExcess <= 0) {
    return {
      delta: clamp(-handExcess * HAND_BELOW_RATE, 0, HAND_BELOW_CAP),
      score01: clamp01(0.7 + (-handExcess) * 0.15),
    };
  }

  return {
    delta: -HAND_EXCESS_PENALTY * handExcess,
    score01: clamp01(0.5 - handExcess * 0.25),
  };
}

function computeValueDelta(
  price: number,
  priceMax: number,
  condScore01: number,
): { delta: number; score01: number } {
  if (price <= 0) return { delta: 0, score01: 0.5 };

  let delta: number;

  if (priceMax > 0) {
    const headroom = 1.0 - price / priceMax;
    if (headroom >= BUDGET_HIGH_HEADROOM) {
      delta = BUDGET_HIGH_BONUS;
    } else if (headroom >= BUDGET_LOW_HEADROOM) {
      delta = BUDGET_HIGH_BONUS * (headroom - BUDGET_LOW_HEADROOM) / (BUDGET_HIGH_HEADROOM - BUDGET_LOW_HEADROOM);
    } else if (headroom >= 0) {
      delta = -BUDGET_LOW_PENALTY * (BUDGET_LOW_HEADROOM - headroom) / BUDGET_LOW_HEADROOM;
    } else {
      delta = -BUDGET_OVER_PENALTY;
    }
  } else {
    return { delta: 0, score01: 0.5 };
  }

  delta = clamp(delta, VAL_DELTA_MIN, VAL_DELTA_MAX);
  if (delta > 0 && condScore01 < 0.55) {
    delta *= condScore01 / 0.55;
  }
  const score01 = clamp01(0.5 + delta / 5.0);
  return { delta, score01 };
}

export function scoreListingAgainstSearch(
  listing: DemoListingInput,
  search: DemoSearchCriteria,
): { score: number; breakdown: ScoreBreakdownPct } {
  const now = new Date().getFullYear();
  const carAge = Math.max(1, now - listing.year);

  const km = computeKmDelta(listing.mileage, carAge);
  const hand = computeHandDelta(listing.hand, carAge);
  const condDelta = clamp(km.delta + hand.delta, COND_DELTA_MIN, COND_DELTA_MAX);
  const condScore01 = 0.70 * km.score01 + 0.30 * hand.score01;

  const val = computeValueDelta(listing.price, search.price_max, condScore01);

  const raw = BASELINE + condDelta + val.delta;
  const score = Math.round(clamp(raw, 0, 10) * 10) / 10;

  return {
    score,
    breakdown: {
      condition: Math.round(condScore01 * 100),
      value: Math.round(val.score01 * 100),
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
