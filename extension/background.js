importScripts("parser.js");

const ALARM_NAME = "carwatch-fetch";
const FETCH_INTERVAL_MINUTES = 15;
const DEFAULT_API_URL = "https://carwatch.duckdns.org";
const FETCH_TIMEOUT_MS = 15000;

// Yad2's own gateway JSON API. We call these from within a real yad2.co.il tab
// (MAIN world) so the request carries the browser's Radware clearance cookie +
// Chrome TLS fingerprint — the HTML /vehicles/cars route is a Radware challenge
// shell and cannot be scraped, and a plain background fetch (no browser
// fingerprint) is blocked. See extension/README notes / parser.js.
const GW_FEED_URL = "https://gw.yad2.co.il/feed-search-vehicles/cars";
const GW_ITEM_URL = "https://gw.yad2.co.il/vehicles-item";
const YAD2_HOME = "https://www.yad2.co.il/";

// Bound per-cycle request volume: one feed call per search, plus at most this
// many item-detail (enrichment) calls. km/city/image are absent from most feed
// rows, so we enrich tokens we have not resolved before.
const MAX_ENRICH_PER_CYCLE = 30;
// Removal check: listings we have resolved before but that the feed no longer
// returns. Feed absence alone is NOT proof (a feed can be partial), so each
// candidate is confirmed against its item page — a 404 means sold/delisted.
// Bounded per cycle so a large backlog cannot spike our request volume.
const MAX_VERIFY_PER_CYCLE = 10;
const KNOWN_TOKENS_CAP = 3000;
const ENRICH_DELAY_MS = 1500; // jittered inside the injected fetcher

let isRunning = false;

// Re-assert the alarm from every service-worker entry point, not just
// onInstalled: if the alarm ever goes missing (cleared storage, Chrome quirk),
// scanning AND both countdowns (popup + web UI) die silently.
async function ensureAlarm() {
  const existing = await chrome.alarms.get(ALARM_NAME);
  if (!existing) {
    chrome.alarms.create(ALARM_NAME, { periodInMinutes: FETCH_INTERVAL_MINUTES });
    console.log("[CarWatch] Alarm set: every", FETCH_INTERVAL_MINUTES, "minutes");
  }
}

chrome.runtime.onInstalled.addListener(() => {
  ensureAlarm();
});
chrome.runtime.onStartup.addListener(() => {
  ensureAlarm();
});

chrome.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name === ALARM_NAME) runFetchCycle();
});

chrome.runtime.onMessage.addListener((msg, _sender, sendResponse) => {
  if (msg.action === "fetchNow") {
    // Respond only once the whole cycle finishes, so the popup can keep its
    // button disabled for the real ~60-90s duration instead of a fixed guess.
    runFetchCycle().finally(() => {
      try {
        sendResponse({ done: true });
      } catch {
        // popup closed before the cycle finished — nothing to respond to
      }
    });
    return true; // keep the message channel open for the async sendResponse
  }
});

async function getConfig() {
  const data = await chrome.storage.sync.get(["apiUrl", "authToken"]);
  return {
    apiUrl: data.apiUrl || DEFAULT_API_URL,
    authToken: data.authToken || "",
  };
}

