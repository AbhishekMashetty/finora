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

## Auth Header Contract (Gateway → Services)

The gateway is the only component that validates JWTs. On every request forwarded to a downstream service, the gateway sets:

- `X-User-Id` — the authenticated user's ID (from the access token's `sub` claim)
- `X-Request-Id` — propagated or generated request id

Downstream services **trust these headers** on the internal docker/K8s network (no direct external access to service ports in production). Services must reject requests missing `X-User-Id` on any route that operates on user-owned data, using shared middleware (`shared/middleware.RequireIdentity`).

*Trade-off:* this is simpler than every service independently validating JWTs/JWKS, at the cost of trusting network boundaries. Documented as a Phase-9+ hardening candidate (per-service JWKS validation) once the services sit behind a real service mesh / NetworkPolicies.

## CORS (Gateway Only)

The gateway is also the only component that applies CORS middleware (`shared/middleware.CORS`). Backend services never do. This isn't just a style choice: `net/http/httputil.ReverseProxy` copies the backend's response headers onto the gateway's outgoing response with `Header().Add`, not `Set`. If a backend also set `Access-Control-Allow-Origin`, the browser would receive it twice as one comma-joined value — which every browser rejects as invalid, even though the underlying request succeeded end-to-end. This exact bug shipped during Phase 0 and was only caught by real browser-based verification (`curl` never enforces CORS, so it looked fine at the API-testing stage). Backend routers apply `RequestID → Logging → Recovery` only.

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

GET    /api/v1/transactions         ?account_id&category&from&to&page  -> 200 {transactions: [], page, total}
POST   /api/v1/transactions         {account_id, category, amount, type, date, note} -> 201 {transaction}
GET    /api/v1/transactions/:id                                        -> 200 {transaction}
PUT    /api/v1/transactions/:id                                        -> 200 {transaction}
DELETE /api/v1/transactions/:id                                        -> 204

GET    /api/v1/categories                                              -> 200 {categories: []}  (Phase 2)
POST   /api/v1/categories           {name, type}                       -> 201 {category}          (Phase 2)
```
All resources are owner-scoped by `X-User-Id`; cross-user access returns `404 NOT_FOUND` (never `403`, to avoid confirming existence).

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

#### budget-service → notification-service (Phase 4 — the overspend-notification trigger)

`GET /api/v1/reports/summary`'s `Summary()` also triggers a real in-app notification when it finds a budget over spent for the requested range: immediately after computing a category's `actual`/`remaining`, if `remaining < 0`, budget-service calls **notification-service** directly over REST, forwarding the caller's `X-User-Id` header exactly as it does for expense-service (same internal-network trust mechanism, no JWT):

```
POST /api/v1/notifications   {title: "Budget exceeded", message: "You've spent {actual} of your {budgeted} {period} {category} budget.", type: "overspend"}
```

Implemented in `services/budget-service/internal/client/notification_client.go` behind `domain.NotificationClient`, so `report_service.go` depends only on that interface, never a concrete HTTP client — same Dependency Inversion pattern as `ExpenseClient`, unit-testable with a fake.

**Dedup rule (so a dashboard reload doesn't spam the user):** `domain.Budget` carries an internal, non-API-visible `LastNotifiedAt *time.Time` field (`json:"-"`). Despite the name, it stores the **period's `from` boundary** that was last notified for, not a wall-clock timestamp — a budget notifies only if `LastNotifiedAt == nil` or `!LastNotifiedAt.Equal(from)` (the report's requested `from`). Concretely: the first dashboard/report load that finds a category newly over budget notifies once for that exact `[from, to]`; reloading the same range doesn't re-notify; querying any *other* period (earlier or later, doesn't matter which) notifies again if that period is also over budget.
>
> An earlier version of this rule compared `LastNotifiedAt` against `from` as a time *ordering* ("notified at-or-after the period start") rather than an exact match, using the real notify timestamp. That broke for out-of-order queries: a user notified today for the current month, who then loaded an older, never-before-viewed over-budget month, would have that legitimate first notification wrongly suppressed — "today" is after the older month's `from`, so the ordering check treated it as already-notified. Caught in review before shipping; fixed by keying dedup on exact-period-match instead of time ordering, and storing the period boundary itself rather than the notify time.

**This is a deliberate, documented trade-off**, not an accident of REST semantics: a `GET` request having a side effect (creating a notification, mutating `LastNotifiedAt`) isn't pure REST, and it exists only because Finora has no event bus yet (per `CLAUDE.md` §2/`plan.md`, async messaging is explicitly deferred to Phase 7). The alternative — a real overspend *event* published when a transaction is created, consumed by notification-service — is exactly what Phase 7 (async messaging seam, `architecture/development-roadmap.md`) replaces this with; this read-triggered version is the pragmatic stand-in until then, chosen over the alternative of building a bespoke polling/cron job (more moving parts, another scheduler to operate, for a problem a future event bus solves properly).

A `Notify` failure (e.g. notification-service unreachable) is logged and swallowed — it never fails the report response, since the report itself was already computed correctly and a transient notification-delivery hiccup shouldn't turn a working read endpoint into a `500`.

### notification-service (owns: notifications)
```
GET    /api/v1/notifications        ?unread_only                       -> 200 {notifications: []}
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
