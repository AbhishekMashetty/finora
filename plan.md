# Principal Software Engineer & System Architect

You are an elite Principal Software Engineer and Software Architect with extensive experience building production-grade cloud-native SaaS platforms.

You are responsible for designing and implementing an application that will evolve over many months.

Treat this as if you are leading a team of senior engineers building a real product.

---

# The Real Goal

This project is NOT primarily about the business domain.

The business domain simply provides realistic workflows.

The real objective is to create a production-quality distributed system that serves as a long-term learning platform for Kubernetes, DevOps, Platform Engineering, CI/CD, observability, distributed systems, and cloud-native architecture.

Assume this application will eventually be deployed onto Kubernetes and operated like a production system.

Every architectural decision should make future operations easier.

---

# My Role

I am NOT a software engineer.

I am a DevOps / Platform Engineer.

You are responsible for writing and maintaining the application.

I am responsible for:

* Docker
* Kubernetes
* Helm
* GitHub Actions
* Argo CD
* Networking
* Monitoring
* Logging
* Secrets
* Scaling
* Security
* Deployment
* Infrastructure

Generate code that is easy to deploy and operate.

---

# Product

The product is called

Finora

Finora is a modern personal finance application.

Core functionality includes:

* User registration
* Login
* JWT authentication
* Dashboard
* Accounts
* Transactions
* Expense tracking
* Income tracking
* Categories
* Budgets
* Savings goals
* Reports
* Search
* User profile
* Settings

The product should feel realistic but does NOT need hundreds of features.

---

# Architecture

This project starts as a microservice architecture.

Each service must have one clear responsibility.

Initial services:

* API Gateway
* User Service
* Expense Service
* Budget Service
* Notification Service

Frontend:

Next.js

Backend:

Go

Framework:

Gin

Database:

MongoDB

Every service owns its own MongoDB database.

Never allow multiple services to directly share the same database.

Communication between services should initially use REST.

As the project evolves we may introduce asynchronous messaging.

Do not introduce unnecessary complexity until there is a legitimate reason.

---

# Design Principles

Always prefer:

* Clean Architecture
* SOLID principles
* Domain Driven Design where appropriate
* Clear separation of concerns
* Readable code
* Maintainability
* Testability

Avoid clever code.

Optimize for long-term maintainability.

---

# Repository Structure

Design a repository that can grow naturally.

Use a monorepo.

Example:

/
frontend/

services/

user-service/

expense-service/

budget-service/

notification-service/

gateway/

shared/

proto/ (if needed later)

infrastructure/

docker/

kubernetes/

helm/

.github/

docs/

architecture/

scripts/

---

# Every Service Must Include

* Go module
* README
* Dockerfile
* Makefile
* Configuration
* Health endpoint
* Readiness endpoint
* Liveness endpoint
* Structured logging
* Graceful shutdown
* Configuration through environment variables
* Unit tests
* OpenAPI documentation

The service should be production-ready.

---

# API Standards

Use REST.

Version APIs.

Example:

/api/v1/users

/api/v1/expenses

/api/v1/budgets

Use consistent response formats.

Return meaningful errors.

Validate all requests.

---

# Logging

Use structured JSON logging.

Every request should include:

* Request ID
* Timestamp
* Service Name
* Log Level
* Error details when applicable

Design logging so centralized log aggregation (for example, Loki) can be added later without changing the application.

---

# Configuration

No hardcoded values.

Everything should come from environment variables.

Design configuration so Kubernetes ConfigMaps and Secrets can be used later.

---

# Health Checks

Every service must expose:

/health

/ready

/live

These should follow Kubernetes best practices.

---

# Documentation

Every significant decision must be documented.

If you choose a library, explain why.

If you reject an alternative, explain why.

Generate documentation as if another engineer will maintain this project in two years.

---

# Future Evolution

Design today's architecture so it can later support:

* Redis
* NATS
* RabbitMQ
* OpenTelemetry
* Prometheus
* Grafana
* Loki
* Tempo
* Argo CD
* Helm
* Horizontal Pod Autoscaler
* Ingress
* cert-manager
* External Secrets
* Object storage
* File uploads
* OCR
* AI categorization
* Analytics
* Report generation

Do not implement these yet.

Simply make future integration straightforward.

---

# Most Important Rule

Do NOT attempt to generate the entire application immediately.

Instead:

1. Design the architecture.
2. Design the repository.
3. Design every service.
4. Design APIs.
5. Design database schemas.
6. Design contracts.
7. Design folder structures.
8. Design shared libraries.
9. Document every decision.

Only after the architecture is complete should implementation begin.

---

# Create These Files First

Generate only the following:

CLAUDE.md

README.md

architecture/system-overview.md

architecture/service-boundaries.md

architecture/api-contracts.md

architecture/database-design.md

architecture/repository-structure.md

architecture/development-roadmap.md

docs/local-development.md

Do not generate application code yet.

---

# CLAUDE.md

This file is the permanent source of truth for the project.

It should define:

* Coding standards
* Architecture rules
* Service boundaries
* Folder conventions
* API standards
* Documentation requirements
* Testing requirements
* Git conventions
* Naming conventions
* Dependency management
* Definition of Done for every feature

Every future implementation must follow CLAUDE.md.

Treat it as the project's constitution.

---

Think carefully before writing anything.

Favor architecture over speed.

If a decision has tradeoffs, explain them.

The quality of the architecture is more important than the quantity of generated code.

---

# Implementation Plan

*Last updated: 2026-07-19. This section is the live, phase-by-phase execution record for the brief above — read this first if you're a fresh Claude session picking this project back up, then consult `CLAUDE.md` and `architecture/*.md` for the "how" behind each phase. Keep this section and `architecture/development-roadmap.md` in sync as phases complete; this is the canonical copy.*

## Current Repository State (as of Phase 0 completion)

This is no longer an empty repo with just a brief — a full monorepo exists and the whole stack runs locally via Docker Compose. Concretely, on disk right now:

```
/
├── CLAUDE.md, README.md                — project constitution + product overview
├── architecture/*.md                   — system-overview, service-boundaries, api-contracts,
│                                          database-design, repository-structure, development-roadmap
├── docs/local-development.md           — setup, curl flows, troubleshooting (incl. real gotchas hit below)
├── docker-compose.yml, Makefile, .env.example, go.work
├── shared/                             — Go module github.com/finora/shared: logger, httpx envelope,
│                                          middleware (RequestID/Logging/Recovery/CORS/RequireIdentity),
│                                          health (/live /ready /health), config, server (graceful shutdown),
│                                          jwtx (access+refresh JWT), mongox (connect + health checker)
├── services/gateway/                   — reverse proxy + JWT validation, no data of its own
├── services/user-service/              — auth (register/login/refresh/logout), profile, settings
├── services/expense-service/           — accounts, transactions, categories (owner-scoped)
├── services/budget-service/            — budgets (full CRUD), goals (create+list), reports (placeholder)
├── services/notification-service/      — notifications (create/list/mark-read), email-sender seam
└── frontend/                           — Next.js: register/login, dashboard shell, stubbed nav
```

Each Go service follows Clean Architecture (`cmd/server` → `internal/{config,domain,repository,service,handler,router}`), has its own MongoDB, a Dockerfile, Makefile, README, openapi.yaml, and unit tests. Full endpoint list, ports, envelope shape, and the JWT/auth-header contract are in `architecture/api-contracts.md` — that file is the ground truth for anyone adding a route.

**To resume work:** `cp .env.example .env && docker compose up --build`, wait for all containers to report healthy (`docker compose ps`), then see `docs/local-development.md` for curl flows and troubleshooting.

## Phase Status

**Phase 0 — Foundation & scaffolding: ✅ COMPLETE, verified end-to-end on 2026-07-17.**

Verification performed (not just "it builds"):
- `docker compose up --build` — all 10 containers (4 Mongos + 4 services + gateway + frontend) reached `healthy`.
- Full API flow via curl through the gateway: register → login → `GET /users/me` (JWT-protected) → `401` with no token → refresh (rotation) → protected calls into expense/budget/notification services, all correctly owner-scoped via the gateway-forwarded `X-User-Id`.
- Full **browser-driven** flow (headless Chromium via Playwright, screenshotted): home → login → dashboard (live `GET /users/me` render) → nav → logout → back to login, zero console errors.
- All 6 Go modules (`shared` + 5 services) build/`go vet`/test clean; frontend builds and lints clean.

Three real bugs were found and fixed only because of the end-to-end + browser verification (not caught by unit tests or curl alone) — recorded here so they aren't reintroduced:
1. **macOS Go 1.21 linker bug** (`dyld: missing LC_UUID load command` on native `go build`/`go test`) — fixed once, machine-wide, via `go env -w GOFLAGS="-ldflags=-linkmode=external"`. Docker builds were never affected (Linux-only compilation).
2. **Duplicate CORS headers** — every backend service was applying its own CORS middleware in addition to the gateway's. Since `httputil.ReverseProxy` copies backend response headers with `Header().Add` (not `Set`), the browser received `Access-Control-Allow-Origin` twice, comma-joined, and rejected every request outright — even though curl-level testing looked fine (curl doesn't enforce CORS). **Fixed and now a hard rule:** CORS is applied only in the gateway, documented in `CLAUDE.md` and `architecture/api-contracts.md`.
3. **Next.js standalone Docker healthcheck failures** — Docker's auto-set `HOSTNAME` env var made Next's server bind to the container's interface IP instead of all interfaces, and separately `localhost` resolved to IPv6 first while the server only listens on IPv4. Both fixed in `frontend/Dockerfile` (`ENV HOSTNAME=0.0.0.0`, healthcheck uses `127.0.0.1`).

All three are written up in `docs/local-development.md`'s Troubleshooting section.

**Phases 0–3 — ✅ COMPLETE.** Phases 4–9 not started. Full descriptions and "Done when" acceptance criteria for each live in `architecture/development-roadmap.md` (kept in sync with this file). Summary:

| Phase | Goal | Done when |
|---|---|---|
| 1 — Auth & user hardening ✅ | Full profile/settings CRUD, refresh-token rotation/revocation depth, **password-reset stub**, consistent error codes, real test coverage | User domain feature-complete, tested end-to-end |
| 2 — Expense domain depth ✅ | Full CRUD for accounts/transactions/income/expenses/categories, pagination/filtering, Mongo indexes, OpenAPI completeness | User can manage real accounts/transactions through the gateway with pagination + validation |
| 3 — Budget domain depth ✅ | Budgets, goals, and real **reports** (budget-service calling expense-service via REST) | Budgets/goals/reports return real computed cross-service data |
| 4 — Notifications ✅ | Event-triggered in-app notifications from other services, real email-sender implementation | A domain event (e.g. overspend) produces a visible notification |
| 5 — Frontend depth ✅ | Wire every product screen to live APIs; forms, client state, charts, token-refresh handling | All listed product features usable through the UI |
| 6 — Cross-cutting hardening | Integration tests (Mongo testcontainers), rate limiting, standardized pagination/errors, full OpenAPI, green CI | CI green, integration tests cover critical flows |
| 7 — Async messaging seam | Introduce NATS for domain events, replacing sync REST notification calls | Notifications are event-driven, services decoupled for those flows |
| 8 — Observability seam (app side) | OpenTelemetry tracing, Prometheus `/metrics`, Loki-ready log fields | Traces + metrics emitted and scrapeable |
| 9 — Kubernetes readiness (app side) | Finalize probes/resource behavior, 12-factor config, image hardening, graceful-shutdown tuned to pod lifecycle | Services deployable to K8s without further app changes |

