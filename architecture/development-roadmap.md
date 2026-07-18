# Development Roadmap

This is the authoritative, phased path from an empty repository to a complete, production-quality Finora. Each phase is independently shippable and leaves the system in a running, healthy state — no phase depends on "finish everything, then integrate." This roadmap mirrors the "Implementation Plan" agreed for this build pass and, per that plan, is also appended to `plan.md` so there is a single source of truth for what's next.

Phases 7–9 intentionally build the integrations `plan.md`'s "Future Evolution" list defers (Redis, NATS/RabbitMQ, OpenTelemetry, Prometheus, Argo CD/Helm/Kubernetes-readiness). Earlier phases only leave structural seams for them — see `architecture/system-overview.md`'s Future Evolution table.

---

### Phase 0 — Foundation & scaffolding — ✅ COMPLETE (2026-07-17)

**Goal:** monorepo layout, the `shared/` library, `go.work`, docker-compose (5 services + 4 Mongos + frontend), all architecture docs, Kubernetes-style health probes, JSON logging, graceful shutdown, env-var configuration. A real **auth vertical slice** plus service skeletons that boot and pass health checks.

**Done when:** `docker compose up` brings every container to healthy; register → login → refresh → `/users/me` works end-to-end through the gateway; one protected downstream call works (proving gateway → service identity forwarding); the frontend dashboard shell loads; `make test` passes.

**Verified:** all of the above, plus a full browser-driven (Playwright) click-through of register → login → dashboard → logout with zero console errors. Three real bugs were caught only by this end-to-end verification and fixed — see `plan.md`'s Implementation Plan section and `docs/local-development.md`'s Troubleshooting section for details (a macOS Go-linker issue, a duplicate-CORS-header bug, and a Next.js standalone Docker healthcheck issue).

---

### Phase 1 — Auth & user domain hardening — ✅ COMPLETE (2026-07-18)

**Goal:** full profile + settings CRUD, refresh-token rotation/revocation, input validation everywhere, a password-reset stub, consistent error codes, and unit + handler tests brought to real coverage.

**Done when:** the user domain is feature-complete and validated end-to-end with tests.

**Verified:** password-reset stub, profile/settings validation (name trim+length, currency format, IANA timezone), and refresh-token reuse detection (replaying a rotated-away token revokes every other session for that user, not just the one request) were all implemented, unit-tested, and confirmed live through the running gateway — including the reuse-detection cascade tested against two real concurrent sessions, not just the in-memory fake. Full details and exact verification steps in `plan.md`'s Implementation Plan → Phase 1 progress log.

---

### Phase 2 — Expense domain depth — ✅ COMPLETE (2026-07-19)

**Goal:** full CRUD for accounts, transactions, income, expenses, and categories; owner-scoping on every query, pagination, filtering/sorting, the Mongo indexes described in `architecture/database-design.md`, and OpenAPI completeness for the service.

**Done when:** a user can manage real accounts/transactions through the gateway with pagination and validation.

**Verified:** an audit at the start of this phase found accounts (full CRUD), transactions (full CRUD, owner-scoped, real pagination + filtering by account/category/date range), and categories (create+list, intentionally minimal per its domain doc comment) were already implemented from earlier work. The one concrete gap was Mongo index creation — entirely absent from `expense-service`, unlike `user-service`'s `EnsureIndexes`. Added `services/expense-service/internal/repository/mongo.go` with an `EnsureIndexes` wired into `cmd/server/main.go` right after the Mongo connection, and confirmed **live** (not just compiled) via `docker compose exec mongo-expense mongosh finora_expenses --eval "db.<collection>.getIndexes()"`: `accounts` has `{user_id: 1}`; `transactions` has the compound `{user_id: 1, account_id: 1, date: 1}` and a sparse compound `{user_id: 1, category_id: 1}`; `categories` has `{user_id: 1}`. Also corrected a stale field name in `architecture/database-design.md` (`category` → `category_id`, matching what the code has actually used all along). Full details and exact commands run in `plan.md`'s Implementation Plan → Phase 2 progress log.

---

### Phase 3 — Budget domain depth — ✅ COMPLETE (2026-07-19)

**Goal:** budgets, savings goals, and **reports** — a cross-service aggregation where budget-service pulls expense data from expense-service via REST — plus budget-vs-actual calculations.

**Done when:** budgets, goals, and reports return real computed data across services.

