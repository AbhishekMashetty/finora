#!/usr/bin/env bash
# Seeds every Finora frontend page with dummy data, purely over the live
# gateway API (the same endpoints the browser uses) -- no direct Mongo
# access, nothing that could drift from what a real user session produces.
#
# Covers: profile, settings, accounts, categories, transactions (enough for
# Overview/Transactions/Reports to have real data), budgets, savings goals,
# and -- since budgets are created before the deliberately-over-budget
# transaction below -- a real notification via Phase 7's event-driven chain
# (transaction.created -> budget-service recomputes actual spend ->
# budget.overspent -> notification-service), not a manual trigger. The only
# pages this doesn't touch are Search (a client-side composition over the
# above, needs no data of its own) and the public landing/login/register
# pages.
#
# This is a shell/curl counterpart to scripts/seed_demo_data.py (Python) --
# prefer that script for anything more elaborate (configurable transaction
# volume/history, a random seed for reproducible runs); this one is
# deliberately small and easy to read/edit inline for a quick one-off.
#
# NOT run automatically -- review it, then run manually:
#   docker compose up --build -d   # bring the stack up first
#   scripts/dummy_data.sh
#
# Override the demo user or gateway URL:
#   EMAIL=me@example.com PASSWORD='Something123!' scripts/dummy_data.sh
#   GATEWAY_URL=http://localhost:9090 scripts/dummy_data.sh
#
# Requires: curl, python3 (stdlib only -- used to pull IDs/tokens out of
# JSON responses and to compute dates portably across macOS/Linux `date`
# flag differences).

set -uo pipefail

GATEWAY_URL="${GATEWAY_URL:-http://localhost:8080}"
EMAIL="${EMAIL:-dummy@example.com}"
PASSWORD="${PASSWORD:-DummyPass123!}"
NAME="${NAME:-Dummy User}"

# ---- helpers ---------------------------------------------------------------

# Pulls a value out of a JSON response via a small python expression, e.g.:
#   echo "$RESPONSE" | json_get "d['data']['account']['id']"
json_get() {
  python3 -c "import json,sys; d=json.load(sys.stdin); print($1)" 2>/dev/null
}

api() {
  local method="$1" path="$2" body="${3:-}"
  local auth_args=()
  [ -n "${AUTH_HEADER:-}" ] && auth_args=(-H "$AUTH_HEADER")
  if [ -n "$body" ]; then
    curl -s -X "$method" "$GATEWAY_URL$path" -H "Content-Type: application/json" "${auth_args[@]}" -d "$body"
  else
    curl -s -X "$method" "$GATEWAY_URL$path" -H "Content-Type: application/json" "${auth_args[@]}"
  fi
}

today_iso() { python3 -c "import datetime; print(datetime.date.today().isoformat())"; }
days_ago_iso() { python3 -c "import datetime,sys; print((datetime.date.today()-datetime.timedelta(days=int(sys.argv[1]))).isoformat())" "$1"; }
months_out_iso() {
  python3 -c "
import datetime, sys
n = int(sys.argv[1])
today = datetime.date.today()
month = today.month - 1 + n
year = today.year + month // 12
month = month % 12 + 1
day = min(today.day, 28)
print(datetime.date(year, month, day).isoformat())
" "$1"
}

# ---- auth: register-or-login, matching seed_demo_data.py's fallback -------

echo "Registering (or logging in as) $EMAIL against $GATEWAY_URL ..."
REGISTER=$(api POST /api/v1/auth/register "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\",\"name\":\"$NAME\"}")
if echo "$REGISTER" | grep -q '"CONFLICT"'; then
  echo "  $EMAIL already exists -- logging in and adding more data on top"
fi

LOGIN=$(api POST /api/v1/auth/login "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}")
ACCESS_TOKEN=$(echo "$LOGIN" | json_get "d['data']['access_token']")
if [ -z "$ACCESS_TOKEN" ]; then
  echo "FATAL: login failed. Response was:"
  echo "$LOGIN"
  exit 1
fi
AUTH_HEADER="Authorization: Bearer $ACCESS_TOKEN"

