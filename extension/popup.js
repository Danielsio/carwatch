document.addEventListener("DOMContentLoaded", async () => {
  const fetchBtn = document.getElementById("fetchBtn");
  const statusDiv = document.getElementById("status");
  const authDiv = document.getElementById("auth");

  fetchBtn.addEventListener("click", () => {
    fetchBtn.disabled = true;
    fetchBtn.textContent = "Fetching...";
    chrome.runtime.sendMessage({ action: "fetchNow" });
    setTimeout(() => {
      fetchBtn.disabled = false;
      fetchBtn.textContent = "Fetch Now";
      refreshAuth();
      refreshStatus();
    }, 10000);
  });

  refreshAuth();
  refreshStatus();

  async function refreshAuth() {
    const data = await chrome.storage.sync.get(["authToken"]);
    authDiv.textContent = "";
    const div = document.createElement("div");
    if (data.authToken) {
      div.textContent = "Authenticated (token present)";
      div.className = "ok";
    } else {
      div.textContent =
        "Not authenticated — open carwatch.duckdns.org and log in";
      div.className = "warn";
    }
    authDiv.appendChild(div);
  }

  async function refreshStatus() {
    const data = await chrome.storage.local.get([
      "lastRun",
      "searches",
      "listings",
      "error",
    ]);
    statusDiv.textContent = "";
    const lines = [];
    if (data.lastRun) {
      lines.push("Last run: " + timeSince(new Date(data.lastRun)) + " ago");
    }
    if (data.searches !== undefined) {
      lines.push("Searches: " + data.searches);
    }
    if (data.listings !== undefined) {
      lines.push("Listings found: " + data.listings);
    }
    if (data.error) {
      lines.push("Error: " + data.error);
    }
    if (lines.length === 0) {
      lines.push("No data yet. Click Fetch Now to start.");
    }
    for (const line of lines) {
      const div = document.createElement("div");
      div.textContent = line;
      if (line.startsWith("Error:")) div.className = "error";
      if (line.startsWith("Listings")) div.className = "ok";
      statusDiv.appendChild(div);
    }
  }

  function timeSince(date) {
    const seconds = Math.floor((new Date() - date) / 1000);
    if (seconds < 60) return seconds + "s";
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return minutes + "m";
    const hours = Math.floor(minutes / 60);
    return hours + "h " + (minutes % 60) + "m";
  }
});
