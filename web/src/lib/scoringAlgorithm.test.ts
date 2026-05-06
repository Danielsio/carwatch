import { describe, it, expect } from "vitest";
import {
  scoreListingAgainstSearch,
  scoreHsl,
  scoreHslAlpha,
  scoreLabel,
  type DemoSearchCriteria,
  type DemoListingInput,
} from "./scoringAlgorithm";

const baseSearch: DemoSearchCriteria = {
  year_min: 2015,
  year_max: 2024,
  price_max: 200000,
  mileage_max: 200000,
  hand_max: 3,
};

const baseListing: DemoListingInput = {
  title: "Test Car",
  year: 2020,
  price: 100000,
  mileage: 80000,
  hand: 2,
  city: "Test",
};

describe("scoreListingAgainstSearch", () => {
  it("returns a score between 0 and 10", () => {
    const { score } = scoreListingAgainstSearch(baseListing, baseSearch);
    expect(score).toBeGreaterThanOrEqual(0);
    expect(score).toBeLessThanOrEqual(10);
  });

  it("returns a breakdown with percentage values", () => {
    const { breakdown } = scoreListingAgainstSearch(baseListing, baseSearch);
    expect(breakdown.price).toBeGreaterThanOrEqual(0);
    expect(breakdown.price).toBeLessThanOrEqual(100);
    expect(breakdown.mileage).toBeGreaterThanOrEqual(0);
    expect(breakdown.mileage).toBeLessThanOrEqual(100);
    expect(breakdown.year).toBeGreaterThanOrEqual(0);
    expect(breakdown.year).toBeLessThanOrEqual(100);
    expect(breakdown.hand).toBeGreaterThanOrEqual(0);
    expect(breakdown.hand).toBeLessThanOrEqual(100);
  });

  it("scores cheaper cars higher than expensive ones", () => {
    const cheap = scoreListingAgainstSearch(
      { ...baseListing, price: 50000 },
      baseSearch,
    );
    const expensive = scoreListingAgainstSearch(
      { ...baseListing, price: 190000 },
      baseSearch,
    );
    expect(cheap.score).toBeGreaterThan(expensive.score);
  });

  it("scores lower mileage higher", () => {
    const low = scoreListingAgainstSearch(
      { ...baseListing, mileage: 30000 },
      baseSearch,
    );
    const high = scoreListingAgainstSearch(
      { ...baseListing, mileage: 180000 },
      baseSearch,
    );
    expect(low.score).toBeGreaterThan(high.score);
  });

  it("scores newer cars higher (year breakdown)", () => {
    const newer = scoreListingAgainstSearch(
      { ...baseListing, year: 2023 },
      baseSearch,
    );
    const older = scoreListingAgainstSearch(
      { ...baseListing, year: 2015 },
      baseSearch,
    );
    expect(newer.breakdown.year).toBeGreaterThan(older.breakdown.year);
  });

  it("scores first hand higher than third hand", () => {
    const first = scoreListingAgainstSearch(
      { ...baseListing, hand: 1 },
      baseSearch,
    );
    const third = scoreListingAgainstSearch(
      { ...baseListing, hand: 3 },
      baseSearch,
    );
    expect(first.score).toBeGreaterThan(third.score);
  });

  it("handles zero price (NaN dimension, redistributes weight)", () => {
    const { score, breakdown } = scoreListingAgainstSearch(
      { ...baseListing, price: 0 },
      baseSearch,
    );
    expect(score).toBeGreaterThanOrEqual(0);
    expect(score).toBeLessThanOrEqual(10);
    expect(breakdown.price).toBe(0);
  });

  it("handles zero mileage (NaN dimension, redistributes weight)", () => {
    const { score, breakdown } = scoreListingAgainstSearch(
      { ...baseListing, mileage: 0 },
      baseSearch,
    );
    expect(score).toBeGreaterThanOrEqual(0);
    expect(score).toBeLessThanOrEqual(10);
    expect(breakdown.mileage).toBe(0);
  });

  it("handles zero hand (neutral 0.5)", () => {
    const { breakdown } = scoreListingAgainstSearch(
      { ...baseListing, hand: 0 },
      baseSearch,
    );
    expect(breakdown.hand).toBe(50);
  });

  it("handles invalid year range gracefully", () => {
    const result = scoreListingAgainstSearch(baseListing, {
      ...baseSearch,
      year_min: 2020,
      year_max: 2020,
    });
    expect(result.breakdown.year).toBe(100);
  });

  it("gives year full marks when range is zero/negative", () => {
    const result = scoreListingAgainstSearch(baseListing, {
      ...baseSearch,
      year_min: 0,
      year_max: 0,
    });
    expect(result.breakdown.year).toBe(100);
  });

  it("price at max scores zero", () => {
    const { breakdown } = scoreListingAgainstSearch(
      { ...baseListing, price: 200000 },
      baseSearch,
    );
    expect(breakdown.price).toBe(0);
  });

  it("price over max scores zero", () => {
    const { breakdown } = scoreListingAgainstSearch(
      { ...baseListing, price: 300000 },
      baseSearch,
    );
    expect(breakdown.price).toBe(0);
  });
});

describe("scoreHsl", () => {
  it("returns red-ish hue for low score", () => {
    expect(scoreHsl(0)).toContain("hsl(0");
  });

  it("returns green-ish hue for high score", () => {
    expect(scoreHsl(10)).toContain("hsl(160");
  });

  it("returns amber-ish hue for mid score", () => {
    const result = scoreHsl(5);
    expect(result).toMatch(/hsl\(80/);
  });

  it("clamps negative scores", () => {
    expect(scoreHsl(-5)).toContain("hsl(0");
  });

  it("clamps scores over 10", () => {
    expect(scoreHsl(15)).toContain("hsl(160");
  });
});

describe("scoreHslAlpha", () => {
  it("includes alpha value", () => {
    expect(scoreHslAlpha(5, 0.5)).toContain("/ 0.5");
  });

  it("clamps alpha to 0-1", () => {
    expect(scoreHslAlpha(5, 2)).toContain("/ 1");
    expect(scoreHslAlpha(5, -1)).toContain("/ 0");
  });
});

describe("scoreLabel", () => {
  it("returns 'מצוין' for 8.5+", () => {
    expect(scoreLabel(9)).toBe("מצוין");
    expect(scoreLabel(8.5)).toBe("מצוין");
  });

  it("returns 'טוב מאוד' for 7-8.4", () => {
    expect(scoreLabel(7)).toBe("טוב מאוד");
    expect(scoreLabel(8)).toBe("טוב מאוד");
  });

  it("returns 'טוב' for 5.5-6.9", () => {
    expect(scoreLabel(5.5)).toBe("טוב");
    expect(scoreLabel(6.5)).toBe("טוב");
  });

  it("returns 'חלש' for below 5.5", () => {
    expect(scoreLabel(3)).toBe("חלש");
    expect(scoreLabel(5.4)).toBe("חלש");
  });
});
