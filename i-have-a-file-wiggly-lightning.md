# Finora — Architecture + Runnable Skeleton (Pass 1)

## Context

`plan.md` is a project brief for **Finora**, a personal-finance SaaS whose *real* purpose is to be a long-term learning platform for Kubernetes / DevOps / platform engineering. The user is a **DevOps/Platform Engineer** who owns all infrastructure (Docker, K8s, Helm, CI/CD, observability, secrets, scaling). I own the **application** (Go/Gin microservices + Next.js frontend), and my job is to produce code that is easy to deploy and operate.

`plan.md` literally says "generate only the docs first, no app code." The user overrode that: they want this pass to deliver **the architecture docs AND a runnable local skeleton** that boots with `docker compose up` and passes health checks — because the near-term goal is "run local," and the eventual goal is orchestrating on Kubernetes (EC2/GKE).

**Confirmed decisions (from interview):**
- Deliverable = docs + runnable skeleton of all 5 services + frontend + MongoDB.
- Local run = `docker compose` primary; Makefile targets for dev. Compose topology designed to translate to K8s later.
- Auth = access + refresh tokens; **gateway validates JWT** and forwards identity (`X-User-Id`) to downstream services.
- Frontend = login/register + dashboard shell + stubbed nav.

**Goal of this pass:** a clean, well-documented monorepo that (a) fully documents the architecture and (b) actually runs end-to-end locally, with a real auth vertical slice and meaningful-but-minimal skeletons for the other services — designed so future infra (K8s, Helm, observability, messaging) drops in without app rewrites.

---

## Architectural Decisions (baked into the build)

- **Language/stack:** Go 1.23 + Gin per service; Next.js (App Router, TypeScript) frontend; MongoDB per service.
- **Structured logging:** Go stdlib **`log/slog`** (JSON handler) rather than zap — zero-dependency, production-adequate, easy to swap. Fields on every log line: `ts`, `level`, `service`, `request_id`, `msg`, `error` (when applicable). Designed so Loki/OTel can consume stdout later with no code change.
- **Database isolation:** **one MongoDB container per service** (`finora_users`, `finora_expenses`, `finora_budgets`, `finora_notifications`). This enforces "never share a DB" physically and maps 1:1 to a per-service StatefulSet in K8s later. *Trade-off:* heavier local footprint vs. a single instance with per-DB users; isolation + production realism win here. Documented in `database-design.md`.
- **Shared library:** a `shared/` Go module (`github.com/finora/shared`) for logger, HTTP response envelope, error types, middleware (request-id, logging, recovery, CORS), health handlers, config loader, graceful-shutdown server wrapper. Local dev uses a **`go.work`** workspace; Docker builds use **build-context = repo root** so each image can copy `shared/` + its service. *Trade-off documented* (workspace vs. published module) in `repository-structure.md`.
- **Auth:** access token (~15m) + refresh token (~7d, stored hashed in user-service Mongo for revocation). HMAC (`JWT_SECRET`) now, structured so asymmetric/JWKS can replace it later. **Gateway** validates the access token, injects `X-User-Id` + `X-Request-Id`; downstream services trust gateway-supplied identity on the internal network for now (documented; future = per-service JWKS validation).
- **API standard:** versioned `/api/v1/...`; consistent response envelope `{ "success", "data", "error", "request_id" }`; error shape `{ "code", "message", "details" }`; request validation at handler boundary.
- **Clean Architecture per service:** `cmd/server` → `internal/{config,domain,repository,service,handler,router,middleware}`. Domain has no framework/Mongo imports; dependencies point inward.
- **Config:** 100% env vars, no hardcoded values; `.env.example` at root; shaped for K8s ConfigMaps/Secrets.
- **Health:** `/live` (always 200 if process up), `/ready` (200 only when Mongo ping ok), `/health` (aggregate). Follows K8s probe conventions.

---

## Repository Structure to Create

```
/
├── CLAUDE.md                       # project constitution (standards, boundaries, DoD)
├── README.md                       # what/why + quickstart
├── docker-compose.yml              # gateway + 4 services + 4 mongos + frontend
├── Makefile                        # up/down/logs/build/test/tidy + per-service passthrough
├── .env.example                    # every env var with sane local defaults
├── .gitignore
├── go.work                         # local workspace tying shared + services
├── frontend/                       # Next.js (App Router, TS): auth + dashboard skeleton
├── services/
│   ├── gateway/                    # Gin reverse proxy + JWT validation + CORS + request-id
│   ├── user-service/               # auth (register/login/refresh/logout), users, profile, settings
│   ├── expense-service/            # accounts, transactions, income, expenses, categories
│   ├── budget-service/             # budgets, savings goals, reports
│   └── notification-service/       # in-app notifications (REST), email provider stub
├── shared/                         # Go module: logger, httpx, middleware, health, config, server
├── infrastructure/
│   ├── docker/                     # mongo init scripts if needed
│   ├── kubernetes/                 # .gitkeep placeholder (user-owned, later)
│   └── helm/                       # .gitkeep placeholder (user-owned, later)
├── .github/workflows/ci.yml        # starter CI (build+test services & frontend) — user owns/evolves
├── architecture/
│   ├── system-overview.md
│   ├── service-boundaries.md
│   ├── api-contracts.md
│   ├── database-design.md
│   ├── repository-structure.md
│   └── development-roadmap.md
├── docs/local-development.md
└── scripts/                        # helper scripts (e.g. wait-for-health, seed)
```