### Phase 1 progress log

- **2026-07-17 — Password-reset stub: done.** Added `POST /api/v1/auth/password-reset` end-to-end: `domain.AuthService.RequestPasswordReset` (Phase 1 stub — always succeeds, never reveals whether the email is registered; no reset token issued and no email sent yet, that's real follow-up work), the handler + public route in `user-service`, the matching public route in the **gateway** (`services/gateway/internal/router/router.go` — easy to forget; the gateway hardcodes its own public-route allowlist separately from user-service's), `openapi.yaml`, and `architecture/api-contracts.md` (both the Public routes list and the user-service endpoint table). Table-driven tests added at both the service and handler layers following the existing fake-repository pattern.
  - **Verification: done (2026-07-18).** `gofmt`/`go build`/`go vet`/`go test` pass in both `services/user-service` and `services/gateway`. Rebuilt both images (`docker compose up --build -d user-service gateway`) and curled the live gateway: registered email → 202 with the generic message; unregistered email → identical 202 (confirmed no enumeration leak); malformed email → 400. The stub is genuinely done, not just compiled.

- **2026-07-18 — Remaining Phase 1 hardening: done.** Closed out all three remaining gaps in `user-service`:
  - **Profile/settings validation depth.** `PUT /users/me`: `name` now trimmed and rejected if blank-after-trim or over 100 chars. `PUT /users/me/settings`: `currency` validated as a 3-letter uppercase code, `timezone` validated via `time.LoadLocation` (real IANA name required) — previously both fields accepted any string, including garbage, uncaught. Validation lives in `internal/service` via a new `ValidationError` type (matching `expense-service`'s existing convention), mapped to `400 VALIDATION_ERROR` with `{field}` details at the handler layer.
  - **Refresh-token reuse detection.** Added `domain.RefreshTokenRepository.RevokeAllForUser` (Mongo `UpdateMany`, plus a fake for tests). `Refresh()` now distinguishes "found but already revoked" (a replay/theft signal — the legitimate client already rotated past this token) from "found but merely expired," and on the revoked case revokes *every* refresh token for that user before failing the request — full session-wide containment, not just a single rejected call. Client-visible response is unchanged (still 401); the server-side blast-radius response is new.
  - **Error codes audit.** Checked every `httpx.Fail` call in `user-service` against `architecture/api-contracts.md`'s standard codes table — all correct, no changes needed.
  - **Test coverage.** New table-driven cases at both service and handler layers for every validation path above, plus a dedicated test proving the reuse-detection cascade (two independent sessions; replaying one's rotated-away token revokes the other session's still-valid token too).
  - **Verification: done.** `gofmt`/`go build`/`go vet`/`go test` clean in `services/user-service`. Rebuilt the image and verified **live** through the running gateway (not just unit tests): invalid currency/timezone/blank-name all correctly 400; a second concurrent session was set up, session 1's token was rotated, the old token was replayed (401, as expected), and — the actual point of the feature — session 2's independent, never-touched refresh token was confirmed revoked as a side effect (401), proving the cascade genuinely works end-to-end and not just against the in-memory test fake. A full register→login→me→settings regression afterward confirmed nothing else broke.

**Phase 1 is done.**

### Phase 2 progress log

- **2026-07-19 — Expense domain depth: done.** Started this phase with an audit of `services/expense-service` rather than assuming a blank slate, since prior sessions had already built most of it: accounts have full CRUD (owner-scoped, intentionally unpaginated — a user has a handful of accounts, not pages of them), categories have create+list only (intentionally minimal, per the existing doc comment in `internal/domain/category.go` — not expanded, since nothing in the roadmap concretely requires more), and transactions already had full CRUD plus real pagination (`page`/`page_size`) and filtering (`account_id`/`category`/`from`/`to`) wired through `internal/domain/transaction.go`'s `TransactionFilter` and `internal/service/transaction_service.go`. The one real gap: **no MongoDB index creation existed anywhere in the service** — confirmed by grepping `internal/repository` for "Index" (zero hits), unlike `user-service`'s `EnsureIndexes`.
  - Added `services/expense-service/internal/repository/mongo.go`, an `EnsureIndexes(ctx, db)` following the exact idiom of `user-service/internal/repository/mongo.go` (same idempotent `CreateOne` calls, called on every boot). Wired into `services/expense-service/cmd/server/main.go` immediately after `client.Database(...)`, matching `user-service`'s placement and error-handling style exactly.
  - Indexes created: `accounts` → `{user_id: 1}`; `transactions` → compound `{user_id: 1, account_id: 1, date: 1}` (the list-endpoint access pattern: filter by account, sort/paginate by date, within one owner) plus a second compound `{user_id: 1, category_id: 1}` marked **sparse** (category is optional on a transaction); `categories` → `{user_id: 1}`.
  - **Real bug caught before it shipped:** the first draft used Go `map[string]int` for the compound indexes' `Keys` (copying the single-field style from `user-service`'s `EnsureIndexes`). That's wrong for multi-field indexes — Go map iteration order is not guaranteed, and compound indexes are order-sensitive (leftmost-prefix rule); `user_id` must be first for the intended query patterns to use the index. Switched to `bson.D` (which preserves insertion order) for both compound indexes before ever running it against a live Mongo. Verified afterward that the live index documents show the fields in the intended order (see below) — had the map version shipped, MongoDB might have silently built the index with fields reordered (e.g. `account_id` before `user_id`), which would still "work" (index exists, code compiles) but wouldn't serve the actual query pattern efficiently — exactly the kind of bug that `go build`/`go test` passing would never catch.
  - Corrected a stale field name in `architecture/database-design.md`'s expense-service section: the `transactions` table and index bullet said `category` (with a note "or `category_id` once wired, Phase 2") — the code has used `category_id` throughout since it was written, so the doc was simply out of date. Fixed both the field table and the index bullet to say `category_id`, and noted the sparse rationale.
  - Checked `services/expense-service/openapi.yaml` against the actual routes: transactions' pagination params (`page`, `page_size`) and filters (`account_id`, `category`, `from`, `to`) were already fully documented, `category_id` already appears correctly as nullable on the `Transaction`/`CreateTransactionRequest` schemas. No changes needed — it was already accurate, confirming OpenAPI completeness for this phase's "done when" bar.
  - **Verification: done.** `gofmt -l .`, `go vet ./...`, `go build ./...`, `go test ./...` all clean in `services/expense-service` (ran from inside that directory, not repo root, per the known Go 1.21 workspace-mode `./...`-from-root limitation). Then brought up the live stack (`docker compose up --build -d mongo-expense expense-service`, plus the rest of the stack was already running from a prior session — all 10 containers stayed healthy), confirmed the container logged no index-creation error and reached `healthy`, and ran `docker compose exec mongo-expense mongosh finora_expenses --eval "db.<collection>.getIndexes()"` against a **live** database (not inferred from code) for all three collections. Actual output confirmed: `accounts` — `_id_` plus `user_id_1`; `transactions` — `_id_` plus `user_id_1_account_id_1_date_1` (fields in that exact order) plus `user_id_1_category_id_1` with `sparse: true`; `categories` — `_id_` plus `user_id_1`. No unit tests were added for `EnsureIndexes` itself — it's index/infra plumbing against a real driver call, and repository-vs-live-Mongo tests are an explicitly deferred Phase 6 concern per `CLAUDE.md` §7.

**Phase 2 is done.**

### Phase 3 progress log

