# CLAUDE.md — Finora Project Constitution

This file is the permanent source of truth for how Finora is built. It exists because `plan.md` requires it: *"Every future implementation must follow CLAUDE.md. Treat it as the project's constitution."* When this document and code disagree, the code is wrong — fix the code, or open a discussion to amend this file deliberately. Do not let them silently drift apart.

Finora has a dual purpose: it is a realistic personal-finance SaaS **and** a long-term Kubernetes/DevOps/platform-engineering learning platform (see `architecture/system-overview.md`). Every rule below is chosen to keep the application easy to containerize, deploy, scale, and operate — that is the standard every decision is measured against.

---

## 1. Coding Standards

- **Clean Architecture, strictly.** Dependencies point inward: `domain` has zero imports of Gin, Mongo driver, or any framework. `service` depends on `domain` interfaces, never on `repository` concrete types. `handler`/`router` are the only layers allowed to know about HTTP. `repository` is the only layer allowed to know about MongoDB.
- **SOLID.** In particular: repositories and services are defined as interfaces in `domain` and implemented elsewhere (Dependency Inversion); a handler does one job (parse/validate/respond), not business logic (Single Responsibility).
- **No clever code.** Optimize for the engineer reading this in two years, not for fewest lines. If a simpler, more explicit version exists, use it.
- **Every significant decision is documented.** If you choose a library, write down why in the relevant `architecture/*.md`. If you reject an alternative, write down why. This is not optional — `plan.md` mandates it, and it is the only way a DevOps-focused owner (not a software engineer) can reason about the app layer.
- **Errors are values, handled at the boundary.** Business logic returns typed domain errors; handlers translate them to the standard envelope error codes (see below). No panics for control flow — `shared/middleware.Recovery` exists as a safety net, not a design pattern.

## 2. Architecture Rules

- **Microservices, one responsibility each.** Five services today: `gateway`, `user-service`, `expense-service`, `budget-service`, `notification-service`. See `architecture/service-boundaries.md` for exact ownership.
- **One MongoDB per service. Never shared.** No service's code, config, or connection string may point at another service's database. This is physically enforced by giving each service its own Mongo container/URI (see `.env.example`, `architecture/database-design.md`).
- **REST for queries, async events for domain notifications (Phase 7).** Synchronous HTTP/JSON through documented contracts (`architecture/api-contracts.md`) remains the default for every request/response interaction, including the one cross-service query call in the fleet (budget-service → expense-service, computing report actuals). **NATS JetStream** is the one sanctioned exception: expense-service publishes `finora.transaction.created`, budget-service consumes it and publishes `finora.budget.overspent`, notification-service consumes that — replacing what used to be a synchronous REST call from budget-service to notification-service. Each publishing service writes to its own Mongo-backed transactional outbox (`shared/outbox`) in the same call path as the triggering write, and a background relay publishes to NATS with at-least-once delivery via durable JetStream consumers. See `architecture/api-contracts.md`'s Async Events section for the full contract. Do not add a second messaging pattern (RabbitMQ, Kafka, a second event bus) without a documented reason — one bus for the whole fleet is the standard.
- **The gateway is the only JWT validator.** Downstream services trust the `X-User-Id` header the gateway injects; they never parse tokens themselves today. This is a documented trade-off (see `architecture/api-contracts.md`, Auth Header Contract) revisited in Phase 9.
- **Every service is independently deployable.** Each has its own `go.mod`, `Dockerfile`, `Makefile`, health endpoints, and boots without any other service running (Mongo of its own excepted).

## 3. Service Boundaries

One-line summary per service (full detail in `architecture/service-boundaries.md`):

| Service | Responsibility |
|---|---|
| `gateway` | Public entrypoint; validates JWTs; reverse-proxies `/api/v1/*` to the owning service; owns no data. |
| `user-service` | Auth (register/login/refresh/logout), user profile, user settings. Owns users + refresh tokens. |
| `expense-service` | Accounts, transactions, categories. Owns all expense/income data. |
| `budget-service` | Budgets, savings goals, reports (future cross-service aggregation of expense data). |
| `notification-service` | In-app notifications; email provider stub behind an interface. |

A service must never reach into another service's database, import another service's internal packages, or assume another service's implementation details. The only sanctioned cross-service interaction is a documented REST call (today: none exist at the service-to-service level; the gateway proxying to each service is the only traffic. Phase 3 adds budget-service → expense-service REST calls for reports.)

## 4. Folder Conventions

Monorepo layout (full tree in `architecture/repository-structure.md`):