**Verified:** goals expanded from create+list to full CRUD; reports rebuilt from a placeholder into a real budget-vs-actual computation via a new `budget-service` → `expense-service` REST call (`internal/client.ExpenseHTTPClient`, resolving a budget's category name to expense-service's category id and summing matching expense transactions in the requested range); Mongo indexes added for `budgets`/`goals` (budget-service had none before, same class of gap Phase 2 fixed in expense-service). Confirmed live end-to-end (not just unit tests): real transactions summed correctly into a report's `actual`/`remaining` figures, a category with no matching transactions correctly reports `actual: 0` rather than erroring, and cross-user access to goals returns 404 never 403.

By explicit user request, this pass also pulled forward a slice of **Phase 5** (frontend depth): the `accounts` and `transactions` pages (already fully backed since Phase 2, but still `ComingSoon` stubs) plus the new `budgets`/`goals`/`reports` pages were all wired to the live gateway API, so the whole budgets → transactions → real report loop can be exercised in the browser. `profile`/`settings`/`search` remain stubs — full Phase 5 frontend depth is still future work.

A conformance review caught two real bugs before this was called done, both fixed and covered by a new regression test/doc-check: (1) `goal_service.go`'s `Update` was re-validating "target_date must be in the future" against the resent value even when the caller wasn't changing the date — since `PUT` is a full-replace and also the only path for progress-logging (`current_amount`), this permanently locked any goal out of further updates once its due date passed; fixed to only reject a target_date that's both changed *and* in the past. (2) the frontend accounts page offered two account types (`investment`, `other`) that expense-service's `ValidAccountType` doesn't accept — guaranteed 400s; trimmed to the real four (`checking`/`savings`/`credit`/`cash`). Full details in `plan.md`'s Phase 3 progress log.

---

### Phase 4 — Notifications — ✅ COMPLETE (2026-07-19)

**Goal:** an in-app notification feed (create / list / mark-read), triggered by other services via REST (e.g. a budget-exceeded event from budget-service); an email provider behind an interface with a real implementation (SMTP or structured log).

**Done when:** a domain event (e.g. overspend) produces a visible notification.

**Verified:** an audit found create/list/mark-read and the `EmailSender` interface + structured-log implementation already existed from earlier scaffolding — the real gaps were (1) nothing ever called `POST /api/v1/notifications` (no trigger existed anywhere), (2) the email sender was constructed but never invoked, (3) no Mongo indexes. Closed all three: budget-service's `report_service.Summary()` now triggers a real overspend notification via a new `domain.NotificationClient` (same Dependency Inversion / `X-User-Id`-forwarding pattern as Phase 3's `ExpenseClient`), notification-service's `Create` now invokes `EmailSender.Send` as a best-effort side effect, and both services got `EnsureIndexes`. A conformance review caught a real dedup bug before this was called done — the original comparison (`LastNotifiedAt.Before(from)`, wall-clock time) could suppress a legitimate notification for an older, never-viewed over-budget period if a more recent period had already notified; fixed to exact-period-match (`LastNotifiedAt` now stores the period's `from` boundary, not a timestamp), with a regression test and live curl re-verification (notified for August, then confirmed a never-viewed April over-budget period still notified, proving the fix). Frontend: a new Notifications page (list, unread filter, mark-read) plus a sidebar unread-count badge, built with the design-system primitives from the frontend redesign — "visible notification" required an actual UI surface, not just a backend endpoint. Full verification detail (live curl transcripts, mongosh index output, email-sender log line) in `plan.md`'s Phase 4 progress log.

**Design note — the overspend trigger is read-triggered, not event-triggered, and that's deliberate.** Finora has no event bus yet (Phase 7 introduces one), so there is no "a transaction was created" domain event for expense-service to publish and notification-service to consume. The only place that already computes "is this budget currently over spent" is budget-service's `report_service.Summary()` (built in Phase 3, which already calls expense-service for real actual-spend-vs-budgeted). Phase 4 wires the overspend notification in at exactly that point: immediately after `Summary()` computes a category's `remaining` and finds it negative, budget-service calls notification-service over REST (see `architecture/api-contracts.md`'s budget-service → notification-service subsection for the full contract, the `LastNotifiedAt`-based dedup rule, and the failure-handling policy). This means `GET /api/v1/reports/summary` — a read endpoint — has a side effect (creating a notification, persisting `LastNotifiedAt`), which is not pure REST semantics. That trade-off is accepted only because there's no legitimate alternative today without building complexity (a bespoke poller/cron, or introducing async messaging early) that `CLAUDE.md`/`plan.md` explicitly say not to add before there's a real reason. **Phase 7 (async messaging seam) is exactly where this gets replaced**: a real overspend event, published when the triggering transaction is created and consumed by notification-service directly, with this read-triggered version removed.

