# notification-service

Owns in-app notifications for Finora. Every route is scoped to the caller's
own data via the `X-User-Id` header the gateway sets after verifying the
caller's JWT (see `architecture/api-contracts.md` — Auth Header Contract).
This service never validates JWTs itself.

## Endpoints

All routes are prefixed `/api/v1` and require `X-User-Id` (enforced by
`shared/middleware.RequireIdentity`). There is no separate "system" ownership
model — even `POST` takes the owner from the header, not the request body.
As of Phase 4, budget-service's overspend-notification trigger (see
`architecture/api-contracts.md`'s budget-service → notification-service
subsection) is exactly such a service-to-service caller: it forwards the
originating user's `X-User-Id` rather than passing `user_id` in the body, so
the ownership model never has two shapes.

| Method | Path                              | Body                          | Response                    |
|--------|-----------------------------------|--------------------------------|------------------------------|
| GET    | `/api/v1/notifications`           | — (`?unread_only=true&page=&page_size=`) | `200 {notifications: [], page, page_size, total}` |
| POST   | `/api/v1/notifications`           | `{title, message, type}`       | `201 {notification}`         |
| PATCH  | `/api/v1/notifications/:id/read`  | —                              | `200 {notification}`         |

Health (public, no `X-User-Id` required):

| Method | Path      | Purpose                                             |
|--------|-----------|------------------------------------------------------|
| GET    | `/live`   | Liveness — always ok if the process is up            |
| GET    | `/ready`  | Readiness — ok only if the Mongo ping succeeds        |
| GET    | `/health` | Aggregate health payload for humans/dashboards        |
| GET    | `/openapi.yaml` | This service's spec, served live from disk      |

See `openapi.yaml` for the full request/response schemas and
`architecture/api-contracts.md` for the response envelope shape shared by
every Finora service.

## Domain model

```
Notification{ ID, UserID, Title, Message, Type, Read bool, CreatedAt }
```

Every query is filtered by `user_id == middleware.UserID(c)`. `GET
/notifications` only ever returns the caller's own notifications;
`?unread_only=true` additionally filters to `Read == false`.

**Pagination (Phase 6):** `?page=` (default 1) / `?page_size=` (default 20,
capped at 100) — the same standard contract every paginated Finora list
endpoint follows, see `architecture/api-contracts.md`'s Pagination section.
The response's `page`/`page_size` are the values the service actually
resolved and applied, not a raw echo of the query string (so they're never
0/0 just because the caller omitted them).

## Mongo indexes

`internal/repository/mongo.go`'s `EnsureIndexes` (wired into `cmd/server/main.go`
right after the Mongo connection, same idiom as the Phase 2/3 fixes in
expense-service/budget-service) creates a compound index on
`notifications` → `{user_id: 1, read: 1}`, since the feed's primary query is
"this user's unread notifications" (`GET /api/v1/notifications?unread_only=true`).
Safe to call on every boot — index creation is idempotent.

## Environment variables

| Variable                         | Meaning                                              | Example (see repo-root `.env.example`)          |
|-----------------------------------|-------------------------------------------------------|--------------------------------------------------|
| `NOTIFICATION_SERVICE_PORT`       | Port the HTTP server binds to (`0.0.0.0:<port>`)      | `8084`                                            |
| `NOTIFICATION_SERVICE_MONGO_URI`  | Mongo connection string                                | `mongodb://mongo-notification:27017/finora_notifications` |
| `LOG_LEVEL`                       | slog level (`debug`, `info`, `warn`, `error`)          | `info`                                            |
| `SHUTDOWN_TIMEOUT`                | Graceful shutdown drain timeout                        | `10s`                                              |
| `CORS_ALLOWED_ORIGINS`            | Accepted for config-load compatibility but **unused** — CORS is applied only by the gateway (see `architecture/api-contracts.md`); a backend applying it too duplicates the header via the reverse proxy | `http://localhost:3000` |

No JWT secrets are needed — this service trusts `X-User-Id` from the gateway.

## Running standalone

