# budget-service

Owns budgets, savings goals, and reports for Finora — a personal-finance SaaS
whose real purpose is to double as a Kubernetes/DevOps learning platform.
This service is one microservice in that fleet, following the same clean
architecture and shared-package conventions as `user-service` and
`expense-service`.

## Responsibilities

- **Budgets** — full CRUD, real MongoDB persistence, owner-scoped.
- **Savings goals** — full CRUD as of Phase 3, including manual progress
  logging (`current_amount` via `PUT`). See below.
- **Reports** — `GET /api/v1/reports/summary` is a real, computed
  budget-vs-actual report as of Phase 3, calling **expense-service** over
  REST for actual spend. See below.

## Endpoints

All routes are prefixed `/api/v1` and require the `X-User-Id` header (set by
the gateway after JWT verification — this service trusts it and never
validates JWTs itself). Every query is scoped to the caller's `user_id`;
accessing another user's resource returns `404 NOT_FOUND`, never `403`.

| Method | Path                      | Body                                                        | Response             |
|--------|---------------------------|--------------------------------------------------------------|-----------------------|
| GET    | `/api/v1/budgets`          | —                                                            | `200 {budgets: []}`   |
| POST   | `/api/v1/budgets`          | `{category, amount, period}`                                 | `201 {budget}`        |
| GET    | `/api/v1/budgets/:id`      | —                                                            | `200 {budget}`        |
| PUT    | `/api/v1/budgets/:id`      | `{category, amount, period}`                                 | `200 {budget}`        |
| DELETE | `/api/v1/budgets/:id`      | —                                                            | `204`                 |
| GET    | `/api/v1/goals`            | —                                                            | `200 {goals: []}`     |
| POST   | `/api/v1/goals`            | `{name, target_amount, target_date}`                          | `201 {goal}`          |
| GET    | `/api/v1/goals/:id`        | —                                                            | `200 {goal}`          |
| PUT    | `/api/v1/goals/:id`        | `{name, target_amount, target_date, current_amount}`          | `200 {goal}`          |
| DELETE | `/api/v1/goals/:id`        | —                                                            | `204`                 |
| GET    | `/api/v1/reports/summary`  | query: `?from&to` (both **required**)                       | `200 {summary}`       |

`period` is one of `weekly` / `monthly` / `yearly`. `target_date` accepts
either an RFC3339 timestamp or a `YYYY-MM-DD` date, and must be in the
future. `current_amount` on goal update is manual progress logging — a value
of exactly `0` is valid (e.g. resetting progress), not treated as "omitted".

Also exposes the standard health trio (`GET /live`, `/ready`, `/health`) via
`shared/health`, with MongoDB wired in as the readiness check, and
`GET /openapi.yaml` — this service's spec, served live from disk.

See `openapi.yaml` for the full OpenAPI 3.0 spec, and
`architecture/api-contracts.md` (repo root) for the cross-service contract
this implements, including the response envelope shape and error codes.

### Reports — real cross-service computation

`GET /api/v1/reports/summary?from=<>&to=<>` (both query params **required**
— 400 `VALIDATION_ERROR` if missing or unparseable) computes one
`CategorySummary` per the caller's budgets in the requested range, by
calling `expense-service` over REST:

1. `GET /api/v1/categories` on expense-service (with the caller's `X-User-Id`
   forwarded) to resolve the budget's `category` name to expense-service's
   `category_id`, matched case-insensitively. If nothing matches, `actual`
   is `0` — **not an error**, the user just hasn't logged anything under
   that name yet.
2. `GET /api/v1/transactions?category=<id>&from=<>&to=<>&page_size=100` on
   expense-service, paginated (bounded at 50 pages / 5,000 transactions per
   category as a safety cap), summing `amount` where `type == "expense"`.
   This filtering happens client-side because expense-service's transaction
   list has no server-side `type` filter today.

Example response:

```json
{
  "success": true,
  "data": {
    "summary": {
      "from": "2026-01-01T00:00:00Z",
      "to": "2026-01-31T00:00:00Z",
      "categories": [
        { "category": "groceries", "period": "monthly", "budgeted": 500, "actual": 320, "remaining": 180 }
      ],
      "total_budgeted": 500,
      "total_actual": 320
    }
  },
  "error": null,
  "request_id": "..."
}
```

