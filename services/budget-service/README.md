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
`shared/health`, with MongoDB wired in as the readiness check.

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

### Reports — Phase 4 overspend-notification trigger

`Summary()` also triggers a real in-app notification the first time it
finds a budget over spent for the requested `[from, to]` range: if a
category's `remaining < 0`, budget-service calls **notification-service**
over REST (`POST /api/v1/notifications`, forwarding the caller's
`X-User-Id`), behind a new `domain.NotificationClient`
(`internal/client/notification_client.go`) — same Dependency Inversion
pattern as `ExpenseClient`.

**Dedup**, so a dashboard/report reload doesn't spam the user: `Budget`
carries an internal, non-API-visible `LastNotifiedAt *time.Time`
(`json:"-"`) that — despite the name — stores the **period's `from`
boundary** last notified for, not a real timestamp. A notification only
fires if `LastNotifiedAt == nil` or it doesn't exactly match the requested
`from`; after a successful notify, `LastNotifiedAt` is persisted as `from`
itself via the existing `BudgetRepository.Update`. Reloading the same range
doesn't re-notify; any *other* period (earlier or later) does, if it's also
over budget — an earlier design compared `LastNotifiedAt` as a time
ordering against `from` using the real notify timestamp, which wrongly
suppressed a legitimate notification for an older period viewed after a
more recent one; fixed to exact-period-match before shipping.

This makes `GET /api/v1/reports/summary` — a read endpoint — have a side
effect, which is **not** pure REST semantics. That's a deliberate,
documented trade-off, accepted only because Finora has no event bus yet
(Phase 7 introduces one and replaces this with a real overspend event) — see
`architecture/api-contracts.md`'s budget-service → notification-service
subsection and `architecture/development-roadmap.md`'s Phase 4 entry for the
full rationale. A `Notify` failure is logged and swallowed, never
propagated — it never turns a working report read into a `500`.

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
| `EXPENSE_SERVICE_URL`         | Base URL for the outbound REST call to expense-service that powers `/api/v1/reports/summary` (see above). Same docker-compose network address the gateway uses. | `http://expense-service:8082` |
| `NOTIFICATION_SERVICE_URL`    | Base URL for the outbound REST call to notification-service that powers the Phase 4 overspend-notification trigger (see above). Same docker-compose network address the gateway uses. | `http://notification-service:8084` |

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
make test          # go test ./...
make tidy          # go mod tidy
make docker-build  # docker build from the repo root (see below)
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
internal/client/       - outbound REST adapters used only by report_service: expense_client.go (domain.ExpenseClient, actual spend) and notification_client.go (domain.NotificationClient, Phase 4 overspend trigger)
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
a `fakeExpenseClient` for `domain.ExpenseClient` and a
`fakeNotificationClient` for `domain.NotificationClient`) — no live Mongo,
no live HTTP. The overspend-notification trigger is covered by
`TestReportService_Summary_OverspendNotification`: a newly-over-budget
category notifies exactly once, a repeat `Summary()` call for the same
period doesn't re-notify (dedup), a later period re-notifies if still over
budget, and an under-budget category never notifies. `internal/handler` is
tested with `httptest` + `gin.TestMode` against fake services, covering
budget CRUD, goal CRUD (including the cross-user-is-404 rule for both), and
the reports endpoint's required from/to validation plus its computed
response shape. Integration testing against real MongoDB/expense-service/
notification-service happens later (Phase 6), not here.

```sh
go test ./...
```
