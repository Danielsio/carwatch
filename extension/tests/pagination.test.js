const test = require("node:test");
const assert = require("node:assert/strict");

const {
  planExtraFeedPages,
  MAX_PAGES_PER_SEARCH,
  MAX_EXTRA_FEED_PAGES_PER_CYCLE,
} = require("../lib.js");
const { parseFeed, extractTotalPages } = require("../parser.js");
const { feedResponse } = require("./fixtures.js");

test("planExtraFeedPages returns nothing when every search fits on one page", () => {
  assert.deepEqual(planExtraFeedPages([1, 1, 1]), []);
});

test("planExtraFeedPages fetches pages 2..N up to the per-search cap", () => {
  // One search spanning 5 pages, cap 3 → pages 2 and 3 only.
  const plan = planExtraFeedPages([5], 3, 100);
  assert.deepEqual(plan, [
    { searchIndex: 0, page: 2 },
    { searchIndex: 0, page: 3 },
  ]);
});

test("planExtraFeedPages is fair: every search's page 2 before any search's page 3", () => {
  // Two broad searches; round-robin means we don't starve the second one.
  const plan = planExtraFeedPages([4, 4], 3, 100);
  assert.deepEqual(plan, [
    { searchIndex: 0, page: 2 },
    { searchIndex: 1, page: 2 },
    { searchIndex: 0, page: 3 },
    { searchIndex: 1, page: 3 },
  ]);
});

test("planExtraFeedPages respects the per-cycle total budget", () => {
  // Five searches each with many pages, budget 3 → only 3 extra fetches, and
  // they go to the earliest searches' page 2 (fairness fills page 2 first).
  const plan = planExtraFeedPages([9, 9, 9, 9, 9], 3, 3);
  assert.equal(plan.length, 3);
  assert.ok(plan.every((p) => p.page === 2), "budget spent on page 2s first");
  assert.deepEqual(plan.map((p) => p.searchIndex), [0, 1, 2]);
});

test("planExtraFeedPages skips searches that do not reach a given page", () => {
  // Search 0 has 3 pages, search 1 has only 1. Search 1 never contributes.
  const plan = planExtraFeedPages([3, 1], 3, 100);
  assert.deepEqual(plan, [
    { searchIndex: 0, page: 2 },
    { searchIndex: 0, page: 3 },
  ]);
});

test("the per-cycle budget bounds a pathological many-broad-searches cycle", () => {
  const manyBroad = new Array(50).fill(20); // 50 searches, 20 pages each
  const plan = planExtraFeedPages(manyBroad);
  assert.ok(
    plan.length <= MAX_EXTRA_FEED_PAGES_PER_CYCLE,
    `plan of ${plan.length} exceeded the per-cycle cap ${MAX_EXTRA_FEED_PAGES_PER_CYCLE}`,
  );
});

// --- extractTotalPages: defensive against the unknown gw payload shape ---

test("extractTotalPages reads a total_pages field", () => {
  assert.equal(extractTotalPages({ data: { pagination: { total_pages: 7 } } }), 7);
  assert.equal(extractTotalPages({ pagination: { pages_count: 4 } }), 4);
});

test("extractTotalPages derives pages from total_items / per_page", () => {
  assert.equal(extractTotalPages({ data: { pagination: { total_items: 95, per_page: 40 } } }), 3);
});

test("extractTotalPages defaults to 1 when the shape is unknown", () => {
  // The whole point: a wrong guess must cost nothing (no pagination), never
  // cause over-fetching.
  assert.equal(extractTotalPages({ data: { private: [] } }), 1);
  assert.equal(extractTotalPages({}), 1);
  assert.equal(extractTotalPages({ pagination: "weird" }), 1);
});

test("parseFeed reports totalPages alongside the listings", () => {
  const withPages = JSON.parse(JSON.stringify(feedResponse));
  withPages.data.pagination = { current_page: 1, total_pages: 5, total_items: 187 };

  const { listings, totalPages, error } = parseFeed(withPages);
  assert.equal(error, undefined);
  assert.ok(listings.length > 0);
  assert.equal(totalPages, 5);
});

test("parseFeed defaults totalPages to 1 when the feed has no pagination block", () => {
  const { totalPages } = parseFeed(feedResponse);
  assert.equal(totalPages, 1);
});
