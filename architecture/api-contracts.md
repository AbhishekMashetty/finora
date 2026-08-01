# API Contracts

This document is the ground truth for every service's HTTP surface. All services must conform to it.

## Ports (local / docker-compose)

| Service              | Container port | Host port | Mongo host port |
|----------------------|-----------------|-----------|------------------|
| frontend              | 3000            | 3000      | -                |
| gateway               | 8080            | 8080      | -                |
| user-service          | 8081            | 8081      | 27017 (mongo-user) |
| expense-service       | 8082            | 8082      | 27018 (mongo-expense) |
| budget-service        | 8083            | 8083      | 27019 (mongo-budget) |
| notification-service  | 8084            | 8084      | 27020 (mongo-notification) |

Only the **gateway** and **frontend** ports are meant to be called by clients. Service ports are exposed to the host for local debugging only; in Kubernetes they are ClusterIP-only.

## Versioning

All routes are prefixed `/api/v1`.

## Pagination (standard contract, Phase 6)

Not every list endpoint paginates — deliberately. `accounts`, `categories`, `budgets`, and `goals` return everything unpaginated, because a real user has a handful of each (dozens at most), not pages of them; adding pagination there would be exactly the kind of unjustified complexity `CLAUDE.md` §10/`plan.md` warn against. **`transactions` and `notifications` do paginate**, because both are genuinely unbounded per user over time — every logged expense, every alert, forever.

Wherever pagination exists, it follows one shape, so a client never has to special-case which service it's talking to:

- **Query params:** `page` (1-indexed, default `1`), `page_size` (default `20`, capped at `100` server-side regardless of what's requested).
- **Response fields**, alongside the resource array (`transactions`, `notifications`, etc.): flat sibling fields `page`, `page_size`, `total` — never nested under a `pagination` object, so existing consumers reading `data.page` as a plain number never break if a future endpoint adopts this contract.
- **The `page`/`page_size` echoed back are the *resolved* values the service actually applied** (after defaulting/capping) — never a raw, possibly-zero echo of what the caller passed in `?page=`. This was a real bug in `transactions`' original Phase 2 implementation (the handler echoed the unvalidated query param directly, so omitting `?page=` echoed back `0`, and `page_size` wasn't returned at all) — caught and fixed project-wide in Phase 6 while establishing this as the documented standard, not just for `transactions` alone.
- `total` is the full matching count across all pages, not the current page's length — required for a client to compute total pages (`Math.ceil(total / page_size)`).

If a currently-unpaginated collection ever needs pagination later (e.g. `accounts` for a power user with dozens of them), it must adopt this exact contract, not invent a new one.

## Response Envelope

Every JSON response (success or error) uses this shape:

```json
{
  "success": true,
  "data": { "...": "..." },
  "error": null,
  "request_id": "8f3e2c1a-..."
}
```

Error response:

```json
{
  "success": false,
  "data": null,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "email is required",
    "details": { "field": "email" }
  },
  "request_id": "8f3e2c1a-..."
}
```

Implemented once in `shared/httpx` (`httpx.Success(c, status, data)`, `httpx.Fail(c, status, code, message, details)`). Services must not hand-roll this shape.

### Standard error codes

| Code                | HTTP Status | Meaning |
|---------------------|-------------|---------|
| `VALIDATION_ERROR`   | 400         | Request failed validation |
| `UNAUTHORIZED`       | 401         | Missing/invalid/expired token |
| `FORBIDDEN`          | 403         | Authenticated but not allowed |
| `NOT_FOUND`          | 404         | Resource does not exist / not owned by caller |
| `CONFLICT`           | 409         | Duplicate resource (e.g. email already registered) |
| `INTERNAL_ERROR`     | 500         | Unhandled server error |
| `RATE_LIMITED`       | 429         | Too many requests from this client (gateway only, see Rate Limiting below) |

## Auth Header Contract (Gateway → Services)

The gateway is the only component that validates JWTs. On every request forwarded to a downstream service, the gateway sets:

- `X-User-Id` — the authenticated user's ID (from the access token's `sub` claim)
- `X-Request-Id` — propagated or generated request id

Downstream services **trust these headers** on the internal docker/K8s network (no direct external access to service ports in production). Services must reject requests missing `X-User-Id` on any route that operates on user-owned data, using shared middleware (`shared/middleware.RequireIdentity`).

*Trade-off:* this is simpler than every service independently validating JWTs/JWKS, at the cost of trusting network boundaries. Documented as a Phase-9+ hardening candidate (per-service JWKS validation) once the services sit behind a real service mesh / NetworkPolicies.

## CORS (Gateway Only)

The gateway is also the only component that applies CORS middleware (`shared/middleware.CORS`). Backend services never do. This isn't just a style choice: `net/http/httputil.ReverseProxy` copies the backend's response headers onto the gateway's outgoing response with `Header().Add`, not `Set`. If a backend also set `Access-Control-Allow-Origin`, the browser would receive it twice as one comma-joined value — which every browser rejects as invalid, even though the underlying request succeeded end-to-end. This exact bug shipped during Phase 0 and was only caught by real browser-based verification (`curl` never enforces CORS, so it looked fine at the API-testing stage). Backend routers apply `RequestID → Logging → Recovery` only.

## Rate Limiting (Gateway Only, Phase 6)

Same reasoning as CORS above: the gateway is the single public entrypoint, so it's the one place a cross-cutting request-shaping concern belongs. Backend services never apply their own rate limiting — they trust the internal network already, and duplicating it per-service would be redundant defense CLAUDE.md's "avoid unnecessary complexity" already argues against.

Implemented as `shared/middleware.RateLimit(requestsPerSecond, burst)`, a per-client-IP token bucket (`golang.org/x/time/rate` — the standard Go rate-limiting primitive, not a third-party dependency), wired into the gateway's middleware chain after CORS. Configured via `RATE_LIMIT_REQUESTS_PER_SECOND` / `RATE_LIMIT_BURST` (defaults 10/s, burst 20 — generous enough for the frontend's legitimate parallel fetches, e.g. the dashboard overview firing 5 concurrent requests on load). A client that exceeds the limit gets `429 RATE_LIMITED` via the standard envelope.

Deliberately **in-memory, not Redis-backed**: Redis is explicitly on `plan.md`'s "Future Evolution — don't introduce before there's a legitimate reason" list, and today's deployment is a single gateway instance (Kubernetes HPA / multi-replica scaling is Phase 9, not built yet). Per-instance limits are exactly right for one instance; they'd under-enforce across multiple replicas (each with its own independent bucket), which is the legitimate reason to introduce Redis for this — not before.

**Fails open on misconfiguration, not closed**: `requestsPerSecond <= 0` or `burst <= 0` disables rate limiting entirely rather than the literal token-bucket reading of "zero capacity" (which would silently reject 100% of traffic). Caught live during implementation — the gateway's pre-existing `router_test.go`, which never set these fields, started failing every single request with 429 the moment this middleware was wired in, before the fail-open guard was added. A rate limiter that isn't configured must never look like a full outage.

## Request Body Size Limit (Gateway Only, Phase 6)

Same "gateway is the one place" reasoning as CORS and rate limiting above. Implemented as `shared/middleware.BodyLimit(maxBytes)`, wired into the gateway's middleware chain right after `Recovery` — before CORS, rate limiting, or proxying do any work, so an oversized body is capped as early as possible. Configured via `MAX_REQUEST_BODY_BYTES` (default `1048576`, 1 MiB — every JSON body this app sends is small). Also fails open (`maxBytes <= 0` disables the limit), for the identical reason `RateLimit` does.

