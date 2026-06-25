import { useMemo } from "react";
import type { Listing } from "@/lib/api";
import { marketComparison } from "@/lib/utils";

export type ListingFilters = {
  /** Free-text match over manufacturer / model / sub-model / city / description. */
  text: string;
  /** Only listings priced meaningfully below market (>5% under reference). */
  dealsOnly: boolean;
  /** Only listings the user has not yet seen. */
  unseenOnly: boolean;
  /** Only listings that have a photo. */
  photoOnly: boolean;
  /** Selected fuel types (engine_type); empty = any. */
  fuels: string[];
  /** Selected gearboxes (gear_box); empty = any. */
  gearboxes: string[];
  /** Selected body types (body_type); empty = any. */
  bodyTypes: string[];
  /** Inclusive upper bound on price; null = no bound. */
  priceMax: number | null;
  /** Inclusive lower bound on year; null = no bound. */
  yearMin: number | null;
  /** Inclusive upper bound on km (ignores listings with unknown km); null = no bound. */
  kmMax: number | null;
};

export const EMPTY_FILTERS: ListingFilters = {
  text: "",
  dealsOnly: false,
  unseenOnly: false,
  photoOnly: false,
  fuels: [],
  gearboxes: [],
  bodyTypes: [],
  priceMax: null,
  yearMin: null,
  kmMax: null,
};

export function isDeal(listing: Listing): boolean {
  const mc = marketComparison(
    listing.price,
    listing.median_price,
    listing.base_price,
  );
  return !!mc && mc.diffPercent > 5;
}

/** Number of active (non-default) filter facets — drives the "clear" affordance. */
export function activeFilterCount(f: ListingFilters): number {
  let n = 0;
  if (f.text.trim()) n++;
  if (f.dealsOnly) n++;
  if (f.unseenOnly) n++;
  if (f.photoOnly) n++;
  if (f.fuels.length) n++;
  if (f.gearboxes.length) n++;
  if (f.bodyTypes.length) n++;
  if (f.priceMax != null) n++;
  if (f.yearMin != null) n++;
  if (f.kmMax != null) n++;
  return n;
}

export type ListingFacets = {
  fuels: string[];
  gearboxes: string[];
  bodyTypes: string[];
  /** Whole-shekel max price across the set (for slider bounds). */
  priceMax: number;
  yearMin: number;
  yearMax: number;
  kmMax: number;
};

/** Distinct facet values + numeric bounds derived from the loaded set. */
export function deriveFacets(listings: Listing[]): ListingFacets {
  const fuels = new Set<string>();
  const gearboxes = new Set<string>();
  const bodyTypes = new Set<string>();
  let priceMax = 0;
  let kmMax = 0;
  let yearMin = Number.POSITIVE_INFINITY;
  let yearMax = 0;

  for (const l of listings) {
    if (l.engine_type) fuels.add(l.engine_type);
    if (l.gear_box) gearboxes.add(l.gear_box);
    if (l.body_type) bodyTypes.add(l.body_type);
    if (l.price > priceMax) priceMax = l.price;
    if (l.km > kmMax) kmMax = l.km;
    if (l.year > 0 && l.year < yearMin) yearMin = l.year;
    if (l.year > yearMax) yearMax = l.year;
  }

  return {
    fuels: [...fuels].sort(),
    gearboxes: [...gearboxes].sort(),
    bodyTypes: [...bodyTypes].sort(),
    priceMax,
    kmMax,
    yearMin: Number.isFinite(yearMin) ? yearMin : 0,
    yearMax,
  };
}

/** Pure predicate-based filter — exported for direct unit testing. */
export function filterListings(
  listings: Listing[],
  f: ListingFilters,
): Listing[] {
  const q = f.text.trim().toLowerCase();
  return listings.filter((l) => {
    if (q) {
      const haystack =
        `${l.manufacturer} ${l.model} ${l.sub_model ?? ""} ${l.city ?? ""} ${l.description ?? ""}`.toLowerCase();
      if (!haystack.includes(q)) return false;
    }
    if (f.unseenOnly && l.seen !== false) return false;
    if (f.photoOnly && !l.image_url) return false;
    if (f.dealsOnly && !isDeal(l)) return false;
    if (f.fuels.length > 0 && (!l.engine_type || !f.fuels.includes(l.engine_type)))
      return false;
    if (
      f.gearboxes.length > 0 &&
      (!l.gear_box || !f.gearboxes.includes(l.gear_box))
    )
      return false;
    if (
      f.bodyTypes.length > 0 &&
      (!l.body_type || !f.bodyTypes.includes(l.body_type))
    )
      return false;
    if (f.priceMax != null && l.price > f.priceMax) return false;
    if (f.yearMin != null && l.year < f.yearMin) return false;
    // km <= 0 means "unknown" (still enriching) — don't exclude on an unknown.
    if (f.kmMax != null && l.km > 0 && l.km > f.kmMax) return false;
    return true;
  });
}

/** Memoized filtering for use in components. */
export function useListingFilters(
  listings: Listing[],
  filters: ListingFilters,
): Listing[] {
  return useMemo(() => filterListings(listings, filters), [listings, filters]);
}
