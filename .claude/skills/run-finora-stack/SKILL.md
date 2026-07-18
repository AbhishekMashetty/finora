---
name: run-finora-stack
description: Use when asked to run, start, boot, launch, or verify the Finora app locally — or to confirm a code change works end-to-end. Brings up the full docker-compose stack (5 Go services + 4 MongoDB + Next.js frontend), waits for health, runs the auth curl flow, and can browser-verify the frontend. Encodes three real bugs already found and fixed once (macOS Go linker, duplicate CORS headers, Next.js standalone Docker healthcheck) so they aren't rediscovered.
---

# Running and Verifying the Finora Stack

This is the project's `run` path — read this in full before improvising a launch sequence, and prefer it over rediscovering things from scratch.

## 1. Bring the stack up

```bash
cd /Users/abhishekmasetty/Projects/finora
[ -f .env ] || cp .env.example .env    # never overwrite an existing .env
docker compose up --build -d
```

Wait for every container to report healthy (10 total: `mongo-user`, `mongo-expense`, `mongo-budget`, `mongo-notification`, `user-service`, `expense-service`, `budget-service`, `notification-service`, `gateway`, `frontend`):

```bash
docker compose ps
# or poll one container:
docker inspect --format='{{.State.Health.Status}}' finora-frontend-1
```

Bring-up order is enforced by `depends_on: condition: service_healthy` in `docker-compose.yml` (Mongos → their service → gateway → frontend), so a healthy `docker compose ps` really does mean the whole chain is good.

Ports: gateway `8080` (the only backend port a client should call), user-service `8081`, expense-service `8082`, budget-service `8083`, notification-service `8084`, frontend `3000`. Full contract: `architecture/api-contracts.md`.

## 2. Smoke-test the API through the gateway

```bash
curl -s -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"demo@finora.dev","password":"Sup3rSecret!","name":"Demo User"}'

LOGIN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"demo@finora.dev","password":"Sup3rSecret!"}')
ACCESS=$(echo "$LOGIN" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['access_token'])")

curl -s http://localhost:8080/api/v1/users/me -H "Authorization: Bearer $ACCESS"
curl -s -o /dev/null -w "no-token -> %{http_code}\n" http://localhost:8080/api/v1/users/me   # expect 401
```

Every response is the `{success, data, error, request_id}` envelope. If register 409s because the demo user already exists from a prior run, just log in instead — Mongo data persists in named volumes across `docker compose down` (not `down -v`).

## 3. Browser-verify the frontend (don't skip this for frontend changes)

`chromium-cli` is not installed in this environment. Use Playwright directly — install once per session in a scratch dir (not inside `frontend/`, to avoid polluting `package.json`):

```bash
mkdir -p /tmp/finora-browser-check && cd /tmp/finora-browser-check
npm init -y >/dev/null 2>&1 && npm install playwright >/dev/null 2>&1
npx playwright install chromium
```

Then drive it with a small Node script: `chromium.launch()` → `page.goto('http://localhost:3000')` → click `text=Log in` → `page.fill('input[type="email"]', ...)` / `input[type="password"]` → click `button[type="submit"]` → `page.waitForURL('**/dashboard')` → assert body text contains the user's name/email (proves the live `GET /users/me` call worked) → click a sidebar link (e.g. `a:has-text("Accounts")`) → click `text=Log out` (note: **"Log out" with a space**, not "Logout") → `page.waitForURL('**/login')`. Screenshot after each step and check `page.on('console', ...)` for errors — a page can render its shell while every fetch silently fails.

## 4. Known gotchas — already fixed, don't rediscover them

- **macOS Go 1.21 linker bug** (`dyld: missing LC_UUID load command` on native `go build`/`go test`): already fixed machine-wide via `go env -w GOFLAGS="-ldflags=-linkmode=external"`. Never affects Docker builds (Linux-only compilation). If you see this error, check `go env GOFLAGS` is still set — don't "fix" it by editing service code.
- **CORS is gateway-only.** Never add `middleware.CORS(...)` to a backend service's router. `httputil.ReverseProxy` copies backend response headers with `Header().Add`, so a backend also setting `Access-Control-Allow-Origin` produces a duplicate, comma-joined value that every browser rejects — even though curl-level testing looks fine (curl doesn't enforce CORS, only real browsers do, which is why this needs step 3 above, not just step 2, to catch). See `architecture/api-contracts.md`'s CORS section and `CLAUDE.md` §5.
- **Next.js standalone Docker + healthchecks:** `frontend/Dockerfile` already sets `ENV HOSTNAME=0.0.0.0` (Docker auto-sets `HOSTNAME` to the container ID, which Next's `server.js` would otherwise bind to instead of all interfaces) and uses `http://127.0.0.1:3000` in its `HEALTHCHECK` (not `localhost`, which resolves to `::1` first while the server only listens IPv4). If you touch this Dockerfile, keep both.
- `NEXT_PUBLIC_API_BASE_URL` is inlined into the client bundle at **build time** — it's a Docker build ARG (`docker-compose.yml`'s `frontend.build.args`), not a runtime `environment:` var. Changing it requires a rebuild, not just a container restart.

## 5. Tear down

```bash
docker compose down          # stops containers, keeps Mongo data volumes
docker compose down -v       # also wipes Mongo data (make clean-volumes does this too)
```
