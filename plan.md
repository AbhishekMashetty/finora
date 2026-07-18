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
