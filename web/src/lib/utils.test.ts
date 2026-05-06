import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { formatPrice, formatKm, safeHref, relativeTime, cn } from "./utils";

describe("cn", () => {
  it("merges class names", () => {
    expect(cn("px-2", "py-1")).toBe("px-2 py-1");
  });

  it("deduplicates conflicting tailwind classes", () => {
    expect(cn("px-2", "px-4")).toBe("px-4");
  });

  it("handles falsy values", () => {
    expect(cn("base", undefined, "end")).toBe("base end");
  });
});

describe("formatPrice", () => {
  it("formats positive prices with currency symbol", () => {
    const result = formatPrice(150000);
    expect(result).toContain("150");
    expect(result).toContain("₪");
  });

  it("returns 'ללא מחיר' for zero", () => {
    expect(formatPrice(0)).toBe("ללא מחיר");
  });

  it("returns 'ללא מחיר' for negative values", () => {
    expect(formatPrice(-100)).toBe("ללא מחיר");
  });
});

describe("formatKm", () => {
  it("formats positive km with unit", () => {
    const result = formatKm(120000);
    expect(result).toContain("120");
    expect(result).toContain('ק"מ');
  });

  it("returns 'לא ידוע' for zero", () => {
    expect(formatKm(0)).toBe("לא ידוע");
  });

  it("returns 'לא ידוע' for null", () => {
    expect(formatKm(null)).toBe("לא ידוע");
  });

  it("returns 'לא ידוע' for undefined", () => {
    expect(formatKm(undefined)).toBe("לא ידוע");
  });

  it("returns 'לא ידוע' for negative values", () => {
    expect(formatKm(-1)).toBe("לא ידוע");
  });
});

describe("safeHref", () => {
  it("returns url string for valid https URL", () => {
    expect(safeHref("https://example.com")).toBe("https://example.com/");
  });

  it("returns url string for valid http URL", () => {
    expect(safeHref("http://example.com")).toBe("http://example.com/");
  });

  it("returns null for javascript: protocol", () => {
    expect(safeHref("javascript:alert(1)")).toBeNull();
  });

  it("returns null for empty string", () => {
    expect(safeHref("")).toBeNull();
  });

  it("returns null for invalid URL", () => {
    expect(safeHref("not-a-url")).toBeNull();
  });

  it("returns null for data: protocol", () => {
    expect(safeHref("data:text/html,<h1>hi</h1>")).toBeNull();
  });
});

describe("relativeTime", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-05-06T08:00:00Z"));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("returns 'היום' for today", () => {
    expect(relativeTime("2026-05-06T06:00:00Z")).toBe("היום");
  });

  it("returns 'אתמול' for yesterday", () => {
    expect(relativeTime("2026-05-05T06:00:00Z")).toBe("אתמול");
  });

  it("returns days for recent dates", () => {
    expect(relativeTime("2026-05-03T06:00:00Z")).toBe("לפני 3 ימים");
  });

  it("returns weeks for older dates", () => {
    expect(relativeTime("2026-04-22T06:00:00Z")).toBe("לפני 2 שבועות");
  });

  it("returns months for much older dates", () => {
    expect(relativeTime("2026-02-06T06:00:00Z")).toBe("לפני 2 חודשים");
  });

  it("returns dash for invalid date", () => {
    expect(relativeTime("not-a-date")).toBe("—");
  });
});
