# Database Design

## The Rule: One MongoDB Per Service, Never Shared

Every service connects to its own MongoDB instance via its own `*_MONGO_URI` env var (see `.env.example`):

| Service | Mongo URI (docker-compose) | Database name |
|---|---|---|
| user-service | `mongodb://mongo-user:27017/finora_users` | `finora_users` |
| expense-service | `mongodb://mongo-expense:27017/finora_expenses` | `finora_expenses` |
| budget-service | `mongodb://mongo-budget:27017/finora_budgets` | `finora_budgets` |
| notification-service | `mongodb://mongo-notification:27017/finora_notifications` | `finora_notifications` |

No service's connection string, code path, or configuration may point at another service's database. This is enforced structurally (each service is only ever given its own URI), not just by convention.

### Why isolation won

- **Blast-radius containment.** A slow query, lock contention, index-building job, or outright crash in one service's Mongo cannot affect another service's availability or latency. In a shared-instance model, a runaway query in expense-service could starve user-service's login path.
- **Independent schema evolution.** Each service's collections change on that service's release cadence. There's no coordination cost or migration lockstep across teams/services — a document shape change in `transactions` has zero blast radius on `users`.
- **Independent scaling.** `finora_expenses` (likely the highest write volume — every transaction) can be given more resources, sharded, or moved to a bigger instance without touching the other three databases.
- **1:1 mapping to Kubernetes later.** Each service's Mongo becomes its own `StatefulSet` + `PersistentVolumeClaim` + `Service` in Kubernetes, with its own resource requests/limits, its own backup schedule, and its own access policy (NetworkPolicy restricting it to exactly one caller). This mapping is exactly why the local docker-compose topology already gives each service a dedicated Mongo container rather than a single one.
- **Credential/security boundary.** Compromise of one service's Mongo credentials never grants access to another service's data, because there is no shared instance to pivot to.

### The trade-off (and why it's accepted)

A single shared MongoDB instance with one database (or one set of scoped credentials) per service is cheaper to run locally: one container, less memory, fewer moving parts to keep healthy in `docker compose up`. This project explicitly accepts the heavier local footprint (four Mongo containers instead of one) because:

1. The isolation is *physical*, not just a permissions convention someone could misconfigure or bypass — and misconfiguration is exactly the kind of mistake that's cheap to make and expensive to detect in a shared instance.
2. This repo's actual purpose is practicing production-shaped systems (see `architecture/system-overview.md`) — running four Mongo containers locally is a feature here, not a cost to be optimized away, because it's what the Kubernetes deployment will also look like.
3. Local resource usage is a solved problem (more RAM, or scale down replicas) whereas retrofitting isolation onto a shared-instance system after data has commingled is a real migration project.

## Per-Service Schema

Field lists below are the essential/known fields as of this pass; services may add fields as features land (Phase 1+ in `architecture/development-roadmap.md`) — additions don't change the isolation model.

### user-service — `finora_users`

**`users`**
| Field | Type | Notes |
|---|---|---|
| `_id` | ObjectId | |
| `email` | string | unique |
| `password_hash` | string | bcrypt |
| `name` | string | |
| `created_at` / `updated_at` | time | |
| `settings` | embedded doc or separate collection | currency, timezone, etc. (see `architecture/api-contracts.md`, `/users/me/settings`) |

Indexes:
- **Unique index on `email`** — enforces one account per email at the database layer (not just application validation), and backs the login lookup.

**`refresh_tokens`**
| Field | Type | Notes |
|---|---|---|
| `_id` | ObjectId | |
| `user_id` | ObjectId/string | references `users._id` |
| `jti_hash` | string | hash of the token's `jti` claim — the raw `jti` is never stored, only a hash, so a leaked database dump doesn't hand out valid revocation lookups |
| `expires_at` | time | mirrors the JWT's `exp` |
| `revoked_at` | time, nullable | set on logout/rotation |
| `created_at` | time | |

Indexes:
- Index on `jti_hash` — the lookup path for refresh/logout/revocation checks.
- TTL index on `expires_at` (Mongo `expireAfterSeconds: 0` on a date field) — expired refresh tokens are automatically reaped by MongoDB rather than needing a cleanup job, keeping the collection bounded.
- Index on `user_id` — supports "revoke all sessions for this user."

### expense-service — `finora_expenses`

