---
name: add-service
description: Use when scaffolding a brand-new Finora microservice (e.g. adding a 6th service beyond gateway/user/expense/budget/notification). Generates the exact Clean Architecture layout, wires it into shared/, go.work, docker-compose.yml, and .env.example, matching every existing service's conventions exactly so the fleet stays learnable and operable at scale (CLAUDE.md §2, §4).
---

# Adding a New Finora Microservice

Every service in this repo (`services/gateway`, `services/user-service`, `services/expense-service`, `services/budget-service`, `services/notification-service`) follows an identical shape. Read `services/notification-service/` first as the smallest complete reference implementation, and `architecture/repository-structure.md` for the rationale, before starting.

## 1. Go module

```bash
cd services/<new-service>
go mod init github.com/finora/<new-service>
```

Add to `go.mod`:
```
require github.com/finora/shared v0.0.0
replace github.com/finora/shared => ../../shared
```
Match dependency versions already used elsewhere to avoid drift: `github.com/gin-gonic/gin v1.10.0`, `go.mongodb.org/mongo-driver v1.17.1`. `go mod tidy` when done. `go 1.21.3` in the `go.mod` (matches the installed toolchain — see the run-finora-stack skill for why not 1.23+).

## 2. Clean Architecture layout (mandatory, identical across services)

```
services/<new-service>/
  go.mod / go.sum, Dockerfile, Makefile, README.md, openapi.yaml
  cmd/server/main.go     - wires config -> mongox.Connect -> repositories -> services -> handlers -> router -> health.Register -> server.Run
  internal/config/       - env loading, wraps shared/config (MustGetEnv for secrets/URIs with no sane default, GetEnv/GetEnvDuration/GetEnvInt for the rest)
  internal/domain/       - plain structs + repository/service interfaces. ZERO imports of gin/mongo-driver/any framework — this is what unit tests mock against
  internal/repository/   - MongoDB implementations of the domain interfaces
  internal/service/      - business logic, constructors take domain interfaces (not concrete Mongo types)
  internal/handler/      - Gin handlers: bind/validate (`c.ShouldBindJSON` + `binding:"required"` tags), call service, respond via shared/httpx. Never touch Mongo directly.
  internal/router/       - gin.Engine assembly
```

## 3. Use the shared library — don't reinvent any of this

- `shared/logger` → `logger.New(serviceName, levelName)` (`log/slog` JSON handler)
- `shared/httpx` → `httpx.Success(c, status, data)` / `httpx.Fail(c, status, code, message, details)`; error codes `httpx.CodeValidation/CodeUnauthorized/CodeForbidden/CodeNotFound/CodeConflict/CodeInternal`. Never hand-roll a JSON shape.
- `shared/middleware` → `RequestID()`, `Logging(logger)`, `Recovery(logger)`, `RequireIdentity()` + `UserID(c)` (reads the gateway-injected `X-User-Id`). **`CORS(...)` goes in the gateway only — never in a backend service** (see `run-finora-stack` skill and `architecture/api-contracts.md` for why: it causes a duplicate-header bug through the reverse proxy).
- `shared/health` → `health.Register(router, serviceName, checkers...)` wires `/live` `/ready` `/health`. Pass a `mongox.Checker{Client: client}` for `/ready`.
- `shared/server` → `server.Run(addr, handler, logger, shutdownTimeout)` — graceful SIGINT/SIGTERM shutdown, don't write your own.
- `shared/mongox` → `mongox.Connect(uri)`.
- `shared/jwtx` → only relevant if this service needs to parse/generate tokens (normally only user-service generates, only the gateway parses).

## 4. Middleware order (router.go)

```
RequestID() -> Logging(logger) -> Recovery(logger) -> [health routes, public] -> /api/v1 group
```
Apply `RequireIdentity()` to the whole `/api/v1` group if every route is owner-scoped (the common case), or to a sub-group if the service also has public routes (user-service's pattern: `/auth/*` public, `/users/*` protected).

## 5. Ownership rule

Every query filters by the caller's id (`middleware.UserID(c)`). A resource that exists but belongs to another user returns **404 NOT_FOUND, never 403** — don't leak existence.

## 6. Dockerfile (multi-stage, build context = REPO ROOT)

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY shared/ ./shared/
COPY services/<new-service>/ ./services/<new-service>/
WORKDIR /app/services/<new-service>
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server

FROM alpine:3.19
RUN adduser -D -u 10001 appuser
COPY --from=builder /out/server /usr/local/bin/server
USER appuser
EXPOSE <port>
HEALTHCHECK --interval=10s --timeout=3s --retries=5 CMD wget -qO- http://localhost:<port>/live || exit 1
ENTRYPOINT ["/usr/local/bin/server"]
```
Document in the README that this must be built with repo root as context: `docker build -f services/<new-service>/Dockerfile .`

## 7. Testing

Table-driven unit tests in `internal/service` against a hand-written fake implementing the domain repository interface — no live Mongo, no testcontainers (that's a Phase 6 concern, deferred). At least one `internal/handler` test via `httptest` + `gin.SetMode(gin.TestMode)` with a fake service.

## 8. Wire it into the rest of the repo (easy to forget any one of these)

- [ ] Add `./services/<new-service>` to `go.work`'s `use (...)` block
- [ ] Add a `mongo-<new-service>` container + the service itself to `docker-compose.yml` (copy an existing service's block — healthcheck, `depends_on: condition: service_healthy`, named volume)
- [ ] Add its env vars (`<NAME>_SERVICE_PORT`, `<NAME>_SERVICE_MONGO_URI`, plus `<NAME>_SERVICE_URL` in the **gateway's** env block) to `.env.example`
- [ ] Add its routes to the gateway's routing table (`services/gateway/internal/router/router.go` and/or `internal/proxy`) and to `architecture/api-contracts.md`'s Per-Service Endpoints + Ports table
- [ ] Add a one-line responsibility summary to `CLAUDE.md` §3 and a full section in `architecture/service-boundaries.md`
- [ ] Run the full CLAUDE.md §11 Definition of Done checklist before calling it finished

For adding a single endpoint to an *existing* service rather than a whole new service, use the `add-endpoint` skill instead.