---

### Phase 5 — Frontend depth — ✅ COMPLETE (2026-07-19)

**Goal:** wire every product screen (accounts, transactions, budgets, goals, reports, search, profile, settings) to live APIs; real forms, client state, charts, auth-guarded routing, and token-refresh handling.

**Done when:** all listed product features are usable through the UI.

**Verified:** accounts/transactions/budgets/goals/reports/notifications were wired incrementally across Phases 3-4 (pulled forward by explicit user request each time, since reports/budgets are meaningless without real data to point at); this pass closed the final three — profile (view/edit name, email read-only), settings (currency/timezone, backed by user-service's Phase 1 endpoints which already had sane defaults + full validation), and search (a client-side composition over the accounts/categories/transactions/budgets/goals endpoints — no dedicated search service exists or was warranted at this data scale). "Charts" is satisfied by the reports page's `BudgetBar` bullet-chart (built via the `dataviz` skill method during the frontend redesign), not a charting library — real data visualization without an unjustified new dependency. Token-refresh handling was already complete from Phase 0 (`lib/api.ts`'s silent-refresh-then-retry). A real, previously undiscovered production bug was caught while testing the new Settings page: `user-service`'s Alpine base image had no `tzdata` package, so `time.LoadLocation` could only ever resolve `"UTC"` inside the container — every genuinely valid IANA timezone was silently rejected with the same error a garbage string gets, invisible to `go test` since unit tests run on a host machine that has tzdata. Fixed in `services/user-service/Dockerfile`, confirmed live (a real zone now succeeds, garbage still correctly 400s), and documented in `docs/local-development.md`'s Troubleshooting section. Full detail in `plan.md`'s Phase 5 progress log.

---

### Phase 6 — Cross-cutting hardening — ✅ COMPLETE (started 2026-07-19)

**Goal:** integration tests using Mongo testcontainers, rate limiting, standardized pagination/error handling across all services, full request-validation coverage, complete OpenAPI specs served as docs, and a green CI matrix (lint/test/build).

**Done when:** CI is green and integration tests cover the critical flows.

**Progress so far:** the two pieces the user explicitly asked for first — integration tests and a working CI matrix — are done. `shared/mongotest` (new) starts a real, disposable `mongo:7` container per test via testcontainers-go, pinned at v0.32.0 (the newest version still compatible with this repo's pinned Go 1.21.3 — v0.34.0+ requires Go 1.22, and upgrading the toolchain project-wide is a separate decision from "add integration tests"). Every service that owns a MongoDB (`user-service`, `expense-service`, `budget-service`, `notification-service` — `gateway` owns no data) got a full `internal/repository/*_integration_test.go` suite, build-tagged `integration` so the default `go test ./...` stays exactly as fast/dependency-free as `CLAUDE.md` §7 requires — these are opt-in via `make test-integration` or CI's separate `go-integration` job. Every repository method is covered against a **real** database: CRUD round-trips, ownership scoping enforced at the actual Mongo query (not just application code), and — critically — that `EnsureIndexes` genuinely creates the documented indexes with the right shape (compound field order, sparse flags, uniqueness). `.github/workflows/ci.yml` (new) runs three jobs: `go-unit` (mirrors `make test`, matrixed per service), `go-integration` (mirrors `make test-integration`, real Docker containers — GitHub's `ubuntu-latest` runners ship Docker pre-installed, no extra setup needed), and `frontend` (build + lint). Full detail, including a real gap the integration tests caught (user-service's `refresh_tokens` collection was missing its documented TTL and `user_id` indexes since Phase 0 — fixed in the same pass), is in `plan.md`'s Phase 6 progress log.

**2026-07-18 update:** the CI trigger was deliberately changed from "every push/PR" to "only when a PR merges to `main`" (`on: pull_request: types: [closed]`, gated on `github.event.pull_request.merged == true`, checking out the real merge commit) — the user didn't want CI burning minutes on every direct push. In the same round: **rate limiting** (gateway-only, `golang.org/x/time/rate`, fails open on unset config, new `RATE_LIMITED` error code), **standardized pagination** (every paginated endpoint's response now echoes the service's *resolved* `page`/`page_size`, not raw request input — this caught and fixed a pre-existing bug in expense-service's transaction list handler, and was applied as new functionality to notification-service which was previously unpaginated), and **OpenAPI specs served live** at `GET /openapi.yaml` on all 5 services including the gateway (new `shared/openapidoc` package, disk-read-at-startup rather than `go:embed` since embedding can't reach a parent directory and every service's spec lives at its service root, not under `internal/`) are all done and live-verified against the real Docker stack. Full detail, including two dependency-pinning incidents (an `x/time` upgrade silently rewriting the pinned Go toolchain version, and a shared-package transitive-dependency gap that broke four services' Docker builds), is in `plan.md`'s Phase 6 progress log.

A **full request-validation coverage audit** (read-only `finora-reviewer` pass over every handler's binding tags against its service-layer validation) found three HIGH-severity gaps, all fixed and live-verified against the real Docker stack in the same round: an unauthenticated body-size DoS vector on `POST /auth/register` (fixed with a new gateway-only `shared/middleware/bodylimit.go`, same fail-open design as `RateLimit`), `expense-service` transactions accepting another user's `category_id` with zero ownership check (fixed by extending `CategoryRepository` with the same `GetByIDForUser` pattern `AccountRepository` already had), and `notification-service`'s `Type` field accepting arbitrary unvalidated strings (fixed with a format regex, deliberately not a closed enum, since the field is meant to stay open for future notification producers). Several LOW-severity gaps (missing amount ceilings, inverted date ranges, missing free-text length caps) were fixed alongside. Full detail is in `plan.md`'s Phase 6 progress log.

**Closing conformance review, 2026-07-18:** an independent `finora-reviewer` pass over the whole round's diff found one real HIGH-severity bug beyond the audit above — `BodyLimit`'s own doc comment was wrong about how oversized-body rejections actually surface at the gateway. The gateway forwards every request through `httputil.ReverseProxy` rather than ever calling `ShouldBindJSON` itself, so an oversized body failed mid-proxy and was misreported as `502 INTERNAL_ERROR`/"upstream service unavailable" (indistinguishable from a real backend outage) instead of the intended `400 VALIDATION_ERROR`. Fixed for real, not just in the comment: `services/gateway/internal/proxy/proxy.go`'s `ErrorHandler` now detects `*http.MaxBytesError` via `errors.As` and reports the correct `400`/`VALIDATION_ERROR` envelope; the existing test was tightened from "not 200" to asserting the exact status/error-code, which is what let the original bug through unnoticed. A handful of documentation-only gaps (stale `openapi.yaml` specs, a stale gateway README, missing `shared/openapidoc` unit tests) were fixed alongside. **Phase 6 is now fully done** — all six goals met, closing review's findings fixed and live-verified. Full detail in `plan.md`'s Phase 6 progress log.


---

### Phase 7 — Async messaging seam

**Goal:** introduce **NATS** for domain events, replacing synchronous REST notification calls with published events while keeping REST for queries. Outbox/idempotency patterns as needed to keep delivery reliable.

**Done when:** notifications are event-driven and services are decoupled for those flows.

---

### Phase 8 — Observability seam (application side)

**Goal:** OpenTelemetry tracing, a Prometheus `/metrics` endpoint per service, and log fields aligned for Loki ingestion. (Collectors, dashboards, and the observability stack itself are the infrastructure owner's responsibility — this phase is only the application-side emission of signals.)

**Done when:** traces and metrics are emitted and scrapeable, and logs are aggregation-ready.

---

### Phase 9 — Kubernetes readiness (application side)

**Goal:** finalize probe behavior and resource usage, strict 12-factor configuration, image hardening (minimal base images, non-root users, no build tooling in the final image), and graceful-shutdown timing tuned to real pod lifecycle events (`SIGTERM`, termination grace period). (Helm charts, manifests, and Argo CD application definitions are authored by the infrastructure owner, not this codebase.)

**Done when:** services are ready to be deployed to Kubernetes without further application changes.

---

## Notes

- Phases 0–6 are application-feature phases; Phases 7–9 are the deliberate, planned introduction of the integrations `plan.md` says to defer until there is a legitimate reason. That reason arrives exactly at these phases.
- Every phase leaves `docker compose up` (or its Phase-9 Kubernetes equivalent) as a working, healthy system — there is no phase where the system is expected to be broken in between.
- This roadmap is a living document: as phases complete, keep this file and `plan.md`'s mirrored section in sync, and record any material change of scope as a decision here with its rationale, per `CLAUDE.md`'s documentation requirements.
