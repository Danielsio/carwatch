importScripts("parser.js");

const ALARM_NAME = "carwatch-fetch";
const FETCH_INTERVAL_MINUTES = 15;
const INTER_SEARCH_DELAY_MS = 3000;
const DEFAULT_API_URL = "https://carwatch.duckdns.org";

chrome.runtime.onInstalled.addListener(() => {
  chrome.alarms.create(ALARM_NAME, { periodInMinutes: FETCH_INTERVAL_MINUTES });
  console.log("[CarWatch] Alarm set: every", FETCH_INTERVAL_MINUTES, "minutes");
});

chrome.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name === ALARM_NAME) {
    runFetchCycle();
  }
});

chrome.runtime.onMessage.addListener((msg) => {
  if (msg.action === "fetchNow") {
    runFetchCycle();
  }
});

async function getConfig() {
  const data = await chrome.storage.sync.get(["apiUrl", "authToken"]);
  return {
    apiUrl: data.apiUrl || DEFAULT_API_URL,
    authToken: data.authToken || "",
  };
}

function buildYad2URL(search) {
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
  return `https://www.yad2.co.il/vehicles/cars?${params}`;
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function runFetchCycle() {
  const config = await getConfig();
  if (!config.authToken) {
    setBadge("!", "#F44");
    await saveStatus({ error: "No auth token configured" });
    return;
  }

  setBadge("...", "#888");

  try {
    const searches = await fetchSearches(config);
    const activeSearches = searches.filter(
      (s) => s.active && s.manufacturer_id > 0 && s.model_id > 0,
    );

    if (activeSearches.length === 0) {
      setBadge("0", "#888");
      await saveStatus({ searches: 0, listings: 0, error: null });
      return;
    }

    let totalListings = 0;
    const allListings = [];

    for (let i = 0; i < activeSearches.length; i++) {
      const search = activeSearches[i];
      if (i > 0) await sleep(INTER_SEARCH_DELAY_MS);

      try {
        const url = buildYad2URL(search);
        console.log(`[CarWatch] Fetching: ${search.name || search.id}`, url);

        const resp = await fetch(url);
        if (!resp.ok) {
          console.warn(`[CarWatch] Yad2 returned ${resp.status} for search ${search.id}`);
          continue;
        }

        const html = await resp.text();
        const result = parseNextData(html);
        if (result.error) {
          console.warn(`[CarWatch] Parse error for search ${search.id}:`, result.error);
          continue;
        }

        console.log(`[CarWatch] Found ${result.listings.length} listings for ${search.name || search.id}`);
        totalListings += result.listings.length;
        allListings.push(...result.listings);
      } catch (err) {
        console.error(`[CarWatch] Fetch error for search ${search.id}:`, err);
      }
    }

    if (allListings.length > 0) {
      await pushListings(config, allListings);
    }

    setBadge(String(totalListings), "#4CAF50");
    await saveStatus({
      searches: activeSearches.length,
      listings: totalListings,
      error: null,
    });
  } catch (err) {
    console.error("[CarWatch] Cycle error:", err);
    setBadge("!", "#F44");
    await saveStatus({ error: err.message });
  }
}

async function fetchSearches(config) {
  const resp = await fetch(`${config.apiUrl}/api/v1/searches`, {
    headers: { Authorization: `Bearer ${config.authToken}` },
  });
  if (!resp.ok) throw new Error(`API returned ${resp.status}`);
  return resp.json();
}

async function pushListings(config, listings) {
  const resp = await fetch(`${config.apiUrl}/api/v1/ext/ingest`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${config.authToken}`,
    },
    body: JSON.stringify({ listings }),
  });
  if (!resp.ok) {
    const text = await resp.text();
    throw new Error(`Ingest failed: ${resp.status} ${text}`);
  }
  const result = await resp.json();
  console.log("[CarWatch] Ingest result:", result);
  return result;
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