A client exceeding the limit gets `400 VALIDATION_ERROR`, not a raw connection error or a `502`. This required a real fix, not just the middleware itself: `BodyLimit` wraps the request body in an `http.MaxBytesReader`, but the gateway forwards every request through `httputil.ReverseProxy` (`services/gateway/internal/proxy/proxy.go`) rather than ever calling `ShouldBindJSON` itself — so the "body too large" read error surfaces mid-proxy as an `*http.MaxBytesError`, which a naive `ErrorHandler` can't distinguish from a genuine backend-unreachable failure. `proxy.go`'s `ErrorHandler` special-cases `*http.MaxBytesError` via `errors.As` to report `400 VALIDATION_ERROR`/"request body too large" instead of falling through to the generic `502 INTERNAL_ERROR`/"upstream service unavailable" branch. This was caught by an independent closing review (a first draft's doc comment claimed the existing bind-error handling covered this case; it didn't, because the gateway never binds — verified live via a real oversized request through the gateway, which returned 502 before the fix and 400 after).

## JWT Contract (shared/jwtx)

- **Access token:** `type=access`, claims `{sub, email, type, iat, exp}`, TTL 15m, signed HS256 with `JWT_ACCESS_SECRET`.
- **Refresh token:** `type=refresh`, claims `{sub, jti, type, iat, exp}`, TTL 7d, signed HS256 with `JWT_REFRESH_SECRET`. The `jti` is persisted (hashed) in user-service's Mongo so it can be revoked/rotated on refresh and logout.
- Only **user-service** issues tokens. Only **gateway** verifies access tokens for routing. `shared/jwtx` provides both `Generate*` and `Parse*` so the two services can't drift on claim shape.

## Public vs Protected Routes (enforced at the gateway)

**Public** (no auth required): `POST /api/v1/auth/register`, `POST /api/v1/auth/login`, `POST /api/v1/auth/refresh`, `POST /api/v1/auth/password-reset`.

**Protected** (Bearer access token required): everything else.

## Per-Service Endpoints

### user-service (owns: users, credentials, refresh tokens)
```
POST   /api/v1/auth/register        {email, password, name}            -> 201 {user}
POST   /api/v1/auth/login           {email, password}                  -> 200 {access_token, refresh_token, user}
POST   /api/v1/auth/refresh         {refresh_token}                    -> 200 {access_token, refresh_token}
POST   /api/v1/auth/logout          {refresh_token}                    -> 204
POST   /api/v1/auth/password-reset  {email}                            -> 202 {message}   (Phase 1 stub — always 202, never leaks whether the email is registered; no token issued/email sent yet)
GET    /api/v1/users/me                                                -> 200 {user}
PUT    /api/v1/users/me             {name, ...}                        -> 200 {user}
GET    /api/v1/users/me/settings                                       -> 200 {settings}
PUT    /api/v1/users/me/settings    {currency, timezone, ...}          -> 200 {settings}
```

### expense-service (owns: accounts, transactions, categories)
```
GET    /api/v1/accounts                                                -> 200 {accounts: []}
POST   /api/v1/accounts             {name, type, currency}             -> 201 {account}
GET    /api/v1/accounts/:id                                            -> 200 {account}
PUT    /api/v1/accounts/:id                                            -> 200 {account}
DELETE /api/v1/accounts/:id                                            -> 204

GET    /api/v1/transactions         ?account_id&category&from&to&page&page_size -> 200 {transactions: [], page, page_size, total}
POST   /api/v1/transactions         {account_id, category, amount, type, date, note} -> 201 {transaction}
GET    /api/v1/transactions/:id                                        -> 200 {transaction}
PUT    /api/v1/transactions/:id                                        -> 200 {transaction}
DELETE /api/v1/transactions/:id                                        -> 204
POST   /api/v1/transactions/import  multipart: account_id, file (CSV)  -> 200 {imported, skipped, errors}

GET    /api/v1/categories                                              -> 200 {categories: []}  (Phase 2)
POST   /api/v1/categories           {name, type}                       -> 201 {category}          (Phase 2)
```
All resources are owner-scoped by `X-User-Id`; cross-user access returns `404 NOT_FOUND` (never `403`, to avoid confirming existence).

### CSV transaction import

