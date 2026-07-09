#!/usr/bin/env bash
#
# web-bench.sh — measure web delivery latency for one or more origins so you can
# compare the CURRENT setup (Oracle VM in Jerusalem, app served by the Go API
# behind Caddy) against a FUTURE Vercel deployment of the SPA — and prove whether
# moving the frontend actually moved the numbers.
#
# It separates the three timings that matter for the "should we move to Vercel?"
# decision:
#
#   1. STATIC (app-shell HTML + a hashed JS asset) — this is what a CDN/Vercel
#      edge would serve. Improvement here = the upside of moving the frontend.
#   2. API (/healthz) — this STAYS on the Oracle backend no matter where the
#      frontend lives. It is the control: it should NOT change between origins.
#      If the API is the slow part, Vercel cannot help it.
#   3. The network breakdown (DNS / TCP / TLS / TTFB) so you can see whether a
#      difference comes from edge proximity or from server think-time.
#
# Each request opens a fresh connection (no keep-alive) to model distinct cold
# visitors, and reports min / median / p95 / max over N samples — tail latency
# is where a single small VM hurts, so the median alone would lie.
#
# Usage:
#   scripts/web-bench.sh                                  # baseline: current origin
#   scripts/web-bench.sh -n 30                            # 30 samples per endpoint
#   scripts/web-bench.sh current=https://carwatch.duckdns.org vercel=https://carwatch.vercel.app
#   scripts/web-bench.sh -a /assets/index-XXpart.js current=https://... vercel=https://...
#
# Targets are "label=url" pairs. With no targets it defaults to the current
# origin. Requires: curl, awk, sort (present in Git Bash on Windows).
#
# Note: deliberately NOT using `set -e`/pipefail — curl|grep|head pipelines emit
# benign SIGPIPE/no-match exits that would otherwise abort a perfectly good run.
set -u

SAMPLES=20
ASSET_OVERRIDE=""
# Paths probed on every target. The app-shell is no-cache HTML (re-fetched each
# load); /healthz is the cheapest API round-trip.
SHELL_PATH="/"
API_PATH="/healthz"

usage() { sed -n '2,40p' "$0" | sed 's/^# \{0,1\}//'; exit "${1:-0}"; }

TARGETS=()
while [ $# -gt 0 ]; do
  case "$1" in
    -n) SAMPLES="$2"; shift 2 ;;
    -a) ASSET_OVERRIDE="$2"; shift 2 ;;
    -h|--help) usage 0 ;;
    *=*) TARGETS+=("$1"); shift ;;
    *) echo "unknown arg: $1" >&2; usage 1 ;;
  esac
