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
    expect(breakdown.condition).toBeGreaterThanOrEqual(0);
    expect(breakdown.condition).toBeLessThanOrEqual(100);
    expect(breakdown.value).toBeGreaterThanOrEqual(0);
    expect(breakdown.value).toBeLessThanOrEqual(100);
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

  it("handles zero price (neutral value)", () => {
    const { score, breakdown } = scoreListingAgainstSearch(
      { ...baseListing, price: 0 },
      baseSearch,
    );
    expect(score).toBeGreaterThanOrEqual(0);
    expect(score).toBeLessThanOrEqual(10);
    expect(breakdown.value).toBe(50);
  });

  it("handles zero mileage (neutral condition km)", () => {
    const { score } = scoreListingAgainstSearch(
      { ...baseListing, mileage: 0 },
      baseSearch,
    );
    expect(score).toBeGreaterThanOrEqual(0);
    expect(score).toBeLessThanOrEqual(10);
  });

  it("handles zero hand (neutral condition hand)", () => {
    const result = scoreListingAgainstSearch(
      { ...baseListing, hand: 0 },
      baseSearch,
    );
    expect(result.score).toBeGreaterThanOrEqual(0);
    expect(result.score).toBeLessThanOrEqual(10);
  });

  it("low-km old car scores well (condition dominant)", () => {
    const decadeOldYear = new Date().getFullYear() - 10;
    const gem = scoreListingAgainstSearch(
      { ...baseListing, year: decadeOldYear, mileage: 40000, hand: 1, price: 80000 },
      baseSearch,
    );
    expect(gem.score).toBeGreaterThanOrEqual(8.0);
    expect(gem.breakdown.condition).toBeGreaterThanOrEqual(70);
  });

  it("high-km car with many owners scores poorly", () => {
    const decadeOldYear = new Date().getFullYear() - 10;
    const beater = scoreListingAgainstSearch(
      { ...baseListing, year: decadeOldYear, mileage: 200000, hand: 4, price: 50000 },
      baseSearch,
    );
    expect(beater.score).toBeLessThan(4.0);
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

  it("drives saturation/lightness from theme-aware CSS vars", () => {
    const c = scoreHsl(5);
    expect(c).toContain("var(--score-saturation");
    expect(c).toContain("var(--score-lightness");
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