`POST /api/v1/transactions/import` is the one endpoint in this fleet that takes `multipart/form-data` instead of JSON — a file upload has no natural JSON representation, and re-encoding a CSV as a JSON string field would just move the real parsing problem into the request body instead of solving it. Every imported row is attached to the `account_id` form field and inherits that account's `currency`; the CSV itself has no currency column, since a single statement is always in one currency.

Column matching is case-insensitive against a small alias list (`date`/`transaction date`/`posted date`; `description`/`merchant`/`payee`/`note`; `type`), because real bank/card exports don't agree on header names and this app isn't trying to auto-detect every issuer's exact format — that's the explicit scope line: a documented, small alias list, not a general bank-format-sniffing engine.

**The amount is carried one of two shapes, resolved in the service layer, not aliased into one column at parse time:** a single signed `amount` column (sign gives the direction — negative = expense, positive = income), or separate unsigned `debit`/`credit` columns (the standard bank-statement shape: exactly one populated per row, Debit = money out = expense, Credit = money in = income). `amount` wins if a row somehow has both. If a row's `type` column is absent, income/expense is inferred from whichever shape is present. **`debit` is deliberately not an alias of `amount`** — it was, in the first version of this feature, and it was a real, user-reported bug: Capital One's actual CSV export has separate Debit/Credit columns holding *unsigned* magnitudes, no signed Amount column at all, so treating a lone "Debit" header as a signed amount and sign-inferring the type meant every purchase (a positive Debit value) was mislabeled as income. Fixed by giving `domain.ImportRow` its own `Debit`/`Credit` fields and a dedicated `resolveImportAmount` step, with a regression test built directly from the real file's shape (a Debit-only purchase row, a Credit-only payment row) so this can't silently regress.

**A bad row is skipped, never fatal to the rest of the file** — the response is always `200` (not `207` or a partial-failure status code; this repo's envelope has no multi-status convention, and inventing one for a single endpoint wasn't worth it) with `{imported, skipped, errors}`, where `errors` is capped at 20 entries but `skipped` always reports the true count. This is a deliberate divergence from every other write endpoint in the fleet, which either fully succeeds or fully fails — a CSV import treats "some rows in a large statement are garbage" as the expected case, not an error condition, since a user importing 200 real transactions shouldn't have the whole import rejected over 2 malformed rows.

Capped at 5000 data rows per request (a defense-in-depth ceiling, not a real-world limit — no bulk-insert path exists, each row is its own `Create`-equivalent call) and subject to the gateway's `MAX_REQUEST_BODY_BYTES` limit like every other request (see the Request Body Size Limit section above) — no special-casing for this endpoint's larger-than-JSON payloads, since a real statement CSV is well under the 1 MiB default.

### budget-service (owns: budgets, savings goals, reports)
```
GET    /api/v1/budgets                                                 -> 200 {budgets: []}
POST   /api/v1/budgets              {category, amount, period}         -> 201 {budget}
GET    /api/v1/budgets/:id                                             -> 200 {budget}
PUT    /api/v1/budgets/:id                                             -> 200 {budget}
DELETE /api/v1/budgets/:id                                              -> 204

GET    /api/v1/goals                                                   -> 200 {goals: []}
POST   /api/v1/goals                {name, target_amount, target_date} -> 201 {goal}
GET    /api/v1/goals/:id                                               -> 200 {goal}
PUT    /api/v1/goals/:id            {name, target_amount, target_date, current_amount} -> 200 {goal}
DELETE /api/v1/goals/:id                                               -> 204

GET    /api/v1/reports/summary      ?from&to (both required)           -> 200 {summary}
```
`current_amount` on `PUT /api/v1/goals/:id` is manual user progress logging (e.g. "I moved $500 into savings this month") — goals do **not** get the cross-service expense-linkage treatment reports get; see the subsection below for what does.

