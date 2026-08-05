# scripts/

Helper scripts for local development. Not part of any service's build —
nothing here ships in a Docker image.

## `seed_demo_data.py`

Populates a demo user's accounts, categories, transactions, budgets, and
savings goals through the live gateway API, so the frontend has real-
looking data to click through instead of empty screens. Every value is
synthetic, but shaped to look real: actual-style bank/card account names,
merchant names per category, income paychecks on a biweekly cadence, and a
couple of categories deliberately budgeted to go "over" so the Reports
page's budget-vs-actual bars and the notification feed both have something
to show.

Requires only Python 3's standard library — no `pip install` needed.

```bash
# bring the stack up first
docker compose up --build -d

# then, from the repo root:
python3 scripts/seed_demo_data.py
```

Logs in at `http://localhost:3000/login` afterward with `demo@example.com`
/ `DemoPass123!` (both overridable via `--email`/`--password`). Run
`python3 scripts/seed_demo_data.py --help` for all options (transaction
count, months of history, a random seed for reproducible runs, a different
`--base-url` for a non-default gateway port).

Re-running with the same `--email` adds more data on top of whatever that
user already has — pass a different `--email` for a separate, independent
demo user instead.

## `dummy_data.sh`

A shell/curl counterpart to `seed_demo_data.py`, for a quick, easy-to-edit
one-off instead of a fuller synthetic dataset. Seeds every gateway-backed
page — profile, settings, accounts, categories, transactions, budgets,
goals — purely over the live API, and deliberately creates its budgets
*before* its one over-budget transaction, so Phase 7's event-driven chain
(`finora.transaction.created` → budget-service recomputes actual spend →
`finora.budget.overspent` → notification-service) fires a real notification
on its own, rather than needing a manual trigger. Doesn't seed Search (a
client-side composition over the above, needs no data of its own) or the
public landing/login/register pages.

Requires only `curl` and `python3` (stdlib only, used both for parsing JSON
responses and computing dates portably across macOS/Linux `date` flag
differences) — no `pip install` needed.

```bash
# bring the stack up first
docker compose up --build -d

# then, from the repo root:
scripts/dummy_data.sh

# override the demo user or gateway URL
EMAIL=me@example.com PASSWORD='Something123!' scripts/dummy_data.sh
GATEWAY_URL=http://localhost:9090 scripts/dummy_data.sh
```

Logs in at `http://localhost:3000/login` afterward with
`dummy@example.com` / `DummyPass123!` by default (both overridable via
`EMAIL`/`PASSWORD` env vars). Re-running with the same `EMAIL` adds more
data on top of whatever that user already has, same fallback behavior as
`seed_demo_data.py`.

## `load_test.sh`

Fires a random mix of authenticated CRUD requests, malformed bodies, bad
tokens, and 404 lookups at the gateway for a fixed duration, then reports a
status-code breakdown. Useful for exercising the gateway's rate limiter
(`shared/middleware.RateLimit`) and confirming the stack survives a burst
without crashing — a local smoke/soak test, not a real benchmark (one curl
subprocess per request has its own overhead).

Requires only `curl` and `python3` (stdlib only) — no `pip install` needed.

```bash
# bring the stack up first
docker compose up --build -d

# then, from the repo root: 60s at up to 25 concurrent requests (defaults)
scripts/load_test.sh

# or override duration / concurrency
scripts/load_test.sh 30 10

# or point at a non-default gateway
GATEWAY_URL=http://localhost:9090 scripts/load_test.sh
```

Registers a fresh throwaway user each run (`loadtest_<timestamp>@example.com`),
seeds it with one account/category/budget, then fires against
`/api/v1/accounts`, `/transactions`, `/budgets`, `/goals`,
`/reports/summary`, `/notifications`, `/live`, `/ready`, plus one
intentionally-invalid request of each kind (nonexistent resource → `404`,
empty required field → `400`, garbage bearer token → `401`).

**Most of this traffic never reaches MongoDB.** The gateway rejects
over-budget requests (`429`) and bad tokens (`401`) itself, before
proxying anywhere, and handler-layer validation failures (`400`) never
reach a repository call either — only requests that clear the rate limiter
and pass validation actually touch a service's database. To see how much
really landed, check the relevant service's Mongo directly afterward, e.g.:

```bash
docker compose exec mongo-expense mongosh finora_expenses --quiet \
  --eval 'db.transactions.countDocuments({note: "load test"})'
```