### Per-service internal layout (each Go service)
```
services/<name>/
├── go.mod / go.sum
├── Dockerfile                      # multi-stage, distroless/alpine, non-root, build ctx = repo root
├── Makefile                        # run / build / test / tidy
├── README.md
├── openapi.yaml                    # OpenAPI 3 spec for the service
├── cmd/server/main.go              # wire config→deps→router→graceful server
└── internal/
    ├── config/                     # env loading + validation
    ├── domain/                     # entities + repository/service interfaces (no framework imports)
    ├── repository/                 # MongoDB implementations
    ├── service/                    # use cases / business logic
    ├── handler/                    # Gin handlers (validation, envelope responses)
    ├── middleware/                 # service-local middleware (identity from gateway headers)
    └── router/                     # route registration + health wiring
```

---

## What actually runs in Pass 1 (scope of the skeleton)

- **user-service — real vertical slice:** `POST /api/v1/auth/register`, `/auth/login`, `/auth/refresh`, `/auth/logout`; `GET/PUT /api/v1/users/me`, settings. Password hashing (bcrypt), refresh-token persistence + rotation, JWT issuance.
- **gateway — real:** route table → services, JWT validation on protected routes, public routes (register/login/refresh) bypass auth, request-id + CORS, reverse proxy.
- **expense-service — meaningful skeleton:** real CRUD for **accounts** and **transactions** (owner-scoped by `X-User-Id`); categories/income/expense modeled in `domain` + `openapi.yaml`, wired minimally.
- **budget-service — meaningful skeleton:** real CRUD for **budgets**; savings goals + reports modeled and stubbed (reports documented as a future cross-service REST call to expense-service).
- **notification-service — meaningful skeleton:** real create/list **notifications** in Mongo; email provider behind an interface (no-op logging impl now).
- **frontend:** register + login pages (token handling), dashboard shell, stubbed nav (accounts, transactions, budgets, goals, reports, profile, settings) calling the gateway.
- **All services:** `/live`, `/ready`, `/health`, JSON logging, graceful shutdown, env config, at least one unit test at the service/handler layer (table-driven, mocked repos), `openapi.yaml`, Dockerfile, Makefile, README.

Every service **boots, connects to its Mongo, and passes health checks** — that's the definition of "runs" for this pass. Endpoints beyond the auth slice can be minimal but must return valid envelopes.

---

## Documentation to write (the docs plan.md asked for)

- **CLAUDE.md** — the constitution: coding standards, Clean Architecture rules, service boundaries, folder conventions, API/response standards, logging spec, testing requirements, Git conventions, naming, dependency management, and a **Definition of Done** checklist for every feature.
- **README.md** — product summary, architecture diagram (mermaid), quickstart (`cp .env.example .env && docker compose up`), service/port table.
- **architecture/system-overview.md** — goals (learning platform), high-level component + data-flow diagram, tech choices w/ rationale, future-evolution map (Redis/NATS/OTel/Prometheus/Loki/Argo CD/HPA/…).
- **architecture/service-boundaries.md** — each service's single responsibility, owned data, allowed/forbidden interactions, cross-service call rules.
- **architecture/api-contracts.md** — response envelope, error codes, versioning, per-service endpoint list, auth header contract (gateway → service).
- **architecture/database-design.md** — per-service DB rationale, collections + schemas + indexes, the "no shared DB" rule, isolation trade-offs.
- **architecture/repository-structure.md** — monorepo layout, Go module/`go.work`/Docker-context strategy and why, where new services go.
- **architecture/development-roadmap.md** — phased plan (Pass 1 skeleton → auth hardening → feature depth → async messaging → observability → K8s), so the user knows what's next.
- **docs/local-development.md** — prerequisites, `docker compose` workflow, per-service native run via Makefile, ports, health-check commands, sample curl flows (register→login→me), troubleshooting.

---

## Key reuse / consistency anchors (avoid duplication)

- All response/error formatting flows through `shared/httpx` — services never hand-roll JSON envelopes.
- All logging via `shared/logger` — no `fmt.Println`, no per-service logger setup.
- Request-id, recovery, CORS, logging middleware live once in `shared/middleware`.
- Health handlers + readiness probe hooks live once in `shared/health`; each service registers its Mongo ping as a readiness check.
- Graceful-shutdown HTTP server wrapper lives once in `shared/server`; every `main.go` uses it.

---

## Verification (how we confirm "runs local")

