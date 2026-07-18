# expense-service

Owns accounts, transactions, and categories for Finora. Every resource is
scoped to the caller's user id (read from the `X-User-Id` header set by the
gateway) — there are no public routes in this service.

See `architecture/api-contracts.md` at the repo root for the authoritative,
cross-service contract (response envelope, error codes, auth header
contract). This README covers only what's specific to expense-service.

## Endpoints

All routes are prefixed `/api/v1` and require an authenticated caller
(`X-User-Id` header). Cross-user access to a resource that exists but is
owned by someone else returns `404 NOT_FOUND` (never `403`), so responses
never confirm the existence of another user's data.

| Method | Path                      | Body                                                      | Response                                  |
|--------|---------------------------|------------------------------------------------------------|--------------------------------------------|
| GET    | `/api/v1/accounts`        | -                                                            | `200 {accounts: []}`                        |
| POST   | `/api/v1/accounts`        | `{name, type, currency}`                                     | `201 {account}`                             |
| GET    | `/api/v1/accounts/:id`    | -                                                            | `200 {account}`                             |
| PUT    | `/api/v1/accounts/:id`    | `{name, type, currency, balance?}`                           | `200 {account}`                             |
| DELETE | `/api/v1/accounts/:id`    | -                                                            | `204`                                        |
| GET    | `/api/v1/transactions`    | query: `account_id, category, from, to, page, page_size`     | `200 {transactions: [], page, total}`       |
| POST   | `/api/v1/transactions`    | `{account_id, category_id?, type, amount, currency, date, note?}` | `201 {transaction}`                     |
| GET    | `/api/v1/transactions/:id`| -                                                            | `200 {transaction}`                         |
| PUT    | `/api/v1/transactions/:id`| `{account_id, category_id?, type, amount, currency, date, note?}` | `200 {transaction}`                    |
| DELETE | `/api/v1/transactions/:id`| -                                                            | `204`                                        |
| GET    | `/api/v1/categories`      | -                                                            | `200 {categories: []}`                       |
| POST   | `/api/v1/categories`      | `{name, type}`                                               | `201 {category}`                            |

Categories are intentionally minimal (create + list only — no update/delete
yet).

Account `type` is one of: `checking`, `savings`, `credit`, `cash`.
Transaction/category `type` is one of: `income`, `expense`.

`GET /transactions` pagination: `page` defaults to 1, `page_size` defaults to
20 and is capped at 100. Optional filters: `account_id`, `category`, and a
`from`/`to` date range applied to the transaction's `date` field (accepts
RFC3339 or `YYYY-MM-DD`).

## Health

Every service exposes the same trio (implemented once in `shared/health`):

```
GET /live    -> 200 always, if the process is up
GET /ready   -> 200 only if MongoDB ping succeeds, else 503
GET /health  -> aggregate, richer payload
```

## Environment variables

| Variable                     | Purpose                                          | Example                                         |
|-------------------------------|---------------------------------------------------|--------------------------------------------------|
| `EXPENSE_SERVICE_PORT`        | Port to bind (`0.0.0.0:<port>`)                    | `8082`                                             |
| `EXPENSE_SERVICE_MONGO_URI`   | MongoDB connection string (required, no default)   | `mongodb://mongo-expense:27017/finora_expenses`   |
| `LOG_LEVEL`                   | `debug`, `info` (default), `warn`, `error`         | `info`                                             |
| `SHUTDOWN_TIMEOUT`            | Graceful shutdown drain window                     | `10s`                                              |
| `CORS_ALLOWED_ORIGINS`        | Accepted for config-load compatibility but **unused** — CORS is applied only by the gateway (see `architecture/api-contracts.md`); a backend applying it too duplicates the header via the reverse proxy | `http://localhost:3000` |

See the repo root `.env.example` for the exact names shared across services.

## Running standalone

```sh
cd services/expense-service
export EXPENSE_SERVICE_MONGO_URI=mongodb://localhost:27018/finora_expenses
export EXPENSE_SERVICE_PORT=8082
make run
```

Requires a reachable MongoDB instance at `EXPENSE_SERVICE_MONGO_URI` — there
is no in-memory fallback, since `/ready` is meant to genuinely reflect
database connectivity.

Normally this service is not run standalone: it's one of several services
started together via the root `docker-compose.yml` (owned by another part of
this build, not yet present at the time this service was written), which
wires up its own MongoDB container (`mongo-expense`, host port 27018 per
`architecture/api-contracts.md`) and the gateway that forwards `X-User-Id`.

## Building the Docker image

The build context must be the **repo root**, because the image needs both
`shared/` and `services/expense-service/`:

```sh
docker build -f services/expense-service/Dockerfile -t finora/expense-service:local .
```

or, equivalently, from this directory: `make docker-build`.

## Development

```sh
make build   # go build ./cmd/server
make test    # go test ./...            (fast, fakes only)
make tidy    # go mod tidy
go test -tags=integration ./...  # real-Mongo integration tests (Phase 6,
                                  # requires Docker — see docs/local-
                                  # development.md's Testing section, or
                                  # `make test-integration` from the repo root)
```

### Architecture

```
cmd/server/main.go     - wires config -> mongo -> repositories -> services -> handlers -> router -> health -> server.Run
internal/config/       - env loading (wraps shared/config)
internal/domain/       - plain structs + repository/service interfaces (no framework imports)
internal/repository/   - MongoDB implementations of domain interfaces
internal/service/      - business logic; depends only on domain interfaces (unit-testable with fakes)
internal/handler/      - Gin handlers: bind/validate, call service, respond via shared/httpx
internal/router/       - gin.Engine + middleware wiring
```

Middleware order: `RequestID -> Logging -> Recovery -> (public health routes) -> /api/v1 group gated by RequireIdentity`. No CORS middleware here — CORS is applied only by the gateway; see `architecture/api-contracts.md`.
