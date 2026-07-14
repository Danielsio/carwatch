const test = require("node:test");
const assert = require("node:assert/strict");

const { parseFeed, parseItemDetail } = require("../parser.js");
const { feedResponse, itemResponse, radwareChallengeBody } = require("./fixtures.js");

test("parseFeed reads every seller bucket and marks commercial correctly", () => {
  const { listings, error } = parseFeed(JSON.stringify(feedResponse));
  assert.equal(error, undefined);
  assert.equal(listings.length, 3);

  const priv = listings.find((l) => l.token === "priv-1");
  const comm = listings.find((l) => l.token === "comm-1");
  // The seller filter (private/commercial) is driven entirely by which bucket
  // the ad came from — get this wrong and users' seller preference silently
  // inverts.
  assert.equal(priv.is_commercial, false);
  assert.equal(comm.is_commercial, true);
});

test("parseFeed maps the fields the backend filters on", () => {
  const { listings } = parseFeed(feedResponse); // object form is accepted too
  const priv = listings.find((l) => l.token === "priv-1");

  assert.equal(priv.manufacturer, "Mazda"); // text_eng preferred
  assert.equal(priv.manufacturer_id, 27);
  assert.equal(priv.model_id, 10514);
  assert.equal(priv.year, 2019);
  assert.equal(priv.price, 82000);
  assert.equal(priv.km, 64000);
  assert.equal(priv.hand, 2);
  assert.equal(priv.city, "תל אביב");
  assert.equal(priv.engine_volume, 1500);
  assert.equal(priv.image_url, "https://img.yad2.co.il/priv-1.jpg");
  assert.equal(priv.page_link, "https://www.yad2.co.il/vehicles/item/priv-1");
  assert.equal(priv.description, "רכב שמור"); // trimmed
});

test("parseFeed derives gearbox from the Hebrew sub-model marker, not English", () => {
  const { listings } = parseFeed(feedResponse);
  const auto = listings.find((l) => l.token === "priv-1"); // "1.5 אוט׳ Active"
  const manual = listings.find((l) => l.token === "priv-2"); // "1.6 ידני"

  // The backend filter matches the exact Hebrew string "אוטומט". Emitting the
  // item endpoint's English "Automatic" made every enriched listing FAIL the
  // gearbox filter and get dropped — which is how km stopped saving.
  assert.equal(auto.gear_box, "אוטומט");
  // Non-automatics stay empty, which the filter skips (rather than asserting
  // "manual" and excluding cars whose sub-model text is merely unusual).
  assert.equal(manual.gear_box, "");
});

test("parseFeed uses the original publish date, never the feed-wide refresh date", () => {
  const { listings } = parseFeed(feedResponse);
  const priv = listings.find((l) => l.token === "priv-1");

  // dates.start is unique per ad; dates.update/rebounce is the feed's
  // "refreshed" timestamp — using it made every ad look posted today.
  assert.equal(priv.created_at, "2026-07-01T09:00:00");
});

test("parseFeed tolerates string ids and missing optional fields", () => {
  const { listings } = parseFeed(feedResponse);
  const sparse = listings.find((l) => l.token === "priv-2");

  assert.equal(sparse.hand, 3); // "3" -> 3
  assert.equal(sparse.km, 0); // absent
  assert.equal(sparse.image_url, ""); // absent
  assert.equal(sparse.price, 0); // seller published no price
  assert.equal(sparse.model, "3"); // falls back to Hebrew text when text_eng is absent
});

test("parseFeed dedupes a token that appears in more than one bucket", () => {
  const dup = JSON.parse(JSON.stringify(feedResponse));
  dup.data.platinum = [dup.data.private[0]];

  const { listings } = parseFeed(dup);
  const tokens = listings.map((l) => l.token);
  assert.equal(new Set(tokens).size, tokens.length);
});

test("parseFeed reports an error instead of inventing listings", () => {
  assert.ok(parseFeed("not json").error);
  assert.ok(parseFeed(radwareChallengeBody).error);
  // A payload with no known buckets is a shape change, not an empty result set:
  // reporting zero listings here would look like "nothing for sale" and could
  // mass-retire live cars.
  assert.ok(parseFeed(JSON.stringify({ data: { unexpected: [] } })).error);
});

test("parseItemDetail extracts the fields the feed omits", () => {
  const { fields, error } = parseItemDetail(JSON.stringify(itemResponse));
  assert.equal(error, undefined);

  assert.equal(fields.token, "priv-2");
  assert.equal(fields.km, 88000);
  assert.equal(fields.city, "חיפה");
  assert.equal(fields.image_url, "https://img.yad2.co.il/priv-2.jpg");
  assert.equal(fields.price, 74000);
  assert.equal(fields.year, 2018);
  assert.equal(fields.hand, 3);
  assert.equal(fields.gear_box, "אוטומט"); // from the Hebrew subModel marker
});

test("parseItemDetail returns only present fields, so blanks cannot wipe known values", () => {
  const bare = { data: { token: "t1", km: 0 } };
  const { fields } = parseItemDetail(bare);

  assert.equal(fields.token, "t1");
  assert.equal(fields.km, 0); // explicitly present
  // Absent fields must be absent from the result, not empty strings: the caller
  // overlays these onto a feed listing.
  assert.ok(!("city" in fields));
  assert.ok(!("image_url" in fields));
  assert.ok(!("price" in fields));
});

test("parseItemDetail rejects junk and challenge pages", () => {
  assert.ok(parseItemDetail("not json").error);
  assert.ok(parseItemDetail(radwareChallengeBody).error);
  assert.ok(parseItemDetail(JSON.stringify({ data: {} })).error); // no token
});