1. `cp .env.example .env`
2. `docker compose up --build` — all containers become healthy (compose `healthcheck` on each service hits `/ready`).
3. `scripts/wait-for-health` (or manual): `curl localhost:8080/health` (gateway) and each service's `/live` + `/ready` return 200.
4. Auth flow through the gateway:
   - `curl -X POST localhost:8080/api/v1/auth/register -d '{...}'` → 201
   - `curl -X POST localhost:8080/api/v1/auth/login -d '{...}'` → returns access + refresh tokens
   - `curl localhost:8080/api/v1/users/me -H "Authorization: Bearer <access>"` → 200 with user
   - `curl -X POST localhost:8080/api/v1/auth/refresh -d '{"refresh_token":"..."}'` → new token pair
5. One protected downstream call proves gateway→service identity forwarding: e.g. `POST /api/v1/accounts` then `GET /api/v1/accounts` returns only the caller's data.
6. Open `http://localhost:3000` → register → login → land on dashboard shell.
7. `make test` — unit tests pass across services.

Success = every container healthy + the auth flow + one protected downstream call + the frontend dashboard all work end-to-end via `docker compose up`.

---

## Implementation Plan — Roadmap to a Fully-Working Finora

The eventual target is a **complete, production-quality Finora**, not just the Pass-1 skeleton. On execution this phased roadmap will be **appended to `plan.md`** (as an "Implementation Plan" section) and mirrored in `architecture/development-roadmap.md`, so there is one authoritative, checkable path from empty repo → finished product. Each phase is independently shippable and leaves the system running.

**Phase 0 — Foundation & scaffolding (this pass):** monorepo layout, `shared/` library, `go.work`, docker-compose (5 services + 4 Mongos + frontend), all docs, K8s-style health probes, JSON logging, graceful shutdown, env config. Real **auth vertical slice** + service skeletons that boot and pass health checks.
*Done when:* `docker compose up` → all healthy; register→login→refresh→`/users/me` works; one protected downstream call works; frontend dashboard loads; `make test` passes.

**Phase 1 — Auth & user domain hardening:** full profile + settings CRUD, refresh-token rotation/revocation, input validation everywhere, password-reset stub, consistent error codes, unit + handler tests to real coverage.
*Done when:* user domain is feature-complete and validated end-to-end with tests.

**Phase 2 — Expense domain depth:** full CRUD for accounts, transactions, income, expenses, categories; owner-scoping, pagination, filtering/sorting, Mongo indexes, OpenAPI completeness.
*Done when:* a user can manage real accounts/transactions through the gateway with pagination + validation.

**Phase 3 — Budget domain depth:** budgets, savings goals, and **reports** (cross-service aggregation pulling expense data from expense-service via REST). Budget-vs-actual calculations.
*Done when:* budgets/goals/reports return real computed data across services.

**Phase 4 — Notifications:** in-app notification feed (create/list/mark-read), triggered by other services via REST (e.g., budget exceeded); email provider behind an interface with a real (SMTP/log) implementation.
*Done when:* a domain event (e.g., overspend) produces a visible notification.

**Phase 5 — Frontend depth:** wire every screen (accounts, transactions, budgets, goals, reports, search, profile, settings) to live APIs; forms, client state, charts, auth-guarded routing, token refresh handling.
*Done when:* all listed product features are usable through the UI.

**Phase 6 — Cross-cutting hardening:** integration tests (Mongo testcontainers), rate limiting, standardized pagination/errors, request validation coverage, complete OpenAPI specs + served docs, CI matrix (lint/test/build) green.
*Done when:* CI is green and integration tests cover the critical flows.

**Phase 7 — Async messaging seam:** introduce **NATS** for domain events (replace synchronous REST notification calls with published events), keeping REST for queries. Outbox/idempotency patterns as needed.
*Done when:* notifications are event-driven and services are decoupled for those flows.

**Phase 8 — Observability seam (app side):** OpenTelemetry tracing, Prometheus `/metrics`, log fields aligned for Loki. (Collectors/dashboards are the user's infra.)
*Done when:* traces + metrics are emitted and scrapeable; logs are aggregation-ready.

**Phase 9 — Kubernetes readiness (app side):** finalize probes/resource behavior, strict 12-factor config, image hardening, graceful-shutdown timing tuned for pod lifecycle. (User authors Helm/manifests/Argo CD.)
*Done when:* services are ready to be deployed to K8s without app changes.

*Note:* Phases 7–9 add integrations `plan.md` says to defer — they are **planned but not built** until their phase, and Pass 0 only leaves structural seams for them.

---

## Notes / boundaries

- I will **not** overwrite `plan.md`'s existing brief — during execution I will **append** the "Implementation Plan" section above to `plan.md` (and mirror it in `architecture/development-roadmap.md`). `CLAUDE.md` becomes the living constitution.
- Infra directories (`kubernetes/`, `helm/`) get placeholders only — those are the user's domain. The starter `.github/workflows/ci.yml` is a convenience the user owns and will evolve.
- No Redis/NATS/RabbitMQ/OTel/Prometheus code in this pass — only structural seams so they slot in later (documented in `development-roadmap.md`).
