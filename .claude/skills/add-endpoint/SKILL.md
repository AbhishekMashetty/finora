---
name: add-endpoint
description: Use when adding a new REST endpoint (or extending an existing resource's CRUD) to an existing Finora service — the common case for Phase 1-4 roadmap work in plan.md. Walks the domain -> repository -> service -> handler -> router -> openapi.yaml -> tests pattern and the CLAUDE.md Definition of Done checklist so nothing is missed.
---

# Adding an Endpoint to an Existing Finora Service

Add code in this order — each layer depends only on the one below it (Clean Architecture, dependencies point inward, per `CLAUDE.md` §1):

1. **`internal/domain`** — add/extend the entity struct and, if new, the repository/service interface method signatures. No framework imports here (no gin, no mongo-driver). This is the contract everything else implements against.
2. **`internal/repository`** — implement the new interface method against MongoDB. Filter every query by the caller's user id (see step 4). Check `architecture/database-design.md` for whether this query pattern needs a new index.
3. **`internal/service`** — business logic and validation (required fields, enum values, cross-entity ownership checks like expense-service's transaction→account check). Constructors take interfaces, not concrete Mongo types.
4. **`internal/handler`** — `c.ShouldBindJSON` + `binding:"required"` tags for validation, call the service, respond via `httpx.Success`/`httpx.Fail` (never a raw `c.JSON`). Ownership rule: a resource that exists but belongs to another user is **404 NOT_FOUND, not 403** — don't leak existence via status code.
5. **`internal/router`** — register the route. If the resource is owner-scoped (the common case), it should already sit inside a group with `middleware.RequireIdentity()` applied — don't add auth logic per-handler.
6. **Gateway** — if this is a genuinely new resource path (not a new method on an existing one), add it to the gateway's routing table (`services/gateway/internal/router/router.go` / `internal/proxy`) so it's actually reachable from outside. Existing prefixes (`/api/v1/accounts`, `/api/v1/transactions`, etc.) are already routed.
7. **`openapi.yaml`** — add the new path/method, matching the `{success, data, error, request_id}` envelope and the standard error codes.
8. **`architecture/api-contracts.md`** — update the Per-Service Endpoints table for this service so it stays the ground-truth contract.
9. **Tests** — a table-driven case in `internal/service`'s test file against the existing hand-written fake (no live Mongo), plus an `internal/handler` case via `httptest` + `gin.TestMode`.

## Before calling it done

Run the `CLAUDE.md` §11 Definition of Done checklist:
- `go build ./...` and `go vet ./...` clean
- `gofmt -l .` clean
- `go test ./...` passing
- Response envelope / error codes used via `shared/httpx` — no hand-rolled JSON
- `openapi.yaml` and `architecture/api-contracts.md` updated to match
- `README.md` updated if env vars or run instructions changed
- No hardcoded config — anything new goes through `shared/config` and gets added to `.env.example`
- Structured logging via `shared/logger`/`shared/middleware.Logging` for anything worth observing — no `fmt.Println`

Then use the `run-finora-stack` skill to actually boot the stack and curl the new endpoint through the gateway — a service that only passes `go test` hasn't been proven to work through the real routing/auth path.
