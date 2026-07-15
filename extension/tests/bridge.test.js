const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

// bridge.js runs in the extension's ISOLATED world and is the security boundary
// where the page hands the extension a credential. It is not a module, so it is
// loaded into a sandbox with a fake window + chrome, and its message handler is
// exercised the way the browser would deliver a postMessage event.

function loadBridge({ origin = "https://carwatch.duckdns.org", storage = {}, fetchImpl } = {}) {
  const listeners = [];
  const calls = { fetch: [], setLocal: [], removeLocal: [], removeSync: [] };

  const chromeStorageLocal = {
    async get(keys) {
      const out = {};
      for (const k of [].concat(keys)) if (k in storage) out[k] = storage[k];
      return out;
    },
    async set(obj) {
      calls.setLocal.push(obj);
      Object.assign(storage, obj);
    },
    async remove(keys) {
      calls.removeLocal.push(keys);
    },
  };

  const sandbox = {
    console: { log() {}, warn() {} },
    JSON,
    navigator: { userAgent: "TestBrowser/1.0" },
    window: {
      location: { origin },
      addEventListener(type, fn) {
        if (type === "message") listeners.push(fn);
      },
    },
    chrome: {
      storage: {
        local: chromeStorageLocal,
        sync: { async remove(keys) { calls.removeSync.push(keys); } },
      },
    },
    fetch:
      fetchImpl ||
      (async (url, opts) => {
        calls.fetch.push({ url, opts });
        return {
          ok: true,
          async json() {
            return { token: "cwx_new-device-token", expires_at: "2027-01-01T00:00:00Z" };
          },
        };
      }),
  };
  vm.createContext(sandbox);
  vm.runInContext(fs.readFileSync(path.join(__dirname, "..", "bridge.js"), "utf8"), sandbox);
  const windowRef = sandbox.window;

  // Deliver a message the way the browser does, populating event.origin.
  const dispatch = async (event) => {
    for (const fn of listeners) await fn(event);
    // give the async exchange a tick to run
    await new Promise((r) => setImmediate(r));
    await new Promise((r) => setImmediate(r));
  };
  return { dispatch, calls, storage, windowRef };
}

const firebaseToken = "aaa.bbb.ccc"; // JWT shape

test("ignores a message whose source is not the window (e.g. an iframe)", async () => {
  const bridge = loadBridge();
  await bridge.dispatch({
    source: {}, // not === window
    origin: "https://carwatch.duckdns.org",
    data: { type: "CW_AUTH_TOKEN", token: firebaseToken },
  });
  assert.equal(bridge.calls.fetch.length, 0, "a message from a foreign source must be ignored");
});

test("ignores a message whose origin is not our own", async () => {
  const bridge = loadBridge();
  // event.source === window but origin is an attacker's page.
  await bridge.dispatch(makeEvent(bridge, { origin: "https://evil.example.com", token: firebaseToken }));
  assert.equal(bridge.calls.fetch.length, 0, "a cross-origin message must never trigger an exchange");
});

test("ignores a token that is not JWT-shaped", async () => {
  const bridge = loadBridge();
  await bridge.dispatch(makeEvent(bridge, { token: "not-a-jwt" }));
  assert.equal(bridge.calls.fetch.length, 0, "a malformed token must not be sent anywhere");
});

test("ignores the wrong message type", async () => {
  const bridge = loadBridge();
  await bridge.dispatch(makeEvent(bridge, { type: "SOMETHING_ELSE", token: firebaseToken }));
  assert.equal(bridge.calls.fetch.length, 0);
});

test("performs the exchange and stores the device token in local, not sync", async () => {
  const bridge = loadBridge();
  await bridge.dispatch(makeEvent(bridge, { token: firebaseToken }));

  assert.equal(bridge.calls.fetch.length, 1, "exactly one exchange call");
  const call = bridge.calls.fetch[0];
  assert.match(call.url, /\/api\/v1\/ext\/token$/);
  assert.equal(call.opts.method, "POST");
  assert.match(call.opts.headers.Authorization, /^Bearer aaa\.bbb\.ccc$/);

  assert.equal(bridge.storage.deviceToken, "cwx_new-device-token", "device token stored in local");
  assert.ok(bridge.calls.removeSync.length > 0, "the borrowed Firebase token is purged from sync");
});

test("does not exchange again when a device token already exists", async () => {
  const bridge = loadBridge({ storage: { deviceToken: "cwx_existing" } });
  await bridge.dispatch(makeEvent(bridge, { token: firebaseToken }));
  assert.equal(bridge.calls.fetch.length, 0, "already connected — no second exchange");
});

test("does not store a payload that is not a cwx_ token", async () => {
  const bridge = loadBridge({
    fetchImpl: async () => ({ ok: true, async json() { return { token: "not-a-device-token" }; } }),
  });
  await bridge.dispatch(makeEvent(bridge, { token: firebaseToken }));
  assert.equal(bridge.storage.deviceToken, undefined, "a non-cwx_ response must be rejected");
});

// makeEvent builds a browser-shaped MessageEvent with event.source === window.
function makeEvent(bridge, { type = "CW_AUTH_TOKEN", origin = "https://carwatch.duckdns.org", token } = {}) {
  return {
    source: bridge.windowRef, // event.source === window: the message is from this page
    origin,
    data: { type, token },
  };
}