```
/
├── frontend/                 # Next.js app
├── services/
│   ├── gateway/
│   ├── user-service/
│   ├── expense-service/
│   ├── budget-service/
│   └── notification-service/
├── shared/                   # Go module: github.com/finora/shared
├── infrastructure/{docker,kubernetes,helm}/
├── .github/workflows/
├── architecture/
├── docs/
└── scripts/
```

Per-service layout (every Go service except the gateway):

```
services/<name>/
├── go.mod / go.sum
├── Dockerfile
├── Makefile
├── README.md
├── openapi.yaml
├── cmd/server/main.go
└── internal/
    ├── config/       # env loading + validation
    ├── domain/       # entities + repository/service interfaces, no framework imports
    ├── repository/   # MongoDB implementations of domain interfaces
    ├── service/      # use cases / business logic
    ├── handler/      # Gin handlers: parse, validate, call service, envelope response
    └── router/       # route registration + health wiring
```

The **gateway** owns no data, so it swaps `domain`/`repository`/`service` for its own concerns:

```
services/gateway/
└── internal/
    ├── config/    # env loading + validation
    ├── proxy/      # reverse-proxy to each downstream service
    ├── authmw/     # JWT validation middleware
    └── router/     # route table + health wiring
```

New services follow the same shape. Do not invent a new layout per service — consistency here is what makes the fleet learnable and operable at scale.

## 5. API Standards

The full contract lives in **`architecture/api-contracts.md`** — read it, don't duplicate it here. Summary of the non-negotiables:

