# user-service

Owns accounts, credentials, refresh tokens, profile and settings for Finora.
It is the **only** service that issues JWTs (`shared/jwtx`); every other
service trusts the `X-User-Id` header the gateway sets after verifying an
access token (see `architecture/api-contracts.md`).

## Endpoints

All routes are prefixed `/api/v1`. `PROTECTED` routes require the gateway to
have set `X-User-Id` (enforced here via `shared/middleware.RequireIdentity()`).

| Method | Path                        | Body                              | Response                                   | Auth      |
|--------|-----------------------------|------------------------------------|---------------------------------------------|-----------|
| POST   | `/auth/register`            | `{email, password, name}`          | `201 {user}`                                | public    |
| POST   | `/auth/login`               | `{email, password}`                | `200 {access_token, refresh_token, user}`   | public    |
| POST   | `/auth/refresh`             | `{refresh_token}`                  | `200 {access_token, refresh_token}`         | public    |
| POST   | `/auth/logout`              | `{refresh_token}`                  | `204`                                        | public*   |
| GET    | `/users/me`                 | —                                   | `200 {user}`                                | protected |
| PUT    | `/users/me`                 | `{name, ...}`                      | `200 {user}`                                | protected |
| GET    | `/users/me/settings`        | —                                   | `200 {settings}`                            | protected |
| PUT    | `/users/me/settings`        | `{currency, timezone, ...}`        | `200 {settings}`                            | protected |

\* `/auth/logout` authenticates via the refresh token in the body, not via
`X-User-Id`, so it deliberately does **not** sit behind `RequireIdentity()`.

Also exposes `GET /live`, `/ready`, `/health` (via `shared/health`).

Every response — success or error — uses the standard envelope from
`shared/httpx` (`{success, data, error, request_id}`). See
`architecture/api-contracts.md` for the full contract and error codes.

## Data model

- **User** — `{id, email, password_hash, name, created_at, updated_at}`,
  stored in the `users` collection. `password_hash` is never serialized to
  JSON (`json:"-"`). A unique index on `email` prevents duplicate accounts
  even under a concurrent registration race.
- **Settings** — `{user_id, currency, timezone, updated_at}`, stored in its
  **own** `settings` collection (keyed by `user_id`, unique index), rather
  than embedded in the user document. This was chosen so settings can be
  read/written independently of the account record (no partial-update
  gymnastics on the user doc, and the user document stays small). A fresh
  registration seeds default settings (`USD` / `UTC`) so `GET
  /users/me/settings` never 404s for a normal user; `PUT` merges only the
  fields provided, leaving the rest untouched.
- **RefreshToken** — `{id, user_id, jti_hash, expires_at, revoked,
  created_at}`, stored in `refresh_tokens` (unique index on `jti_hash`).
  Only the **SHA-256 hash** of the token's JTI is ever persisted — never the
  raw JTI or the raw token — so a database leak can't be used to forge a
  refresh token.

## Auth flows

- **Register**: validates email/password/name, rejects a duplicate email
  with `409 CONFLICT`, bcrypt-hashes the password (default cost), and seeds
  default settings.
- **Login**: generic `401 UNAUTHORIZED "invalid credentials"` for both an
  unknown email and a wrong password (never reveals which). On success,
  issues an access token + a refresh token (with a fresh JTI), and persists
  `sha256(jti)` with its expiry.
- **Refresh**: parses and validates the refresh token, looks up
  `sha256(jti)` — missing/revoked/expired all yield `401`. On success it
  **rotates**: the presented token's record is marked revoked and a brand
  new access+refresh pair (new JTI) is issued and persisted. The old
  refresh token can never be replayed after this. **Reuse detection**: if
  the presented token's record is found but is *already* revoked (as
  opposed to merely unknown or time-expired), that's treated as evidence of
  token theft/replay — the legitimate client already moved on to the next
  token in the rotation chain, so anyone still presenting the old one isn't
  it. The response is still just `401`, but server-side every refresh token
  belonging to that user is revoked (`RevokeAllForUser`), forcing full
  re-authentication on all devices/sessions, not just failing this one
  request.
- **Logout**: parses the refresh token and revokes its record. Idempotent —
  an already-revoked, expired, or entirely unknown token still returns
  `204`, so this endpoint never leaks whether a token was ever valid.

## Profile & settings validation

- `PUT /users/me` — `name` is trimmed of surrounding whitespace and must be
  non-blank after trimming (a whitespace-only name is rejected), and at
  most 100 characters. Violations return `400 VALIDATION_ERROR`.
- `PUT /users/me/settings` — `currency` and `timezone` are optional
  (omitted fields leave the existing value untouched), but when present are
  validated: `currency` must be exactly 3 uppercase ASCII letters (a
  lightweight format check, not a full ISO-4217 lookup table); `timezone`
  must be a real IANA timezone name (validated via Go's
  `time.LoadLocation`). Violations return `400 VALIDATION_ERROR`.

This validation lives in `internal/service` (not the handler), matching the
same service-layer-owns-business-validation convention used by
`expense-service` for its enum checks (account/transaction type) — see
`internal/service/errors.go`'s `ValidationError` type.

## Config (env vars)

| Var                       | Purpose                                    | Default (if optional) |
|----------------------------|---------------------------------------------|------------------------|
| `USER_SERVICE_PORT`        | HTTP bind port                              | `8081`                 |
| `USER_SERVICE_MONGO_URI`   | Mongo connection string (required)          | —                      |
| `JWT_ACCESS_SECRET`        | HS256 secret for access tokens (required)   | —                      |
| `JWT_REFRESH_SECRET`       | HS256 secret for refresh tokens (required)  | —                      |
| `JWT_ACCESS_TTL`           | Access token lifetime                       | `15m`                  |
| `JWT_REFRESH_TTL`          | Refresh token lifetime                      | `168h`                 |
| `LOG_LEVEL`                | `debug`/`info`/`warn`/`error`               | `info`                 |
| `SHUTDOWN_TIMEOUT`         | Graceful shutdown drain window              | `10s`                  |
| `CORS_ALLOWED_ORIGINS`     | Accepted for config-load compatibility but **unused** — CORS is applied only by the gateway (see `architecture/api-contracts.md`); a backend applying it too duplicates the header via the reverse proxy | `http://localhost:3000` |

Exact names match `.env.example` at the repo root — copy that file to `.env`
before running via docker-compose.

## Running standalone

```sh
cd services/user-service
export USER_SERVICE_MONGO_URI=mongodb://localhost:27017/finora_users
export JWT_ACCESS_SECRET=dev-access-secret-change-me
export JWT_REFRESH_SECRET=dev-refresh-secret-change-me
make run
```

Requires a reachable MongoDB — there is no local Mongo running natively on
a fresh dev machine; use a container (`docker run -p 27017:27017 mongo:7`)
or the root docker-compose stack described below.

```sh
make build   # go build ./...
make test    # go test ./...
make tidy    # go mod tidy
```

Normally this service is **not** run standalone — it's started via the
root `docker-compose.yml` (being built by another agent in parallel; it
does not exist yet), which wires up its dedicated `mongo-user` container
and the other Finora services.

## Docker

The build context must be the **repo root**, not this directory, because
the multi-stage build also needs `shared/`:

```sh
# from the repo root
docker build -f services/user-service/Dockerfile -t user-service .
```

or `make docker-build` from this directory (it points the context at the
repo root for you).

## Module setup

This service depends on `github.com/finora/shared` via a local `replace`
directive (`replace github.com/finora/shared => ../../shared`) so it builds
standalone today. A `go.work` workspace at the repo root will supersede
this during final cross-service integration.
