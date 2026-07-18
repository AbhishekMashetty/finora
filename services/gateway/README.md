# gateway

The single public entry point for the Finora API. Every client request
(frontend, mobile, curl) hits the gateway first — it is the only Finora
component that validates JWTs, and the only service port meant to be called
from outside the docker/Kubernetes network (see
`architecture/api-contracts.md`).

For each request the gateway:

1. Runs the shared middleware chain: `RequestID -> Logging -> Recovery ->
   BodyLimit -> CORS -> RateLimit` (Phase 6). `BodyLimit` caps request body
   size (`MAX_REQUEST_BODY_BYTES`, default 1 MiB) as early as possible, so an
   oversized body is rejected before CORS/proxying do any work; an oversized
   request is reported as `400 VALIDATION_ERROR`, not a generic proxy error
   — see `services/gateway/internal/proxy/proxy.go`'s `ErrorHandler`, which
   special-cases this. `RateLimit` is a per-client-IP token bucket
   (`RATE_LIMIT_REQUESTS_PER_SECOND`/`RATE_LIMIT_BURST`). Both fail open if
   unset — see `architecture/api-contracts.md`'s Rate Limiting section.
2. Serves `/live`, `/ready`, `/health` directly (public, via `shared/health`;
   the gateway owns no data, so it registers no checkers — `/ready` always
   mirrors `/live`) and `GET /openapi.yaml` (the gateway's own aggregate
   OpenAPI spec, served live — see `architecture/api-contracts.md`'s OpenAPI
   section).
3. For the three public auth routes, proxies straight through with no JWT
   check.
4. For every other `/api/v1/*` route, validates the `Authorization: Bearer
   <token>` header as an **access** token (`shared/jwtx`), and on success
   overwrites `X-User-Id` with the token's `sub` claim and sets
   `X-Request-Id`, before reverse-proxying to the owning backend. A missing,
   malformed, expired, or wrong-type token is rejected with `401
   UNAUTHORIZED` and never reaches a backend.

The gateway never generates tokens (only user-service does) and never trusts
any `X-User-Id` sent by the original client — it is always overwritten with
the value validated from the JWT.

## Routing table

| Route(s)                                                                 | Auth      | Backend                  |
|---------------------------------------------------------------------------|-----------|---------------------------|
| `POST /api/v1/auth/register`                                              | public    | `USER_SERVICE_URL`        |
| `POST /api/v1/auth/login`                                                 | public    | `USER_SERVICE_URL`        |
| `POST /api/v1/auth/refresh`                                               | public    | `USER_SERVICE_URL`        |
| `POST /api/v1/auth/logout`                                                | protected | `USER_SERVICE_URL`        |
| `GET/PUT /api/v1/users/me`, `GET/PUT /api/v1/users/me/settings`           | protected | `USER_SERVICE_URL`        |
| `/api/v1/accounts*`, `/api/v1/transactions*`, `/api/v1/categories*`       | protected | `EXPENSE_SERVICE_URL`     |
| `/api/v1/budgets*`, `/api/v1/goals*`, `/api/v1/reports*`                  | protected | `BUDGET_SERVICE_URL`      |
| `/api/v1/notifications*`                                                  | protected | `NOTIFICATION_SERVICE_URL`|
| `GET /live` `/ready` `/health`                                            | public    | gateway itself            |

Every matched request is forwarded with its full original path and query
string unchanged — the gateway does no path rewriting.

## Environment variables

| Variable                    | Meaning                                              |
|------------------------------|-------------------------------------------------------|
| `GATEWAY_PORT`               | Local bind port; binds `0.0.0.0:<port>` (default `8080`) |
| `USER_SERVICE_URL`           | Base URL of user-service                              |
| `EXPENSE_SERVICE_URL`        | Base URL of expense-service                           |
| `BUDGET_SERVICE_URL`         | Base URL of budget-service                            |
| `NOTIFICATION_SERVICE_URL`   | Base URL of notification-service                      |
| `JWT_ACCESS_SECRET`          | HS256 secret used to verify access tokens (must match user-service's signing secret) |
| `LOG_LEVEL`                  | `debug`/`info`/`warn`/`error` (default `info`)         |
| `SHUTDOWN_TIMEOUT`           | Graceful shutdown drain period, e.g. `10s` (default `10s`) |
| `CORS_ALLOWED_ORIGINS`       | Comma-separated list of allowed origins (default `http://localhost:3000`) |
| `RATE_LIMIT_REQUESTS_PER_SECOND` | Per-client-IP sustained request rate (default `10`); `<= 0` disables rate limiting |
| `RATE_LIMIT_BURST`           | Per-client-IP burst allowance above the sustained rate (default `20`); `<= 0` disables rate limiting |
| `MAX_REQUEST_BODY_BYTES`     | Max request body size in bytes before a `400 VALIDATION_ERROR` (default `1048576`, 1 MiB); `<= 0` disables the limit |

Exact names match `.env.example` at the repo root. `USER_SERVICE_URL`,
`EXPENSE_SERVICE_URL`, `BUDGET_SERVICE_URL`, `NOTIFICATION_SERVICE_URL` and
`JWT_ACCESS_SECRET` are required — the process panics at boot if any is
unset (fail fast rather than serve traffic it can't route or authenticate).

## Running standalone

```sh
cd services/gateway
export GATEWAY_PORT=8080
export USER_SERVICE_URL=http://localhost:8081
export EXPENSE_SERVICE_URL=http://localhost:8082
export BUDGET_SERVICE_URL=http://localhost:8083
export NOTIFICATION_SERVICE_URL=http://localhost:8084
export JWT_ACCESS_SECRET=dev-access-secret-change-me
make run
```

Normally the gateway is run as one container among several via the root
`docker-compose.yml` (not yet present in this repo — being built separately),
which wires up all four backend services plus the frontend on the shared
`finora` network, so `*_SERVICE_URL` values point at container DNS names
(e.g. `http://user-service:8081`) rather than `localhost`.

## Build & test

```sh
make build   # go build -o bin/server ./cmd/server
make test    # go test ./...
make tidy    # go mod tidy
```

## Docker

The build context must be the **repo root**, since the image needs both
`shared/` and `services/gateway/`:

```sh
# from repo root:
docker build -f services/gateway/Dockerfile -t finora/gateway .
# or, from services/gateway:
make docker-build
```

## Module setup

This module (`github.com/finora/gateway`) depends on the shared module
(`github.com/finora/shared`) via a local `replace` directive in `go.mod`
pointing at `../../shared`. This is temporary — it will be superseded by a
repo-root `go.work` workspace once that's added (not owned by this service).