function buildFeedURL(search) {
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
  params.set("page", "1");
  return `${GW_FEED_URL}?${params}`;
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function fetchWithTimeout(url, options = {}, timeoutMs = FETCH_TIMEOUT_MS) {
  const controller = new AbortController();
  const id = setTimeout(() => controller.abort(), timeoutMs);
  try {
    return await fetch(url, { ...options, signal: controller.signal });
  } finally {
    clearTimeout(id);
  }
}

// ---- Yad2 tab management ---------------------------------------------------

function waitForTabComplete(tabId, timeoutMs) {
  return new Promise((resolve) => {
    let settled = false;
    const finish = () => {
      if (settled) return;
      settled = true;
      chrome.tabs.onUpdated.removeListener(listener);
      resolve();
    };
    const timer = setTimeout(finish, timeoutMs);
    function listener(id, info) {
      if (id === tabId && info.status === "complete") {
        clearTimeout(timer);
        finish();
      }
    }
    chrome.tabs.onUpdated.addListener(listener);
    chrome.tabs
      .get(tabId)
      .then((t) => {
        if (t && t.status === "complete") {
          clearTimeout(timer);
          finish();
        }
      })
      .catch(() => {});
  });
}

// Returns the id of a live yad2 tab whose session has cleared Radware, creating
// a pinned background tab if the user has none open. Reuses the same managed
// tab across cycles so we don't spawn a new tab every 15 minutes.
async function ensureYad2Tab() {
  const { yad2TabId } = await chrome.storage.local.get("yad2TabId");
  if (yad2TabId != null) {
    try {
      const t = await chrome.tabs.get(yad2TabId);
      if (t && /yad2\.co\.il/.test(t.url || "")) {
        await keepTabAlive(t.id);
        // Chrome's Memory Saver can discard an idle background tab between
        // cycles. A discarded tab has no live page context, so executeScript
        // fetches silently fail — reactivate it and let Radware re-clear.
        if (t.discarded) {
          console.warn("[CarWatch] yad2 tab was discarded; reactivating");
          await reloadYad2Tab(t.id);
        }
        return t.id;
      }
    } catch {
      // tab was closed; fall through
    }
  }

  const existing = await chrome.tabs.query({ url: "*://*.yad2.co.il/*" });
  if (existing.length) {
    await chrome.storage.local.set({ yad2TabId: existing[0].id });
    await keepTabAlive(existing[0].id);
    if (existing[0].discarded) {
      console.warn("[CarWatch] found existing yad2 tab but it was discarded; reactivating");
      await reloadYad2Tab(existing[0].id);
    }
    return existing[0].id;
  }

  const tab = await chrome.tabs.create({ url: YAD2_HOME, active: false, pinned: true });
  await chrome.storage.local.set({ yad2TabId: tab.id });
  await keepTabAlive(tab.id);
  await waitForTabComplete(tab.id, 45000);
  await sleep(3500); // let the Radware JS challenge settle a clearance cookie
  return tab.id;
}

// Pin the scraping tab out of Chrome's Memory Saver reach. When the tab is
// auto-discarded, its page context is gone and every executeScript fetch
// (feed + km enrichment) returns nothing — the main reason enrichment lands
// 0 km on a long-lived background tab that works fine when freshly opened.
async function keepTabAlive(tabId) {
  try {
    await chrome.tabs.update(tabId, { autoDiscardable: false });
  } catch {
    // best-effort; older Chrome or races are non-fatal
  }
}

async function reloadYad2Tab(tabId) {
  try {
    await chrome.tabs.reload(tabId);
    await waitForTabComplete(tabId, 45000);
    await sleep(3500);
  } catch (err) {
    console.warn("[CarWatch] tab reload failed:", err);
  }
}

// Runs inside the yad2 tab's MAIN world: fetches each gw URL with the page's
// credentials and returns the raw text. Must be fully self-contained.
function inPageFetchAll(urls, delayMs) {
  return (async () => {
    const out = [];
    for (let i = 0; i < urls.length; i++) {
      if (i > 0) {
        await new Promise((r) => setTimeout(r, delayMs + Math.random() * delayMs));
      }
      try {
        const resp = await fetch(urls[i], {
          credentials: "include",
          headers: { accept: "application/json" },
        });
        out.push({ url: urls[i], status: resp.status, body: await resp.text() });
      } catch (e) {
        out.push({ url: urls[i], status: 0, error: String(e) });
      }
    }
    return out;
  })();
}

async function fetchViaYad2Tab(tabId, urls) {
  if (urls.length === 0) return [];
  const results = await chrome.scripting.executeScript({
    target: { tabId },
    world: "MAIN",
    func: inPageFetchAll,
    args: [urls, ENRICH_DELAY_MS],
  });
  return (results && results[0] && results[0].result) || [];
}

// A 404/410 from the item endpoint is Yad2 telling us the listing is gone
// (sold or delisted) — a real answer, not a block. Keep it distinct from
// looksBlocked: treating it as a block hides removals AND triggers pointless
// tab reloads.
function looksGone(r) {
  return !!r && (r.status === 404 || r.status === 410);
}

// A response whose body is HTML (Radware "302/verifying" shell) rather than
// JSON means the tab's clearance lapsed.
function looksBlocked(r) {
  if (looksGone(r)) return false;
  const b = (r && r.body ? r.body : "").trimStart();
  return r && r.status !== 200 ? true : b.startsWith("<");
}

// A fetch batch is unusable when executeScript returned nothing (a discarded
// or broken tab yields an empty array, not looksBlocked-shaped results) or
// when every result is a block/challenge page. Either warrants a tab reload.
function batchUnusable(results) {
  return results.length === 0 || results.every(looksBlocked);
}

// ---- enrichment cache (token -> resolved item fields) ----------------------
//
// Maps a token to the fields we resolved from the item endpoint (km, city,
// image, gearbox, ...). We RE-APPLY it to the feed listings every cycle and
// push the values, so a listing's km survives even if a single backend save
// was dropped (self-healing — no more remove+re-add), and we never re-fetch
// what we already have. Replaces the old token-only "known" set, whose
// one-shot nature stranded any listing whose km failed to persist once.

async function getEnrichedCache() {
  const { enrichedCache } = await chrome.storage.local.get("enrichedCache");
  return enrichedCache && typeof enrichedCache === "object" ? enrichedCache : {};
}

async function saveEnrichedCache(cache) {
  const tokens = Object.keys(cache);
  if (tokens.length > KNOWN_TOKENS_CAP) {
    // Object keys keep insertion order — drop the oldest overflow.
    for (const t of tokens.slice(0, tokens.length - KNOWN_TOKENS_CAP)) delete cache[t];
  }
  await chrome.storage.local.set({ enrichedCache: cache });
}

// Overlay resolved fields onto a feed listing without wiping known values with
// blanks/zeros (the feed omits km/city/image; the item endpoint fills them).
function applyEnrichment(listing, data) {
  for (const [k, v] of Object.entries(data)) {
    if (k === "token") continue;
    if (v !== undefined && v !== "" && v !== 0) listing[k] = v;
  }
}

function shuffle(a) {
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [a[i], a[j]] = [a[j], a[i]];
  }
  return a;
}

