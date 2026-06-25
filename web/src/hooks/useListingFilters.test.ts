import { describe, it, expect } from "vitest";
import {
  EMPTY_FILTERS,
  filterListings,
  deriveFacets,
  activeFilterCount,
  isDeal,
  type ListingFilters,
} from "./useListingFilters";
import type { Listing } from "@/lib/api";

function listing(over: Partial<Listing> = {}): Listing {
  return {
    token: Math.random().toString(36).slice(2),
    manufacturer: "Toyota",
    model: "Corolla",
    year: 2018,
    price: 80000,
    km: 60000,
    hand: 2,
    city: "תל אביב",
    page_link: "https://www.yad2.co.il/item/x",
    first_seen_at: "2026-06-10T00:00:00Z",
    ...over,
  };
}

function f(over: Partial<ListingFilters> = {}): ListingFilters {
  return { ...EMPTY_FILTERS, ...over };
}

describe("filterListings", () => {
  it("returns everything with empty filters", () => {
    const items = [listing(), listing(), listing()];
    expect(filterListings(items, EMPTY_FILTERS)).toHaveLength(3);
  });

  it("matches free text across manufacturer/model/sub_model/city/description", () => {
    const items = [
      listing({ manufacturer: "Mazda", model: "3" }),
      listing({ description: "רכב במצב מצוין" }),
      listing({ city: "חיפה" }),
    ];
    expect(filterListings(items, f({ text: "mazda" }))).toHaveLength(1);
    expect(filterListings(items, f({ text: "מצוין" }))).toHaveLength(1);
    expect(filterListings(items, f({ text: "חיפה" }))).toHaveLength(1);
    expect(filterListings(items, f({ text: "nomatch" }))).toHaveLength(0);
  });

  it("filters unseen only", () => {
    const items = [
      listing({ seen: false }),
      listing({ seen: true }),
      listing({ seen: undefined }),
    ];
    expect(filterListings(items, f({ unseenOnly: true }))).toHaveLength(1);
  });

  it("filters photo only", () => {
    const items = [
      listing({ image_url: "https://img/1.jpg" }),
      listing({ image_url: undefined }),
    ];
    expect(filterListings(items, f({ photoOnly: true }))).toHaveLength(1);
  });

  it("filters deals only (>5% under reference)", () => {
    const items = [
      listing({ price: 90000, base_price: 100000 }), // 10% under → deal
      listing({ price: 99000, base_price: 100000 }), // 1% under → not a deal
      listing({ price: 80000 }), // no reference → not a deal
    ];
    expect(filterListings(items, f({ dealsOnly: true }))).toHaveLength(1);
  });

  it("filters by selected fuels", () => {
    const items = [
      listing({ engine_type: "בנזין" }),
      listing({ engine_type: "היברידי" }),
      listing({ engine_type: undefined }),
    ];
    expect(filterListings(items, f({ fuels: ["היברידי"] }))).toHaveLength(1);
    expect(
      filterListings(items, f({ fuels: ["בנזין", "היברידי"] })),
    ).toHaveLength(2);
  });

  it("filters by selected body types", () => {
    const items = [
      listing({ body_type: "sedan" }),
      listing({ body_type: "hatchback" }),
      listing({ body_type: undefined }),
    ];
    expect(filterListings(items, f({ bodyTypes: ["sedan"] }))).toHaveLength(1);
    expect(
      filterListings(items, f({ bodyTypes: ["sedan", "hatchback"] })),
    ).toHaveLength(2);
  });

  it("filters by price/year ceilings and floors", () => {
    const items = [
      listing({ price: 50000, year: 2015 }),
      listing({ price: 120000, year: 2022 }),
    ];
    expect(filterListings(items, f({ priceMax: 100000 }))).toHaveLength(1);
    expect(filterListings(items, f({ yearMin: 2020 }))).toHaveLength(1);
  });

  it("treats unknown km (<=0) as not excluded by kmMax", () => {
    const items = [
      listing({ km: 200000 }),
      listing({ km: 0 }), // unknown — keep
    ];
    expect(filterListings(items, f({ kmMax: 100000 }))).toHaveLength(1);
    expect(
      filterListings(items, f({ kmMax: 100000 })).some((l) => l.km === 0),
    ).toBe(true);
  });

  it("combines multiple filters (AND)", () => {
    const items = [
      listing({ price: 70000, seen: false, engine_type: "בנזין" }),
      listing({ price: 70000, seen: true, engine_type: "בנזין" }),
      listing({ price: 200000, seen: false, engine_type: "בנזין" }),
    ];
    const out = filterListings(
      items,
      f({ unseenOnly: true, priceMax: 100000, fuels: ["בנזין"] }),
    );
    expect(out).toHaveLength(1);
  });
});

describe("deriveFacets", () => {
  it("collects distinct sorted fuels/gearboxes and numeric bounds", () => {
    const items = [
      listing({ engine_type: "בנזין", gear_box: "אוטומטית", price: 50000, km: 30000, year: 2016 }),
      listing({ engine_type: "דיזל", gear_box: "ידנית", price: 150000, km: 90000, year: 2021 }),
      listing({ engine_type: "בנזין", gear_box: "אוטומטית", price: 80000, km: 10000, year: 2019 }),
    ];
    const facets = deriveFacets(items);
    expect(facets.fuels).toEqual(["בנזין", "דיזל"]);
    expect(facets.gearboxes).toEqual(["אוטומטית", "ידנית"]);
    expect(facets.priceMax).toBe(150000);
    expect(facets.kmMax).toBe(90000);
    expect(facets.yearMin).toBe(2016);
    expect(facets.yearMax).toBe(2021);
  });

  it("collects distinct body types (skipping empty)", () => {
    const items = [
      listing({ body_type: "sedan" }),
      listing({ body_type: "hatchback" }),
      listing({ body_type: "sedan" }),
      listing({ body_type: undefined }),
    ];
    const facets = deriveFacets(items);
    expect(facets.bodyTypes).toEqual(["hatchback", "sedan"]);
  });

  it("is safe on an empty set", () => {
    const facets = deriveFacets([]);
    expect(facets.fuels).toEqual([]);
    expect(facets.yearMin).toBe(0);
  });
});

describe("activeFilterCount", () => {
  it("counts active facets", () => {
    expect(activeFilterCount(EMPTY_FILTERS)).toBe(0);
    expect(
      activeFilterCount(f({ text: "x", dealsOnly: true, priceMax: 100000 })),
    ).toBe(3);
    expect(activeFilterCount(f({ text: "   " }))).toBe(0);
  });

  it("counts bodyTypes as active", () => {
    expect(
      activeFilterCount(f({ bodyTypes: ["sedan"] })),
    ).toBe(1);
  });
});

describe("isDeal", () => {
  it("is true only when meaningfully under reference", () => {
    expect(isDeal(listing({ price: 90000, base_price: 100000 }))).toBe(true);
    expect(isDeal(listing({ price: 99000, base_price: 100000 }))).toBe(false);
    expect(isDeal(listing({ price: 80000 }))).toBe(false);
  });
});
