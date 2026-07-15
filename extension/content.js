// Content script running in MAIN world (same context as the page).
//
// Intercepts fetch() to capture the Firebase Bearer token the CarWatch web app
// sends to its own API, and hands it to the bridge (ISOLATED world) via
// postMessage. That token is short-lived and is used exactly once: the bridge
// exchanges it for the extension's own long-lived credential.
//
// The target origin is location.origin, never "*": a wildcard broadcasts a live
// bearer token to every frame on the page, including any third-party iframe.

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
        const token = authHeader.slice(7);
        // Never re-broadcast our own credential: the extension's requests do not
        // go through this page, but a future one might, and a cwx_ token is
        // long-lived — leaking it to the page would be far worse than leaking a
        // Firebase token that expires within the hour.
        if (!token.startsWith("cwx_")) {
          window.postMessage(
            { type: "CW_AUTH_TOKEN", token },
            window.location.origin,
          );
        }
      }
    }
  } catch {
    // ignore
  }
  return originalFetch.apply(this, args);
};