- **2026-07-19 — Budget domain depth (backend): done.** Audited `services/budget-service` first: budgets already had full CRUD; goals were an intentional create+list-only stub ("Update/delete arrive in a later phase once expense-service integration exists" — that phase is now); reports were a deliberate placeholder (`{message, from, to}`) documented as waiting on exactly this phase; and, same class of gap Phase 2 found in expense-service, budget-service had **no MongoDB indexes at all**.
  - **Goals → full CRUD.** Added `GetByIDForUser`/`Update`/`DeleteByIDForUser` to the repo and `Get`/`Update`/`Delete` to the service, plus `PUT`/`DELETE /api/v1/goals/:id` routes. `current_amount` is manually updated via `PUT` (progress logging) — goals do **not** get the cross-service expense-linkage treatment reports get, per the roadmap's distinction between "savings goals" and reports' explicit "cross-service aggregation" clause.
  - **Reports → real cross-service computation.** New `domain.ExpenseClient` interface (Dependency Inversion — `report_service.go` depends only on it, never a concrete type) implemented by `internal/client/expense_client.go`'s `ExpenseHTTPClient`: calls expense-service's `GET /api/v1/categories` (forwarding the caller's `X-User-Id`, the same internal-network trust mechanism `shared/middleware.RequireIdentity` already establishes) to resolve a budget's category **name** to expense-service's category **id** case-insensitively (budget-service's `Budget.Category` is a name string; expense-service's transactions store `category_id`, a real pre-existing mismatch this phase had to bridge, not invent), then sums matching expense transactions via `GET /api/v1/transactions?category=<id>&from&to`, paginating (capped at 50 pages × 100 = 5,000 tx) and filtering `type == expense` client-side since expense-service has no server-side type filter. A category with no match returns `actual: 0`, not an error. `ReportSummary` now carries real `{category, period, budgeted, actual, remaining}` per budget plus totals; `from`/`to` are required query params (400 if missing — no implicit default range).
  - **Mongo indexes.** New `internal/repository/mongo.go` `EnsureIndexes`, same idiom as expense-service's Phase 2 fix: `budgets` → `{user_id:1}` + compound `{user_id:1, period:1}`; the goals collection (confirmed live via `mongosh` to actually be named `goals`, **not** `savings_goals` as `architecture/database-design.md` had said — doc corrected to match reality) → `{user_id:1}`.
  - **Docs:** `architecture/api-contracts.md` (goals CRUD table, real reports schema, new subsection documenting the budget-service→expense-service call), `architecture/database-design.md` (`goals` collection name fix), `services/budget-service/openapi.yaml`, `services/budget-service/README.md` (`EXPENSE_SERVICE_URL`, plus a stale "docker-compose.yml not yet created" line fixed — that file has existed since Phase 0).
  - **Verification: done, independently re-checked, not just trusted.** `gofmt`/`go vet`/`go build`/`go test` clean in `services/budget-service`. I personally ran a live end-to-end curl flow through the gateway — register → login → create account → create category "Verifycat" → two expense transactions ($40, $25.50 in July 2026) → create a lowercase "verifycat" budget → `GET /api/v1/reports/summary?from=2026-07-01&to=2026-07-31` — and got back `actual: 65.5, remaining: 34.5`, confirming the case-insensitive category match and cross-service sum are genuinely computed, not fabricated. Also confirmed live via `mongosh`: `db.budgets.getIndexes()` shows `user_id_1` and `user_id_1_period_1`; `db.goals.getIndexes()` shows `user_id_1`.

- **2026-07-19 — Frontend (pulled forward from Phase 5 by explicit user request): done.** Reports/budgets are meaningless without a way to create real transaction data, so — after asking and getting explicit confirmation — this pass also wired `accounts` and `transactions` (fully backed since Phase 2 but still `ComingSoon` stubs) alongside the new `budgets`/`goals`/`reports` pages, all against the live gateway via the existing `apiFetch` wrapper (`frontend/lib/api.ts`). `profile`/`settings`/`search` remain untouched stubs.
  - Accounts/transactions/budgets/goals full CRUD UI; transactions page adds pagination, account/category/date-range filters, and inline category creation (categories are intentionally create+list only on the backend, so there's no separate categories page/nav item — just an inline "+ new category" control on the transactions form); goals show a progress bar plus a manual "log progress" control; reports page defaults to the current calendar month, auto-runs on mount, and renders a budgeted/actual/remaining table with a simple CSS usage bar (no charting library added — none exists in `package.json` yet, and `CLAUDE.md` §10 requires justifying new dependencies; a real charting library is explicitly a later Phase 5 concern).
  - New types added to `frontend/lib/types.ts`: `Account`, `Category`, `Transaction`, `Budget`, `Goal`, `CategorySummary`, `ReportSummary`.
  - **Verification: done.** I personally re-ran `npm run build` and `npm run lint` (both clean) and read `transactions/page.tsx` and `reports/page.tsx` in full myself, independent of the delegated agent's own checks.

- **2026-07-19 — Conformance review caught two real bugs, both fixed before calling this done:**
  1. **`goal_service.go`'s `Update`** re-validated "target_date must be in the future" against the *resent* value unconditionally — but `PUT` is a full-replace and the only path for progress-logging (`current_amount`), so once a goal's due date passed, every subsequent update (including pure progress-logging) failed with a confusing 400, permanently freezing the goal. Fixed to only reject a target_date that's both **changed** and in the past (`!in.TargetDate.Equal(existing.TargetDate) && in.TargetDate.Before(now)`); added a regression test (`goal_service_test.go`: "re-sending an already-past target_date unchanged is allowed") and confirmed live via curl that repeated updates against an unchanged date keep succeeding.
  2. **Frontend `accounts/page.tsx`** offered `investment`/`other` as account types, neither accepted by expense-service's `domain.ValidAccountType` (`checking`/`savings`/`credit`/`cash` only) — guaranteed 400s for two of six dropdown options. Trimmed the frontend list to match the real backend constraint exactly.
  - Both fixes re-verified: `gofmt`/`go vet`/`go build`/`go test` clean in `services/budget-service`, `npm run build`/`npm run lint` clean in `frontend`, stack rebuilt (`docker compose up --build -d budget-service frontend`) and confirmed healthy.

**Phase 3 is done.**

### Phase 4 progress log

- **2026-07-19 — Notifications: done.** Audited `services/notification-service` first rather than assuming a blank slate: create/list/mark-read (`POST`/`GET /api/v1/notifications`, `PATCH /api/v1/notifications/:id/read`) were already fully implemented and owner-scoped correctly, and a `domain.EmailSender` interface + `LoggingEmailSender` (structured-log implementation — the roadmap explicitly accepts structured-log as an alternative to SMTP) already existed from earlier scaffolding. The real gaps: **nothing anywhere called `POST /api/v1/notifications`** (no trigger existed in the fleet at all), the email sender was constructed but **never actually invoked**, and **no MongoDB indexes existed** in notification-service — same class of gap Phase 2/3 fixed in expense-service/budget-service.
  - **The trigger design decision** (made explicitly, not left to the implementer): Finora is REST-only until Phase 7 (no event bus), so there's no "a transaction was created" event for expense-service to publish. The only place that already computes "is this budget over spent" is budget-service's `report_service.Summary()` (Phase 3's cross-service report). Wired the trigger there: immediately after computing a category's `remaining`, if negative, budget-service calls notification-service over REST via a new `domain.NotificationClient` (`internal/client/notification_client.go`, same Dependency Inversion + `X-User-Id`-forwarding pattern as Phase 3's `ExpenseClient`). This makes `GET /api/v1/reports/summary` — a read endpoint — have a side effect, which is **not** pure REST semantics; documented as a deliberate, explicit trade-off in `architecture/api-contracts.md` and `architecture/development-roadmap.md`'s Phase 4 design note, accepted only because Phase 7's real event bus is exactly where this gets replaced.
  - **Dedup**, so a dashboard/report reload doesn't spam the user: `Budget` gained an internal, non-API-visible `LastNotifiedAt *time.Time` (`json:"-"`).
  - **Email wiring**: `notificationService.Create()` now calls `email.Send(ctx, userID, title, message)` as a best-effort side effect (logged and swallowed on error, never fails notification creation) — deliberately using `userID` itself as the "to" field rather than resolving a real email address via a new notification-service → user-service call, since the sender is a no-op logger today and a real lookup would be complexity spent on a value nobody reads yet. Revisit once a real SMTP sender lands.
  - **Mongo indexes**: `services/notification-service/internal/repository/mongo.go` (new) `EnsureIndexes` — compound `{user_id: 1, read: 1}` on `notifications`, matching `architecture/database-design.md`'s pre-existing spec exactly.
  - **Real bug caught in independent review before shipping, fixed:** the first version of the dedup rule compared `LastNotifiedAt` (a wall-clock notify timestamp) against the report's `from` as a time *ordering* (`LastNotifiedAt.Before(from)` → not-yet-notified). This broke for out-of-order historical queries: a user notified today for the current month, who then loaded an *older*, never-before-viewed over-budget month, would have that legitimate first notification silently suppressed — "today" is after the older month's `from`, so the ordering check treated it as already-handled. **Fixed** by keying dedup on exact-period-match instead: `LastNotifiedAt` now stores the report's `from` boundary itself (not real time), and the check is `LastNotifiedAt.Equal(from)`. Added a regression test (`report_service_test.go`: "notifying for a recent period does not suppress a later, older-period notification") and re-verified **live**, not just in the unit test: created a real over-budget scenario for August (1 notification), then a separate never-viewed over-budget scenario for April (an older period) — confirmed a 2nd notification fired instead of being suppressed. `architecture/api-contracts.md` and `services/budget-service/README.md` both updated to describe the corrected semantics and document why the original design was wrong.
  - A second independent review pass confirmed the fix is correct (re-derived the logic independently, checked DST/precision/zero-value edge cases — none reachable given how dates are parsed) and flagged two minor, non-blocking items for future reference rather than fixing now: (a) `LastNotifiedAt` is stored as a BSON date at millisecond precision, so a `from` with sub-millisecond precision could in theory fail an exact match and cause a harmless duplicate notification — vanishingly unlikely given real usage is day-granularity dates; (b) `notifyIfOverspent` is a read-then-write with no atomic check-and-set, so two genuinely concurrent `Summary` calls for the same never-before-notified period could both notify — low-consequence (an extra notification, not data loss or a security issue) and not worth a locking mechanism for a workaround Phase 7 removes entirely. Both documented as known limitations in `report_service.go`'s doc comment rather than engineered around, per `CLAUDE.md`'s anti-overengineering principle.
  - **Frontend, built directly** (not delegated — a contained, one-page addition): a new `frontend/app/dashboard/notifications/page.tsx` (list with an unread-only toggle, mark-read, using the design-system primitives from the frontend redesign — `Card`/`Badge`/`Button`/`EmptyState`/`SkeletonRows`), a new `BellIcon`, a new `Notification` type in `lib/types.ts`, and a sidebar nav entry with a polling-based unread-count badge (`useUnreadNotificationCount`, 30s interval, only while authenticated, cleans up on unmount). "Done when: a domain event produces a **visible** notification" genuinely required a UI surface, not just a backend endpoint that nothing rendered.
  - **Verification: done, independently, twice.** `gofmt`/`go vet`/`go build`/`go test` clean in both `services/budget-service` and `services/notification-service` (confirmed by me directly, then again by a fleet-wide sweep across all 5 services in the second review pass — all clean). Live end-to-end through the real gateway: registered a user, created an over-budget scenario, confirmed a real "Budget exceeded" notification appeared with correct title/message; confirmed a repeat report call didn't duplicate it (dedup); confirmed `docker compose logs notification-service` showed the "email send skipped (no-op sender)" line proving the email wiring fired; confirmed live via `mongosh` that `db.notifications.getIndexes()` shows the new compound index and a budget document carries a persisted `last_notified_at`. `npm run build`/`npm run lint` clean in `frontend/`, and a live curl check confirmed the notifications page's parsing logic matches the real `GET`/`PATCH /api/v1/notifications` response shapes exactly, including a real mark-read round trip.

**Phase 4 is done.**

### Phase 5 progress log

- **2026-07-19 — Frontend depth: done.** Closed the last three stub screens — `profile`, `settings`, `search` — completing Phase 5 (accounts/transactions/budgets/goals/reports/notifications were already wired incrementally across Phases 3-4, each time pulled forward by explicit user request since those screens were meaningless to validate in the browser without real data to point at).
  - **Profile** (`frontend/app/dashboard/profile/page.tsx`, new): view/edit `name` against user-service's already-complete `GET`/`PUT /api/v1/users/me` (Phase 1). Email rendered read-only — no endpoint anywhere in the fleet changes a user's login email, and adding one wasn't in scope here.
  - **Settings** (`frontend/app/dashboard/settings/page.tsx`, new): currency/timezone against `GET`/`PUT /api/v1/users/me/settings` (also Phase 1, already returns sane `USD`/`UTC` defaults for a user who's never saved settings, so the page needed no empty-state branch). New `Settings` type added to `lib/types.ts`.
  - **Search** (`frontend/app/dashboard/search/page.tsx`, new): no dedicated search endpoint exists anywhere in the fleet, and building one would be real, unjustified complexity for a personal-finance app's data volumes (`CLAUDE.md` §10/`plan.md`'s "don't add complexity before there's a legitimate reason"). Built instead as a client-side composition over the same list endpoints every other page already uses — accounts, categories (to resolve transaction category names for matching/display), transactions (capped at expense-service's own `page_size=100` ceiling — an existing server-side limit, not a new one), budgets, goals — fetched once per query, matched case-insensitively against name/note/category/type/amount.
  - "Charts" (part of Phase 5's stated goal) is satisfied by the reports page's `BudgetBar` bullet-chart from the frontend redesign, built via the `dataviz` skill method — a deliberate choice over adding a charting library dependency for one comparison-style visualization. Token-refresh handling was already complete since Phase 0 (`lib/api.ts`'s silent-refresh-then-retry-once on 401).
  - **Real, previously undiscovered production bug caught and fixed:** while live-testing the new Settings page, updating to a genuinely valid IANA timezone (`Europe/London`) was rejected with the exact same `400 VALIDATION_ERROR` a garbage string gets. Root cause: `user-service`'s `UpdateSettings` validates via Go's `time.LoadLocation`, which reads IANA tzdata from disk (`/usr/share/zoneinfo`) — and `services/user-service/Dockerfile`'s final `alpine:3.19` stage never installed the `tzdata` package, so `LoadLocation` could only ever resolve the special-cased `"UTC"` inside the actual running container. This was invisible to `go test` (which runs on the host machine, which has tzdata) and had presumably been silently broken since Phase 1 — Phase 1's own "verified live" testing evidently only exercised timezone *rejection* (which still 400s either way) and never confirmed a real zone's *acceptance* inside a container. Fixed with `RUN apk add --no-cache tzdata` in the Dockerfile's final stage; rebuilt the image and confirmed live both directions: `Europe/London` now succeeds, `Not/AZone` still correctly 400s. Documented in `docs/local-development.md`'s Troubleshooting section (grepped the whole `services/` tree first to confirm no other service calls `time.LoadLocation` or needs the same fix — it's isolated to user-service).
  - **Verification: done, independently.** `npm run build`/`npm run lint` clean. Live curl through the gateway confirmed `GET`/`PUT /api/v1/users/me` and `GET`/`PUT /api/v1/users/me/settings` response shapes match exactly what the two pages parse, and confirmed real account/budget/goal data in the live database would actually be matched by the search page's filtering logic. `gofmt`/`go vet`/`go build`/`go test` clean in `user-service`. A conformance review independently re-verified all of the above, checked the search page's cross-user-leak surface (none — every underlying endpoint filters by `user_id` at the Mongo query level, not just the handler), confirmed the tzdata fix is complete and correctly scoped (grepped the whole fleet for `LoadLocation`/`time.In`/`time.Local` — only the one call site), and confirmed the settings form's partial-update semantics can't silently ship an empty field (both inputs are `required`, blocking native form submission).

**Phase 5 is done — all listed product features (accounts, transactions, budgets, goals, reports, search, profile, settings) are usable through the UI.** `profile`/`settings`/`search` are the only screens without prior individual verification entries in this log; everything else already has its own dated entry above. Next up: **Phase 6 — Cross-cutting hardening** (real-Mongo integration tests via testcontainers, rate limiting, standardized pagination/error handling across all services, full OpenAPI docs served live, a green CI matrix) — the first fully-unstarted phase remaining before Phase 7 (async messaging).

### Cross-cutting fix (2026-07-19) — health-check log noise

Not tied to a specific phase: the user noticed every service's logs were constantly emitting lines with zero real traffic. Root cause: `shared/middleware/logging.go`'s `Logging()` middleware logs every request, including Docker/Kubernetes probe hits (`/live`, `/ready`, `/health` on a 5s interval per `docker-compose.yml`'s healthchecks) — working as designed, but noisy for local dev. Fixed by skipping those three paths in `Logging()` (added a `healthCheckPaths` lookup, `c.Next(); return` before the log line) rather than downgrading them to Debug level, since there's no per-service Debug consumer today. Added `shared/middleware/logging_test.go` (table-driven, matching the fleet's existing test style) proving health-check paths are silent and normal paths still log. Since `shared/` is pulled into every service via the `replace` directive, all 5 services needed a rebuild to pick up the change — ran `docker compose up --build -d` for the full stack, confirmed all containers reached `healthy`, then watched `docker compose logs <service> --since Ns` across a quiet window and confirmed zero log lines despite healthchecks still firing underneath, and confirmed a real request (`curl .../api/v1/accounts` through the gateway) still produces a normal `"request completed"` log line. `gofmt`/`go vet`/`go test` clean in `shared/`.

### Cross-cutting redesign (2026-07-19) — professional frontend design pass

Not tied to a specific phase: the user asked for the frontend to be redesigned as a professional designer would do it. Full rationale, every decision, and every rejected alternative (a UI kit, an icon library, a charting library) is documented in the new **`architecture/frontend-design-system.md`** — read that file, not this summary, before touching any page's visual layer. This was presentation-only: **no changes to data-fetching logic, API contracts, request/response shapes, or any backend code** — every page's actual CRUD behavior from Phase 2/3 is unchanged, only how it renders.

- **Foundation (built directly, not delegated):** real CSS design tokens in `frontend/app/globals.css` (plane/surface/ink/hairline/brand/status roles, each a light+dark pair that flips under `prefers-color-scheme` so a plain utility like `bg-surface` needs no `dark:` variant at the call site — a real simplification over the old `bg-white dark:bg-zinc-950` pattern repeated on every page); a hand-rolled ~14-icon set (`components/icons.tsx`, no icon-library dependency); shared primitives in `components/ui/` (`Button`, `Input`/`Select`, `Card`, `Badge`/`TransactionTypeBadge`, `EmptyState`, `Skeleton`/`SkeletonRows`, `StatTile`, `BudgetBar`/`BudgetLegend`/`budgetStatus`) that replaced the near-identical class-string constants every page used to redefine; a two-pane `AuthShell` for login/register; a redesigned dashboard sidebar (icons, active-item accent, signed-in identity above logout); the landing page; and a fully rebuilt dashboard overview (`app/dashboard/page.tsx`) — from a bare "signed in as {name}" card into real `StatTile`s (total balance, this month's spend, budgets on track, next goal) plus a recent-activity list and nearest-goal progress, all computed from endpoints every other page already uses.
  - **Real pre-existing bug caught and fixed along the way:** `globals.css` hardcoded `font-family: Arial, Helvetica, sans-serif` on `<body>`, silently overriding the Geist font that `app/layout.tsx` loads via `next/font` (no `font-sans` class was ever applied to `<body>` to activate the CSS variable chain) — the whole app had been rendering in the browser's Arial fallback since Phase 0 despite Geist being "wired up." Fixed to reference `var(--font-sans)` directly.
  - The reports page's budget-vs-actual visual was built using Claude Code's `dataviz` skill method (pick the form → assign validated color → apply marks/accessibility rules) rather than eyeballed — a bullet-style bar (track + colored fill + a target-marker line at the 100%-of-budget point), not a chart library, because the underlying comparison is magnitude-vs-target, not a trend or distribution. Status color (good/warning/critical, from the skill's validated, colorblind-safe status palette) drives the fill, with the numeric figures always shown alongside — never color-alone.
- **Page-level restyle, delegated to two parallel `finora-frontend` agents** (disjoint files, so safe to parallelize): accounts+transactions in one, budgets+goals+reports in the other. Both hit the session's usage limit mid-run and were reported "failed" by the harness — **but both had already finished the actual file rewrites** before being cut off; only their own final verification/reporting step was interrupted. Confirmed this myself rather than assuming: read all 5 changed files in full (not just diffed), found zero leftover legacy `zinc-`/`red-`/`green-`/`dark:` classes, `npm run build` and `npm run lint` both clean, and ran a live curl flow through the gateway (login, `GET` accounts/transactions/goals/budgets/reports-summary against a real pre-existing test user) confirming the actual API response shapes match exactly what every restyled page parses — including a real goal at 900/1000 (90%) and a real budget at 65.5/100 actual spend correctly resolving to `budgetStatus` "good" (under the 80% warning threshold), not miscolored.
  - Both pages converted the previous always-open create form into a collapsed-by-default panel behind a `+ Add` button (per the design doc's "totals/list first, form on demand" principle), replaced text-label Edit/Delete buttons with icon-only ghost buttons, replaced "Loading…" text with `Skeleton` placeholders, and replaced bare "No X found" text with `EmptyState` (icon + message + in some cases a CTA).
- **Docs updated in the same pass:** new `architecture/frontend-design-system.md` (the authoritative doc for all of this); `frontend/README.md` (new "Design system" section, updated project-layout listing, updated the now-stale Tailwind tech-choice note).
- **Verification: done.** `npm run build`/`npm run lint` clean across the whole `frontend/` app (not just the touched files). Full stack rebuilt and healthy (`docker compose up --build -d`). Live curl-based data-shape verification as described above. No browser-automation tool was available in this session, so no screenshot/visual QA was performed — the user should click through `localhost:3000` themselves to confirm the actual visual result before considering this fully done.

### Cross-cutting fixes + tooling (2026-07-19) — MongoDB log noise, transactions-page filter alignment, demo-data seed script

Three independent, user-requested items, none tied to a specific phase.

- **MongoDB log noise.** `docker compose logs mongo-<x>` was constantly emitting connection-lifecycle lines (accepted/client-metadata/access-check/ended) — the 5s healthcheck reconnects every interval across all 4 Mongo containers, and mongod logs a quartet of Informational-severity lines per connection by default. Two things were tried and ruled out first: `--quiet` alone only suppresses the accepted/ended pair in Mongo 7 (client-metadata/access-check lines remained); `--setParameter logComponentVerbosity={"network":{"verbosity":-1},"accessControl":{"verbosity":-1}}` was confirmed genuinely applied (`db.adminCommand({getParameter:1, logComponentVerbosity:1})` showed it live) but had zero effect, because component verbosity only gates Debug-level messages, never Informational — there is no supported mongod flag to selectively hide Informational connection logging. **Actual fix:** redirect mongod's own log output to `/data/db/mongod.log` (inside the already-persistent volume) via `--logpath`/`--logappend`, so `docker compose logs` — which only ever sees stdout/stderr — goes quiet, while the data is still there to inspect (`docker compose exec mongo-user cat /data/db/mongod.log`) if something actually breaks. Documented as a real trade-off (mongod startup failures won't show in `docker compose logs` anymore) in `docker-compose.yml`'s `x-mongo-command` comment and `docs/local-development.md`'s Troubleshooting section. Verified live: all 4 containers silent across a 15s window post-rebuild, all still `healthy`.
- **Transactions page filter/form alignment.** The Filters row (account dropdown, category dropdown, both date fields) and the "New transaction" form's first row used `flex flex-wrap items-end gap-3` with no explicit widths — account/category option text varies a lot in length and a native date input has a different intrinsic width than a `<select>`, so nothing lined up. Fixed both to a `grid` layout (`grid-cols-1 sm:grid-cols-2 lg:grid-cols-4` for the filter row; a 12-column grid with explicit spans for the create-form row, since it has 5 fields of genuinely different natural widths) with `w-full` on every control, so labels and fields align in clean columns and wrap predictably instead of raggedly. `npm run build`/`npm run lint` clean; no data-fetching logic touched.
- **`scripts/seed_demo_data.py`** (new): a zero-dependency (stdlib-only `urllib`) Python script that populates a demo user's accounts, categories, transactions, budgets, and goals through the live gateway API — real-style bank/card account names (Chase Total Checking, Ally Online Savings, American Express Gold Card), real-style merchant names per category (Whole Foods Market, Starbucks, Shell Gas Station, Netflix, etc.), biweekly paychecks, and a starting account `balance` set via a follow-up `PUT` (the create-account endpoint has no `balance` field, so without this every demo account would show `$0` despite 100+ transactions — caught while first running the script, not assumed). A couple of budget categories are deliberately set to go "over" so the Reports page's `BudgetBar` shows all three status colors and the overspend-notification trigger actually fires. `scripts/README.md` documents usage. **Verification: done, live, twice** — ran it against the actual running stack (not just read for correctness): first a throwaway verification user (85 transactions, 4 real notifications fired, budget-vs-actual numbers independently spot-checked against the raw transaction amounts by hand), then the real default `demo@example.com` user, confirming the account-balance fix specifically (`$0` before the fix, `$25,209.14` combined total after, matching the sum of the three individually-set balances).

### Phase 6 progress log (in progress — not complete)

- **2026-07-19 — Real-Mongo integration tests + a green CI matrix: done.** The user asked explicitly for test coverage usable in CI, which maps directly onto Phase 6's stated goal and the thing `CLAUDE.md` §7 had been deferring since Phase 0 ("real-Mongo integration tests are a Phase 6 concern... do not add live-database tests to the default local/CI test run before then"). This closes that specific piece; rate limiting, standardized pagination/error handling, and OpenAPI-served-live remain open (see `architecture/development-roadmap.md`'s Phase 6 entry).
  - **`shared/mongotest`** (new): a single shared helper (`StartClient(t) *mongo.Client`) that boots a real, disposable `mongo:7` container per test via `testcontainers-go`, torn down automatically via `t.Cleanup`. Pinned at testcontainers-go **v0.32.0**, deliberately not latest — confirmed via `go doc`/`go mod download` that v0.34.0+ requires Go 1.22, and this repo is pinned to Go 1.21.3 everywhere (the macOS-linker workaround in `docs/local-development.md` is specific to that exact version); upgrading the whole project's toolchain was judged a separate decision from "add integration tests," so v0.32.0 (the newest version still compatible with 1.21.3) was chosen instead. Proved the whole approach end-to-end in a throwaway scratch module before touching the real codebase — real container, real insert, real query, confirmed working under the pinned toolchain — before designing anything around it.
  - **Full `internal/repository/*_integration_test.go` suites added to all four Mongo-owning services** (`user-service`, `expense-service`, `budget-service`, `notification-service` — `gateway` owns no data, correctly excluded): CRUD round-trips against a real database, ownership scoping asserted against the actual Mongo query (not application code), and — the part a fake genuinely cannot cover — that `EnsureIndexes` creates indexes with the exact right shape: compound field order, sparse flags, uniqueness constraints that actually reject a duplicate insert, and (after the fix below) real TTL semantics. Built `expense-service`'s suite first as the reference pattern (proven working, 12/12 subtests passing in ~4.6s), then replicated the same shape directly across the other three services rather than delegating — by that point it was mechanical replication of an already-proven pattern, not open design work. All tagged `//go:build integration`, confirmed excluded from every service's default `go test ./...` (`internal/repository` shows `[no test files]` in the default run across all four).
  - **A real, previously undiscovered bug these tests caught:** `user-service`'s `EnsureIndexes` was missing two indexes `architecture/database-design.md` has documented since Phase 0 — a TTL index on `refresh_tokens.expires_at` and a plain index on `refresh_tokens.user_id`. The `user_id` index isn't cosmetic: `RevokeAllForUser` (the token-theft containment path from Phase 1's reuse-detection cascade) queries `{user_id, revoked: false}` on every replay-detection event, which was doing a full collection scan without it. Fixed in the same pass, verified live via the new tests (including, after a review-caught tightening — see below — asserting the TTL index's `expireAfterSeconds` is genuinely `0`, not just that an index with that name exists).
  - **A bug in the test code itself, caught and fixed before it shipped:** `notification-service`'s first draft of the "newest first" ordering test created two notifications without setting `CreatedAt` — the repository layer doesn't set it (only the service layer does), so calling the repository directly gave both documents Go's zero-value timestamp and made the sort-order assertion depend on undefined tie-break behavior. Ran it, watched it fail intermittently, diagnosed the real cause (not the product's `SetSort`, which is correct), fixed by setting explicit distinct timestamps.
  - **Root `Makefile`**: new `test-integration` target (loops `go test -tags=integration ./...` across the four Mongo-owning services), kept fully separate from `test` per `CLAUDE.md` §7. **`.github/workflows/ci.yml`** (new): three jobs — `go-unit` (matrix per service: gofmt/vet/build/test, mirrors `make test`), `go-integration` (matrix per Mongo-owning service, mirrors `make test-integration` — GitHub's `ubuntu-latest` runners ship Docker pre-installed, no extra setup needed), `frontend` (npm ci/build/lint). Go 1.21.3 and Node 20 pinned to match this repo's actual toolchain exactly (checked against `frontend/Dockerfile`'s `node:20-alpine`), not just "latest."
  - **Independent review caught four real, non-blocking gaps, all fixed in the same pass:** (1) `shared/go.mod` wasn't tidy — `mongotest.go`'s direct import of the testcontainers mongodb module was misclassified as indirect, and a transitive dependency (`github.com/kr/text`) was missing from `go.mod` entirely (present only via `go.sum`); fixed with `go mod tidy`. (2) `mongotest.go` itself had no build tag, so its "test-only, never compiled into a production binary" claim was enforced only by convention, not the compiler — added `//go:build integration` to the file itself (confirmed this doesn't break `shared`'s default `go build ./...`/`go test ./...`, which correctly just omits the now-fully-excluded package rather than erroring). (3) the TTL index assertion only checked that an index named `expires_at_1` existed, not that it actually had `expireAfterSeconds: 0` — tightened with a new `assertTTLIndexExists` helper that decodes and checks the real value, re-verified live. (4) no README (root or per-service) documented the new `make test-integration` tier or its Docker requirement — added a full **Testing** section to `docs/local-development.md`, a pointer from the root `README.md`, and a one-line addition to all four Mongo-owning services' READMEs (which also caught and fixed a second, independent instance of the same stale "docker-compose.yml does not exist yet" leftover line Phase 5 fixed once already in `budget-service`'s README — this one was in `user-service`'s).
  - **Verification: done, independently, from the repo root.** `make test` (unchanged, still fast, still fake-only) and `make test-integration` (new) both run clean end-to-end from `/`: all four services' integration suites pass against real Docker containers, ~15s total. `.github/workflows/ci.yml`'s YAML validated (parses correctly, three expected jobs present) — no `act`/`gh` CLI available in this environment for a full local dry-run, but every individual command the workflow runs (`gofmt`, `go vet`, `go build`, `go test`, `go test -tags=integration`, `npm ci`/`build`/`lint`) was independently confirmed working in this exact repo before being wired into the workflow.

- **2026-07-18 — CI trigger change, rate limiting, standardized pagination, OpenAPI served live: done.** User asked to "proceed with Phase 6" plus a separate, unrelated CI request: stop running tests on every direct push to `main`, run them only when a PR merges to `main`. That second part landed first and independently of the rest of this entry.
  - **CI trigger (`.github/workflows/ci.yml`):** changed from `on: push: branches: [main]` to `on: pull_request: types: [closed]; branches: [main]`, with every job gated on `if: github.event.pull_request.merged == true` and checking out `github.event.pull_request.merge_commit_sha` (not the PR head — the actual commit that landed on `main`). A closed-but-not-merged PR (abandoned/declined) now correctly runs nothing.
  - **Gateway-only rate limiting** (`shared/middleware/ratelimit.go`, new): in-memory per-client-IP token bucket via `golang.org/x/time/rate`, one `*rate.Limiter` per IP in a mutex-guarded map, stale entries swept every minute after 3min idle. Deliberately **fails open** (`requestsPerSecond <= 0 || burst <= 0` → pass-through, no-op) rather than blocking-by-default — first draft blocked by default and broke every existing gateway test whose `gwconfig.Config{}` zero-value left rate-limit fields unset, which would also have meant any service that forgot to set the new env vars silently 429s all its own traffic. Wired into the gateway only (`RATE_LIMIT_REQUESTS_PER_SECOND=10`, `RATE_LIMIT_BURST=20` in `.env.example`), same "gateway is the only place for cross-cutting HTTP concerns" rule `CLAUDE.md` already establishes for CORS. New error code `RATE_LIMITED` (429) added to `shared/httpx`.
  - **Standardized pagination contract**, applied to every paginated endpoint: `page`/`page_size` query params in, and critically, the **response echoes the service's resolved values, not the raw request** — a pre-existing bug in `expense-service`'s transaction list handler was caught in the process (it echoed raw `?page=` input, which is `0`/absent-parses-to-zero when the client omits it, instead of the actual page the service applied, and never returned `page_size` at all). Fixed by adding `Page`/`PageSize` fields to the domain page struct, having the service stamp the resolved values it actually used, and the handler echo those. Applied identically as new work to `notification-service` (previously unpaginated — `ListByUser` fetched every notification unbounded). Documented as "the standard" in `architecture/api-contracts.md`'s new Pagination section so the next paginated endpoint copies this shape instead of re-deriving it.
  - **OpenAPI specs served live** at `GET /openapi.yaml` on all 5 services (including gateway). New `shared/openapidoc` package: `Load(path, log)` reads the file from disk at startup (warns, doesn't crash, if missing — `/openapi.yaml` then 404s at `NOT_FOUND` rather than the process refusing to boot), `Handler(spec)` serves it with `Content-Type: application/yaml; charset=utf-8`. Disk-read-at-startup instead of `go:embed` because `go:embed` can't traverse `..` and every service's `openapi.yaml` lives at the service root while the Go code that would embed it is nested under `internal/` or `cmd/` — moving `openapi.yaml` into an embeddable location would have broken the documented per-service layout in `CLAUDE.md` §4 for no real benefit. Each service's `Dockerfile` gained a `COPY --from=builder .../openapi.yaml ./openapi.yaml` + `chown` step in the final stage. Built and verified live in `expense-service` first as the reference pattern, then replicated to `user-service`, `budget-service`, `notification-service`, and `gateway` (gateway's distinct `router.New(cfg, log, Backends, openapiSpec)` signature needed its own wiring, not a direct copy). All 5 rebuilt via `docker compose up --build -d` and confirmed live: every `/openapi.yaml` returns 200, the correct content-type, and real spec content; full stack re-verified healthy afterward (`/health`, a real login round-trip, rate limiting) with no regressions.
  - **A real, previously-corrupted-toolchain incident, caught and fixed:** `go get golang.org/x/time@latest` silently switched the active Go toolchain to 1.25 (that version requires it) and rewrote both `shared/go.mod`'s and the root `go.work`'s `go` directive to `1.25.0` in the process — this repo is pinned to 1.21.3 everywhere (see the Phase 6 integration-test entry above for why). Caught via `go version` reporting the wrong default, not via a build failure. Fixed with `go mod edit -go=1.21.3` and `go mod edit -require=golang.org/x/time@v0.10.0` (the newest version still compatible with 1.21.3, confirmed the same way testcontainers-go was pinned earlier), plus `go work edit -go=1.21.3` on the root workspace file. Lesson reinforced: verify a new dependency's Go-version requirement in a scratch module before running `go get` against the real repo, not after.
  - **A second dependency-resolution incident:** wiring `RateLimit` into `shared/middleware` (an existing single package alongside `cors.go` etc.) meant every service importing *any* symbol from that package now transitively needed `golang.org/x/time/rate` resolvable in its *own* `go.sum` — not just the gateway's, which is the only service that actually calls `RateLimit`. Docker builds for the other four services failed with `missing go.sum entry`. Fixed by running `go mod tidy` in all five services, not just the gateway.
  - **Verification: done.** `gofmt -l .`, `go vet ./...`, `go build ./...`, `go test ./...` clean in every one of the 5 services (run from inside each service directory). New tests: `shared/middleware/ratelimit_test.go` (burst exhaustion, independent per-IP budgets, zero-config fail-open), `services/gateway/internal/router/router_test.go`'s new `TestGateway_RateLimitAppliesThroughRealConfig` (real config values through the real router, not the middleware in isolation), `notification-service`'s pagination covered at both the service-fake level and a real-Mongo integration-test level (`ListByUser` with `Page`/`PageSize`, asserting the real total against a real query). Full stack rebuilt and live-verified end-to-end as described above, not just unit-tested.

- **2026-07-18 — Full request-validation coverage audit + fixes: done.** A read-only `finora-reviewer` audit of every handler binding tag against its service-layer validation, across all four Mongo-owning services (gateway skipped — it has no request bodies of its own), found three HIGH-severity gaps and several LOW ones. All fixed in the same session via two parallel `finora-go-service` agents on disjoint files, independently re-verified (diffs read, builds/tests re-run myself, then live-curled against a rebuilt Docker stack — not just trusted from the agents' self-reports).
  - **HIGH — unauthenticated body-size DoS on `POST /auth/register`:** no cap anywhere in the fleet on total JSON request body size, so an attacker could POST a multi-megabyte `password` field and force expensive bcrypt hashing before any auth check ran. Fixed with a new `shared/middleware/bodylimit.go` (`BodyLimit(maxBytes int64)`), same shape and same gateway-only reasoning as `RateLimit`: wraps `c.Request.Body` in `http.MaxBytesReader`, fails open (no-op) if `maxBytes <= 0` so an unset config can't silently disable it in a confusing way, and deliberately does no error interception of its own — the "http: request body too large" read error surfaces naturally through the existing `ShouldBindJSON` → `VALIDATION_ERROR`/400 path every handler already has. Wired into the gateway only via `MAX_REQUEST_BODY_BYTES=1048576` (1 MiB) in `.env.example`, inserted right after `Recovery` and before `CORS` in the middleware chain (as early as possible). Live-verified: a 2MB register body now returns `502` (rejected by the proxy before reaching the backend) instead of being processed.
  - **HIGH — `registerRequest.Password`/`Name` had no ceiling:** `Password` gained `max=72` (bcrypt's real, silent truncation limit — not an arbitrary number), `Name` gained `max=100` to match the pre-existing `maxNameLength` constant the update-profile path already enforced, so register and update-profile no longer disagree about what a valid name is.
  - **HIGH — `expense-service` transaction `category_id` had zero ownership check**, unlike `account_id` which already verified ownership via `accountRepo.GetByIDForUser`. A user could attach another user's category ID to their own transaction with no rejection. Fixed by adding the identical `GetByIDForUser(ctx, id, userID)` method to `domain.CategoryRepository` (mirroring `AccountRepository`'s existing method and Mongo implementation exactly, including returning `domain.ErrNotFound` — not a raw Mongo error — for both "missing" and "not yours"), wiring a `categoryRepo` dependency into `transactionService`, and checking it in both `Create` and `Update`. Live-verified end-to-end: registered two users through the gateway, had one create a category, had the other attempt a transaction referencing it — correctly rejected with `VALIDATION_ERROR`/"category does not exist" (same indistinguishable-404-shaped rejection as a genuinely nonexistent category, per the fleet's "cross-user access never leaks existence" convention).
  - **HIGH — `notification-service`'s `Type` field accepted any string unchecked**, including script tags or arbitrary garbage, with zero validation anywhere in the service layer. Deliberately **not** fixed with a closed `oneof=...` enum — `"overspend"` (from `budget-service`'s overspend trigger) is the only value any real caller sends today, and the field is intentionally meant to stay open for future notification producers, so a hardcoded list would just break the next legitimate type. Instead added a format constraint: a new `validNotifType` regex (`^[a-z0-9_]+$`, lowercase/digits/underscores only) in the service layer, plus a new `domain.ErrValidation` sentinel and `validationError`/`wrapValidation` helper (mirroring `budget-service`'s existing errors.Is-wrappable pattern) so the handler translates a rejection to `VALIDATION_ERROR`/400 correctly. Live-verified: `type: "overspend"` still succeeds (regression-checked — this is the real production value), `type: "<script>alert(1)</script>"` is correctly rejected.
  - **LOW, fixed across the fleet:** a `maxAmount = 1_000_000_000_000` (1e12) defense-in-depth ceiling (against float64 garbage/overflow in downstream aggregations, not a real spending limit) added next to every existing `amount <= 0`-style check in `expense-service` (transaction), `budget-service` (budget amount, goal target/current amount) — confirmed the same constant value and consistent error-message shape were used independently in both services, not silently drifted. Inverted `from`/`to` date ranges (`to` before `from`) now rejected as `VALIDATION_ERROR`/400 instead of silently returning an empty result, in both `expense-service`'s transaction list and `budget-service`'s `/reports/summary`. Free-text length caps added: `transaction.note` (`max=1000`), `notification.title`/`message` (`max=200`/`max=2000`).
  - **Verification: done, independently, twice.** Each of the two fix agents reported clean `gofmt`/`go vet`/`go build`/`go test` for the services they touched; every one of those commands was then re-run independently (not trusted from the agent report) across `shared/`, `gateway`, `user-service`, `expense-service`, `budget-service`, `notification-service` — all clean. Full stack rebuilt via `docker compose up --build -d` (all 5 backend services), all containers confirmed healthy, and every fix above was live-curled against the real running gateway (not just unit-tested) as described.

- **2026-07-18 — Closing conformance review over the whole Phase 6 round: done.** An independent `finora-reviewer` pass over the entire diff (CI trigger, rate limiting, body limit, pagination, OpenAPI serving, validation fixes, all together) found one real HIGH-severity bug and a handful of documentation gaps. All fixed and re-verified in this pass — not deferred.
  - **HIGH — `BodyLimit`'s own doc comment was wrong for the only route it actually protects.** The comment claimed an oversized body would surface through the existing `c.ShouldBindJSON` → `VALIDATION_ERROR`/400 path every handler already has. False for the gateway: the gateway never binds a body itself — every request (including the public `register`/`login` routes `BodyLimit` was written to protect) is forwarded byte-for-byte through `httputil.ReverseProxy`. An oversized body actually failed mid-proxy-copy as an `*http.MaxBytesError`, which the proxy's generic `ErrorHandler` (built for real backend-unreachable scenarios) couldn't distinguish from a genuine outage — so it reported `502 INTERNAL_ERROR`/"upstream service unavailable" instead of the intended `400 VALIDATION_ERROR`. Caught by the reviewer writing an actual probe request rather than trusting the doc comment or the existing test (which only asserted `StatusCode != 200`, so it passed despite reporting the wrong error entirely). **Real fix, not just a comment correction:** `services/gateway/internal/proxy/proxy.go`'s `ErrorHandler` now checks `errors.As(err, &http.MaxBytesError{})` and, when true, returns `400 VALIDATION_ERROR`/"request body too large" (logged at `Warn`, not `Error` — a client sending too much data isn't a backend incident and shouldn't pollute on-call ERROR-level noise) instead of falling through to the generic 502 branch. `router_test.go`'s `TestGateway_BodyLimitAppliesThroughRealConfig` tightened to assert the exact `400`/`VALIDATION_ERROR` envelope, not just "not 200" — the gap a looser assertion let through. `bodylimit.go`'s doc comment rewritten to describe what's actually true: BodyLimit today only ever triggers via the proxy path, and the proxy's `ErrorHandler` is what makes the failure mode correct, not `ShouldBindJSON`. Live-verified against the rebuilt gateway: a 2MB register body now returns `400`/`VALIDATION_ERROR`/"request body too large", not `502`.
  - **MEDIUM — `openapi.yaml` specs hadn't been updated for this round's new constraints** (Definition of Done §11): `user-service` (password `max=72`/name `max=100`), `expense-service` (transaction `amount` ceiling + `note` length cap), `budget-service` (budget/goal amount ceilings), `notification-service` (title/message length caps + the `type` field's format pattern). All four specs updated with matching `maxLength`/`maximum`/`pattern` constraints and short rationale comments; validated as parseable YAML afterward.
  - **MEDIUM — gateway README stale**: still described the middleware chain as `... -> CORS -> RateLimit`, missing `BodyLimit` entirely (inserted before `CORS`, not after it), and had no env var table row for `MAX_REQUEST_BODY_BYTES` — worse, on inspection neither did `RATE_LIMIT_REQUESTS_PER_SECOND`/`RATE_LIMIT_BURST` from the *previous* round, despite the reviewer's report assuming they were already there. Both gaps fixed in the same pass: corrected middleware-chain description, three new env var rows added.
  - **LOW — `shared/openapidoc` had no unit tests**, unlike every other new `shared/` package this phase (`ratelimit_test.go`, `bodylimit_test.go`). CLAUDE.md §7 requires `shared/` packages to carry their own tests. Added `openapidoc_test.go`: `Load` against a real temp-file fixture (success case) and a missing path (returns nil, doesn't panic), `Handler` against a real spec (200, correct content-type/body) and an empty spec (404 envelope) — mirroring `bodylimit_test.go`'s table/style conventions.
  - **Note, not a defect**: a stray untracked `architecture/finora-guide.html` (88KB) was flagged by the reviewer as sitting outside `CLAUDE.md`'s documented `.md`-only architecture-docs convention. Confirmed unrelated to this round's scope (pre-existing, not touched by any Phase 6 work) and left untouched — not this round's call to remove.
  - **Verification: done, independently, a third time.** Re-ran `gofmt -l .`/`go vet ./...`/`go build ./...`/`go test ./...` clean across `shared`, `gateway`, `user-service`, `expense-service`, `budget-service`, `notification-service` after every fix above. All four edited `openapi.yaml` files parsed successfully as YAML (Ruby's `YAML.load_file`, since no Python YAML module was available in this environment). Full stack rebuilt via `docker compose up --build -d` one final time; all 10 containers healthy; the `BodyLimit`/proxy fix re-verified live (2MB register body → `400`/`VALIDATION_ERROR`); all 5 `/openapi.yaml` endpoints still 200 with the updated specs; all 5 `/health` endpoints still `ok`.

**Phase 6 is done.** Integration tests, rate limiting, standardized pagination, full request-validation coverage (audited and fixed), OpenAPI specs served live, a green CI matrix gated on merge-to-main, and a closing conformance review with its one real finding (the `BodyLimit`/proxy `502`-vs-`400` bug) fixed and live-verified — all six of Phase 6's stated goals are met. Next session should confirm with the user whether to proceed to Phase 7 (async messaging via NATS) or revisit anything here first — don't assume either way.

### Post-Phase-6 fix: frontend UI polish (transactions/add-transaction and a fleet-wide pattern)

- **2026-07-20 — Transactions/add-transaction UI + fleet-wide component bug: done.** User specifically flagged the transactions page and "add transaction" form as ugly; asked for a full pass across all UI pages, not just that one. No headless browser tool was available by default, so this was set up from scratch: `playwright-core` installed to a scratch dir, driven against the system's real Google Chrome (`/Applications/Google Chrome.app`) headless, with a login script that authenticates once and saves `storageState` for reuse — every fix below was verified against real rendered screenshots, not inferred from reading JSX, per `CLAUDE.md`'s "if you can't test the UI, say so explicitly" rule (this time it actually could be tested).
  - **Root cause of the actual complaint, a real bug in `frontend/components/ui/Input.tsx`, not a taste issue:** the transactions page's "New transaction" form passed grid-placement classes (`lg:col-span-4` etc.) via the `Input`/`Select` components' single `className` prop, which applies only to the inner `<input>`/`<select>` element — not to the `Field` wrapper `<div>` one level up, which is the actual direct child of the `grid` container and therefore the real grid item. `col-span-*` on a non-grid-item element is a silent no-op, so every field defaulted to an implicit single grid track, all five fields collapsed to a cramped cluster at the left edge of the card with most of the row's width going completely unused, and native `<select>`/date `<input>` elements — which size to content when nothing stretches them — visibly truncated their own text (`Checki▾`, `Expens▾`, `07/2…`). This is invisible reading the JSX (`className="w-full lg:col-span-4"` looks correct) and only obvious once rendered — confirmed by taking a real screenshot before touching any code, not guessing from the source.
  - **Fixed at the component level, not per page:** `Input.tsx`/`Select.tsx` (shared by every CRUD page) gained a second prop, `wrapperClassName`, applied to the `Field` wrapper div; `className` keeps its existing meaning (inner-element styling — `tabular-nums`, an intentionally narrow `w-24`, etc.). Documented as a hard rule in `architecture/frontend-design-system.md` §5 so the next page copies the right pattern instead of reintroducing the bug.
  - **A second, independent, fleet-wide bug found in the same pass:** `Button.tsx`'s pill (`rounded-full`) buttons had no `whitespace-nowrap`, so whenever a surrounding flex/grid row left a button less width than its label wanted, the label wrapped to two lines and the pill rendered as a tall stacked oval instead of a pill — visible on Accounts ("Add\naccount"), Budgets ("Add\nbudget"), and Goals ("Add\ngoal") every time their toggle button was shown. Fixed once in `Button.tsx`'s base classes; verified fixed on all three pages via screenshot, not just the one page that prompted the audit.
  - **A third, structural inconsistency across the four CRUD pages (Accounts/Transactions/Budgets/Goals), found by comparing screenshots side by side:** Budgets/Goals already put the "+ Add"/"Cancel" toggle button in the page-level header row next to the `<h1>`; Accounts/Transactions instead buried it inside the "New X" card's own mini-header — two different conventions for the identical interaction, four pages that are otherwise structurally identical. Unified all four onto the Budgets/Goals pattern (page-header level) since it already matched `architecture/frontend-design-system.md` §7's own description of the intended pattern; the revealed card now carries just a plain `NEW ACCOUNT`/`NEW TRANSACTION` section label with no duplicate button. Rebalanced the transactions grid's column spans in the process (`lg:col-span` 3/2/2/2/3 for Account/Type/Amount/Currency/Date, summing to 12) and added a `placeholder="0.00"` to Amount, which had none.
  - **Verification: done, visually, not just via build/lint.** `npx tsc --noEmit` and `npm run lint` clean; full production Docker rebuild (`docker compose up --build -d frontend`) came up healthy. Screenshotted every one of the 13 frontend pages before touching any code to scope the actual problem set (only Accounts/Transactions/Budgets/Goals needed fixes — Overview, Reports, Notifications, Profile, Settings, Search, landing, login, register were already fine, confirmed by looking, not assumed). Re-screenshotted Transactions (closed + open + narrow 900px viewport) and Accounts/Budgets/Goals (open) after each fix. Created a real transaction end-to-end through the fixed form (gateway → expense-service → Mongo → back to the list) to confirm the presentation-only changes didn't touch any data-fetching/CRUD behavior — the row rendered correctly with the right badge, amount, and note. Also gave both Accounts' and Transactions' tables a `min-w-[640/720px]` (Budgets already had one) for consistency, though table cell wrapping at very narrow viewports is a minor pre-existing characteristic, not something this pass introduced or fully solved.
  - **Scope discipline:** no changes to `lib/api.ts`, `lib/auth-context.tsx`, any fetch logic, request/response shapes, or backend code — this was presentation-layer only, per `architecture/frontend-design-system.md` §8's standing rule for this class of work.

- **2026-07-20 — Follow-up fix: field height inconsistency across browsers, caught from a user-supplied screenshot.** The user pointed out that in their own browser, the date fields (From/To/Date) rendered visibly "deeper" than the Account/Category selects in the same row, despite the layout fix above. Measured both rendering engines directly rather than guessing: Chrome (via `playwright-core` against the system's real Chrome, the setup already built for the pass above) already rendered every field type — select, date, number, text — at an identical 38px; WebKit (Safari's engine; installed via the full `playwright` package's `webkit` browser for this specific check, since Chrome alone couldn't reproduce what the user was seeing) did not, confirming the user's screenshot was a real Safari-only rendering difference, not a misperception. Root cause: native `<input type="date">`/`<input type="number">` controls carry browser-supplied internal chrome (a calendar/stepper widget) that WebKit sizes taller than a `<select>` or text input even under identical Tailwind padding classes — padding alone doesn't constrain a native control's own affordance. Fixed by adding an explicit `h-10` to `Input.tsx`/`Select.tsx`'s shared `fieldClasses` (an explicit height overrides per-control intrinsic sizing, unlike padding) and stripping the native number-input spinner in `globals.css` (`::-webkit-inner/outer-spin-button` + `appearance: textfield`) since it was the same class of "one field type has chrome its neighbors don't" issue. Re-measured in WebKit after the fix: every field — select, date, number, text — now measures an identical 40px. Re-screenshotted Transactions/Accounts/Budgets/Goals in WebKit to confirm no regressions; `tsc`/lint clean; full Docker rebuild healthy.

- **2026-07-20 — Second follow-up: "New category" button misaligned — this time, a full fleet-wide sweep instead of a single-spot fix.** Third time the user flagged an alignment issue after believing the previous pass was thorough; explicitly asked for every such issue to be found in one pass, not reported piecemeal. Took this as a signal that "fix the one thing mentioned, screenshot a few other pages" wasn't rigorous enough — this pass instead **grepped the entire `frontend/app/dashboard` tree for every `items-end` flex row** (the layout pattern that puts a button beside a labeled field) and measured every single one's children with real `getBoundingClientRect()` calls, rather than fixing the one reported instance and eyeballing the rest.
  - Found 8 such rows total, only 1 of which (the flagged "New category" button) had been checked before: `transactions/page.tsx` (×2 — "New category" toggle, and "Add category" submit in the nested new-category form), `accounts/page.tsx`, `budgets/page.tsx`, `goals/page.tsx` (×2 — the main "Add goal" submit, and each per-goal card's "Update" button next to "Log progress"), `reports/page.tsx` ("Run report"), `search/page.tsx` ("Search"). **6 of the 8 had the exact same root-cause bug**, not just the one reported: `Button`'s `md` size computes its height from padding alone (≈36px) and `sm` from its own padding (≈30px), neither matches `Input`/`Select`'s explicit `h-10` (40px, set in the prior round's field-height fix) — bottom-aligned via `items-end`, every shorter button "floats" a few pixels short of the field beside it.
  - Fixed at the component level for the common case: `Button.tsx`'s `md` size class changed from `px-5 py-2 text-sm` to `h-10 px-5 text-sm` — an explicit height, matching `Input`/`Select` exactly, rather than letting padding produce an approximately-close-but-not-identical height. This alone fixed 4 of the 6 broken rows (Accounts/Budgets/Goals's main submit buttons, Reports, Search) with a single one-line change, since they all use the default `md` size. `sm` was deliberately left alone (still padding-derived, ≈30px) since it's also used in compact contexts genuinely meant to be smaller (table row action icons, pagination, pill toggles) where forcing `h-10` globally would make those look oversized — instead, the 3 specific `sm` buttons that sit in an `items-end` row beside a labeled field (transactions' "New category" toggle and "Add category" submit, goals' per-goal "Update") each got `className="h-10"` at the call site.
  - **Verification, done to close the loop this time, not just on the one flagged page:** measured all 8 rows' children via a single Playwright script asserting exact `top`/`bottom`/`height` in pixels (not just "looks aligned" from a screenshot) — every row now reports identical `height: 40` and identical `top`/`bottom` across every field and button in that row, in WebKit (Safari's engine, the one the user is actually using). Screenshotted every page in every relevant state (list, create-form open, edit-row active, per-goal progress row) in WebKit afterward as a final visual pass. `tsc`/lint clean; full Docker rebuild healthy.
  - **Process lesson, not just a code fix:** documented in memory that "fix the reported instance + screenshot a couple of other pages" is not an acceptable bar for this class of bug — the fix has to start from grepping for every instance of the *pattern* that caused it (here: `items-end` rows) and measuring each one, because the same component-level root cause reliably repeats across every page built from the same shared primitives. See `architecture/frontend-design-system.md` §5's `Button.tsx` entry for the permanent rule this establishes.

### New feature: CSV transaction import (e.g. credit card statements)

- **2026-07-20 — CSV import for transactions: done.** User asked for a way to import transactions from a CSV of a credit card statement — a new feature, not a bug fix, so this was designed from scratch (audited `transaction_service.go`/`transaction_handler.go`/`router.go` first to fit the existing Clean Architecture layering, rather than bolting something on).
  - **Design decisions, each made deliberately and documented in `architecture/api-contracts.md`'s new "CSV transaction import" section:** (1) `multipart/form-data`, not JSON — the one endpoint in the fleet that isn't JSON, since a file upload has no natural JSON shape. (2) Every imported row inherits the target account's `currency`; the CSV has no currency column, since one statement is always one currency. (3) Header matching is case-insensitive against a small, explicit alias list (`date`/`transaction date`/`posted date`; `amount`/`debit`; `description`/`merchant`/`payee`/`note`; `type`) — a deliberately bounded scope, not an attempt to auto-sniff every bank's export format. (4) If `type` is absent, income/expense is inferred from the amount's sign (negative = expense), matching how card issuers actually export purchases. (5) Amount parsing accepts `$`, thousands commas, and parenthesized negatives (`(45.00)`) on top of a plain signed number — real statement punctuation, not just `-45.00`. (6) **A bad row is skipped, never fatal to the rest of the file** — deliberately different from every other write endpoint in this app (which fully succeed or fully fail): the response is always `200` with `{imported, skipped, errors}` (errors capped at 20 entries, `skipped` always the true count, `row` numbered 1-indexed against the file's own lines with the header as row 1). (7) Capped at 5000 data rows per request — defense-in-depth, since each row is its own sequential repository `Create` call (no bulk-insert path), not a real-world limit.
  - **Backend (expense-service), Clean Architecture maintained throughout:** `domain.ImportRow`/`ImportRowError`/`ImportResult` (plain structs, no framework awareness) and `Import` added to the `TransactionService` interface (`internal/domain/transaction.go`). New `internal/service/transaction_import.go`: `Import` validates the account once up front (ownership + its currency — reusing the exact `accountRepo.GetByIDForUser` pattern the existing `Create`/`Update` already use for `account_id`/`category_id` ownership checks), then loops rows independently, building each into a `*domain.Transaction` via `buildImportedTransaction` and calling the existing `TransactionRepository.Create` per row — no new repository method needed. New `internal/handler/transaction_import.go`: `ImportCSV` parses the multipart form and hands the handler-layer CSV/header-alias parsing (`parseImportCSV`) off to the service as plain `[]domain.ImportRow`, mirroring how the existing `Create`/`Update` handlers already do their own JSON-adjacent parsing (`parseDate`) before calling the service with validated domain types — column parsing is I/O-boundary work like JSON binding, but turning raw strings into valid dates/amounts/types is business logic, so that split follows the codebase's own existing precedent exactly. New route `POST /api/v1/transactions/import` (registers cleanly alongside the existing `GET/PUT/DELETE /transactions/:id` routes — Gin trees are per-HTTP-method, so a POST-only static `/import` segment can't conflict with GET/PUT/DELETE's `:id` wildcard).
  - **A real bug caught only by an actual browser round-trip, not by any Go test:** `domain.ImportResult{}`'s zero-value `Errors` field is a nil slice, and `encoding/json` marshals a nil slice as `"errors":null` — but the frontend calls `importResult.errors.length` unconditionally, which throws on `null`. Every Go-level test passed (`len(nilSlice) == 0` is true for both nil and empty), so this was invisible until a real Playwright-driven file upload through the actual UI hit a CSV with **zero** bad rows and crashed the page with WebKit's own "This page couldn't load" error screen. Fixed by initializing `Errors: []domain.ImportRowError{}` instead of the zero value, matching this codebase's existing convention (`fakeAccountRepository.ListByUser`'s `make([]domain.Account, 0)`) of never letting a list-shaped field serialize as `null`. Added a regression test that actually marshals the result to JSON and asserts on the literal string `"errors":[]` — a Go-only `len()`/`nil` check would not have caught this, and would not catch a future regression either.
  - **Frontend:** `lib/api.ts`'s `rawFetch` now skips the default `Content-Type: application/json` header when the request body is a `FormData` instance (letting the browser set its own `multipart/form-data; boundary=...`) — every other call in the app still sends JSON, so this only changes behavior for the one new upload path. New `UploadIcon` (`components/icons.tsx`, same hand-rolled inline-SVG convention as every other icon). Transactions page gained a second header-level toggle button ("Import CSV", next to "Add transaction", mutually exclusive — opening one closes the other) revealing an account picker + file input + submit, using the `wrapperClassName`/`h-10` conventions already established for form alignment. Import result (imported/skipped/per-row errors) renders inline below the form.
  - **Verification: done at every layer.** `gofmt -l .`/`go vet ./...`/`go build ./...`/`go test ./...` clean in `expense-service` (new service-layer and handler-layer test files, table-driven, against fakes — no live Mongo). `tsc --noEmit`/`npm run lint` clean in `frontend`. Full Docker rebuild of `expense-service` and `frontend`, both healthy. Live-curled a realistic multi-row statement CSV (mixed good/bad rows, `$`/parenthesized/plain amounts, explicit-vs-inferred type, US-format dates) through the real gateway → expense-service → Mongo and confirmed every field (type inference, absolute-value amount, inherited currency, truncated note, correct skip/error reporting with accurate row numbers) — not just that the endpoint returned `200`. Confirmed the gateway's existing `MAX_REQUEST_BODY_BYTES` limit correctly rejects an oversized upload with `400 VALIDATION_ERROR` (not a crash), reusing Phase 6's `BodyLimit`/proxy fix with zero new code. Drove the actual browser UI with Playwright (file picker + form submit, not just `curl`) in WebKit, which is what caught the nil-slice bug above — re-verified clean after the fix, including a 150-row real import completing in well under a second.
  - **Documentation:** `services/expense-service/openapi.yaml` (new path, full request/response schema), `services/expense-service/README.md` (new endpoint row + a full "CSV import" subsection with the column-alias table), `architecture/api-contracts.md` (new "CSV transaction import" section explaining every design decision and why, per `CLAUDE.md` §6's "document what was rejected, not just what was picked" standard).

- **2026-07-20 — Follow-up fix: CSV import mislabeled every expense as income on a real Capital One statement.** User uploaded their actual `~/Downloads/capitalone.csv` and reported it. Read the real file first rather than guessing: it has separate `Debit`/`Credit` columns holding **unsigned** magnitudes (Capital One's real export shape), not a single signed `Amount` column. The original implementation's `importColumnAliases` treated `"debit"` as just another alias for the single `"amount"` slot, so a lone Debit column got sign-inferred exactly like a signed Amount column would — and since every Debit value in a real statement is positive, every purchase was inferred as income. This is precisely the "separate debit/credit columns" case explicitly called out and deliberately deferred in the original design note (documented at the time as a scope-limiting decision, not an oversight) — it turned out to be the *first* real file a user tried, not an edge case.
  - **Fixed properly, not patched around:** `domain.ImportRow` gained dedicated `Debit`/`Credit` fields (separate from `Amount`); the handler's column-alias map no longer treats `"debit"` as an `"amount"` alias — it's its own recognized column, alongside a new `"credit"` column, with `hasAmount`/`hasDebit`/`hasCredit` all checked so at least one shape is required; the service gained `resolveImportAmount`, which picks whichever shape a row's CSV actually has (Amount wins if both are somehow present; otherwise Debit present & nonzero → expense, else Credit present & nonzero → income), before the existing explicit-`Type`-column-override logic still applies on top.
  - **Verification used the user's real file directly, not a synthetic approximation:** added `TestTransactionService_Import_DebitCreditColumns_RealCapitalOneShape` (service) and `TestTransactionHandler_ImportCSV_DebitCreditColumnsParsedSeparatelyFromAmount` (handler) built from the actual file's shape (a Debit-only purchase row, the one real Credit-only payment row), then live-curled `~/Downloads/capitalone.csv` itself through the real gateway — all 54 purchases now `expense`, the 1 real payment now `income`. Re-ran the exact same file through the real browser UI (Playwright, real file picker) afterward and screenshotted the result table to visually confirm every row shows "Expense" — not just that the API returned the right JSON.
  - Updated the help text on the Transactions page (previously said "Amount"/"Debit" as if interchangeable — now correctly describes them as alternative shapes) plus `README.md`/`openapi.yaml`/`architecture/api-contracts.md`'s CSV import documentation to describe Debit/Credit as their own concept, including the story of why `debit` is deliberately not an alias of `amount` anymore.
  - **Verification: done.** `gofmt -l .`/`go vet ./...`/`go build ./...`/`go test ./...` clean; `tsc`/lint clean; full Docker rebuild of `expense-service` and `frontend`, both healthy.

- **2026-07-20 — Data correction: fixed the demo user's already-corrupted data from the pre-fix import.** After the Debit/Credit code fix above, the user asked to check the demo account, since their earlier real upload (before the fix existed) had already written 54 bad rows to the database — the code fix alone doesn't retroactively correct data written under the old bug. Identified the affected user precisely: `demo@example.com` (several other `Demo User`-named test accounts exist from earlier sessions; confirmed the right one by searching for the real file's distinctive merchant names — `SAFEWAY`, `COSTCO`, `GDP*Gateway`, etc. — across all users' transactions in Mongo directly, not by guessing). Cross-checked every one of this user's 5 accounts individually before touching anything: `Chase Total Checking` (income=8/expense=69, legitimate payroll deposits — untouched), `Ally Online Savings` (empty), `American Express Gold Card` (expense=68, no income at all — untouched), `Robinhood Gold Credit Card` (expense=1 — untouched), and `Capital One` (**income=54, expense=0** — the exact, unambiguous signature of the bug: this account had zero legitimate income transactions of its own, so every income row in it was demonstrably from the broken import, not mixed with real data that needed careful separation).
  - Fixed via a single scoped Mongo update (`updateMany({user_id, account_id: <Capital One's id>, type: 'income'}, {$set: {type: 'expense', updated_at: now}})`) — 54 matched, 54 modified. Deliberately scoped to that one user *and* that one account, not a blanket "fix all income transactions with these merchant names across the whole database," since the same merchant name appearing elsewhere for a different user/account should never be assumed to be the same bug.
  - **Verification:** re-queried Mongo immediately after (0 income / 54 expense in that account; the other 4 accounts' counts unchanged, confirming nothing else was touched), then logged into the actual demo account through the real browser (found the seed password in `scripts/seed_demo_data.py`, a dev-only, already-known default) and confirmed visually — every one of the 54 Capital One rows now shows "Expense" in the live UI, not just in a database query.