This was the only service-to-service REST call in the fleet through Phase 3
(see `architecture/api-contracts.md` and `CLAUDE.md` §3). Implemented behind
`domain.ExpenseClient` (`internal/client/expense_client.go`), so
`internal/service/report_service.go` depends only on the interface, never
the concrete HTTP client — unit-tested with a fake, no live HTTP calls in
`go test`.

### Reports is now a pure read (Phase 4's REST overspend trigger was removed in Phase 7)

Through Phase 4–6, `Summary()` also triggered a real in-app notification as
a side effect of this `GET` request, over REST, whenever it found a budget
over spent. **That's gone as of Phase 7** — `domain.NotificationClient` and
`internal/client/notification_client.go` are deleted, `NewReportService` is
back to a 2-arg constructor (`budgetRepo, expenseClient`), and `Summary()`
has no side effects. The overspend check moved to an event-driven path —
see the next section.

### Async events (Phase 7) — event-driven overspend detection

This service both **consumes** and **publishes** domain events over NATS
JetStream (see `architecture/api-contracts.md`'s Async Events section for
the full contract):

- **Consumes** `finora.transaction.created` (published by expense-service
  after every transaction write) via a durable consumer
  (`budget-service-transaction-created`). On receipt,
  `internal/service/overspend_service.go`'s `OverspendService.HandleTransactionCreated`
  lists the event's user's budgets and, for each, recomputes the **current
  period's** actual spend via the existing `expenseClient.SumExpensesByCategory`
  REST call (the same synchronous query `Summary()` uses — kept as REST,
  since it's a genuine query, not a notification). A per-budget
  expense-service error is logged and skipped, not fatal to checking the
  user's other budgets.
- **Publishes** `finora.budget.overspent` (via `internal/events.OutboxPublisher`,
  the same Mongo-backed outbox pattern expense-service uses) when a budget's
  recomputed actual exceeds its amount, consumed in turn by
  notification-service.

**Dedup rule (unchanged behavior, new trigger):** `domain.Budget` still
carries `LastNotifiedAt *time.Time` (`json:"-"`), storing the **period's
`from` boundary** last notified for, not a real timestamp. A budget only
publishes if `LastNotifiedAt == nil` or it doesn't exactly match the
current period's `from`; after a successful publish, `LastNotifiedAt` is
persisted via the existing `BudgetRepository.Update`. This is the identical
rule Phase 4 established for the REST trigger — see
`architecture/development-roadmap.md`'s Phase 4 entry for why exact-period-
match was chosen over a time-ordering comparison — just evaluated once per
`transaction.created` event instead of once per report read. A publish
failure (enqueue or the outbox relay) is logged and swallowed, never
propagated — the overspend check itself already completed correctly.

### Savings goals — full CRUD

`current_amount` is manual user progress logging, updated via
`PUT /api/v1/goals/:id` alongside the other fields (full-replace PUT,
matching budgets' Update convention). It is **not** derived from
expense-service — goals don't get the cross-service linkage reports get,
per the roadmap's explicit distinction between "savings goals" and reports'
cross-service aggregation.

## Environment variables

| Variable                     | Meaning                                          | Default                 |
|-------------------------------|---------------------------------------------------|--------------------------|
| `BUDGET_SERVICE_PORT`         | Port to bind (`0.0.0.0:<port>`)                    | `8083`                  |
| `BUDGET_SERVICE_MONGO_URI`    | MongoDB connection string (required, no default)   | —                       |
| `LOG_LEVEL`                   | `debug` / `info` / `warn` / `error`                | `info`                  |
| `SHUTDOWN_TIMEOUT`            | Graceful-shutdown drain duration (Go duration)     | `10s`                   |
| `CORS_ALLOWED_ORIGINS`        | Accepted for config-load compatibility but **unused** — CORS is applied only by the gateway (see `architecture/api-contracts.md`); a backend applying it too duplicates the header via the reverse proxy | `http://localhost:3000` |
| `EXPENSE_SERVICE_URL`         | Base URL for the outbound REST call to expense-service that powers `/api/v1/reports/summary` and the event-driven overspend check (see above). Same docker-compose network address the gateway uses. | `http://expense-service:8082` |
| `NATS_URL`                    | NATS JetStream connection string (Phase 7 — consumes `finora.transaction.created`, publishes `finora.budget.overspent`) | `nats://nats:4222` |
| `OUTBOX_RELAY_INTERVAL`       | How often the outbox relay polls for unpublished `finora.budget.overspent` events and retries publishing them | `2s` |

Exact names match `.env.example` at the repo root. No JWT secrets are read —
this service only trusts the `X-User-Id` header forwarded by the gateway.

## Running standalone

Requires Go 1.21.3 and a reachable MongoDB instance.

```sh
cd services/budget-service
export BUDGET_SERVICE_MONGO_URI="mongodb://localhost:27019/finora_budgets"
make run
```

Other targets:

```sh
make build         # go build ./...
make test          # go test ./...            (fast, fakes only)
make tidy          # go mod tidy
make docker-build  # docker build from the repo root (see below)
go test -tags=integration ./...  # real-Mongo integration tests (Phase 6,
                                  # requires Docker — see docs/local-
                                  # development.md's Testing section, or
                                  # `make test-integration` from the repo root)
```

### Normal usage: docker-compose

In normal local development this service is started via the root
`docker-compose.yml` alongside its own dedicated Mongo instance
(`mongo-budget`, host port `27019`, per `architecture/api-contracts.md`).
Running it standalone as above is mainly
useful for fast iteration on this service alone.

## Docker build

The `Dockerfile` is multi-stage and its build context must be the **repo
root**, not this directory, because the builder stage needs both `shared/`
and `services/budget-service/`:

```sh
# from the repository root
docker build -f services/budget-service/Dockerfile -t budget-service:local .
```

(`make docker-build` does this `cd` for you.)

## Architecture

```
cmd/server/main.go     - wires config -> mongo -> EnsureIndexes -> repositories -> services -> handlers -> router -> health -> server.Run
internal/config/       - env loading, wraps shared/config
internal/domain/       - plain structs + repository/service interfaces (no gin/mongo-driver imports)
internal/repository/   - MongoDB implementations of domain interfaces, plus EnsureIndexes
internal/service/      - business logic; depends on domain interfaces only, so it's unit-testable with fakes
internal/client/       - outbound REST adapter used by report_service and overspend_service: expense_client.go (domain.ExpenseClient, actual spend)
internal/events/       - domain.EventPublisher implementation (OutboxPublisher, wraps shared/outbox.Store) - Phase 7
internal/handler/      - Gin handlers: bind/validate, call service, respond via httpx.Success/Fail
internal/router/       - gin.Engine assembly and middleware order
```

Middleware order: `RequestID -> Logging -> Recovery`, then public
health routes, then the `/api/v1` group with `RequireIdentity` applied to
the whole group (every route in this service is owner-scoped). No CORS
middleware here — CORS is applied only by the gateway; see
`architecture/api-contracts.md`.

## Testing

`internal/service` is unit-tested with table-driven tests against
hand-written fakes implementing the domain repository interfaces (including
a `fakeExpenseClient` for `domain.ExpenseClient` and a `fakeEventPublisher`
for `domain.EventPublisher`) — no live Mongo, no live HTTP, no live NATS.
The event-driven overspend check is covered by
`overspend_service_test.go`'s `TestOverspendService_HandleTransactionCreated`
(6 subtests: a newly-over-budget category publishes exactly once, a repeat
event for the same period doesn't re-publish (dedup), a later period
re-publishes if still over budget, an under-budget category never
publishes, a per-budget expense-client error doesn't block checking the
user's other budgets, and a budget-repo `List` error propagates) plus
`TestCurrentPeriodStart` (monthly/yearly/weekly period-boundary logic).
`internal/handler` is tested with `httptest` + `gin.TestMode` against fake
services, covering budget CRUD, goal CRUD (including the cross-user-is-404
rule for both), and the reports endpoint's required from/to validation plus
its computed response shape. Integration testing against real
MongoDB/NATS happens at the `shared/eventbus`/`shared/outbox` layer
(`//go:build integration`, see `architecture/api-contracts.md`'s Async
Events section), not here.

```sh
go test ./...
```