// ---- main cycle ------------------------------------------------------------

// The scan schedule as the backend should see it: when this cycle started and
// when the alarm fires next. Sent with every ingest push so the web UI's
// "next scan" countdown ticks toward the SAME Chrome alarm the popup shows —
// without this the site counts down to the server's maintenance loop, which
// no longer scans Yad2 and drifts minutes away from the real schedule.
async function getScanSchedule(startedAtISO) {
  try {
    const alarm = await chrome.alarms.get(ALARM_NAME);
    if (alarm && alarm.scheduledTime) {
      return {
        started_at: startedAtISO,
        next_run_at: new Date(alarm.scheduledTime).toISOString(),
        interval_sec: Math.round(
          (alarm.periodInMinutes || FETCH_INTERVAL_MINUTES) * 60,
        ),
      };
    }
  } catch {
    // alarms unavailable — push without a schedule report
  }
  return null;
}

async function runFetchCycle() {
  if (isRunning) {
    console.log("[CarWatch] Cycle already running, skipping");
    return;
  }
  isRunning = true;
  const cycleStartedAt = new Date().toISOString();

  try {
    const config = await getConfig();
    if (!config.authToken) {
      setBadge("!", "#F44");
      await saveStatus({ error: "No auth token — open carwatch.duckdns.org while logged in" });
      return;
    }

    setBadge("...", "#888");

    const searches = await fetchSearches(config);
    const activeSearches = searches.filter(
      (s) => s.active && s.manufacturer_id > 0 && s.model_id > 0,
    );
    if (activeSearches.length === 0) {
      setBadge("0", "#888");
      await saveStatus({ searches: 0, listings: 0, error: null });
      return;
    }

    const tabId = await ensureYad2Tab();

    // 1) Fetch each search's list feed.
    let feedResults = await fetchViaYad2Tab(tabId, activeSearches.map(buildFeedURL));
    if (batchUnusable(feedResults)) {
      console.warn("[CarWatch] feed batch unusable (empty or all blocked); reloading yad2 tab and retrying");
      await reloadYad2Tab(tabId);
      feedResults = await fetchViaYad2Tab(tabId, activeSearches.map(buildFeedURL));
    }

    // Merge into a token->listing map, preferring the row that already has km.
    const byToken = new Map();
    let blocked = 0;
    for (const r of feedResults) {
      if (looksBlocked(r)) {
        blocked++;
        continue;
      }
      const parsed = parseFeed(r.body);
      if (parsed.error) {
        console.warn("[CarWatch] feed parse:", parsed.error, r.url);
        continue;
      }
      for (const l of parsed.listings) {
        const prev = byToken.get(l.token);
        if (!prev || ((prev.km || 0) <= 0 && (l.km || 0) > 0)) byToken.set(l.token, l);
      }
    }

    // 2) Re-apply everything we've already resolved (self-healing: the km is
    // re-pushed every cycle so a dropped save recovers), then enrich only what
    // is STILL missing km.
    const cache = await getEnrichedCache();
    let cachedApplied = 0;
    for (const l of byToken.values()) {
      const c = cache[l.token];
      if (c) {
        applyEnrichment(l, c);
        if ((l.km || 0) > 0) cachedApplied++;
      }
    }
    let toEnrich = [...byToken.values()].filter((l) => (l.km || 0) <= 0);
    toEnrich = shuffle(toEnrich).slice(0, MAX_ENRICH_PER_CYCLE);

    // Diagnostics surfaced in the popup so failures are visible without DevTools.
    let itemGot = 0, itemOk = 0, itemBlocked = 0, itemParseErr = 0, gotKm = 0, itemErr = "";
    if (toEnrich.length) {
      const itemURLs = toEnrich.map((l) => `${GW_ITEM_URL}/${l.token}`);
      let itemResults = await fetchViaYad2Tab(tabId, itemURLs);
      // If the batch was unusable — empty (a discarded/broken tab makes
      // executeScript return nothing) or every result blocked (stale clearance
      // / Radware challenge) — reload the tab once and retry, same recovery the
      // feed path uses. Without this a single bad tab state zeroes out km.
      if (batchUnusable(itemResults)) {
        console.warn("[CarWatch] item batch unusable (empty or all blocked); reloading yad2 tab and retrying enrichment");
        await reloadYad2Tab(tabId);
        itemResults = await fetchViaYad2Tab(tabId, itemURLs);
      }
      itemGot = itemResults.length;
      for (const r of itemResults) {
        if (looksBlocked(r)) {
          itemBlocked++;
          if (!itemErr) itemErr = `blk s${r.status}${r.error ? " " + r.error : ""}`;
          continue;
        }
        const parsed = parseItemDetail(r.body);
        if (parsed.error) {
          itemParseErr++;
          if (!itemErr) itemErr = "perr:" + parsed.error;
          continue;
        }
        itemOk++;
        const l = byToken.get(parsed.fields.token);
        if (!l) continue;
        applyEnrichment(l, parsed.fields);
        if ((l.km || 0) > 0) {
          gotKm++;
          // Cache the resolved fields so future cycles re-push km without
          // re-fetching, and recover from a dropped save.
          const { token: _t, ...resolved } = parsed.fields;
          cache[l.token] = resolved;
        }
      }
      await saveEnrichedCache(cache);
    }
    console.log(
      `[CarWatch] enrich cacheApplied=${cachedApplied} tried=${toEnrich.length} returned=${itemGot} ok=${itemOk} blocked=${itemBlocked} parseErr=${itemParseErr} gotKm=${gotKm} ${itemErr}`,
    );

    // 3) Removal check. A token we resolved before but that no feed returned
    // this cycle is only a CANDIDATE — the feed may be partial, and acting on
    // absence alone would retire live listings. Confirm each against its item
    // page: 404/410 means sold or delisted. Anything else (200, or a block) is
    // left alone and re-checked next cycle.
    const removedTokens = [];
    let verifyTried = 0;
    if (!blocked) {
      const candidates = Object.keys(cache).filter((t) => !byToken.has(t));
      const toVerify = shuffle(candidates).slice(0, MAX_VERIFY_PER_CYCLE);
      verifyTried = toVerify.length;
      if (toVerify.length) {
        const byURL = new Map(
          toVerify.map((t) => [`${GW_ITEM_URL}/${t}`, t]),
        );
        const results = await fetchViaYad2Tab(tabId, [...byURL.keys()]);
        for (const r of results) {
          const token = byURL.get(r.url);
          if (!token || !looksGone(r)) continue;
          removedTokens.push(token);
          delete cache[token];
        }
        if (removedTokens.length) {
          await saveEnrichedCache(cache);
          console.log(
            `[CarWatch] removal: confirmed ${removedTokens.length}/${toVerify.length} gone (404)`,
          );
        }
      }
    }

    const listings = [...byToken.values()];
    const cycle = await getScanSchedule(cycleStartedAt);
    // Push even with nothing to report: the push still carries the scan
    // schedule, which keeps the web UI countdown in sync with the alarm.
    if (listings.length > 0 || removedTokens.length > 0 || cycle) {
      await pushListings(config, listings, removedTokens, cycle);
    }

    const enriched = listings.filter((l) => (l.km || 0) > 0).length;
    setBadge(String(listings.length), blocked ? "#E65100" : "#4CAF50");
    await saveStatus({
      searches: activeSearches.length,
      listings: listings.length,
      enriched,
      diag: `feeds ${feedResults.length - blocked}/${feedResults.length} ok · cache:${cachedApplied} · enrich try:${toEnrich.length} ret:${itemGot} ok:${itemOk} blk:${itemBlocked} perr:${itemParseErr} km:${gotKm} · gone:${removedTokens.length}/${verifyTried}${itemErr ? " · " + itemErr : ""}`,
      error: blocked ? `${blocked} feed(s) blocked by anti-bot` : null,
    });
  } catch (err) {
    console.error("[CarWatch] Cycle error:", err);
    setBadge("!", "#F44");
    await saveStatus({ error: err.message });
  } finally {
    isRunning = false;
  }
}

