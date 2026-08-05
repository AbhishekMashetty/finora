#!/usr/bin/env bash
# Mixed-traffic load generator against the local Finora gateway.
#
# Fires a random mix of authenticated CRUD requests, malformed bodies, bad
# tokens, and 404 lookups at the gateway for a fixed duration, then reports
# a status-code breakdown. Useful for exercising the gateway's rate limiter
# (shared/middleware.RateLimit) and confirming the stack survives a burst
# without crashing. This is a smoke/soak test for local iteration, not a
# real benchmark -- one curl subprocess per request has its own overhead,
# so throughput numbers say more about this script than about the app.
#
# Because the gateway rate-limits (and JWT/validation-rejects) requests
# before they ever reach a backend service, most of the traffic this
# script generates never touches MongoDB -- only the requests that clear
# the rate limiter and pass validation do. To see how many really landed,
# check each service's own database afterward (e.g. count transactions
# with note="load test" in finora_expenses).
#
# Requires: curl, python3 (stdlib only -- used to pull the access_token out
# of the login response).
#
# Usage:
#   scripts/load_test.sh [duration_seconds] [max_concurrency]
#
# Examples:
#   scripts/load_test.sh                          # 60s, 25 concurrent requests
#   scripts/load_test.sh 30 10                     # 30s, 10 concurrent requests
#   GATEWAY_URL=http://localhost:9090 scripts/load_test.sh

set -uo pipefail

GATEWAY_URL="${GATEWAY_URL:-http://localhost:8080}"
DURATION_SECONDS="${1:-60}"
MAX_CONCURRENCY="${2:-25}"

RESULTS_FILE="$(mktemp -t finora-load-test)"
trap 'rm -f "$RESULTS_FILE"' EXIT

EMAIL="loadtest_$(date +%s)@example.com"
PASSWORD="LoadTest123!"

echo "Registering $EMAIL against $GATEWAY_URL ..."
curl -s -X POST "$GATEWAY_URL/api/v1/auth/register" -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\",\"name\":\"Load Test\"}" > /dev/null

LOGIN_RESPONSE=$(curl -s -X POST "$GATEWAY_URL/api/v1/auth/login" -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}")
ACCESS_TOKEN=$(echo "$LOGIN_RESPONSE" | python3 -c "import json,sys; print(json.load(sys.stdin)['data']['access_token'])" 2>/dev/null)

if [ -z "$ACCESS_TOKEN" ]; then
  echo "FATAL: login failed, aborting. Response was:"
  echo "$LOGIN_RESPONSE"
  exit 1
fi
AUTH_HEADER="Authorization: Bearer $ACCESS_TOKEN"

ACCOUNT_ID=$(curl -s -X POST "$GATEWAY_URL/api/v1/accounts" -H "$AUTH_HEADER" -H "Content-Type: application/json" \
  -d '{"name":"Load Test Checking","type":"checking","currency":"USD"}' \
  | python3 -c "import json,sys; print(json.load(sys.stdin)['data']['account']['id'])" 2>/dev/null)

curl -s -X POST "$GATEWAY_URL/api/v1/categories" -H "$AUTH_HEADER" -H "Content-Type: application/json" \
  -d '{"name":"loadtest","type":"expense"}' > /dev/null

curl -s -X POST "$GATEWAY_URL/api/v1/budgets" -H "$AUTH_HEADER" -H "Content-Type: application/json" \
  -d '{"category":"loadtest","amount":100,"period":"monthly"}' > /dev/null

echo "Seeded user=$EMAIL account=$ACCOUNT_ID"
echo "Firing mixed traffic for ${DURATION_SECONDS}s at up to ${MAX_CONCURRENCY} concurrent requests..."

fire() {
  local method="$1" path="$2" body="${3:-}" header="${4:-$AUTH_HEADER}"
  local status
  if [ -n "$body" ]; then
    status=$(curl -s -o /dev/null -w "%{http_code}" -m 5 -X "$method" "$GATEWAY_URL$path" \
      -H "$header" -H "Content-Type: application/json" -d "$body")
  else
    status=$(curl -s -o /dev/null -w "%{http_code}" -m 5 -X "$method" "$GATEWAY_URL$path" -H "$header")
  fi
  echo "$method $path -> $status" >> "$RESULTS_FILE"
}

now_iso() { date -u +%Y-%m-%dT%H:%M:%SZ; }

END=$((SECONDS + DURATION_SECONDS))
while [ $SECONDS -lt $END ]; do
  case $((RANDOM % 12)) in
    0) fire GET /api/v1/accounts ;;
    1) fire GET /api/v1/transactions ;;
    2) fire POST /api/v1/transactions "{\"account_id\":\"$ACCOUNT_ID\",\"type\":\"expense\",\"amount\":$((RANDOM % 200 + 1)),\"currency\":\"USD\",\"date\":\"$(now_iso)\",\"note\":\"load test\"}" ;;
    3) fire GET /api/v1/budgets ;;
    4) fire GET /api/v1/goals ;;
    5) fire GET "/api/v1/reports/summary?from=2026-01-01&to=2026-12-31" ;;
    6) fire GET /api/v1/notifications ;;
    7) fire GET /live ;;
    8) fire GET /ready ;;
    9) fire GET "/api/v1/accounts/doesnotexist999" ;;                      # expect 404
    10) fire POST /api/v1/accounts '{"name":""}' ;;                        # malformed -> expect 400
    11) fire GET /api/v1/accounts "" "Authorization: Bearer garbage" ;;    # bad token -> expect 401
  esac &

  # bash 3.2 (macOS's default /bin/bash) has no `wait -n`, so throttle by
  # polling the running-job count instead of blocking on the next exit.
  while [ "$(jobs -r | wc -l)" -ge "$MAX_CONCURRENCY" ]; do
    sleep 0.05
  done
done
wait

echo ""
echo "=== Status code breakdown ==="
awk '{print $NF}' "$RESULTS_FILE" | sort | uniq -c | sort -rn
echo ""
echo "Total requests fired: $(wc -l < "$RESULTS_FILE")"
