const test = require("node:test");
const assert = require("node:assert/strict");

const {
  buildFeedURL,
  itemURL,
  looksGone,
  looksBlocked,
  batchUnusable,
  applyEnrichment,
  mergeFeedListings,
  trimCache,
  removalCandidates,
  chunkListings,
  activeSearches,
} = require("../lib.js");

const params = (url) => new URL(url).searchParams;

test("buildFeedURL maps a search onto the gw query", () => {
  const p = params(
    buildFeedURL({
      manufacturer_id: 27,
      model_id: 10514,
      year_min: 2018,
      year_max: 2021,
      price_min: 50000,
      price_max: 90000,
      max_km: 100000,
      max_hand: 2,
      engine_min_cc: 1400,
      photo_only: true,
    }),
  );

  assert.equal(p.get("manufacturer"), "27");
  assert.equal(p.get("model"), "10514");
  assert.equal(p.get("year"), "2018-2021");
  assert.equal(p.get("price"), "50000-90000");
  assert.equal(p.get("km"), "-1-100000"); // Yad2 wants an open lower bound
  assert.equal(p.get("hand"), "0-2");
  assert.equal(p.get("engineval"), "1400--1");
  assert.equal(p.get("imgOnly"), "1");
  assert.equal(p.get("page"), "1");
});

test("buildFeedURL fills open-ended ranges rather than sending a half range", () => {
  // A search with only an upper bound must still send a complete range, or Yad2
  // ignores the filter entirely and the user gets unfiltered noise.
  const p = params(buildFeedURL({ manufacturer_id: 1, model_id: 2, year_max: 2020, price_max: 60000 }));
  assert.equal(p.get("year"), "2000-2020");
  assert.equal(p.get("price"), "0-60000");
});

test("buildFeedURL omits filters that are not set", () => {
  const p = params(buildFeedURL({ manufacturer_id: 1, model_id: 2 }));
  for (const key of ["year", "price", "km", "hand", "engineval", "ownerID", "imgOnly"]) {
    assert.equal(p.get(key), null, `${key} should be absent`);
  }
});

test("buildFeedURL translates the seller filter to Yad2's ownerID", () => {
  assert.equal(params(buildFeedURL({ seller_filter: "private" })).get("ownerID"), "1");
  assert.equal(params(buildFeedURL({ seller_filter: "commercial" })).get("ownerID"), "2");
  assert.equal(params(buildFeedURL({ seller_filter: "dealer" })).get("ownerID"), "2");
  assert.equal(params(buildFeedURL({ seller_filter: "any" })).get("ownerID"), null);
});

test("buildFeedURL defaults to page 1 even when mapped over an array", () => {
  // `searches.map(buildFeedURL)` would hand Array.map's index to the page
  // argument — every search after the first would silently request a different
  // page. The callers pass an arrow for this reason; this pins the default.
  const searches = [{ manufacturer_id: 1, model_id: 2 }, { manufacturer_id: 3, model_id: 4 }];
  const urls = searches.map((s) => buildFeedURL(s));
  for (const u of urls) assert.equal(params(u).get("page"), "1");
});

test("looksGone recognises a retired listing, and never confuses it with a block", () => {
  assert.ok(looksGone({ status: 404 }));
  assert.ok(looksGone({ status: 410 }));
  assert.ok(!looksGone({ status: 200 }));

  // A "gone" answer is a real answer: classifying it as a block would hide
  // removals and trigger pointless tab reloads.
  assert.ok(!looksBlocked({ status: 404, body: "" }));
});

test("looksBlocked catches the Radware challenge shell served with a 200", () => {
  assert.ok(looksBlocked({ status: 200, body: "<html>Verifying...</html>" }));
  assert.ok(looksBlocked({ status: 200, body: "\n  <!doctype html>" })); // leading whitespace
  assert.ok(looksBlocked({ status: 403, body: "{}" }));
  assert.ok(looksBlocked({ status: 0, error: "NetworkError" }));
  assert.ok(!looksBlocked({ status: 200, body: '{"data":{}}' }));
});

test("batchUnusable treats an empty result set as unusable", () => {
  // A discarded/broken tab makes executeScript return nothing at all — not
  // block-shaped results — and that must still trigger a tab reload.
  assert.ok(batchUnusable([]));
  assert.ok(batchUnusable([{ status: 200, body: "<html>" }]));
  assert.ok(!batchUnusable([{ status: 200, body: "{}" }, { status: 200, body: "<html>" }]));
});

