const ALARM_NAME = "carwatch-fetch";
const el = (id) => document.getElementById(id);

document.addEventListener("DOMContentLoaded", () => {
  el("ver").textContent = "v" + chrome.runtime.getManifest().version;

  const fetchBtn = el("fetchBtn");
  fetchBtn.addEventListener("click", () => {
    fetchBtn.disabled = true;
    fetchBtn.textContent = "Fetching…";
    // The background resolves this only when the cycle (feed + ~30 item
    // enrichments, ~60-90s) actually finishes, so the button reflects real
    // completion instead of a fixed timeout.
    const done = () => {
      fetchBtn.disabled = false;
      fetchBtn.textContent = "Fetch Now";
      refresh();
    };
    chrome.runtime.sendMessage({ action: "fetchNow" }).then(done, done);
  });

  el("diagToggle").addEventListener("click", () => {
    el("diag").classList.toggle("show");
  });

  refresh();
  setInterval(tickCountdown, 1000); // live countdown
  setInterval(refresh, 5000); // pick up a cycle that finished while open
});

async function refresh() {
  const { authToken } = await chrome.storage.sync.get(["authToken"]);
  const authed = !!authToken;
  el("authDot").className = "dot " + (authed ? "ok" : "warn");
  el("hint").style.display = authed ? "none" : "block";

  const d = await chrome.storage.local.get([
    "lastRun", "searches", "listings", "enriched", "diag", "error",
  ]);
  el("sSearches").textContent = d.searches ?? "—";
  el("sListings").textContent = d.listings ?? "—";
  el("sKm").textContent = d.enriched ?? "—";
  el("lastRun").textContent = d.lastRun
    ? "last run " + timeSince(new Date(d.lastRun)) + " ago"
    : "never run";
  el("diag").textContent = d.diag || "no data yet";

  if (d.error) {
    el("err").style.display = "block";
    el("err").textContent = "⚠ " + d.error;
  } else {
    el("err").style.display = "none";
  }

  tickCountdown();
}

async function tickCountdown() {
  let alarm = null;
  try {
    alarm = await chrome.alarms.get(ALARM_NAME);
  } catch {
    /* alarms unavailable */
  }
  const cd = el("countdown");
  const sub = el("nextSub");
  if (!alarm || !alarm.scheduledTime) {
    cd.textContent = "—";
    sub.textContent = "not scheduled — click Fetch Now";
    return;
  }
  const ms = alarm.scheduledTime - Date.now();
  if (ms <= 0) {
    cd.textContent = "now";
    sub.textContent = "fetching…";
    return;
  }
  const m = Math.floor(ms / 60000);
  const s = Math.floor((ms % 60000) / 1000);
  cd.textContent = m + ":" + String(s).padStart(2, "0");
  sub.textContent = "every 15 min while Chrome is open";
}

function timeSince(date) {
  const sec = Math.floor((Date.now() - date) / 1000);
  if (sec < 60) return sec + "s";
  const m = Math.floor(sec / 60);
  if (m < 60) return m + "m";
  const h = Math.floor(m / 60);
  return h + "h " + (m % 60) + "m";
}