# ---- Profile & Settings pages ----------------------------------------------

echo "Updating profile & settings..."
api PUT /api/v1/users/me "{\"name\":\"$NAME\"}" > /dev/null
api PUT /api/v1/users/me/settings '{"currency":"USD","timezone":"America/Los_Angeles"}' > /dev/null

# ---- Accounts page ----------------------------------------------------------

echo "Creating accounts..."
CHECKING_ID=$(api POST /api/v1/accounts '{"name":"Everyday Checking","type":"checking","currency":"USD"}' | json_get "d['data']['account']['id']")
api PUT "/api/v1/accounts/$CHECKING_ID" '{"name":"Everyday Checking","type":"checking","currency":"USD","balance":3400}' > /dev/null

SAVINGS_ID=$(api POST /api/v1/accounts '{"name":"High-Yield Savings","type":"savings","currency":"USD"}' | json_get "d['data']['account']['id']")
api PUT "/api/v1/accounts/$SAVINGS_ID" '{"name":"High-Yield Savings","type":"savings","currency":"USD","balance":9800}' > /dev/null

CREDIT_ID=$(api POST /api/v1/accounts '{"name":"Rewards Credit Card","type":"credit","currency":"USD"}' | json_get "d['data']['account']['id']")
api PUT "/api/v1/accounts/$CREDIT_ID" '{"name":"Rewards Credit Card","type":"credit","currency":"USD","balance":-450}' > /dev/null

echo "  checking=$CHECKING_ID savings=$SAVINGS_ID credit=$CREDIT_ID"

# ---- Categories -------------------------------------------------------------

echo "Creating categories..."
GROCERIES_ID=$(api POST /api/v1/categories '{"name":"Groceries","type":"expense"}' | json_get "d['data']['category']['id']")
DINING_ID=$(api POST /api/v1/categories '{"name":"Dining","type":"expense"}' | json_get "d['data']['category']['id']")
TRANSPORT_ID=$(api POST /api/v1/categories '{"name":"Transportation","type":"expense"}' | json_get "d['data']['category']['id']")
ENTERTAINMENT_ID=$(api POST /api/v1/categories '{"name":"Entertainment","type":"expense"}' | json_get "d['data']['category']['id']")
SALARY_ID=$(api POST /api/v1/categories '{"name":"Salary","type":"income"}' | json_get "d['data']['category']['id']")

# ---- Budgets page (created BEFORE the over-budget transaction below --
# Phase 7's overspend check runs off the transaction.created event, so the
# budget must already exist for the async chain to find it over spent) ----

echo "Creating budgets (Entertainment deliberately set low, to go over)..."
api POST /api/v1/budgets '{"category":"Groceries","amount":400,"period":"monthly"}' > /dev/null
api POST /api/v1/budgets '{"category":"Dining","amount":200,"period":"monthly"}' > /dev/null
api POST /api/v1/budgets '{"category":"Transportation","amount":150,"period":"monthly"}' > /dev/null
api POST /api/v1/budgets '{"category":"Entertainment","amount":40,"period":"monthly"}' > /dev/null

# ---- Transactions page (+ feeds Overview/Reports) ---------------------------

echo "Creating transactions..."
api POST /api/v1/transactions "{\"account_id\":\"$CHECKING_ID\",\"category_id\":\"$SALARY_ID\",\"type\":\"income\",\"amount\":3200,\"currency\":\"USD\",\"date\":\"$(days_ago_iso 14)T00:00:00Z\",\"note\":\"Acme Corp Payroll\"}" > /dev/null
api POST /api/v1/transactions "{\"account_id\":\"$CHECKING_ID\",\"category_id\":\"$SALARY_ID\",\"type\":\"income\",\"amount\":3200,\"currency\":\"USD\",\"date\":\"$(days_ago_iso 0)T00:00:00Z\",\"note\":\"Acme Corp Payroll\"}" > /dev/null

