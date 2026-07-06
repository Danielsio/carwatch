// Content script running in MAIN world (same context as the page).
// Intercepts fetch() to capture the Firebase Bearer token from API calls
// and posts it via window.postMessage for the bridge script to pick up.

const API_PATH = "/api/v1/";
const originalFetch = window.fetch;

window.fetch = function (...args) {
  try {
    const url = typeof args[0] === "string" ? args[0] : args[0]?.url || "";
    if (url.includes(API_PATH)) {
      const options = args[1] || {};
      const headers = options.headers || {};
      let authHeader = null;

      if (headers instanceof Headers) {
        authHeader = headers.get("Authorization");
      } else if (typeof headers === "object") {
        authHeader =
          headers["Authorization"] || headers["authorization"] || null;
      }

      if (authHeader && authHeader.startsWith("Bearer ")) {
        window.postMessage(
          { type: "CW_AUTH_TOKEN", token: authHeader.slice(7) },
          "*",
        );
      }
    }
  } catch {
    // ignore
  }
  return originalFetch.apply(this, args);
};