`GET /api/v1/reports/summary` returns a real, computed budget-vs-actual report — one entry per the caller's budgets in the `[from, to]` range:
```json
{
  "summary": {
    "from": "2026-01-01T00:00:00Z",
    "to": "2026-01-31T00:00:00Z",
    "categories": [
      { "category": "groceries", "period": "monthly", "budgeted": 500, "actual": 320, "remaining": 180 }
    ],
    "total_budgeted": 500,
    "total_actual": 320
  }
}
```
`from`/`to` are **required** query params (RFC3339 or `YYYY-MM-DD`); missing or unparseable values are a `400 VALIDATION_ERROR` — there is no implicit default range.

#### budget-service → expense-service (the only service-to-service call in the fleet today)

To compute `actual` for a budget's category, budget-service calls **expense-service** directly over REST, forwarding the caller's `X-User-Id` header exactly as the gateway would (no JWT — this is an internal-network call, same trust model as `shared/middleware.RequireIdentity`):

1. `GET /api/v1/categories` on expense-service, to resolve the budget's `Category` (a plain name, e.g. `"groceries"`) to expense-service's `category_id` — matched case-insensitively against the returned categories' `name`. If no category name matches, `actual` is `0`, **not an error** — the user simply hasn't logged anything under that name yet.
2. `GET /api/v1/transactions?category=<id>&from=<>&to=<>&page_size=100` on expense-service, paginated (capped at 50 pages = 5,000 transactions per category, per request, as a safety bound), summing `amount` for transactions where `type == "expense"`. This filtering happens **client-side** in budget-service because expense-service's `ListTransactionsInput`/`TransactionFilter` has no server-side `type` filter today (`?type=expense` is not honored by expense-service, only `?category=` is — a known pre-existing wrinkle in expense-service's query-param naming, not fixed here, only worked around from the caller's side).

Implemented in `services/budget-service/internal/client/expense_client.go` behind `domain.ExpenseClient`, so `internal/service/report_service.go` depends only on that interface (Dependency Inversion), never a concrete HTTP client — this keeps report_service unit-testable with a fake, per `CLAUDE.md` §7.

#### budget-service → notification-service — SUPERSEDED by Phase 7's async event (see Async Events, below)

Through Phase 4–6, `GET /api/v1/reports/summary`'s `Summary()` triggered a real in-app notification synchronously, over REST, as a side effect of a `GET` request — a documented, deliberate trade-off accepted only because Finora had no event bus yet. **Phase 7 removes this entirely.** `Summary()` is a pure read again (`domain.NotificationClient` and `internal/client/notification_client.go` are deleted; `NewReportService` is now 2-arg: `budgetRepo, expenseClient`). The overspend check and notification are now event-driven — see the Async Events section below for the replacement, including the equivalent dedup rule (now keyed the same way, `LastNotifiedAt`-exact-period-match, just triggered by a consumed event instead of a report read).

#### budget-service → expense-service remains synchronous REST (unchanged by Phase 7)

The cross-service call above (resolving a category name and summing expense transactions) is a genuine **query** — a client asking "what is the actual spend right now" — which is exactly the kind of interaction Phase 7 explicitly keeps as REST (`CLAUDE.md` §2: "REST for queries, async events for domain notifications"). It is reused as-is by the new event-driven overspend check (`OverspendService.HandleTransactionCreated`, see below) to recompute a budget's current-period actual whenever a `transaction.created` event arrives.

## Async Events (Phase 7)

Finora's one messaging exception to "REST only" (`CLAUDE.md` §2): **NATS JetStream**, chosen over core NATS pub/sub specifically for at-least-once delivery + message persistence + durable consumers — a subscriber that's down when an event is published still receives it once it comes back up, which core pub/sub cannot do. One shared JetStream-enabled NATS instance (`nats`, docker-compose) serves the whole fleet, on one stream:

| | |
|---|---|
| **Stream** | `FINORA_EVENTS` (subjects: `finora.>`, retention: limits policy, 30-day max age) |
| **Subjects** | `finora.transaction.created` (published by expense-service, consumed by budget-service), `finora.budget.overspent` (published by budget-service, consumed by notification-service) |