GROCERY_MERCHANTS=("Whole Foods Market" "Trader Joe's" "Safeway" "Costco Wholesale")
for i in 1 2 3 4 5; do
  m="${GROCERY_MERCHANTS[$((RANDOM % 4))]}"
  amt=$(( (RANDOM % 80) + 20 ))
  api POST /api/v1/transactions "{\"account_id\":\"$CHECKING_ID\",\"category_id\":\"$GROCERIES_ID\",\"type\":\"expense\",\"amount\":$amt,\"currency\":\"USD\",\"date\":\"$(days_ago_iso $((i * 3)))T00:00:00Z\",\"note\":\"$m\"}" > /dev/null
done

DINING_MERCHANTS=("Starbucks" "Chipotle Mexican Grill" "Sweetgreen" "Local Bistro")
for i in 1 2 3 4 5 6; do
  m="${DINING_MERCHANTS[$((RANDOM % 4))]}"
  amt=$(( (RANDOM % 45) + 8 ))
  api POST /api/v1/transactions "{\"account_id\":\"$CREDIT_ID\",\"category_id\":\"$DINING_ID\",\"type\":\"expense\",\"amount\":$amt,\"currency\":\"USD\",\"date\":\"$(days_ago_iso $((i * 2)))T00:00:00Z\",\"note\":\"$m\"}" > /dev/null
done

for i in 1 2 3; do
  amt=$(( (RANDOM % 60) + 15 ))
  api POST /api/v1/transactions "{\"account_id\":\"$CREDIT_ID\",\"category_id\":\"$TRANSPORT_ID\",\"type\":\"expense\",\"amount\":$amt,\"currency\":\"USD\",\"date\":\"$(days_ago_iso $((i * 5)))T00:00:00Z\",\"note\":\"Uber\"}" > /dev/null
done

# Deliberately over the 40/month Entertainment budget set above -- this is
# the transaction whose finora.transaction.created event should trigger a
# real finora.budget.overspent -> notification chain (see architecture/
# api-contracts.md's Async Events section).
api POST /api/v1/transactions "{\"account_id\":\"$CREDIT_ID\",\"category_id\":\"$ENTERTAINMENT_ID\",\"type\":\"expense\",\"amount\":75,\"currency\":\"USD\",\"date\":\"$(today_iso)T00:00:00Z\",\"note\":\"AMC Theatres\"}" > /dev/null

# ---- Goals page --------------------------------------------------------------

echo "Creating savings goals..."
GOAL1_ID=$(api POST /api/v1/goals "{\"name\":\"Emergency Fund\",\"target_amount\":10000,\"target_date\":\"$(months_out_iso 8)\"}" | json_get "d['data']['goal']['id']")
api PUT "/api/v1/goals/$GOAL1_ID" "{\"name\":\"Emergency Fund\",\"target_amount\":10000,\"target_date\":\"$(months_out_iso 8)\",\"current_amount\":3200}" > /dev/null

GOAL2_ID=$(api POST /api/v1/goals "{\"name\":\"Trip to Japan\",\"target_amount\":5000,\"target_date\":\"$(months_out_iso 5)\"}" | json_get "d['data']['goal']['id']")
api PUT "/api/v1/goals/$GOAL2_ID" "{\"name\":\"Trip to Japan\",\"target_amount\":5000,\"target_date\":\"$(months_out_iso 5)\",\"current_amount\":900}" > /dev/null

# ---- Reports page (a plain read -- the notification above already fired
# asynchronously; this just warms/exercises the budget-vs-actual endpoint) --

echo "Reading this month's report (Reports page data)..."
MONTH_FROM=$(python3 -c "import datetime; print(datetime.date.today().replace(day=1).isoformat())")
api GET "/api/v1/reports/summary?from=${MONTH_FROM}&to=$(today_iso)" > /dev/null

echo ""
echo "Done. Notifications page may take a few seconds to show the overspend"
echo "alert -- it travels expense-service -> outbox -> NATS -> budget-service"
echo "-> outbox -> NATS -> notification-service (default relay poll: 2s each hop)."
echo ""
echo "Log in at http://localhost:3000/login with:"
echo "  email:    $EMAIL"
echo "  password: $PASSWORD"
