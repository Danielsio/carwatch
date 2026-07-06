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
const KNOWN_TOKENS_CAP = 3000;
const ENRICH_DELAY_MS = 1500; // jittered inside the injected fetcher

let isRunning = false;

chrome.runtime.onInstalled.addListener(() => {
  chrome.alarms.create(ALARM_NAME, { periodInMinutes: FETCH_INTERVAL_MINUTES });
  console.log("[CarWatch] Alarm set: every", FETCH_INTERVAL_MINUTES, "minutes");
});

chrome.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name === ALARM_NAME) runFetchCycle();
});

chrome.runtime.onMessage.addListener((msg) => {
  if (msg.action === "fetchNow") runFetchCycle();
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
      if (t && /yad2\.co\.il/.test(t.url || "")) return t.id;
    } catch {
      // tab was closed; fall through
    }
  }

  const existing = await chrome.tabs.query({ url: "*://*.yad2.co.il/*" });
  if (existing.length) {
    await chrome.storage.local.set({ yad2TabId: existing[0].id });
    return existing[0].id;
  }

  const tab = await chrome.tabs.create({ url: YAD2_HOME, active: false, pinned: true });
  await chrome.storage.local.set({ yad2TabId: tab.id });
  await waitForTabComplete(tab.id, 45000);
  await sleep(3500); // let the Radware JS challenge settle a clearance cookie
  return tab.id;
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

// A response whose body is HTML (Radware "302/verifying" shell) rather than
// JSON means the tab's clearance lapsed.
function looksBlocked(r) {
  const b = (r && r.body ? r.body : "").trimStart();
  return r && r.status !== 200 ? true : b.startsWith("<");
}

// ---- known-token cache (client-side mirror of DB pre-fill) -----------------

async function getKnownTokens() {
  const { knownTokens } = await chrome.storage.local.get("knownTokens");
  return new Set(Array.isArray(knownTokens) ? knownTokens : []);
}

async function saveKnownTokens(set) {
  let arr = [...set];
  if (arr.length > KNOWN_TOKENS_CAP) arr = arr.slice(arr.length - KNOWN_TOKENS_CAP);
  await chrome.storage.local.set({ knownTokens: arr });
}

function shuffle(a) {
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [a[i], a[j]] = [a[j], a[i]];
  }
  return a;
}

// ---- main cycle ------------------------------------------------------------

async function runFetchCycle() {
  if (isRunning) {
    console.log("[CarWatch] Cycle already running, skipping");
    return;
  }
  isRunning = true;

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
    if (feedResults.length && feedResults.every(looksBlocked)) {
      console.warn("[CarWatch] all feeds blocked; reloading yad2 tab and retrying");
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

    // 2) Enrich listings missing km (skip tokens we've resolved before).
    const known = await getKnownTokens();
    let toEnrich = [...byToken.values()].filter(
      (l) => (l.km || 0) <= 0 && !known.has(l.token),
    );
    toEnrich = shuffle(toEnrich).slice(0, MAX_ENRICH_PER_CYCLE);

    if (toEnrich.length) {
      const itemResults = await fetchViaYad2Tab(
        tabId,
        toEnrich.map((l) => `${GW_ITEM_URL}/${l.token}`),
      );
      for (const r of itemResults) {
        if (looksBlocked(r)) continue;
        const parsed = parseItemDetail(r.body);
        if (parsed.error) continue;
        const l = byToken.get(parsed.fields.token);
        if (!l) continue;
        for (const [k, v] of Object.entries(parsed.fields)) {
          if (k === "token") continue;
          if (v !== undefined && v !== "" && v !== 0) l[k] = v;
        }
        if ((l.km || 0) > 0) known.add(l.token);
      }
      await saveKnownTokens(known);
    }

    const listings = [...byToken.values()];
    if (listings.length > 0) await pushListings(config, listings);

    const enriched = listings.filter((l) => (l.km || 0) > 0).length;
    setBadge(String(listings.length), blocked ? "#E65100" : "#4CAF50");
    await saveStatus({
      searches: activeSearches.length,
      listings: listings.length,
      enriched,
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

async function pushListings(config, listings) {
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
        body: JSON.stringify({ listings }),
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
