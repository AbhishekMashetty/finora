# Repository Structure

Finora is a monorepo. This document is the ground truth for where things live, and explains the two build-tooling conventions (Go module wiring, Docker build context) that are easy to get wrong if copied from a typical single-service repo.

## Full Tree

```
/
├── CLAUDE.md                       # project constitution
├── README.md                       # what/why + quickstart
├── plan.md                         # original project brief
├── .env.example                    # every env var, with local defaults
├── .gitignore
├── docker-compose.yml              # gateway + 4 services + 4 mongos + frontend (final wiring step)
├── go.work                         # root workspace tying shared/ + all services together (supersedes per-service replace directives once present)
├── Makefile                        # up/down/logs/build/test/tidy + per-service passthrough
│
├── frontend/                       # Next.js (App Router, TypeScript)
│
├── services/
│   ├── gateway/                    # reverse proxy + JWT validation + CORS + request-id; owns no data
│   ├── user-service/               # auth, profile, settings — owns finora_users
│   ├── expense-service/            # accounts, transactions, categories — owns finora_expenses
│   ├── budget-service/             # budgets, goals, reports — owns finora_budgets
│   └── notification-service/       # notifications — owns finora_notifications
│
├── shared/                         # Go module: github.com/finora/shared
│   ├── logger/                     # slog JSON logger construction
│   ├── httpx/                      # response envelope + error codes
│   ├── middleware/                 # request-id, logging, recovery, CORS, identity
│   ├── health/                     # /live, /ready, /health handlers + Checker interface
│   ├── config/                     # env var helpers (MustGetEnv, GetEnv, GetEnvDuration, GetEnvInt)
│   ├── server/                     # graceful-shutdown HTTP server wrapper
│   ├── jwtx/                       # JWT generate/parse, shared claim shape
│   └── mongox/                     # Mongo connect helper + health.Checker adapter
│
├── infrastructure/
│   ├── docker/                     # mongo init scripts, etc. (app-owner-maintained helpers)
│   ├── kubernetes/                 # placeholder — owned by the infra maintainer
│   └── helm/                       # placeholder — owned by the infra maintainer
│
├── .github/
│   └── workflows/ci.yml            # starter CI (build+test services & frontend); infra owner evolves it
│
├── architecture/
│   ├── system-overview.md
│   ├── service-boundaries.md
│   ├── api-contracts.md
│   ├── database-design.md
│   ├── repository-structure.md     # this file
│   └── development-roadmap.md
│
├── docs/
│   └── local-development.md
│
└── scripts/                        # helper scripts (e.g. wait-for-health, seed data)
```

## Per-Service Layout

Every Go service (except the gateway) follows this exact layout — see `CLAUDE.md` §4 for the rule and rationale:

```
services/<name>/
├── go.mod / go.sum
├── Dockerfile
├── Makefile
├── README.md
├── openapi.yaml
├── cmd/server/main.go
└── internal/
    ├── config/
    ├── domain/
    ├── repository/
    ├── service/
    ├── handler/
    └── router/
```

The **gateway** differs because it owns no data — no `domain`/`repository`/`service` triad:

```
services/gateway/
├── go.mod / go.sum
├── Dockerfile
├── Makefile
├── README.md
├── openapi.yaml
├── cmd/server/main.go
└── internal/
    ├── config/
    ├── proxy/       # reverse-proxy to each downstream service
    ├── authmw/      # JWT validation middleware
    └── router/
```

Where a new service is added later, it goes under `services/<new-name>/` with this same shape — no exceptions, no per-service bespoke structure.

## Go Module Wiring: `replace` directive today, `go.work` supersedes it

**Today**, every service's `go.mod` contains:

```
replace github.com/finora/shared => ../../shared
```

This lets each service build **standalone** — `cd services/user-service && go build ./...` works with only that service and `shared/` checked out, which matters because:
- Docker build stages copy only `shared/` and one `services/<name>/` directory into the build context (see below) and must resolve the module graph with nothing else present.
- Services are being built by independent agents/contributors in parallel; each must be buildable/testable in isolation without the whole monorepo being finished.

**Once all services exist**, a root `go.work` file supersedes the per-service `replace` directives for local development:

```
go 1.21.3

use (
	./shared
	./services/gateway
	./services/user-service
	./services/expense-service
	./services/budget-service
	./services/notification-service
)
```

### Why workspace mode is better long-term

- **Single build/test command from the root:** `go build ./...` and `go test ./...` at the repo root cover every module at once — useful for a top-level `make test` and for CI running the whole fleet in one pass.
- **Consistent IDE tooling:** `gopls` resolves `shared/` imports correctly across every service without needing each editor window scoped to one service's directory.
- **No version drift across services:** with per-service `replace` directives only, it's possible (though discouraged) for one service to point at a different `shared/` revision than another if someone edits one `go.mod` and not the others. A `go.work` makes "everyone uses the `shared/` that's actually checked out" the only option during local development.

### Why the `replace` directive isn't simply deleted once `go.work` exists

`go.work` only affects **local** builds — it is never picked up inside a Docker build (each Dockerfile's `go build` runs in a container that doesn't have the workspace file's siblings guaranteed to be laid out the same way, and Docker layer caching benefits from each service's module graph being self-contained). The `replace` directive stays in each `go.mod` as the fallback that makes every service correctly buildable **outside** of workspace mode too — Docker images, a contributor who only clones one service's directory, CI matrix jobs that build one service at a time.

## Docker Build Context: repo root

Every service's Dockerfile needs to `COPY` both `shared/` and its own `services/<name>/` directory, because `shared/` is a real Go module the service imports, not a vendored copy. That means the Docker **build context** must be the repository root, not the service's own directory — otherwise `shared/` simply isn't visible to the build.

```bash
# Run from the repo root:
docker build -f services/user-service/Dockerfile .
```

This is **not** Docker Compose's default per-service behavior (Compose conventionally treats a service's own directory as both its build context and Dockerfile location). `docker-compose.yml` sets each service's build stanza explicitly to point the context at the root while keeping the Dockerfile path service-specific:

```yaml
services:
  user-service:
    build:
      context: .
      dockerfile: services/user-service/Dockerfile
```

Every service in `docker-compose.yml` follows this same pattern. When adding a new service, mirror this build stanza — a context scoped to `services/<name>/` will fail to build the moment the Dockerfile tries to `COPY ../../shared ./shared`.