Each subject's stream/subject-name constants and payload structs are **independently defined as matching literal values** in each service's own `internal/domain/event.go` — not shared via a `shared/` Go package — because every service is an independently buildable Go module (`CLAUDE.md` §10) and a `shared` type would violate that. This is a cross-service string contract, verified by convention and by the integration/E2E tests below, not by the compiler. Keep the three services' `event.go` files in sync by hand whenever a payload shape changes.

### `finora.transaction.created`

Published by expense-service immediately after a transaction is persisted (`transactionService.Create`, and once per row in the CSV import path), best-effort — a publish failure is logged and never fails the transaction write itself, since the write already succeeded and failing the caller would wrongly suggest a retry (which would create a duplicate transaction).

```json
{
  "transaction_id": "...", "user_id": "...", "account_id": "...", "category_id": "...",
  "type": "expense", "amount": 75, "currency": "USD", "date": "2026-07-31T00:00:00Z"
}
```

Consumed by budget-service's durable consumer `budget-service-transaction-created`. On receipt, `OverspendService.HandleTransactionCreated(ctx, userID, now)` lists the user's budgets and, for each, recomputes the **current period's** actual spend via the existing synchronous `expenseClient.SumExpensesByCategory` REST call (see above) — it does not trust anything about the event payload beyond `user_id`, so a stale or partial event can't corrupt the check. Only the current period is evaluated (an event has no caller-supplied date range the way a report request does); a backdated transaction still correctly contributes to the current period's sum because `SumExpensesByCategory` sums by date range, not by "recently created." A per-budget expense-service error is logged and skipped, not fatal to checking the user's other budgets.

### `finora.budget.overspent`

Published by budget-service (`OverspendService.notifyIfOverspent`) when a budget's recomputed `actual` exceeds its `amount`, using the **same dedup rule** Phase 4–6 used for the REST version: `domain.Budget.LastNotifiedAt` stores the current period's `from` boundary, and a budget only publishes if that boundary hasn't already been notified for. Deduped again at the transport layer via JetStream's `Msg-Id` header (`jetstream.WithMsgID`, keyed on `budgetID + ":" + periodStart`) — if the outbox relay retries a publish (e.g. after a crash between enqueueing and the broker ack), the duplicate collapses into one delivered message within NATS's dedup window, rather than relying solely on the application-level check.

```json
{
  "budget_id": "...", "user_id": "...", "category": "groceries", "period": "monthly",
  "budgeted": 50, "actual": 75, "period_start": "2026-07-01T00:00:00Z"
}
```

Consumed by notification-service's durable consumer `notification-service-budget-overspent`. `OverspendConsumer.HandleBudgetOverspent` formats `title: "Budget exceeded"` / `message: "You've spent {actual} of your {budgeted} {period} {category} budget."` and calls the existing `notificationService.Create` — the same notification shape the old REST trigger produced, so the frontend Notifications page needed zero changes.

### The outbox pattern (`shared/outbox`), and why it is not a true atomic outbox

