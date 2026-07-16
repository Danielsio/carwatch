// Pure logic shared by the service worker, extracted so it can be tested
// without a browser. Nothing in here may touch chrome.*, fetch, or timers —
// background.js owns all of that. This is the code whose silent breakage costs
// real data (wrong feed query, dropped listings, wiped km, wrongly retired
// cars), so it is the code that gets tests.

// Yad2's own gateway JSON API. Called from within a real yad2.co.il tab so the
// request carries the browser's Radware clearance cookie + Chrome TLS
// fingerprint; the HTML route is a challenge shell and a plain background fetch
// is blocked.
const GW_FEED_URL = "https://gw.yad2.co.il/feed-search-vehicles/cars";
const GW_ITEM_URL = "https://gw.yad2.co.il/vehicles-item";

// The ingest endpoint rejects > 500 listings per request, so pushes are
// chunked. A user with many/broad searches can exceed that; without chunking
// the whole cycle's ingestion would 400 and be lost.
const MAX_INGEST_BATCH = 400;

// Feed pagination bounds. Only page 1 of each search was ever fetched, so a
// broad search silently missed everything on page 2+. These cap the extra work:
// at most MAX_PAGES_PER_SEARCH pages of any one search, and at most
// MAX_EXTRA_FEED_PAGES_PER_CYCLE extra page-fetches across ALL searches in a
// cycle — so a user with many broad searches cannot make one cycle balloon into
// dozens of requests against Yad2.
const MAX_PAGES_PER_SEARCH = 3;
const MAX_EXTRA_FEED_PAGES_PER_CYCLE = 8;

// Bound on the token->fields enrichment cache kept in chrome.storage.local.
const KNOWN_TOKENS_CAP = 3000;

// Translates a saved search into the gw feed query. Every mismatch here is a
// silently wrong result set — too-broad (noise) or too-narrow (missed cars) —
// so the mapping is pinned by tests.
function buildFeedURL(search, page = 1) {
  const params = new URLSearchParams();
  if (search.manufacturer_id) params.set("manufacturer", search.manufacturer_id);
  if (search.model_id) params.set("model", search.model_id);
  if (search.year_min || search.year_max) {
    params.set("year", `${search.year_min || 2000}-${search.year_max || 2030}`);
  }
  if (search.price_min || search.price_max) {
    params.set("price", `${search.price_min || 0}-${search.price_max || 9999999}`);
  }
  if (search.max_km) params.set("km", `-1-${search.max_km}`);
  if (search.max_hand) params.set("hand", `0-${search.max_hand}`);
  if (search.engine_min_cc) params.set("engineval", `${search.engine_min_cc}--1`);
  if (search.seller_filter === "private") params.set("ownerID", "1");
  else if (["commercial", "dealer"].includes(search.seller_filter))
    params.set("ownerID", "2");
  if (search.photo_only) params.set("imgOnly", "1");
  params.set("Order", "1");
  params.set("page", String(page));
  return `${GW_FEED_URL}?${params}`;
}

function itemURL(token) {
  return `${GW_ITEM_URL}/${token}`;
}

// A 404/410 from the item endpoint is Yad2 telling us the listing is gone (sold
// or delisted) — a real answer, not a block. Keep it distinct from looksBlocked:
// treating it as a block hides removals AND triggers pointless tab reloads.
function looksGone(r) {
  return !!r && (r.status === 404 || r.status === 410);
}

// A response whose body is HTML (Radware "verifying" shell) rather than JSON
// means the tab's clearance lapsed.
function looksBlocked(r) {
  if (looksGone(r)) return false;
  const b = (r && r.body ? r.body : "").trimStart();
  return r && r.status !== 200 ? true : b.startsWith("<");
}

// A fetch batch is unusable when executeScript returned nothing (a discarded or
// broken tab yields an empty array, not looksBlocked-shaped results) or when
// every result is a block/challenge page. Either warrants a tab reload.
function batchUnusable(results) {
  return results.length === 0 || results.every(looksBlocked);
}

// Overlay resolved item fields onto a feed listing WITHOUT wiping known values
// with blanks/zeros: the feed omits km/city/image and the item endpoint fills
// them, so a missing field must never overwrite a present one.
function applyEnrichment(listing, data) {
  for (const [k, v] of Object.entries(data)) {
    if (k === "token") continue;
    if (v !== undefined && v !== "" && v !== 0) listing[k] = v;
  }
}

// Merge every search's feed into one token -> listing map, preferring the row
// that already carries km (the same car can appear in several searches' feeds,
// and only some rows come with km inline).
function mergeFeedListings(parsedListingsPerFeed) {
  const byToken = new Map();
  for (const listings of parsedListingsPerFeed) {
    for (const l of listings) {
      const prev = byToken.get(l.token);
      if (!prev || ((prev.km || 0) <= 0 && (l.km || 0) > 0)) byToken.set(l.token, l);
    }
  }
  return byToken;
}