test("applyEnrichment never overwrites a known value with a blank or zero", () => {
  const listing = { token: "t", km: 64000, city: "תל אביב", price: 82000 };
  applyEnrichment(listing, { token: "t", km: 0, city: "", price: 0, image_url: "https://img/x.jpg" });

  assert.equal(listing.km, 64000);
  assert.equal(listing.city, "תל אביב");
  assert.equal(listing.price, 82000);
  assert.equal(listing.image_url, "https://img/x.jpg"); // real value still lands
});

test("applyEnrichment fills missing values and ignores the token", () => {
  const listing = { token: "t", km: 0 };
  applyEnrichment(listing, { token: "OTHER", km: 88000, city: "חיפה" });

  assert.equal(listing.token, "t"); // never reassigned from the payload
  assert.equal(listing.km, 88000);
  assert.equal(listing.city, "חיפה");
});

test("mergeFeedListings prefers the row that already carries km", () => {
  const withoutKm = { token: "a", km: 0, price: 1 };
  const withKm = { token: "a", km: 50000, price: 1 };

  // Order must not matter: the same car appears in several searches' feeds and
  // only some rows come with km inline. Losing it means an extra item fetch at
  // best, a km-less listing at worst.
  assert.equal(mergeFeedListings([[withoutKm], [withKm]]).get("a").km, 50000);
  assert.equal(mergeFeedListings([[withKm], [withoutKm]]).get("a").km, 50000);
});

test("mergeFeedListings collapses duplicates across feeds", () => {
  const merged = mergeFeedListings([
    [{ token: "a", km: 1 }, { token: "b", km: 2 }],
    [{ token: "b", km: 2 }, { token: "c", km: 3 }],
  ]);
  assert.deepEqual([...merged.keys()].sort(), ["a", "b", "c"]);
});

test("trimCache drops the oldest entries once the cap is passed", () => {
  const cache = {};
  for (let i = 0; i < 5; i++) cache[`t${i}`] = { km: i };

  trimCache(cache, 3);
  assert.deepEqual(Object.keys(cache), ["t2", "t3", "t4"]); // oldest two gone
});

test("trimCache leaves an under-cap cache alone", () => {
  const cache = { a: {}, b: {} };
  trimCache(cache, 10);
  assert.deepEqual(Object.keys(cache), ["a", "b"]);
});

test("removalCandidates only proposes tokens the feed stopped returning", () => {
  const cache = { gone: {}, present: {} };
  const byToken = new Map([["present", {}], ["new", {}]]);

  // These are candidates ONLY: each is confirmed with a 404 against its item
  // page before anything is retired, because a feed can be partial.
  assert.deepEqual(removalCandidates(cache, byToken), ["gone"]);
});

test("chunkListings keeps a push under the ingest cap", () => {
  const listings = Array.from({ length: 900 }, (_, i) => ({ token: `t${i}` }));
  const chunks = chunkListings(listings, 400);

  assert.deepEqual(chunks.map((c) => c.length), [400, 400, 100]);
  // Nothing may be dropped: the endpoint 400s above its cap, and an unchunked
  // push would lose the whole cycle's ingestion.
  assert.equal(chunks.flat().length, 900);
});

test("chunkListings returns no chunks for an empty push", () => {
  // The caller still sends a listing-less push when it carries removals or a
  // schedule report — that decision lives there, not here.
  assert.deepEqual(chunkListings([]), []);
});

test("activeSearches skips paused searches and ones with no car selected", () => {
  const searches = [
    { id: 1, active: true, manufacturer_id: 27, model_id: 10514 },
    { id: 2, active: false, manufacturer_id: 27, model_id: 10514 }, // paused
    { id: 3, active: true, manufacturer_id: 0, model_id: 10514 }, // incomplete
    { id: 4, active: true, manufacturer_id: 27, model_id: 0 }, // incomplete
  ];
  assert.deepEqual(activeSearches(searches).map((s) => s.id), [1]);
});

test("itemURL builds the item-detail endpoint", () => {
  assert.equal(itemURL("abc123"), "https://gw.yad2.co.il/vehicles-item/abc123");
});
