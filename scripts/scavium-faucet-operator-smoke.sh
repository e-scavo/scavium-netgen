#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${SCAVIUM_FAUCET_BASE_URL:-http://127.0.0.1:18080}"
ADMIN_TOKEN="${SCAVIUM_FAUCET_ADMIN_TOKEN:-}"

curl_check() {
  local name="$1"
  shift
  printf '[smoke] %s\n' "$name"
  curl -fsS --max-time 5 "$@" >/dev/null
}

curl_check "health" "$BASE_URL/health"
curl_check "ready" "$BASE_URL/ready"
curl_check "public status" "$BASE_URL/api/v1/status"
curl_check "public tokens" "$BASE_URL/api/v1/tokens"

if [ -n "$ADMIN_TOKEN" ]; then
  curl_check "admin dashboard" -H "Authorization: Bearer $ADMIN_TOKEN" "$BASE_URL/api/v1/admin/dashboard"
  curl_check "admin metrics json" -H "Authorization: Bearer $ADMIN_TOKEN" "$BASE_URL/api/v1/admin/metrics"
  curl_check "admin metrics prometheus" -H "Authorization: Bearer $ADMIN_TOKEN" "$BASE_URL/api/v1/admin/metrics/prometheus"
else
  printf '[smoke] admin checks skipped: SCAVIUM_FAUCET_ADMIN_TOKEN is not set\n'
fi

printf '[smoke] completed\n'