// Drop the oldest overflow from the enrichment cache. Object keys keep
// insertion order, so the first keys are the oldest.
function trimCache(cache, cap = KNOWN_TOKENS_CAP) {
  const tokens = Object.keys(cache);
  if (tokens.length > cap) {
    for (const t of tokens.slice(0, tokens.length - cap)) delete cache[t];
  }
  return cache;
}

// Tokens we have resolved before that no feed returned this cycle. These are
// only CANDIDATES for removal — the feed may be partial, and acting on absence
// alone would retire live listings — so each is confirmed against its item page
// (404 => gone) before anything is retired.
function removalCandidates(cache, byToken) {
  return Object.keys(cache).filter((t) => !byToken.has(t));
}

// Removal treats a 404 from the item endpoint as proof a listing is gone. That
// is safe only as long as a 404 actually means "gone" — if Yad2 renames the
// endpoint or changes its token format, EVERY item fetch 404s and the extension
// would retire live listings by the cycleful, silently, over days.
//
// pickCanaryToken returns a token we KNOW is live right now: one present in this
// cycle's feed. Fetching its item page before trusting any 404 is the tripwire —
// if even the canary 404s, the endpoint is broken, not the listings, so removals
// must be skipped this cycle. Returns null when the feed is empty (nothing to
// vouch for, so removals are skipped anyway).
function pickCanaryToken(byToken) {
  const it = byToken.keys().next();
  return it.done ? null : it.value;
}

// canaryConfirmsEndpointHealthy reports whether the canary's item response looks
// like a real listing (a 200 that parses), i.e. the endpoint is behaving. A
// block (stale clearance) is inconclusive, not proof of breakage — but it is
// also not confirmation, so it returns false and removals wait for a cleaner
// cycle. Only an unambiguous healthy response green-lights retirement.
function canaryConfirmsEndpointHealthy(result) {
  return !!result && result.status === 200 && !looksBlocked(result);
}

// Split a push into ingest-sized chunks. Returns [] for no listings: callers
// decide whether a listing-less push is still worth sending (it can still carry
// removals or a schedule report).
function chunkListings(listings, size = MAX_INGEST_BATCH) {
  const chunks = [];
  for (let i = 0; i < listings.length; i += size) {
    chunks.push(listings.slice(i, i + size));
  }
  return chunks;
}

// Which searches are worth scanning at all.
function activeSearches(searches) {
  return (searches || []).filter(
    (s) => s.active && s.manufacturer_id > 0 && s.model_id > 0,
  );
}

// Plan which extra feed pages to fetch after page 1.
//
// Input: one entry per active search giving how many pages that search spans
// (from the page-1 response's pagination). Output: a flat, bounded list of
// { searchIndex, page } for pages 2..N, fairly distributed so one very broad
// search cannot eat the whole per-cycle budget while others get nothing.
//
// Fairness matters: without it, a search with 50 pages would consume the entire
// budget and a second search's page 2 would never be fetched. So this fills
// round-robin — every search's page 2 before any search's page 3 — up to the
// per-cycle cap.
function planExtraFeedPages(
  perSearchTotalPages,
  maxPagesPerSearch = MAX_PAGES_PER_SEARCH,
  maxTotal = MAX_EXTRA_FEED_PAGES_PER_CYCLE,
) {
  const plan = [];
  // page goes 2, 3, ... up to the per-search cap; for each page, sweep searches.
  for (let page = 2; page <= maxPagesPerSearch && plan.length < maxTotal; page++) {
    for (let i = 0; i < perSearchTotalPages.length && plan.length < maxTotal; i++) {
      const total = perSearchTotalPages[i] || 1;
      if (page <= total) plan.push({ searchIndex: i, page });
    }
  }
  return plan;
}

// Export for Node-based tests; harmless in the service worker, where these are
// just globals defined by importScripts.
if (typeof module !== "undefined" && module.exports) {
  module.exports = {
    GW_FEED_URL,
    GW_ITEM_URL,
    MAX_INGEST_BATCH,
    KNOWN_TOKENS_CAP,
    MAX_PAGES_PER_SEARCH,
    MAX_EXTRA_FEED_PAGES_PER_CYCLE,
    planExtraFeedPages,
    buildFeedURL,
    itemURL,
    looksGone,
    looksBlocked,
    batchUnusable,
    applyEnrichment,
    mergeFeedListings,
    trimCache,
    removalCandidates,
    pickCanaryToken,
    canaryConfirmsEndpointHealthy,
    chunkListings,
    activeSearches,
  };
}
