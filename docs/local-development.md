# Local Development

## Prerequisites

- Docker + Docker Compose (v2, the `docker compose` CLI plugin — not the standalone `docker-compose` v1 binary)
- Go **1.21.3** (matches `shared/go.mod` and every service's `go.mod`; see the Troubleshooting section below for a real toolchain gotcha on this version)
- Node.js **20+** (for `frontend/`, Next.js App Router + TypeScript)
- `make` (every service and the repo root ship a `Makefile`)

## Primary Workflow: Docker Compose

```bash
git clone <this-repo>
cd finora
cp .env.example .env
docker compose up --build
```

This builds and starts: `gateway`, `user-service`, `expense-service`, `budget-service`, `notification-service`, their four dedicated MongoDB containers (`mongo-user`, `mongo-expense`, `mongo-budget`, `mongo-notification`), and `frontend`. Compose health checks poll each service's `/ready` endpoint; wait for every container to report `healthy` before hitting the API.

> **Status note:** `docker-compose.yml` is one of the last pieces wired up in this build. If it isn't present yet in your checkout, the above is the intended, documented workflow once it lands — check the repo root or `docker compose config` for current status. Everything else in this document (ports, health endpoints, curl flows, the native workflow) is accurate against the finalized `architecture/api-contracts.md` and `.env.example` regardless of that file's status.

Environment variables all come from `.env` (copied from `.env.example`) — see that file for the exact, current list (ports, Mongo URIs, JWT secrets/TTLs, CORS origins, log level, shutdown timeout). No service hardcodes configuration; see `CLAUDE.md` §2 and §11.

## Alternative Workflow: Native, Per-Service (tight iteration)

For fast edit/rebuild cycles on a single service without rebuilding its Docker image every time:

```bash
# 1. Bring up just the Mongo containers (and anything else this service needs) via compose:
docker compose up mongo-user   # substitute the Mongo this service owns

# 2. Run the service natively against it:
cd services/user-service
make run   # wraps `go run ./cmd/server`, reads .env from the repo root (or exported env vars)
```

This is the simplest way to get a real, reachable MongoDB for a native run without standing up a separate local Mongo install — you still get isolation (only that one service's Mongo container needs to be up) while iterating on Go code with normal `go run`/`go build` speed instead of a Docker rebuild per change.

### Connecting a GUI client (MongoDB Compass, etc.)

Each Mongo container's port 27017 is published to the host, one per service so they don't collide (`MONGO_USER_PORT`/`MONGO_EXPENSE_PORT`/`MONGO_BUDGET_PORT`/`MONGO_NOTIFICATION_PORT` in `.env.example`, defaulting to `27017`/`27018`/`27019`/`27020`). Connect Compass (or `mongosh`, or any client) with a plain, no-auth URI — these containers have no auth configured, matching this repo's "dev-only, no external network exposure" posture:

```
mongodb://localhost:27017/finora_users          # mongo-user
mongodb://localhost:27018/finora_expenses       # mongo-expense
mongodb://localhost:27019/finora_budgets        # mongo-budget
mongodb://localhost:27020/finora_notifications  # mongo-notification
```

This host port mapping is purely for local tooling — the services themselves never use `localhost`; they always talk to each other over the docker network by container name on the container-internal `27017` (`USER_SERVICE_MONGO_URI` etc. in `.env.example`), completely unaffected by which host port is mapped.

The service's own `Makefile` also exposes `make build`, `make test`, and `make tidy` — see each service's `README.md` for specifics; they all follow the same target names per `CLAUDE.md`.

## Testing

Two separate tiers, matching `CLAUDE.md` §7:

- **`make test`** (or `go test ./...` inside any service) — fast, unit-level, runs against hand-written in-memory fakes of the `domain` repository interfaces. No Docker, no network, no external state. This is what runs on every `go test` invocation and what CI's `go-unit` job runs.
- **`make test-integration`** — real-MongoDB integration tests, added in Phase 6 (`shared/mongotest`, backed by [testcontainers-go](https://golang.testcontainers.org/)). **Requires Docker running locally** — each test spins up a genuine, disposable `mongo:7` container (the same image `docker-compose.yml` runs in production), exercises the actual Mongo-backed repository implementation against it, and tears the container down automatically. Covers `user-service`, `expense-service`, `budget-service`, and `notification-service` (every service that owns a MongoDB — `gateway` owns no data, so it has none). These live in `internal/repository/*_integration_test.go` files, build-tagged `integration` so they're **excluded from `make test`/plain `go test ./...`** — they only run when explicitly requested:
  ```bash
  make test-integration
  # or, for a single service:
  cd services/expense-service && go test -tags=integration ./...
  ```
  This tier is slower (each test's own container startup, a few seconds) but tests real behavior a fake can't: that a unique index genuinely rejects a duplicate, that a compound index has the right field order, that a TTL index actually expires after the right duration, that ownership scoping is enforced by the real Mongo query and not just application code. CI runs this as its own `go-integration` job (`.github/workflows/ci.yml`), separate from `go-unit`, on GitHub's `ubuntu-latest` runners (Docker is pre-installed there, no extra setup needed).

## Ports

| Service | Host port | Notes |
|---|---|---|
| frontend | 3000 | http://localhost:3000 |
| gateway | 8080 | the only API entrypoint clients should use |
| user-service | 8081 | exposed for local debugging only |
| expense-service | 8082 | exposed for local debugging only |
| budget-service | 8083 | exposed for local debugging only |
| notification-service | 8084 | exposed for local debugging only |
| mongo-user | 27017 | |
| mongo-expense | 27018 | |
| mongo-budget | 27019 | |
| mongo-notification | 27020 | |

Full contract: `architecture/api-contracts.md`.

## Health Checks

Every service (including the gateway) exposes the same three endpoints. From the host, against the gateway:

```bash
curl localhost:8080/live
curl localhost:8080/ready
curl localhost:8080/health
```

And the same against each service directly (useful when isolating which container is unhealthy):

```bash
curl localhost:8081/live   && curl localhost:8081/ready   && curl localhost:8081/health   # user-service
curl localhost:8082/live   && curl localhost:8082/ready   && curl localhost:8082/health   # expense-service
curl localhost:8083/live   && curl localhost:8083/ready   && curl localhost:8083/health   # budget-service
curl localhost:8084/live   && curl localhost:8084/ready   && curl localhost:8084/health   # notification-service
```

`/live` returns `200` as soon as the process is up, with no dependency checks — it should never flap due to Mongo being slow. `/ready` returns `503` until the service's Mongo ping succeeds — this is the one Compose/Kubernetes should gate traffic on. `/health` aggregates the same checks with a richer payload for humans. All three are implemented once in `shared/health` and identical across every service.

## Sample End-to-End Curl Flow

Register, log in, fetch the authenticated user, then refresh the token pair — all through the gateway:

```bash
# 1. Register
curl -s -X POST localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"jane@example.com","password":"correct-horse-battery-staple","name":"Jane Doe"}'
# -> 201, envelope with {user}

# 2. Login
curl -s -X POST localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"jane@example.com","password":"correct-horse-battery-staple"}'
# -> 200, envelope with {access_token, refresh_token, user}

# Save the tokens from the response, then:
ACCESS_TOKEN="<paste access_token here>"
REFRESH_TOKEN="<paste refresh_token here>"

# 3. Fetch the authenticated user
curl -s localhost:8080/api/v1/users/me \
  -H "Authorization: Bearer $ACCESS_TOKEN"
# -> 200, envelope with {user}

# 4. Refresh the token pair
curl -s -X POST localhost:8080/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d "{\"refresh_token\":\"$REFRESH_TOKEN\"}"
# -> 200, envelope with {access_token, refresh_token} (rotated)
```

Every response above is wrapped in the standard `{success, data, error, request_id}` envelope — see `architecture/api-contracts.md` for the full shape and error codes.

## Troubleshooting

**Where did MongoDB's logs go? `docker compose logs mongo-user` shows nothing**

By design, as of 2026-07-19 — see `docker-compose.yml`'s `x-mongo-command` comment for the full story. mongod's default logging emits a connection-lifecycle line quartet (accepted/client-metadata/access-check/ended) for *every* client connection, including the 5s healthcheck reconnecting constantly across all 4 Mongo containers — real noise, but `--quiet` and even an explicit `--setParameter logComponentVerbosity` override (confirmed applied, had zero effect) can't suppress it: component verbosity only gates Debug-level messages, never Informational, and connection logging is Informational. The actual fix redirects mongod's own log output to `/data/db/mongod.log` (inside the persistent volume) instead of stdout, so `docker compose logs` — which only ever sees a container's stdout/stderr — goes quiet. The data is still there: `docker compose exec mongo-user cat /data/db/mongod.log` (or `tail -f` for a live stream). This is a real trade-off: if mongod ever fails to start outright, `docker compose logs` won't show why anymore — go straight to the log file (or, if the container won't even stay up long enough to `exec` into, mount the same named volume from a throwaway container to read it).

**`dyld: missing LC_UUID load command` on native `go build` / `go test` (macOS)**

On newer macOS versions, Go 1.21.3's internal linker can produce binaries missing an `LC_UUID` load command that `dyld` expects, causing native builds/tests to fail to execute with a `dyld` error — this is a known Go-internal-linker/macOS-dyld incompatibility for this Go version, not a bug in this codebase.

This has already been fixed globally on this machine by forcing the external (system) linker instead of Go's internal one:

```bash
go env -w GOFLAGS="-ldflags=-linkmode=external"
```

This is a one-time, machine-wide `go env` setting (persisted in `$GOENV`), not something to add per-service or commit into any Makefile.

**This only affects native host builds** (`go build`, `go run`, `go test` executed directly on macOS). It **never affects Docker builds** — every Dockerfile compiles for Linux inside a Linux container, where this macOS/dyld interaction doesn't exist. If `docker compose up --build` works but `make run`/`go test ./...` fails locally with an `LC_UUID` error, this is the cause — verify the `GOFLAGS` setting above is in effect (`go env GOFLAGS`) rather than suspecting the code.

**Container reports unhealthy / `/ready` returns 503**

Almost always Mongo connectivity: confirm the service's `*_MONGO_URI` env var matches its dedicated Mongo container's compose service name (not `localhost`) and that the corresponding `mongo-*` container is itself healthy (`docker compose ps`). Each service's readiness is gated solely on its own Mongo ping (`shared/mongox.Checker`) — a different service's Mongo being down has no effect on this one's `/ready`.

**`401 UNAUTHORIZED` on a route you expect to be public**

Check `architecture/api-contracts.md`'s Public vs Protected Routes list — only `register`, `login`, and `refresh` bypass auth at the gateway. Everything else, including `logout`, requires a valid `Authorization: Bearer` header.

**Browser console: "Access-Control-Allow-Origin header contains multiple values" / CORS errors from the frontend**

CORS is exclusively the **gateway's** responsibility — it is the only component a browser ever talks to directly. Backend services (`user-service`, `expense-service`, `budget-service`, `notification-service`) must **not** register `shared/middleware.CORS(...)` themselves: `httputil.ReverseProxy` copies the backend's response headers onto the gateway's response using `Header().Add` (not `Set`), so if a backend also sets `Access-Control-Allow-Origin`, the browser sees it twice, comma-joined, and rejects the response outright even though every individual request succeeded. This exact bug shipped once during Phase 0's initial build and was caught by end-to-end browser verification (curl alone doesn't trigger CORS enforcement — only a real browser does). If you're adding a new service, only wire `middleware.CORS` into the **gateway's** router, never a backend's.

**Next.js frontend container: healthcheck fails / `wget: can't connect to remote host: Connection refused` from inside the container, even though the host can reach it**

Two independent standalone-Next.js-in-Docker gotchas, both already fixed in `frontend/Dockerfile`:
1. Docker sets the `HOSTNAME` env var to the container ID by default, and Next's standalone `server.js` binds to `process.env.HOSTNAME || '0.0.0.0'` — without an explicit `ENV HOSTNAME=0.0.0.0` override, it ends up listening only on the container's own interface IP, not loopback.
2. Even with that fixed, `localhost` resolves to `::1` (IPv6) before `127.0.0.1` inside the container, and the server only listens on IPv4 — so a healthcheck or debugging command against `http://localhost:3000` still gets refused. Use `http://127.0.0.1:3000` explicitly instead. (Go services aren't affected: `net.Listen("tcp", ":PORT")` binds dual-stack, so `localhost` works fine against the gateway and every backend service.)

**Every timezone except `UTC` gets rejected as `VALIDATION_ERROR` when updating settings, even a genuinely valid IANA name like `Europe/London`**

`user-service`'s `PUT /api/v1/users/me/settings` validates `timezone` via Go's `time.LoadLocation`, which reads the IANA tzdata files from disk (`/usr/share/zoneinfo`) rather than embedding them — and the `alpine:3.19` base image `services/user-service/Dockerfile` builds its final stage from doesn't ship tzdata by default. `LoadLocation("UTC")` always succeeds (Go special-cases it, no disk lookup needed) so this can look like the validation logic itself is broken when it's actually the base image missing a package — and worse, `go test` never catches it, since unit tests run on the host machine (which has tzdata), not inside the container. Found live while building the Settings frontend page in Phase 5: a real zone was rejected with the exact same error message a garbage string gets. Fixed by adding `RUN apk add --no-cache tzdata` to the Dockerfile's final stage. If you ever see every timezone but `UTC` fail validation against a *running container* while the exact same input passes in `go test`, this is the cause — check whether `/usr/share/zoneinfo` exists inside the container (`docker compose exec user-service sh -c "ls /usr/share/zoneinfo"`) before suspecting the validation code.
