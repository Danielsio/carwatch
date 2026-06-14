# Runbook: Yad2 is blocking us

**Audience:** operator on call. **Goal:** recognize, triage, and recover when Yad2's anti-bot system (PerimeterX / HUMAN) starts challenging CarWatch's scraper or enricher.

This is the operational counterpart to the design analysis in
[../yad2-anti-bot-strategy.md](../yad2-anti-bot-strategy.md). Read that for *why*
the stealth client is shaped the way it is; read this when results have stopped
flowing.

---

## 1. What the system does automatically (do not fight it)

Most challenge episodes are self-correcting. Before intervening, know what is
already happening:

- **Circuit breaker** (`internal/fetcher/circuitbreaker.go`, wired in
  `internal/app/app.go`): after **5 consecutive fetch failures** it opens and
  stops hitting Yad2 for a **10-minute cooldown**, then half-opens to test
  recovery. This is intentional load-shedding — an open breaker is the system
  protecting itself and our IP reputation, not a bug.
- **Adaptive scheduler backoff**: on `ErrChallenge`/`ErrRateLimited` the poll
  interval multiplier grows (×2, capped ×4) and halves back toward normal on
  success.
- **Enricher adaptive rate limiter** (`internal/enricher/ratelimit.go`): doubles
  its per-request delay on a challenge and enters cooldown, recovering on
  success.
- **Bounded in-flight fetches** (`internal/fetcher/yad2/client.go`): stranded
  requests are capped, so a sustained block cannot pile up connections.
- **Per-listing gone-detection**: 404/410 item pages are dead-lettered, not
  retried (so a wave of removed listings is not mistaken for a block).

**Default action for a short episode (< ~30 min): do nothing.** Watch and let
backoff + circuit breaker ride it out.

## 2. Detect & confirm

Symptoms: notifications dry up; admin dashboard shows cycles completing with 0
new listings; listings stuck without km/city/image.

Confirm it is a block (not an outage or a code bug) via the **per-source health
metrics** on the scraper/enricher `/healthz`:

```bash
# scraper (8081), enricher (8084) — adjust host as needed
curl -s http://localhost:8081/healthz | jq '.sources.yad2'
```

Look at the `yad2` source object:
- `challenges` climbing relative to `fetches` → anti-bot is challenging us.
- `success_rate` dropping toward 0, `last_success` going stale → active block.
- `errors` climbing without `challenges` → likely a *different* problem
  (network, Yad2 outage, markup change) — see §5.

Cross-check logs (component `yad2`, `circuit_breaker`, `scheduler`,
`enricher`): look for `anti-bot challenge detected`, `circuit breaker ...
opened`, `bot challenge during enrichment, backing off`.

## 3. Triage by severity

| Situation | Action |
|---|---|
| Brief spike, breaker still closed, `success_rate` recovering | **Wait.** Backoff is handling it. |
| Breaker open, intermittent success on half-open | **Wait** one or two cooldowns (10–20 min). Recovery is expected. |
| Sustained block > ~1h, `success_rate` ≈ 0 across cycles | Intervene — §4. |
| `challenges` ≈ 0 but `errors` high | Not a block — §5. |

## 4. Intervention (sustained block)

Apply the **least invasive** step first; re-check `/healthz` after each.

1. **Rotate / refresh proxies.** Blocks are usually IP-reputation driven. If
   running with a proxy pool (`http.proxies` in `config.yaml`), replace exhausted
   proxies with fresh residential IPs and redeploy the scraper + enricher. If
   running without proxies, this is the single highest-leverage fix — add some.
2. **Back off harder, voluntarily.** Temporarily raise `polling.interval` (e.g.
   15m → 45m) and lower `polling.max_concurrent_fetches`. Slower + gentler often
   restores reputation faster than hammering through. Redeploy.
3. **Pause scraping to let reputation cool.** Stop the scraper (and enricher)
   for 30–60 min:
   ```bash
   docker compose -f docker-compose.prod.yaml stop scraper enricher
   # ...wait...
   docker compose -f docker-compose.prod.yaml start scraper enricher
   ```
   The API, bot, and notifier keep serving existing data while paused.
4. **Verify the request shape still matches a real browser.** If challenges are
   total and immediate (not reputation-based), Yad2 may have changed detection.
   Confirm UA list, header order, and TLS fingerprint in
   `internal/fetcher/yad2/client.go` are current. **Do not casually edit these** —
   they are the anti-bot envelope; changes need testing against a live challenge.
   Escalate per [../yad2-anti-bot-strategy.md](../yad2-anti-bot-strategy.md).

## 5. Not actually a block?

If `errors` are high but `challenges` are ~0:
- **Markup/parse change**: logs show `parse page`/`parse item` errors and the
  enrich dead-letter stream (`carwatch:enrich:dead`) fills up. Yad2 changed its
  `__NEXT_DATA__` structure → parser needs updating (`parser.go` /
  `item_parser.go`). This is a code fix, not an ops action.
- **Yad2 outage / network**: errors are timeouts/5xx. Wait; the circuit breaker
  and backoff will ride it out.
- **Our infra**: check the scraper/enricher containers are healthy and Redis is
  reachable.

## 6. After recovery

- Confirm `/healthz` `yad2.success_rate` is back near normal and `last_success`
  is recent.
- Revert any temporary `polling.interval` / `max_concurrent_fetches` changes.
- If you rotated proxies or changed the request shape, note what worked in the
  PR/issue so the next on-call has the context.
