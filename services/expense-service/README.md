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
| GET    | `/api/v1/transactions`    | query: `account_id, category, from, to, page, page_size`     | `200 {transactions: [], page, page_size, total}` |
| POST   | `/api/v1/transactions`    | `{account_id, category_id?, type, amount, currency, date, note?}` | `201 {transaction}`                     |
| GET    | `/api/v1/transactions/:id`| -                                                            | `200 {transaction}`                         |
| PUT    | `/api/v1/transactions/:id`| `{account_id, category_id?, type, amount, currency, date, note?}` | `200 {transaction}`                    |
| DELETE | `/api/v1/transactions/:id`| -                                                            | `204`                                        |
| POST   | `/api/v1/transactions/import` | `multipart/form-data: account_id, file` (CSV)           | `200 {imported, skipped, errors: [{row, message}]}` |
| GET    | `/api/v1/categories`      | -                                                            | `200 {categories: []}`                       |
| POST   | `/api/v1/categories`      | `{name, type}`                                               | `201 {category}`                            |

Categories are intentionally minimal (create + list only — no update/delete
yet).

Account `type` is one of: `checking`, `savings`, `credit`, `cash`.
Transaction/category `type` is one of: `income`, `expense`.

`GET /transactions` pagination: `page` defaults to 1, `page_size` defaults to
20 and is capped at 100 — the standard contract every paginated Finora list
endpoint follows, see `architecture/api-contracts.md`'s Pagination section.
The response's `page`/`page_size` are the values actually resolved and
applied server-side, not a raw echo of the query string (fixed in Phase 6 —
previously `page` echoed back `0` whenever `?page=` was omitted, and
`page_size` wasn't returned at all). Optional filters: `account_id`,
`category`, and a `from`/`to` date range applied to the transaction's `date`
field (accepts RFC3339 or `YYYY-MM-DD`).

### CSV import

`POST /transactions/import` bulk-creates transactions from an uploaded CSV
(e.g. a downloaded credit card statement export), all attached to the
`account_id` form field and inheriting that account's `currency` — the CSV
itself carries no currency column. Header matching is case-insensitive
against a small alias list, since real exports don't agree on column names:

| Field | Recognized headers |
|---|---|
| Date (required) | `date`, `transaction date`, `posted date` |
| Description (optional) | `description`, `merchant`, `payee`, `note` |
| Type (optional) | `type` — `income` or `expense`; if absent, inferred from whichever amount shape is present (see below) |

The amount itself is carried one of two shapes — **exactly one must be present** (Amount wins if a file somehow has both):

- **`amount`** — a single signed column; sign gives the direction (negative = expense, positive = income).
- **`debit` and/or `credit`** — the standard bank-statement shape, two separate *unsigned* columns, only one populated per row (Debit for money out = expense, Credit for money in = income). This is what real exports actually use — e.g. Capital One's own CSV export has exactly this shape, with no signed Amount column at all. `debit` is **not** an alias for `amount`: a lone Debit column holds unsigned magnitudes, so treating it as a signed amount and sign-inferring the type would label every purchase as income — a real bug this app shipped with once, fixed by treating Debit/Credit as their own concept.

Amount/Debit/Credit all accept a leading `$`, thousands-separator commas, and
parenthesized negatives (`(45.00)`), on top of a plain signed number. Date
accepts RFC3339, `YYYY-MM-DD`, or `MM/DD/YYYY`.

A row that fails to parse (bad date, bad amount, unrecognized explicit
type) is **skipped, not fatal** — the rest of the file still imports. The
response always returns `200` with `{imported, skipped, errors}`; `errors`
is capped at 20 entries (`row` is 1-indexed against the file's own lines,
header included, so the first data row is row 2), but `skipped` always
reports the true total. At most 5000 data rows per request — see
`architecture/api-contracts.md` for the full contract, including why this
inherits the gateway's `MAX_REQUEST_BODY_BYTES` limit like every other
request.

## Async events (Phase 7)

After every successful transaction write (`POST /transactions`, `PUT /transactions/:id`'s underlying `Create` path, and each row of a CSV import), this service publishes a `finora.transaction.created` event to NATS JetStream — consumed by budget-service to recompute overspend status. See `architecture/api-contracts.md`'s Async Events section for the full payload/contract.

Publishing goes through a **Mongo-backed transactional outbox**, not a direct NATS publish: `transactionService.publishCreated` enqueues the event into this service's own `outbox_events` collection in the same call path as the transaction insert, and a background relay (`shared/outbox.Relay`, polling every `OUTBOX_RELAY_INTERVAL`) drains it to NATS with retry-on-failure. A publish failure (enqueue or relay) is logged and never fails the transaction write — the write already succeeded, and failing the caller would wrongly suggest a retry (which would create a duplicate transaction).

## Health

Every service exposes the same trio (implemented once in `shared/health`):

```
GET /live    -> 200 always, if the process is up
GET /ready   -> 200 only if MongoDB ping succeeds, else 503
GET /health  -> aggregate, richer payload
```

Also serves `GET /openapi.yaml` — this service's spec, live from disk (see
`architecture/api-contracts.md`'s OpenAPI section).

## Environment variables

| Variable                     | Purpose                                          | Example                                         |
|-------------------------------|---------------------------------------------------|--------------------------------------------------|
| `EXPENSE_SERVICE_PORT`        | Port to bind (`0.0.0.0:<port>`)                    | `8082`                                             |
| `EXPENSE_SERVICE_MONGO_URI`   | MongoDB connection string (required, no default)   | `mongodb://mongo-expense:27017/finora_expenses`   |
| `LOG_LEVEL`                   | `debug`, `info` (default), `warn`, `error`         | `info`                                             |
| `SHUTDOWN_TIMEOUT`            | Graceful shutdown drain window                     | `10s`                                              |
| `CORS_ALLOWED_ORIGINS`        | Accepted for config-load compatibility but **unused** — CORS is applied only by the gateway (see `architecture/api-contracts.md`); a backend applying it too duplicates the header via the reverse proxy | `http://localhost:3000` |
| `NATS_URL`                    | NATS JetStream connection string (Phase 7 — publishes `finora.transaction.created`) | `nats://nats:4222` |
| `OUTBOX_RELAY_INTERVAL`       | How often the outbox relay polls for unpublished events and retries publishing them | `2s` |

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
internal/events/       - domain.EventPublisher implementation (OutboxPublisher, wraps shared/outbox.Store) - Phase 7
internal/handler/      - Gin handlers: bind/validate, call service, respond via shared/httpx
internal/router/       - gin.Engine + middleware wiring
```

Middleware order: `RequestID -> Logging -> Recovery -> (public health routes) -> /api/v1 group gated by RequireIdentity`. No CORS middleware here — CORS is applied only by the gateway; see `architecture/api-contracts.md`.