- All routes versioned under `/api/v1/...`.
- Every response (success or error) uses the `shared/httpx` envelope: `{success, data, error, request_id}`. Services must call `httpx.Success`/`httpx.Fail` — never hand-roll a JSON response shape.
- Standard error codes: `VALIDATION_ERROR` (400), `UNAUTHORIZED` (401), `FORBIDDEN` (403), `NOT_FOUND` (404), `CONFLICT` (409), `INTERNAL_ERROR` (500).
- Every service exposes `/live`, `/ready`, `/health` via `shared/health`, wired to a real dependency check (Mongo ping) for `/ready`.
- All requests validated at the handler boundary before reaching the service layer.
- **CORS is applied only in the gateway**, never in a backend service. `httputil.ReverseProxy` copies backend response headers with `Header().Add`, so a backend also setting `Access-Control-Allow-Origin` produces a duplicate, comma-joined header value that browsers reject — this shipped once during Phase 0 and was only caught by real browser-based end-to-end testing (curl doesn't enforce CORS). Backend routers get `RequestID → Logging → Recovery` only.

## 6. Documentation Requirements

- Every significant architectural or library decision is written down with its rationale, in the relevant `architecture/*.md` file.
- Every rejected alternative is written down with why it lost. "We picked X" is incomplete; "we picked X over Y because Z" is the bar.
- Every service ships its own `README.md` (what it does, how to run it standalone, its env vars) and `openapi.yaml` (its actual HTTP surface).
- Documentation is updated in the same change that introduces the behavior it describes — not as a follow-up.

## 7. Testing Requirements

- Unit tests live at the **service (business logic) layer**, against fake/in-memory implementations of the `domain` repository interfaces — never against a live MongoDB. This keeps `go test ./...` fast and dependency-free for local dev and CI.
- Use **table-driven tests** (Go idiom): a slice of `{name, input, want, wantErr}` cases run through a `t.Run` loop, so new cases are one line, not a new function.
- Handler-layer tests may use `httptest` against the Gin router with a fake service, asserting on the envelope shape and status code.
- Real-Mongo integration tests are a **Phase 6** concern (testcontainers), explicitly deferred — do not add live-database tests to the default local/CI test run before then.
- `shared/` packages carry their own unit tests (see `shared/jwtx/jwtx_test.go` for the expected style: one test function per behavior, plain `testing`, no assertion framework needed for this scale).

## 8. Git Conventions

**Conventional Commits** (`type(scope): summary`, e.g. `feat(user-service): add refresh token rotation`) is the standard for this repo.

Why: the repo owner operates CI/CD and Argo CD and will eventually want automated changelogs and semantic-version-driven image tags per service; Conventional Commits is the de facto standard with the widest tooling support (commitlint, release-please, semantic-release) for exactly that automation, and it costs nothing to adopt from day one. Alternative considered: free-form messages with a good first line (simpler, no tooling to learn) — rejected because retrofitting structured commit history onto an existing repo later is far more painful than typing `feat(...)`/`fix(...)` now, and this repo is explicitly meant to grow CI/CD sophistication over time.

Types in use: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `ci`, `build`. Scope is typically the service name (`user-service`, `gateway`, `shared`, `infra`) or `docs`/`repo` for cross-cutting changes.

## 9. Naming Conventions

- **Go code:** standard Go idioms — `camelCase` for unexported, `PascalCase` for exported, package names short/lowercase/no underscores, errors as `ErrXxx` sentinel values or wrapped with `%w`. Follow `gofmt`/`go vet` without exception.
- **MongoDB collections:** `snake_case`, plural nouns — e.g. `users`, `refresh_tokens`, `accounts`, `transactions`, `categories`, `budgets`, `savings_goals`, `notifications`. See `architecture/database-design.md` for the full per-service list.
- **Environment variables:** `SCREAMING_SNAKE_CASE`, exactly as enumerated in `.env.example` (e.g. `JWT_ACCESS_SECRET`, `USER_SERVICE_MONGO_URI`, `GATEWAY_PORT`). Never invent a new env var name without adding it to `.env.example` in the same change.
- **HTTP routes:** lowercase, plural resource nouns, versioned — `/api/v1/accounts/:id`, per `architecture/api-contracts.md`.

## 10. Dependency Management

- Each service has its own `go.mod`. `shared/` is its own Go module (`github.com/finora/shared`).
- **Today:** every service's `go.mod` uses a `replace github.com/finora/shared => ../../shared` directive so each service builds standalone (important for isolated Docker build stages and for any agent building one service without the whole repo checked out).
- **Once all services exist:** a root `go.work` supersedes the per-service replace directives for local development — `go build ./...` from the repo root, shared IDE tooling (gopls resolves `shared/` without per-module hacks), and no risk of one service quietly pinning a stale `shared/` version relative to another. See `architecture/repository-structure.md` for the full trade-off discussion. Docker builds are unaffected either way — build context is always the repo root.
- Frontend dependencies via `npm` (Next.js, App Router, TypeScript), lockfile committed.
- Do not add a dependency (Go or npm) without a one-line justification in the relevant service's `README.md` or the PR description — every third-party package is something the DevOps owner may eventually need to patch, scan for CVEs, or license-audit.

## 11. Definition of Done

A feature, endpoint, or service change is **not done** until all of the following are true:

- [ ] Builds clean: `go build ./...` (or the service's `make build`) with no warnings from `go vet`.
- [ ] `gofmt -l` reports no unformatted files.
- [ ] Unit tests written (table-driven, against fakes) and passing: `make test` / `go test ./...`.
- [ ] Health checks unaffected or updated: `/live`, `/ready`, `/health` still behave per `shared/health` conventions; new dependencies registered as a `health.Checker` if they affect readiness.
- [ ] Response envelope and standard error codes used via `shared/httpx` — no hand-rolled JSON shapes.
- [ ] `openapi.yaml` updated to match the actual routes/schemas.
- [ ] Service `README.md` updated if behavior, env vars, or run instructions changed.
- [ ] No hardcoded configuration — new config values added to `.env.example` and read via `shared/config`.
- [ ] Structured logging used for anything worth observing (`shared/logger`, `shared/middleware.Logging`) — no `fmt.Println`/`log.Println`.
- [ ] The change is reviewed against this file (`CLAUDE.md`) — architecture rules, service boundaries, and naming conventions respected.
- [ ] Any new significant decision (library, pattern, trade-off) is documented in the relevant `architecture/*.md`, including why alternatives were rejected.

## 12. Claude Code Tooling

`.claude/skills/` and `.claude/agents/` hold project-specific automation, built from what was actually learned shipping Phase 0 — prefer these over rediscovering things from scratch:

- **Skills:** `run-finora-stack` (boot the full docker-compose stack, health-check it, curl the auth flow, browser-verify the frontend — encodes three real bugs already found and fixed once), `add-service` (scaffold a new microservice matching every existing convention), `add-endpoint` (extend an existing service through all its layers plus the Definition of Done).
- **Agents:** `finora-go-service` (backend Go work, pre-loaded with the `shared/` library API and architecture rules), `finora-frontend` (Next.js work, pre-loaded with the auth/token/API-wrapper conventions), `finora-reviewer` (read-only conformance review against this file).

Keep these in sync the same way as any other documentation here: when a convention changes, update the skill/agent file in the same change, not as a follow-up.

**Every future implementation must follow this file.**