async function fetchSearches(config) {
  const resp = await fetchWithTimeout(`${config.apiUrl}/api/v1/searches`, {
    headers: { Authorization: `Bearer ${config.authToken}` },
  });
  if (!resp.ok) throw new Error(`searches API returned ${resp.status}`);
  return resp.json();
}

// The ingest endpoint rejects > 500 listings per request, so push in chunks.
// A user with many/broad searches can exceed that; without chunking the whole
// cycle's ingestion would 400 and be lost.
const MAX_INGEST_BATCH = 400;

async function pushListings(config, listings, removedTokens = [], cycle = null) {
  let last;
  // Removed tokens ride the FIRST chunk only, so a multi-chunk push applies
  // them exactly once. A push with no listings but pending removals or a
  // schedule report still goes out, otherwise a sold-out (or empty) cycle
  // could never report anything.
  //
  // The cycle report rides EVERY chunk — the backend accumulates stats for
  // chunks that share the same started_at, so repeating it is safe and lets
  // any chunk (including a lost first one) sync the schedule.
  let pendingRemoved = removedTokens;
  for (let i = 0; i < listings.length; i += MAX_INGEST_BATCH) {
    last = await pushBatch(
      config,
      listings.slice(i, i + MAX_INGEST_BATCH),
      pendingRemoved,
      cycle,
    );
    pendingRemoved = [];
  }
  if (listings.length === 0 && (pendingRemoved.length > 0 || cycle)) {
    last = await pushBatch(config, [], pendingRemoved, cycle);
  }
  return last;
}

async function pushBatch(config, listings, removedTokens = [], cycle = null) {
  for (let attempt = 0; attempt < 3; attempt++) {
    if (attempt > 0) await sleep(2000 * attempt);
    const resp = await fetchWithTimeout(
      `${config.apiUrl}/api/v1/ext/ingest`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${config.authToken}`,
        },
        body: JSON.stringify({
          listings,
          removed_tokens: removedTokens,
          cycle: cycle || undefined,
        }),
      },
      30000,
    );
    if (resp.ok) {
      const result = await resp.json();
      console.log("[CarWatch] Ingest result:", result);
      return result;
    }
    if (attempt === 2) {
      const text = await resp.text();
      throw new Error(`Ingest failed: ${resp.status} ${text}`);
    }
    console.warn(`[CarWatch] Ingest attempt ${attempt + 1} failed: ${resp.status}`);
  }
}

function setBadge(text, color) {
  chrome.action.setBadgeText({ text });
  chrome.action.setBadgeBackgroundColor({ color });
}

async function saveStatus(status) {
  await chrome.storage.local.set({
    lastRun: new Date().toISOString(),
    ...status,
  });
}