done
[ ${#TARGETS[@]} -eq 0 ] && TARGETS=("current=https://carwatch.duckdns.org")

CURL_OPTS=(-ks --no-keepalive -H 'Accept-Encoding: br,gzip,zstd' -A 'carwatch-web-bench/1')
TIMEFMT='%{time_namelookup} %{time_connect} %{time_appconnect} %{time_starttransfer} %{time_total} %{size_download} %{http_code}'

# stats <space-separated-ms-values>  ->  "min median p95 max mean"
stats() {
  printf '%s\n' "$@" | sort -n | awk '
    { v[NR]=$1; sum+=$1 }
    END {
      n=NR; if (n==0){ print "- - - - -"; exit }
      p95i=int(0.95*(n-1))+1; medi=int(0.50*(n-1))+1;
      printf "%.0f %.0f %.0f %.0f %.1f", v[1], v[medi], v[p95i], v[n], sum/n
    }'
}

# one-off header probe: report encoding + any CDN/edge cache markers
probe_headers() {
  local url="$1"
  curl "${CURL_OPTS[@]}" -I "$url" 2>/dev/null | awk '
    BEGIN{IGNORECASE=1}
    /^content-encoding:/  {enc=$2}
    /^cache-control:/     {sub(/^[^:]*: */,""); cc=$0}
    /^age:/               {age=$2}
    /^x-vercel-cache:/    {sub(/^[^:]*: */,""); cdn="vercel:" $0}
    /^cf-cache-status:/   {sub(/^[^:]*: */,""); cdn="cf:" $0}
    /^x-cache:/           {sub(/^[^:]*: */,""); cdn="x-cache:" $0}
    END{ printf "enc=%s cdn=%s", (enc?enc:"-"), (cdn?cdn:"-") }'
}

discover_asset() {
  local base="$1"
  # --compressed so curl decompresses the body before we grep it (the timing
  # path below intentionally leaves bodies compressed; discovery must not).
  curl -ks --compressed -A 'carwatch-web-bench/1' "$base/" 2>/dev/null \
    | grep -oE '/assets/[^"]+\.(js|css)' | head -1
}

bench_path() {
  local label="$1" base="$2" path="$3" kind="$4"
  local url="${base%/}$path"
  local ttfbs=() totals=() size="" code="" enc_cdn=""
  enc_cdn="$(probe_headers "$url" || true)"
  for _ in $(seq 1 "$SAMPLES"); do
    # On failure curl still writes the -w format with zeros + http_code 000;
    # `|| true` just keeps set -e happy. (Do NOT `|| echo ...` — TIMEFMT has no
    # trailing newline, so the echo would concatenate onto curl's output and
    # corrupt the read.)
    read -r dns conn tls ttfb total sz hc < <(curl "${CURL_OPTS[@]}" -o /dev/null -w "$TIMEFMT" "$url" 2>/dev/null || true)
    ttfbs+=("$(awk -v x="$ttfb" 'BEGIN{print x*1000}')")
    totals+=("$(awk -v x="$total" 'BEGIN{print x*1000}')")
    size="$sz"; code="$hc"
  done
  local ts; ts="$(stats "${ttfbs[@]}")"
  local tt; tt="$(stats "${totals[@]}")"
  # columns: kind path  ttfb(min/med/p95/max)  total(med/p95)  size  code  enc/cdn
  printf "  %-13s %-10s ttfb[ %-6s %-6s %-6s %-6s ]  total[ med %-6s p95 %-6s ]  %sB  %s  %s\n" \
    "$kind" "$path" \
    $(echo "$ts" | awk '{print $1, $2, $3, $4}') \
    "$(echo "$tt" | awk '{print $2}')" "$(echo "$tt" | awk '{print $3}')" \
    "$size" "$code" "$enc_cdn"
}

echo "================================================================================"
echo " CarWatch web delivery benchmark   samples=$SAMPLES   $(date -u '+%Y-%m-%d %H:%M:%SZ')"
VANTAGE_JSON="$(curl -ks https://ipinfo.io/json 2>/dev/null || true)"
V_CITY="$(printf '%s\n' "$VANTAGE_JSON" | grep '"city"'    | sed -E 's/.*: *"([^"]*)".*/\1/')"
V_CC="$(printf '%s\n'   "$VANTAGE_JSON" | grep '"country"' | sed -E 's/.*: *"([^"]*)".*/\1/')"
echo " vantage: ${V_CITY:-unknown}, ${V_CC:-?}"
echo " times in ms. ttfb = time-to-first-byte (network + server think-time)."
echo "================================================================================"

for t in "${TARGETS[@]}"; do
  label="${t%%=*}"; base="${t#*=}"
  echo ""
  echo "▶ $label  ($base)"
  asset="${ASSET_OVERRIDE:-$(discover_asset "$base" || true)}"
  bench_path "$label" "$base" "$SHELL_PATH" "app-shell"
  if [ -n "$asset" ]; then
    bench_path "$label" "$base" "$asset" "static-asset"
  else
    echo "  (no /assets/* asset discovered — pass one with -a /assets/foo.js)"
  fi
  bench_path "$label" "$base" "$API_PATH" "api"
done

cat <<'EOF'

--------------------------------------------------------------------------------
Reading the result:
  • app-shell + static-asset  → what Vercel/CDN edge would serve. A real win
    shows up here as lower ttfb (and a cdn= hit marker on repeat runs).
  • api (/healthz)            → control. This stays on the Oracle VM in Jerusalem
    in BOTH worlds. If it is the slow line, moving the frontend changes nothing.
  • Compare ttfb p95/max, not just median — the small VM's pain is tail latency.
Run the same command against both origins once Vercel is live, e.g.:
  scripts/web-bench.sh -n 30 current=https://carwatch.duckdns.org vercel=https://<app>.vercel.app
--------------------------------------------------------------------------------
EOF
