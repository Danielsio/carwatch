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
  pctDiff: number;
  absDiff: number;
  label: string;
  color: string;
};

export function marketComparison(
  price: number,
  medianPrice: number | undefined,
): MarketComparison | null {
  if (!medianPrice || medianPrice <= 0 || price <= 0) return null;
  const pctDiff = Math.round(100 * (1 - price / medianPrice));
  const absDiff = medianPrice - price;
  if (pctDiff > 5) {
    return {
      pctDiff,
      absDiff,
      label: `${pctDiff}% מתחת לשוק`,
      color: "text-emerald-500",
    };
  }
  if (pctDiff >= -5) {
    return {
      pctDiff,
      absDiff,
      label: "קרוב למחיר השוק",
      color: "text-muted-foreground",
    };
  }
  return {
    pctDiff,
    absDiff,
    label: `${-pctDiff}% מעל השוק`,
    color: "text-amber-500",
  };
}

export function relativeTime(dateStr: string): string {
  const date = new Date(dateStr);
  if (Number.isNaN(date.getTime())) return "—";
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffDays = Math.max(0, Math.floor(diffMs / (1000 * 60 * 60 * 24)));

  if (diffDays === 0) return "היום";
  if (diffDays === 1) return "אתמול";
  if (diffDays < 7) return `לפני ${diffDays} ימים`;
  const weeks = Math.floor(diffDays / 7);
  if (diffDays < 30) return weeks === 1 ? "לפני שבוע" : `לפני ${weeks} שבועות`;
  const months = Math.floor(diffDays / 30);
  return months === 1 ? "לפני חודש" : `לפני ${months} חודשים`;
}
