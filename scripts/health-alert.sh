#!/usr/bin/env bash
set -euo pipefail

# CarWatch health alerter / dead-man's switch.
#
# Run periodically (see carwatch-health-alert.timer). For each app service it
# checks /healthz and alerts the Telegram admin when:
#   - a service is unreachable (dead-man's switch), or
#   - a service reports status "degraded" (the scraper sets this when no
#     successful cycle has completed within its degraded threshold), or
#   - the scraper's persist-failure / claim-release-failure counters have
#     increased since the last run (the only permanent listing-loss path; see
#     internal/scheduler/processing.go and the F10 metrics).
#
# Designed to be dependency-light: no jq required, only docker + wget + curl.
# Exit 0 always (a monitoring run failing should not page on its own); problems
# are reported via Telegram.

COMPOSE_DIR="${1:-/home/ubuntu/carwatch}"
STATE_DIR="${CARWATCH_STATE_DIR:-/var/lib/carwatch}"
STATE_FILE="${STATE_DIR}/health-alert.state"

SERVICES=(api bot-poller scraper notifier)
HEALTH_PORTS=(8080 8082 8081 8083)

TOKEN=$(grep -oP 'TELEGRAM_BOT_TOKEN=\K[^\s]+' "${COMPOSE_DIR}/.env" 2>/dev/null || echo "")
ADMIN_CHAT=$(grep -oP 'admin_chat_id:\s*\K[0-9]+' "${COMPOSE_DIR}/config.yaml" 2>/dev/null || echo "")
[ -z "$ADMIN_CHAT" ] && ADMIN_CHAT=$(grep -oP 'CARWATCH_ADMIN_CHAT_ID=\K[0-9]+' "${COMPOSE_DIR}/.env" 2>/dev/null || echo "")

alerts=()

notify() {
  local msg="$1"
  echo "ALERT: ${msg}"
  if [ -n "$TOKEN" ] && [ -n "$ADMIN_CHAT" ]; then
    curl -fsS --max-time 10 \
      "https://api.telegram.org/bot${TOKEN}/sendMessage" \
      --data-urlencode "chat_id=${ADMIN_CHAT}" \
      --data-urlencode "text=🚨 CarWatch: ${msg}" >/dev/null 2>&1 || \
      echo "WARN: failed to send Telegram alert"
  else
    echo "WARN: TELEGRAM_BOT_TOKEN or admin chat id not found; cannot send alert"
  fi
}

# Fetch a service's /healthz body via the container (matches smoke-test.sh).
health_body() {
  local svc="$1" port="$2"
  docker exec "carwatch-${svc}" wget -qO- --timeout=5 "http://localhost:${port}/healthz" 2>/dev/null || true
}

# Extract an integer field like "field":123 from a JSON blob (no jq).
json_int() {
  local body="$1" field="$2"
  echo "$body" | grep -oP "\"${field}\":\s*\K[0-9]+" | head -1
}

for i in "${!SERVICES[@]}"; do
  svc="${SERVICES[$i]}"
  port="${HEALTH_PORTS[$i]}"
  body="$(health_body "$svc" "$port")"

  if [ -z "$body" ]; then
    alerts+=("service '${svc}' is UNREACHABLE on :${port}/healthz")
    continue
  fi
  if echo "$body" | grep -q '"status":"degraded"'; then
    alerts+=("service '${svc}' reports status=degraded")
  fi
done

# Scraper failure counters: alert only on an increase since the last run.
scraper_body="$(health_body scraper 8081)"
if [ -n "$scraper_body" ]; then
  pf="$(json_int "$scraper_body" persist_failures)"; pf="${pf:-0}"
  cr="$(json_int "$scraper_body" claim_release_failures)"; cr="${cr:-0}"

  prev_pf=0; prev_cr=0
  if [ -f "$STATE_FILE" ]; then
    # shellcheck disable=SC1090
    source "$STATE_FILE" 2>/dev/null || true
    prev_pf="${PREV_PERSIST_FAILURES:-0}"
    prev_cr="${PREV_CLAIM_RELEASE_FAILURES:-0}"
  fi

  if [ "$pf" -gt "$prev_pf" ]; then
    alerts+=("scraper persist_failures rose ${prev_pf} → ${pf} (listings delayed; retried next cycle)")
  fi
  if [ "$cr" -gt "$prev_cr" ]; then
    alerts+=("scraper claim_release_failures rose ${prev_cr} → ${cr} — PERMANENT listing loss; investigate")
  fi

  mkdir -p "$STATE_DIR" 2>/dev/null || true
  {
    echo "PREV_PERSIST_FAILURES=${pf}"
    echo "PREV_CLAIM_RELEASE_FAILURES=${cr}"
  } > "$STATE_FILE" 2>/dev/null || true
fi

if [ "${#alerts[@]}" -eq 0 ]; then
  echo "OK: all services healthy"
  exit 0
fi

msg="$(printf '%s; ' "${alerts[@]}")"
notify "${msg%; }"
exit 0
