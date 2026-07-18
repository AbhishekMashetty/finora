# Service Boundaries

This document defines what each service is responsible for, what data it owns, what it is explicitly forbidden from doing, and the rules governing any interaction between services. It is the enforceable complement to the one-line summaries in `CLAUDE.md` §3.

General rule, applying to every service below: **a service may only access data through its own MongoDB connection.** Any need to read another service's data must go through that service's REST API (`architecture/api-contracts.md`), never through a shared connection string, a shared collection, or a backdoor query. This is what keeps each service independently deployable, scalable, and replaceable.

---

## gateway

**Single responsibility:** public entrypoint. Terminates client connections, validates JWT access tokens, injects trusted identity headers, reverse-proxies to the owning service, handles CORS and request-id assignment.

**Data it owns:** none. The gateway is stateless and holds no MongoDB connection — its `/ready` check is liveness-only (see `architecture/api-contracts.md`, Health Endpoints).

**Explicitly forbidden from:**
- Containing business logic (validation beyond "is this a well-formed request," data transformation, computed fields). That belongs in the owning service.
- Talking to any MongoDB instance directly.
- Making routing decisions based on anything other than the URL path / documented route table.

**Allowed interactions:** proxies every request to exactly one downstream service based on path prefix. Never fans a single client request out to multiple services itself — if a response needs data from two services, that aggregation happens in one of the *services* (e.g. budget-service calling expense-service for reports), not in the gateway.

---

## user-service

**Single responsibility:** authentication and user identity — registration, login, token refresh/logout, profile, and settings.

**Data it owns (MongoDB `finora_users`):** `users`, `refresh_tokens`. See `architecture/database-design.md` for fields/indexes.

**Explicitly forbidden from:**
- Querying `finora_expenses`, `finora_budgets`, or `finora_notifications`.
- Issuing tokens on behalf of another service, or accepting a token-issuance request that didn't originate from its own `/auth/*` handlers.

**Allowed interactions:** none outbound today. `user-service` is a pure data owner — every other service only ever receives the user id via the gateway's `X-User-Id` header, never by calling back into `user-service`. If a future feature needs to display user profile data alongside another service's resource (e.g. "created by" on a shared budget), that's a documented REST call *into* user-service, added when the feature is built — not built preemptively.

---

## expense-service

**Single responsibility:** accounts, transactions, categories — the record of what money moved, where, and when.

**Data it owns (MongoDB `finora_expenses`):** `accounts`, `transactions`, `categories`.

**Explicitly forbidden from:**
- Querying another service's MongoDB (`finora_users`, `finora_budgets`, `finora_notifications`) under any circumstance.
- Returning another user's data — every query is scoped by `X-User-Id`; cross-user access returns `404 NOT_FOUND` (never `403`, per `architecture/api-contracts.md`, to avoid confirming a resource's existence to a non-owner).
- Computing budget-vs-actual comparisons or notification triggers itself — those are budget-service's and notification-service's jobs respectively; expense-service only ever reports its own raw data.

**Allowed interactions:** none outbound today. It is called *by* the gateway (client traffic) and, starting Phase 3, will be called *by* `budget-service` over REST for report aggregation (see below). It never initiates calls to other services.

---

## budget-service

**Single responsibility:** budgets, savings goals, and reports (budget-vs-actual, aggregate summaries).

**Data it owns (MongoDB `finora_budgets`):** `budgets`, `savings_goals`.

**Explicitly forbidden from:**
- Querying `finora_expenses` directly. Even though reports need transaction data, that data must be fetched via `expense-service`'s REST API, never via a shared/borrowed Mongo connection.
- Persisting a cached copy of expense-service's data as its own source of truth (a short-lived in-memory aggregation for a single report request is fine; a duplicated `transactions` collection in `finora_budgets` is not).

**Allowed interactions:** this is the one documented case of planned service-to-service REST traffic — **`budget-service` → `expense-service`**, for `GET /api/v1/reports/summary`, per Phase 3 of `architecture/development-roadmap.md`. Until Phase 3 ships, `/api/v1/reports/summary` is a stub that returns an empty/placeholder envelope; it must not silently start querying another service's database as a shortcut.

Today (Phase 0), no service actually calls another service — the only real cross-boundary traffic is the gateway proxying client requests to each service. This section documents the *first* sanctioned exception once it's built.

---

## notification-service

**Single responsibility:** in-app notifications — create, list, mark-read. Email delivery sits behind a provider interface (no-op/logging implementation today).

**Data it owns (MongoDB `finora_notifications`):** `notifications`.

**Explicitly forbidden from:**
- Querying any other service's MongoDB to decide what notifications to create. It receives notification content from the calling context (originating user's `X-User-Id`), it does not go looking for triggering conditions itself.
- Sending real email/SMS today — the provider interface exists so this can be implemented later (Phase 4) without changing callers.

**Allowed interactions:** receives creation requests via REST, forwarding the originating caller's `X-User-Id` as the owner (per `architecture/api-contracts.md`, notifications never take a `user_id` in the request body — this keeps the ownership model identical whether the caller is a human via the gateway or, starting Phase 4, another service like budget-service reacting to an overspend event). No outbound calls today.

---

## Summary Table

| Service | Owns (Mongo DB) | May call | May be called by |
|---|---|---|---|
| gateway | — | user-service, expense-service, budget-service, notification-service (proxy only) | client, frontend |
| user-service | `finora_users` | — (none today) | gateway |
| expense-service | `finora_expenses` | — (none today) | gateway; budget-service (Phase 3, reports) |
| budget-service | `finora_budgets` | expense-service (Phase 3, reports) | gateway |
| notification-service | `finora_notifications` | — (none today) | gateway; any service (Phase 4, event-triggered) |

Any interaction not in this table is out of bounds. If a feature seems to need one, it belongs in this document as a change to service boundaries — not as an ad-hoc shortcut in code.
