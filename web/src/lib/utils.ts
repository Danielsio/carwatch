import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function formatPrice(price: number): string {
  if (price <= 0) return "ללא מחיר";
  return price.toLocaleString("he-IL") + " ₪";
}

export function formatKm(km: number | null | undefined): string {
  if (km === null || km === undefined || km === 0) return "לא ידוע";
  if (km > 0) return km.toLocaleString("he-IL") + " ק\"מ";
  return "לא ידוע";
}

export function safeHref(raw: string): string | null {
  try {
    const u = new URL(raw);
    if (u.protocol === "http:" || u.protocol === "https:") return u.toString();
  } catch {
    // invalid URL
  }
  return null;
}

export type MarketComparison = {
  label: string;
  /** Positive when listing price is below reference (better deal vs reference). */
  diffPercent: number;
  isBelow: boolean;
  source: "base" | "median";
  /** Signed reference − listing (positive when listing is cheaper than reference). */
  absDiff: number;
  color: string;
};

function marketComparisonAgainstRef(
  price: number,
  referencePrice: number,
  source: "base" | "median",
): MarketComparison | null {
  if (!referencePrice || referencePrice <= 0 || price <= 0) return null;
  const diffPercent = Math.round(100 * (1 - price / referencePrice));
  const absDiff = referencePrice - price;
  const isBelow = price < referencePrice;

  if (diffPercent > 5) {
    return {
      diffPercent,
      absDiff,
      isBelow,
      source,
      label:
        source === "base"
          ? `${diffPercent}% מתחת למחירון`
          : `${diffPercent}% מתחת לממוצע`,
      color: "text-emerald-500",
    };
  }
  if (diffPercent >= -5) {
    return {
      diffPercent,
      absDiff,
      isBelow,
      source,
      label: source === "base" ? "קרוב למחירון" : "קרוב לממוצע",
      color: "text-muted-foreground",
    };
  }
  return {
    diffPercent,
    absDiff,
    isBelow,
    source,
    label:
      source === "base"
        ? `${-diffPercent}% מעל המחירון`
        : `${-diffPercent}% מעל הממוצע`,
    color: "text-amber-500",
  };
}

/** Compare listing price to Yad2 price list (base) when present, else to cohort median. */
export function marketComparison(
  price: number,
  medianPrice?: number,
  basePrice?: number,
): MarketComparison | null {
  if (basePrice != null && basePrice > 0) {
    return marketComparisonAgainstRef(price, basePrice, "base");
  }
  if (medianPrice != null && medianPrice > 0) {
    return marketComparisonAgainstRef(price, medianPrice, "median");
  }
  return null;
}

export function listingSource(pageLink: string): string | null {
  if (!pageLink) return null;
  if (pageLink.toLowerCase().includes("yad2")) return "Yad2";
  return null;
}

export function relativeTime(dateStr: string): string {
  const date = new Date(dateStr);
  if (Number.isNaN(date.getTime())) return "—";
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  if (diffMs < 0) {
    return date.toLocaleDateString("he-IL");
  }
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

  if (diffDays === 0) return "היום";
  if (diffDays === 1) return "אתמול";
  if (diffDays < 7) return `לפני ${diffDays} ימים`;
  const weeks = Math.floor(diffDays / 7);
  if (diffDays < 30) return weeks === 1 ? "לפני שבוע" : `לפני ${weeks} שבועות`;
  const months = Math.floor(diffDays / 30);
  return months === 1 ? "לפני חודש" : `לפני ${months} חודשים`;
}
