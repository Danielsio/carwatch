// Bridge script running in the ISOLATED world.
//
// It receives the page's short-lived Firebase token from content.js and trades
// it, once, for the extension's OWN long-lived credential (POST /ext/token).
// That exchange is the whole reason the extension keeps scanning after the last
// CarWatch tab is closed: a Firebase ID token expires in about an hour and the
// extension cannot refresh one, so borrowing it meant scanning silently died an
// hour after the user navigated away.
//
// Everything crossing the page boundary is treated as hostile input:
//
//   - only messages from THIS window and THIS origin are considered (a message
//     from an iframe, or a page on another origin, is not our web app);
//   - the payload's shape is checked before use, so a script on the page cannot
//     poison the extension's stored credential with junk;
//   - the credential is written to chrome.storage.local, never .sync — sync
//     roams a live bearer token through Google's servers to every Chrome profile
//     the user is signed into, which is a lot of surface for a secret this
//     powerful.

const EXPECTED_ORIGIN = "https://carwatch.duckdns.org";

// A Firebase ID token is a JWT: three dot-separated base64url segments. Pinning
// the shape means a page script cannot hand us an arbitrary string and have the
// extension send it anywhere.
const JWT_RE = /^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/;

function isTrustedMessage(event) {
  if (event.source !== window) return false;
  // event.origin is set by the browser and cannot be forged by page script.
  if (event.origin !== window.location.origin) return false;
  if (window.location.origin !== EXPECTED_ORIGIN) return false;
  return true;
}

function looksLikeFirebaseToken(token) {
  return typeof token === "string" && token.length < 4096 && JWT_RE.test(token);
}

// Exchange the page's Firebase token for our own credential. Runs at most once
// per page load, and only when we do not already hold a working token.
async function exchangeForDeviceToken(firebaseToken) {
  const { deviceToken } = await chrome.storage.local.get("deviceToken");
  if (deviceToken) return; // already connected

  const resp = await fetch(`${EXPECTED_ORIGIN}/api/v1/ext/token`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${firebaseToken}`,
    },
    body: JSON.stringify({ label: navigator.userAgent.slice(0, 100) }),
  });
  if (!resp.ok) {
    console.warn("[CarWatch] extension token exchange failed:", resp.status);
    return;
  }

  const data = await resp.json();
  if (!data || typeof data.token !== "string" || !data.token.startsWith("cwx_")) {
    console.warn("[CarWatch] extension token exchange returned an unexpected payload");
    return;
  }

  await chrome.storage.local.set({
    deviceToken: data.token,
    deviceTokenExpiresAt: data.expires_at || null,
  });
  // The borrowed Firebase token has done its job; do not keep a copy of it.
  await chrome.storage.local.remove("authToken");
  await chrome.storage.sync.remove(["authToken"]);
  console.log("[CarWatch] extension connected with its own token");
}

let exchanging = false;

window.addEventListener("message", (event) => {
  if (!isTrustedMessage(event)) return;

  const data = event.data;
  if (!data || data.type !== "CW_AUTH_TOKEN") return;
  if (!looksLikeFirebaseToken(data.token)) return;

  if (exchanging) return;
  exchanging = true;
  exchangeForDeviceToken(data.token)
    .catch((err) => console.warn("[CarWatch] token exchange error:", err))
    .finally(() => {
      exchanging = false;
    });
});
