# Monitoring & alerting

CarWatch runs on a single VM at personal scale, so the alerting baseline is a
lightweight **dead-man's switch** rather than a full Prometheus/Alertmanager
stack. It pages the Telegram admin when something is actually wrong and stays
silent otherwise.

## What is monitored

`scripts/health-alert.sh` runs every 15 minutes (systemd timer) and inspects
each app service's `/healthz`:

| Condition | Source | Why it matters |
|---|---|---|
| Service unreachable | no `/healthz` response | container down / crash loop |
| `status: "degraded"` | `internal/health` (set when the scraper has had no successful cycle within its degraded threshold) | scraping has stalled — the product's core function |
| `persist_failures` increased | F10 counter (`internal/scheduler/processing.go`) | listings delayed (retried next cycle) |
| `claim_release_failures` increased | F10 counter | **permanent** listing loss for a user — investigate |

The failure counters are compared against the previous run (state in
`/var/lib/carwatch/health-alert.state`) so a single incident pages once rather
than every 15 minutes.

## Install (on the VM)

```bash
sudo cp /home/ubuntu/carwatch/scripts/carwatch-health-alert.service /etc/systemd/system/
sudo cp /home/ubuntu/carwatch/scripts/carwatch-health-alert.timer   /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now carwatch-health-alert.timer

# verify
systemctl list-timers carwatch-health-alert.timer
sudo systemctl start carwatch-health-alert.service && journalctl -u carwatch-health-alert -n 20
```

The script reads `TELEGRAM_BOT_TOKEN` from `.env` and the admin chat id from
`config.yaml` (`telegram.admin_chat_id`) or `CARWATCH_ADMIN_CHAT_ID` in `.env`.
It exits 0 even when it alerts, so a noisy run never trips other automation.

## Metrics endpoint (optional)

Each service also exposes Prometheus/OTel metrics at `/metrics` (auth-gated on
non-local binds via `telemetry.auth_token`), including
`carwatch.listings.persist_failures` and
`carwatch.dedup.claim_release_failures`. Point a scraper at it if/when a
Grafana stack is added; the dead-man's switch above is the zero-infrastructure
baseline until then.

## Related

- [yad2-blocked-runbook.md](yad2-blocked-runbook.md) — when scraping is being
  blocked rather than broken.
- [backup-and-recovery.md](backup-and-recovery.md) — the analogous backup
  timer.
