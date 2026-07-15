const test = require("node:test");
const assert = require("node:assert/strict");

const {
  pickCanaryToken,
  canaryConfirmsEndpointHealthy,
  removalCandidates,
} = require("../lib.js");

test("pickCanaryToken returns a token that is live in this cycle's feed", () => {
  const byToken = new Map([["live-1", {}], ["live-2", {}]]);
  const canary = pickCanaryToken(byToken);
  assert.ok(byToken.has(canary), "the canary must be a token we KNOW is live");
});

test("pickCanaryToken returns null for an empty feed", () => {
  // Nothing to vouch for → removals get skipped anyway.
  assert.equal(pickCanaryToken(new Map()), null);
});

test("canaryConfirmsEndpointHealthy only trusts a real 200 item page", () => {
  assert.ok(canaryConfirmsEndpointHealthy({ status: 200, body: '{"data":{"token":"x"}}' }));

  // The failure modes that must NOT green-light removals:
  assert.ok(!canaryConfirmsEndpointHealthy({ status: 404, body: "" }), "endpoint 404s → broken");
  assert.ok(!canaryConfirmsEndpointHealthy({ status: 200, body: "<html>Verifying...</html>" }), "challenge shell");
  assert.ok(!canaryConfirmsEndpointHealthy({ status: 0, error: "NetworkError" }), "network error");
  assert.ok(!canaryConfirmsEndpointHealthy(null), "no canary result at all");
  assert.ok(!canaryConfirmsEndpointHealthy(undefined));
});

// The scenario the whole tripwire exists for, simulated end-to-end over the
// pure helpers: Yad2 renames the item endpoint, so EVERYTHING 404s — including
// listings that are demonstrably still live (they came back in the feed this
// very cycle). Not one of them may be retired.
test("a broken item endpoint (everything 404s) retires nothing", () => {
  // This cycle's feed still returned these — they are alive.
  const byToken = new Map([["alive-1", {}], ["alive-2", {}]]);
  // Our cache also holds tokens no longer in the feed: removal candidates.
  const cache = { "alive-1": {}, "alive-2": {}, "cand-1": {}, "cand-2": {} };

  const candidates = removalCandidates(cache, byToken);
  assert.deepEqual(candidates.sort(), ["cand-1", "cand-2"]);

  // The canary is a live token; the broken endpoint 404s it too.
  const canary = pickCanaryToken(byToken);
  const canaryResult = { url: `item/${canary}`, status: 404, body: "" };

  const removed = [];
  if (canaryConfirmsEndpointHealthy(canaryResult)) {
    // (not taken — this is the bug path we are proving cannot run)
    for (const c of candidates) removed.push(c);
  }
  assert.equal(removed.length, 0, "a broken endpoint must retire zero listings");
});

// The healthy path still works: the canary resolves, so genuine 404s retire.
test("a healthy endpoint still retires the genuinely-gone listings", () => {
  const byToken = new Map([["alive-1", {}]]);
  const cache = { "alive-1": {}, "sold-1": {} };
  const candidates = removalCandidates(cache, byToken);

  const canary = pickCanaryToken(byToken);
  const canaryResult = { url: `item/${canary}`, status: 200, body: '{"data":{"token":"alive-1"}}' };

  const removed = [];
  if (canaryConfirmsEndpointHealthy(canaryResult)) {
    for (const c of candidates) {
      // sold-1 genuinely 404s
      const r = { status: 404 };
      if (r.status === 404) removed.push(c);
    }
  }
  assert.deepEqual(removed, ["sold-1"], "genuine sales are still retired when the endpoint is healthy");
});