**`accounts`**
| Field | Type | Notes |
|---|---|---|
| `_id` | ObjectId | |
| `user_id` | string | owner, from `X-User-Id` |
| `name` | string | |
| `type` | string | e.g. checking, savings, credit |
| `currency` | string | ISO 4217 |
| `created_at` / `updated_at` | time | |

Indexes:
- Index on `user_id` — every list/lookup is owner-scoped.

**`transactions`**
| Field | Type | Notes |
|---|---|---|
| `_id` | ObjectId | |
| `user_id` | string | owner |
| `account_id` | ObjectId | references `accounts._id` |
| `category_id` | string, nullable | references `categories._id`; optional (a transaction may be uncategorized) |
| `amount` | decimal/number | |
| `type` | string | income / expense |
| `date` | time | |
| `note` | string | |
| `created_at` / `updated_at` | time | |

Indexes:
- **Compound index on `(user_id, account_id, date)`** — the transaction list endpoint filters by account and paginates/sorts by date within a single owner's data; this compound index serves that access pattern directly instead of requiring a collection scan or in-memory sort.
- Compound index on `(user_id, category_id)`, sparse — supports category-filtered listing and future report aggregation; sparse because `category_id` is optional and most documents may omit it.

**`categories`** (Phase 2)
| Field | Type | Notes |
|---|---|---|
| `_id` | ObjectId | |
| `user_id` | string | owner (or null for a shared system default set) |
| `name` | string | |
| `type` | string | income / expense |

Indexes:
- Index on `user_id`.

### budget-service — `finora_budgets`

**`budgets`**
| Field | Type | Notes |
|---|---|---|
| `_id` | ObjectId | |
| `user_id` | string | owner |
| `category` | string | matches expense-service's category naming |
| `amount` | decimal/number | budgeted limit |
| `period` | string | e.g. monthly |
| `created_at` / `updated_at` | time | |

Indexes:
- Index on `user_id`.
- Compound index on `(user_id, period)` — the budget list access pattern is "this user's budgets, often narrowed to one period" (e.g. reports summing only monthly budgets), so this is now built rather than speculative (Phase 3).

**`goals`** (Phase 3 — full CRUD)
| Field | Type | Notes |
|---|---|---|
| `_id` | ObjectId | |
| `user_id` | string | owner |
| `name` | string | |
| `target_amount` | decimal/number | |
| `target_date` | time | |
| `current_amount` | decimal/number | manually updated by the user via `PUT` (progress logging) — never derived from expense-service; goals do not get the cross-service linkage reports get |
| `created_at` / `updated_at` | time | |

Indexes:
- Index on `user_id`.

**Note on the collection name:** this table was originally sketched as `savings_goals`, but `services/budget-service/internal/repository/mongo_goal_repository.go` has always used `db.Collection("goals")` — verified directly against the code during Phase 3 rather than trusting this doc (the same kind of doc/code drift Phase 2 caught for expense-service's `category_id` field). The collection was **not** renamed to match the doc; the doc is corrected to match the code, since there's no compelling reason to churn a working collection name and no data yet to migrate.

Reports (`/api/v1/reports/summary`) are computed on read from `budgets` plus a real REST call to expense-service (`GET /api/v1/categories` then `GET /api/v1/transactions?category=<id>&...`, see `architecture/api-contracts.md`'s budget-service → expense-service subsection) — not a persisted collection.

### notification-service — `finora_notifications`

**`notifications`**
| Field | Type | Notes |
|---|---|---|
| `_id` | ObjectId | |
| `user_id` | string | owner, from `X-User-Id` |
| `title` | string | |
| `message` | string | |
| `type` | string | e.g. info, warning, overspend |
| `read` | bool | |
| `created_at` | time | |

Indexes:
- **Compound index on `(user_id, read)`** — the notification feed's primary query is "this user's unread notifications" (`GET /api/v1/notifications?unread_only`), which this index serves directly.

## General Indexing Principle

Every collection above indexes at minimum on `user_id` (or the field playing that role), because **every query in this system is owner-scoped** — there is no endpoint that lists across users. Additional compound indexes are added only where a documented query pattern (pagination, filtering, sorting) needs one — indexes are not added speculatively, per `CLAUDE.md`'s "avoid unnecessary complexity" principle applied to schema design.
