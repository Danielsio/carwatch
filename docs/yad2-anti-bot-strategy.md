# Yad2 Anti-Bot Strategy — Problem Analysis & Approach Comparison

**Author:** Daniel Sionov  
**Date:** 2026-05-06  
**Status:** Awaiting review from senior engineers / cyber team  

---

## 1. Problem Statement

CarWatch is a personal car-listing aggregator that periodically scrapes Yad2 (Israel's
largest classifieds site) for vehicle listings matching user-defined criteria.

The application needs to:
1. Fetch listing feeds (search results pages) — provides basic metadata (model, year, price, token).
2. Enrich individual listings by fetching each listing's detail page (`/vehicles/item/{token}`) — provides km, city, image URL.

**The km field is critical** — users set a maximum mileage filter. If enrichment fails,
km remains unknown (0), and the listing is filtered out as a safety measure (we cannot
show listings that might exceed the user's km limit).

### Anti-Bot System in Use

Yad2 uses **PerimeterX / HUMAN Security** (formerly ShieldSquare). Indicators observed:
- Challenge pages containing "perfdrive" and "shieldsquare" JavaScript
- Browser fingerprinting and TLS fingerprint analysis
- Rate limiting with escalating challenge frequency
- IP reputation scoring

### Impact on CarWatch

| Failure Mode | User Impact |
|---|---|
| Challenge on listing feed | Entire search returns 0 results for that cycle |
| Challenge on item detail page | Listing's km stays unknown → filtered out |
| Sustained challenges | Circuit breaker opens → no results for extended period |

---

## 2. What We've Implemented (Current State)

### 2.1 TLS Fingerprinting (azuretls-client)

Uses `azuretls-client` to present a Chrome-like TLS fingerprint (JA3/JA4). This is the
foundation — without it, Go's default `net/http` TLS handshake is trivially identifiable
as non-browser traffic.

### 2.2 Realistic Headers & User-Agent Rotation

Randomized User-Agent strings from a pool of real Chrome versions. Full header sets
mimicking real browser requests (Accept, Accept-Language, sec-ch-ua, etc.).

### 2.3 Proxy Pool with Rotation

A pool of residential/datacenter proxies. On challenge detection, the proxy is marked
unhealthy and rotated out. Provides IP diversity to avoid IP-based rate limiting.

### 2.4 Jittered Request Delays

- Between paginated pages: 1.5–2.5s (randomized)
- Between item enrichment requests: 1.5–3.5s (randomized)
- Prevents fixed-interval detection patterns

### 2.5 Shuffled Enrichment Order

Each cycle, the order of listings to enrich is shuffled randomly. Ensures different
listings get priority if the per-cycle cap is hit or a challenge aborts the batch.

### 2.6 Soft Resume on Challenge

Instead of aborting all enrichment on the first bot challenge:
1. Back off for 7 seconds
2. Rotate to the next proxy
3. Continue enriching remaining candidates
4. Only abort if 3 consecutive challenges are hit

### 2.7 DB Pre-Fill (Safety Net)

Once a listing's km/city/image is successfully enriched, it's stored in the database.
On subsequent cycles, if enrichment fails for that listing, the previously-stored data
is used. This means a listing only needs to be successfully enriched **once** — after
that it's immune to enrichment failures.

### 2.8 Circuit Breaker

Protects against cascading failures. If too many consecutive requests fail, the circuit
opens and stops all requests for a cooldown period, preventing IP/account burning.

---

## 3. Alternative Approaches

### 3.1 Authenticated Session (Login as Real User)

**Description:** Create a real Yad2 account and authenticate before scraping. Maintain
a session cookie for all requests.

**How it helps:**
- Authenticated users receive a higher trust score from PerimeterX
- Fewer initial challenge pages served to logged-in sessions
- Request pattern looks more like a real user browsing

**Risks:**
- Account ban = total loss of access (single point of failure)
- Need to handle login flow, token refresh, CAPTCHA during login
- More identifiable if Yad2 investigates (linked to phone/email)
- ToS violation is more clear-cut with an identifiable account

**Complexity:** Medium (login flow, session management, refresh logic)

| Criterion | Score (1-5) |
|---|---|
| Effectiveness against bot detection | 4 |
| Resilience (failure recovery) | 2 |
| Implementation complexity | 3 |
| Long-term sustainability | 2 |
| Risk of total lockout | High |
| **Overall** | **2.5/5** |

---

### 3.2 Headless Browser (Playwright / Puppeteer)

**Description:** Use a real browser engine to render pages, execute JavaScript challenges
natively, and extract data from the rendered DOM.

**How it helps:**
- Passes all JavaScript-based challenges natively (the browser solves them)
- Real browser fingerprint (not simulated)
- Can handle dynamic content loading

**Risks:**
- Very high resource consumption (RAM, CPU) — especially for frequent polling
- Slower than HTTP-based fetching (full page render per request)
- Still detectable via WebDriver flags, canvas fingerprinting, etc.
- Anti-detect browsers (Multilogin, GoLogin) add cost
- Increases infrastructure complexity significantly

**Complexity:** High (browser lifecycle, resource management, anti-detect config)

| Criterion | Score (1-5) |
|---|---|
| Effectiveness against bot detection | 5 |
| Resilience (failure recovery) | 3 |
| Implementation complexity | 1 |
| Long-term sustainability | 3 |
| Risk of total lockout | Medium |
| **Overall** | **3.0/5** |

---

### 3.3 Current Approach (Stealth HTTP + Proxy + DB Pre-Fill)

**Description:** The approach currently implemented — TLS fingerprinting, proxy rotation,
jittered delays, shuffled enrichment, soft challenge recovery, and DB pre-fill.

**How it helps:**
- No single point of failure (no account to ban, multiple proxies)
- DB pre-fill means enrichment only needs to succeed once per listing
- Graceful degradation (partial enrichment is still useful)
- Low resource footprint

**Risks:**
- PerimeterX may escalate detection (TLS fingerprint databases update)
- Residential proxy quality varies
- If Yad2 moves to mandatory JS challenges on all pages, HTTP-only breaks

**Complexity:** Medium (already implemented)

| Criterion | Score (1-5) |
|---|---|
| Effectiveness against bot detection | 3 |
| Resilience (failure recovery) | 5 |
| Implementation complexity | 4 (already done) |
| Long-term sustainability | 4 |
| Risk of total lockout | Low |
| **Overall** | **4.0/5** |

---

### 3.4 Hybrid: Authenticated + Anonymous Fallback

**Description:** Primary path uses an authenticated Yad2 session for higher trust score.
Falls back to anonymous + proxy rotation when the session is challenged or expires.

**How it helps:**
- Best of both: auth reduces challenge frequency, anonymous provides resilience
- If account is banned, system continues operating in anonymous mode
- Can use auth for enrichment (high-value, single-page requests) and anonymous for feeds

**Risks:**
- Additional complexity managing two code paths
- Account still at risk of being burned
- Need to monitor session health and switch automatically

**Complexity:** Medium-High

| Criterion | Score (1-5) |
|---|---|
| Effectiveness against bot detection | 4 |
| Resilience (failure recovery) | 4 |
| Implementation complexity | 2 |
| Long-term sustainability | 3 |
| Risk of total lockout | Low |
| **Overall** | **3.5/5** |

---

### 3.5 Official API / Data Partnership

**Description:** Contact Yad2 about an official API or data feed. Some classifieds sites
offer commercial access for aggregators.

**How it helps:**
- 100% reliable, no anti-bot issues
- Clean, structured data
- Legal and sustainable

**Risks:**
- Yad2 may not offer this (or may not offer it for individual/small projects)
- May be expensive
- May have usage restrictions

**Complexity:** Low (technical), High (business/negotiation)

| Criterion | Score (1-5) |
|---|---|
| Effectiveness against bot detection | 5 |
| Resilience (failure recovery) | 5 |
| Implementation complexity | 5 |
| Long-term sustainability | 5 |
| Risk of total lockout | None |
| **Overall** | **5.0/5** (if available) |

---

### 3.6 RSS / Alternative Data Sources

**Description:** Some aggregator sites or RSS feeds provide Yad2 listing data indirectly
(e.g., price comparison sites, car valuation services).

**How it helps:**
- Avoids Yad2's anti-bot entirely
- May provide pre-enriched data (km, city already included)

**Risks:**
- Data freshness may lag behind Yad2 directly
- Coverage may be incomplete
- Third-party dependency

**Complexity:** Low-Medium

| Criterion | Score (1-5) |
|---|---|
| Effectiveness against bot detection | 5 (no bot to fight) |
| Resilience (failure recovery) | 4 |
| Implementation complexity | 4 |
| Long-term sustainability | 3 |
| Risk of total lockout | Low |
| **Overall** | **3.5/5** |

---

## 4. Scoring Summary

| # | Approach | Overall Score | Best For |
|---|---|---|---|
| 3.5 | Official API | 5.0 | Long-term, if Yad2 offers it |
| 3.3 | Stealth HTTP + DB Pre-Fill (current) | 4.0 | Resilient daily operation |
| 3.4 | Hybrid Auth + Anonymous | 3.5 | Reducing challenge rate |
| 3.6 | Alternative Data Sources | 3.5 | Avoiding the problem entirely |
| 3.2 | Headless Browser | 3.0 | Guaranteed challenge bypass |
| 3.1 | Authenticated Only | 2.5 | Quick win, high risk |

---

## 5. Recommendation

**Short-term (now):** Stay with approach 3.3 (current implementation). Monitor success
rates over the next 1-2 weeks. The DB pre-fill ensures that once a listing is enriched,
it stays enriched permanently — meaning reliability improves over time as the database
accumulates data.

**Medium-term (if challenge rate exceeds 30%):** Layer on approach 3.4 (hybrid auth) for
enrichment requests specifically. Keep anonymous for feed fetching.

**Long-term:** Investigate 3.5 (official API) and 3.6 (alternative sources) to eliminate
the arms race entirely.

---

## 6. Questions for Review

1. Is the TLS fingerprint approach (`azuretls-client`) sufficient against PerimeterX's
   current detection, or should we invest in headless browser for enrichment?
2. What's the risk/reward of using an authenticated session? Is account burn acceptable
   if we have anonymous fallback?
3. Are there known Israeli proxy providers with better residential IP reputation for
   Yad2 specifically?
4. Should we consider a CAPTCHA-solving service (2captcha, Anti-Captcha) as an
   additional layer for when challenges are served?
5. Is there a way to reverse-engineer the PerimeterX challenge cookie generation
   without a full browser (e.g., isolated JS execution via V8/QuickJS)?

---

## 7. Technical Context

- **Language:** Go
- **HTTP Client:** azuretls-client (Chrome TLS fingerprint)
- **Anti-bot system:** PerimeterX / HUMAN Security
- **Polling interval:** Configurable (default ~10 min with jitter)
- **Enrichment cap:** Max listings per cycle to limit request volume
- **Data persistence:** PostgreSQL (prod) / SQLite (dev)