Both expense-service and budget-service publish through a **transactional outbox**: a write to a new `outbox_events` Mongo collection (own database, per `CLAUDE.md` §2's one-DB-per-service rule) in the same call path as the triggering write (the transaction insert; the budget-overspent detection), plus a separate background `shared/outbox.Relay` goroutine that polls for unpublished events (`OUTBOX_RELAY_INTERVAL`, default `2s`) and publishes them to NATS, retrying indefinitely on failure (an event stays unpublished, and gets retried, until a publish succeeds).

**This is deliberately not a true atomic outbox.** The textbook pattern writes the business row and the outbox row in one atomic transaction (impossible to have one without the other). MongoDB multi-document ACID transactions require a replica set; this project's Mongo containers run standalone (`docker-compose.yml` has no `--replSet` flag, per the local-footprint trade-off `architecture/database-design.md` already documents). So `Enqueue` is a second, non-atomic insert immediately after the primary write — a narrow race window where the primary write succeeds but the process crashes before the outbox insert commits, silently losing that one event. This is accepted, not engineered around, for the same reason budget-service's pre-Phase-7 `notifyIfOverspent` race was documented rather than eliminated: a real fix (Mongo replica sets fleet-wide, just for this) is a materially bigger infrastructure change than the problem it solves for a local/learning-scale deployment. If replica sets are ever adopted for another reason, revisit making this a true atomic transaction at the same time.

Idempotency on the delivery side is what actually keeps this reliable despite the above: JetStream's durable consumers (`AckExplicitPolicy`, `MaxDeliver: 10`) guarantee at-least-once delivery once an event *is* published, and `Msg-Id`-based dedup (see above) collapses producer-side publish retries into one delivered message — so the remaining risk is narrowed to "the process crashes in the few-millisecond window between the primary Mongo write committing and the outbox insert," not "any event can be silently duplicated or lost after that point."

### Testing

`shared/eventbus` and `shared/outbox` each carry a `//go:build integration` suite (`shared/natstest`/`shared/mongotest` spin up real, disposable `nats:2-alpine`/`mongo:7` containers via testcontainers-go) proving: publish/subscribe round-trips, that a durable consumer created *after* a publish still receives it (the whole point of JetStream over core pub/sub), `Msg-Id` dedup, and the outbox+relay+real-NATS path end-to-end. Each service's new business logic (`publishCreated`, `OverspendService`, `OverspendConsumer`) is unit-tested with fakes, no live infrastructure, per `CLAUDE.md` §7 — the event-publish call in `transactionService.Create`/`OverspendService.notifyIfOverspent` never fails the primary operation even if the fake publisher returns an error, mirroring the "errors are values, handled at the boundary" rule for this specific fail-open case.

### notification-service (owns: notifications)
```
GET    /api/v1/notifications        ?unread_only&page&page_size        -> 200 {notifications: [], page, page_size, total}  (paginated, Phase 6 — see Pagination section above)
POST   /api/v1/notifications        {title, message, type}              -> 201 {notification}  (owner = X-User-Id, same as every other route; budget-service's Phase 4 overspend trigger forwards the originating user's X-User-Id rather than passing user_id in the body, so the ownership model never has two shapes)
PATCH  /api/v1/notifications/:id/read                                  -> 200 {notification}
```

### gateway
Routes all of the above under `/api/v1/*` to the owning service; also exposes:
```
GET /health   /ready   /live
```
aggregating nothing special — its own liveness only (it doesn't own data).

## Health Endpoints (every service)

```
GET /live    -> 200 {"status":"ok"}                      always ok if process is up
GET /ready   -> 200 {"status":"ok"} | 503 {"status":"not_ready", "checks":[...]}   ok only if Mongo ping succeeds
GET /health  -> 200 aggregate of the above, richer payload for humans/dashboards
```
Implemented once via `shared/health`; each service registers its Mongo client as a `Checker`.

## OpenAPI Specs Served Live (Phase 6)

```
GET /openapi.yaml -> 200, Content-Type: application/yaml; charset=utf-8, raw spec bytes
```

Every service (including the gateway) exposes its own `openapi.yaml` at this path — not just as a static file in the repo, but as a real endpoint any of the 5 running containers answers. Implemented via `shared/openapidoc`: `Load(path, log)` reads the file from disk once at startup (a missing file logs a warning and leaves the route returning `404 NOT_FOUND` rather than crashing the process — a docs gap is not a reason to fail health checks), `Handler(spec)` serves the bytes with the right content type. Each service's `Dockerfile` copies its `openapi.yaml` into the final image alongside the binary.

Deliberately **not** `go:embed`: every service's `openapi.yaml` lives at the service root (`services/<name>/openapi.yaml`), while the Go code that would embed it lives under `internal/` or `cmd/` — `go:embed` cannot reference a parent directory (`..`), and moving the spec into an embeddable location would break the per-service layout `CLAUDE.md` §4 documents. Disk-read-at-startup was chosen over the alternative of restructuring the repo around the tooling.