```sh
cd services/notification-service
export NOTIFICATION_SERVICE_MONGO_URI=mongodb://localhost:27020/finora_notifications
export NOTIFICATION_SERVICE_PORT=8084
make run
```

Requires a reachable MongoDB instance at `NOTIFICATION_SERVICE_MONGO_URI`.

Normally, though, this service is not run standalone — it's one container in
the root `docker-compose.yml` (being built by another agent in parallel, not
present in this repo yet), alongside its own `mongo-notification` instance and
the other Finora services, all reachable through the gateway.

### Makefile targets

```sh
make run           # go run ./cmd/server
make build         # go build -o bin/server ./cmd/server
make test          # go test ./...            (fast, fakes only)
make tidy          # go mod tidy
make docker-build  # builds the image (see note on build context below)
go test -tags=integration ./...  # real-Mongo integration tests (Phase 6,
                                  # requires Docker — see docs/local-
                                  # development.md's Testing section, or
                                  # `make test-integration` from the repo root)
```

### Docker build context

The `Dockerfile` `COPY`s the sibling `shared/` module, so it must be built
with the **repository root** as the build context, not this directory:

```sh
# from the repo root:
docker build -f services/notification-service/Dockerfile .
```

`make docker-build` does this for you (it `cd`s up to the repo root first).

## Testing

`internal/service` is unit-tested with table-driven tests against a
hand-written in-memory fake of `domain.NotificationRepository` and a fake
`domain.EmailSender` — no live Mongo, no testcontainers. `internal/handler`
is tested with `httptest`/`gin.TestMode` against a fake
`domain.NotificationService`. Coverage includes: create, list scoped to the
caller, the `unread_only` filter, mark-as-read (including the not-found and
cross-user cases), that `Create` invokes the email seam with the expected
`to`/`subject`/`body` (`TestNotificationService_Create`), and that a `Send`
error never fails `Create`'s own result
(`TestNotificationService_Create_EmailSendErrorDoesNotFailCreate`).

```sh
go test ./...
```

## Email-sending seam (wired in, as of Phase 4)

`internal/domain` defines:

```go
type EmailSender interface {
    Send(ctx context.Context, to, subject, body string) error
}
```

`internal/service.LoggingEmailSender` is the only implementation today — it
logs the would-be send via the shared `slog` logger instead of actually
sending anything, which is the roadmap's explicitly-accepted "structured
log" alternative to a real SMTP sender (`architecture/development-roadmap.md`
Phase 4). Since Phase 4, `notificationService.Create` calls `email.Send`
as a **best-effort** side effect after every successful notification
creation — a `Send` error is logged and swallowed, never propagated, since
the notification itself was already persisted successfully.

The `to` argument passed to `Send` is the notification's `userID` itself,
not a real resolved email address. This is a deliberate simplification:
resolving `userID` to a real address would require a new
notification-service → user-service REST call (a new cross-service edge,
new config, a new architecture doc section) purely to feed a log line that
`LoggingEmailSender.Send` discards without ever reading it. Revisit only
once a real SMTP-backed sender lands and the recipient address starts
actually mattering.

A real SMTP-backed implementation can be dropped in later without changing
`NewNotificationService`'s constructor signature.

## Clean architecture layout

```
cmd/server/main.go     - wires config -> mongo -> EnsureIndexes -> repositories -> services -> handlers -> router -> health -> server.Run
internal/config/       - env loading, wraps shared/config
internal/domain/       - plain structs + repository/service interfaces (no gin/mongo-driver imports)
internal/repository/   - MongoDB implementation of domain.NotificationRepository, plus EnsureIndexes
internal/service/      - business logic + the no-op LoggingEmailSender; depends only on domain interfaces
internal/handler/      - Gin handlers: bind/validate, call service, respond via httpx
internal/router/       - gin.Engine assembly, middleware order, route registration
```

## Middleware order

`RequestID() -> Logging(logger) -> Recovery(logger)`, then public health
routes, then `/api/v1` with `middleware.RequireIdentity()` applied to the
whole group. No CORS middleware here — CORS is applied only by the
gateway; see `architecture/api-contracts.md`.
