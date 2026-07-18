# Finora — Security Posture Analysis

**Date:** 2026-07-18
**Scope:** Full codebase review (5 Go microservices, shared library, Next.js frontend, docker-compose, CI).
**Purpose:** Gauge readiness to deploy to production. Finora was built for local-MacBook development; this report assesses what would need to change before it faces the public internet.

**Bottom line:** The *application-layer* code is clean and shows real security discipline — bcrypt password hashing, refresh-token rotation with reuse-detection, strict per-user data ownership in every query, no password-hash leakage, no SQL/NoSQL injection surface. **However, the deployment and platform posture is not production-ready.** The single most serious issue is that every backend service trusts an unauthenticated `X-User-Id` header *and* those services' ports are published directly to the host — a complete authentication bypass in the current compose topology. There is also no transport encryption, no MongoDB authentication, no rate limiting, and JWT secrets default to well-known placeholder strings.

Do **not** deploy to production as-is. The critical and high findings below must be resolved first.

---

## Severity summary

| # | Severity | Finding |
|---|----------|---------|
| 1 | 🔴 Critical | Backend services trust `X-User-Id` with no auth, and their ports are published to the host → auth bypass |
| 2 | 🔴 Critical | JWT signing secrets default to public placeholder values |
| 3 | 🟠 High | MongoDB runs with no authentication enabled |
| 4 | 🟠 High | No transport encryption (all HTTP, no TLS) |
| 5 | 🟠 High | No rate limiting / brute-force protection on auth endpoints |
| 6 | 🟡 Medium | JWT parsing does not pin the signing algorithm (`WithValidMethods`) |
| 7 | 🟡 Medium | Access + refresh tokens stored in `localStorage` (XSS-exfiltratable) |
| 8 | 🟡 Medium | No request body size limit / header size limit (DoS) |
| 9 | 🟡 Medium | Gin runs in debug mode (never set to ReleaseMode) |
| 10 | 🟡 Medium | No security response headers (HSTS, X-Content-Type-Options, CSP, etc.) |
| 11 | 🟢 Low | Validation errors echo raw binder error strings to clients |
| 12 | 🟢 Low | Health/readiness endpoints are public and expose dependency state |
| 13 | 🟢 Low | Password policy is minimal (length ≥ 8 only) |
| 14 | 🟢 Low | Go 1.21.3 toolchain is behind current patch releases |

Legend: 🔴 must fix before any production exposure · 🟠 fix before production · 🟡 fix soon after / hardening · 🟢 good-practice / defense-in-depth.

---

## Critical findings

### 1. 🔴 Backend services trust `X-User-Id` with no authentication — and their ports are exposed

**Where:** `shared/middleware/identity.go`, `docker-compose.yml` (lines 90, 109, 128, 147), architecture "Auth Header Contract".

The design is: the **gateway is the only JWT validator**. It validates the bearer token and injects a trusted `X-User-Id` header; every downstream service (`user-service`, `expense-service`, `budget-service`, `notification-service`) simply trusts that header via `middleware.RequireIdentity()`:

```go
userID := c.GetHeader(UserIDHeader) // X-User-Id
if userID == "" { /* 401 */ }
c.Set("user_id", userID) // fully trusted from here on
```

This is a legitimate pattern **only if** the backend services are unreachable except through the gateway. In the current `docker-compose.yml`, that assumption is broken — every backend publishes its port to the host:

```yaml
user-service:         ports: ["8081:8081"]
expense-service:      ports: ["8082:8082"]
budget-service:       ports: ["8083:8083"]
notification-service: ports: ["8084:8084"]
```

**Impact:** Anyone who can reach those ports can bypass authentication entirely by sending a request with a hand-picked `X-User-Id`. Example — read any user's transactions with no token:

```bash
curl -H "X-User-Id: <any-user-uuid>" http://host:8082/api/v1/transactions
```

Because user IDs are UUIDs this isn't trivially enumerable, but it is a full authn/authz bypass against any known/leaked user ID, and it exposes every service's entire data surface (read, write, delete). On a laptop bound to loopback this is contained; the moment this compose file (or an equivalent K8s manifest with NodePort/LoadBalancer services) faces a network, it is critical.

**Fix:**
- Do **not** publish backend ports in any non-local environment. Only the gateway (`8080`) and frontend (`3000`) should be reachable. Put the four services on an internal-only network.
- In Kubernetes, make the backend services `ClusterIP`-only and enforce it with a `NetworkPolicy` that permits ingress to each service **only from the gateway pod**.
- Defense-in-depth: add authenticated service-to-service calls (mTLS or a shared internal token / signed header) so a backend never trusts a bare `X-User-Id` from an arbitrary source. Note the internal clients (`budget-service/internal/client/*`) already forward `X-User-Id` directly — they too rely entirely on network isolation today.

### 2. 🔴 JWT signing secrets default to public placeholder values

**Where:** `.env.example`, `.env`, `docker-compose.yml` lines 86–87, 169.

```yaml
JWT_ACCESS_SECRET: ${JWT_ACCESS_SECRET:-dev-access-secret-change-me}
JWT_REFRESH_SECRET: ${JWT_REFRESH_SECRET:-dev-refresh-secret-change-me}
```

The compose file falls back to these hard-coded strings if the env vars are unset, and the committed `.env` uses exactly those values. Anyone who has seen this repo knows the secret. Since tokens are HS256 (symmetric), knowing the secret lets an attacker **forge a valid access token for any user ID** — game over for the entire auth model.

The config layer does the right thing at the service boundary (`MustGetEnv("JWT_ACCESS_SECRET")` fails fast if truly absent), but the compose default silently supplies the weak value, so a "successful" boot proves nothing.

**Fix:**
- Remove the `:-dev-...` fallbacks from `docker-compose.yml` (or keep them only in an explicitly-named local override file), so a missing secret fails loudly instead of booting insecurely.
- Generate secrets with real entropy (≥ 32 bytes, e.g. `openssl rand -base64 48`), inject via a real secret store (K8s Secret sealed with SOPS/Sealed-Secrets/Vault), never from a committed `.env`.
- Add a startup check that rejects known placeholder values and short secrets (e.g. refuse any secret < 32 bytes or matching `*-change-me`).
- Rotate access and refresh secrets on separate schedules; consider moving to asymmetric signing (RS256/EdDSA) so only user-service holds the private key and the gateway verifies with a public key — this also removes the secret-sharing requirement between the two services.

---

## High findings

### 3. 🟠 MongoDB runs with authentication disabled

**Where:** `docker-compose.yml` — the four `mongo:*` services set no `MONGO_INITDB_ROOT_USERNAME`/`PASSWORD` and pass no `--auth`.

Each Mongo instance accepts unauthenticated connections from anything that can reach it on the container network. Combined with finding #1's port-exposure pattern, if a Mongo port is ever published (they are **not** today — good), the database is wide open. Even without host exposure, any compromised container on the Docker/K8s network gets unauthenticated read/write to every database.

**Fix:** Enable auth (`--auth` + root credentials from secrets), give each service a least-privilege user scoped to only its own database, require TLS for the Mongo connection in production, and enforce network policy so only the owning service can reach its Mongo instance.

### 4. 🟠 No transport encryption (plaintext HTTP everywhere)

**Where:** `shared/server/server.go` uses `srv.ListenAndServe()` (not `ListenAndServeTLS`); gateway binds `0.0.0.0:8080` plaintext; frontend talks to `http://localhost:8080`.

Bearer tokens, credentials on login/register, and all personal-finance data travel in cleartext. On a laptop that's fine; on any real network it's a sniffing/MITM exposure and precludes secure cookies / HSTS.

**Fix:** Terminate TLS at an ingress/load balancer (or the gateway) in front of the fleet; redirect HTTP→HTTPS; serve the frontend over HTTPS so `Secure` cookies and HSTS become usable. This is prerequisite to fixing #7 properly.

### 5. 🟠 No rate limiting or brute-force protection

**Where:** No rate-limit middleware anywhere; auth routes are wide open: `POST /api/v1/auth/{login,register,refresh,password-reset}`.

`Login` does a constant-ish bcrypt comparison and returns a generic `invalid credentials` (good — no user enumeration there), but nothing throttles attempts. This permits credential stuffing / password brute-forcing, and `register` can be abused to mass-create accounts (resource exhaustion, and every register triggers a bcrypt hash — a cheap amplification DoS).

**Fix:** Add rate limiting at the gateway (per-IP and per-account), exponential backoff / temporary lockout on repeated login failures, and a CAPTCHA or proof-of-work on register if abuse appears. Consider a global request-rate ceiling at the ingress.

---

## Medium findings

### 6. 🟡 JWT parsing does not pin the signing algorithm

**Where:** `shared/jwtx/jwtx.go` — `Parse` calls `jwt.ParseWithClaims(...)` with a keyfunc that returns the HMAC secret unconditionally and **no `jwt.WithValidMethods([]string{"HS256"})`**.

golang-jwt v5 rejects the `none` algorithm unless explicitly opted in, so the classic `alg:none` bypass isn't directly reachable, and since the system only uses a symmetric secret (no RSA public key in play) the RS256→HS256 confusion attack isn't currently exploitable either. But not pinning the accepted algorithm is a latent footgun: the day someone introduces asymmetric keys, this code would happily accept an attacker-chosen algorithm.

**Fix:** Pass `jwt.WithValidMethods([]string{"HS256"})` (or the chosen alg) to every parse call. Also consider setting and validating `iss`/`aud` claims to bind tokens to this system.

### 7. 🟡 Tokens stored in `localStorage`

**Where:** `frontend/lib/api.ts` (`localStorage.setItem(ACCESS_TOKEN_KEY/REFRESH_TOKEN_KEY, ...)`), `frontend/lib/auth-context.tsx`.

Both the short-lived access token *and* the 7-day refresh token live in `localStorage`, which is readable by any JavaScript on the page — so any XSS (including a compromised npm dependency) can exfiltrate a long-lived refresh token and fully hijack the session. This is explicitly acknowledged in-code as a Phase-0 simplification with a plan to move to httpOnly cookies.

**Fix (already the stated plan):** Move tokens to `httpOnly`, `Secure`, `SameSite` cookies set by Next.js route handlers; keep the refresh token out of JS entirely. Pair with a Content-Security-Policy (see #10) to reduce XSS reach. The refresh-token *rotation with reuse detection* on the backend (see "Positives") is a strong mitigation once a stolen token is used by both parties, but it doesn't prevent silent theft.

### 8. 🟡 No request body / header size limits

**Where:** `shared/server/server.go` sets neither `http.Server.MaxHeaderBytes` nor a per-request `http.MaxBytesReader`; handlers `ShouldBindJSON` arbitrarily large bodies.

An attacker can post very large bodies/headers to exhaust memory. The report client bounds its own outbound pagination (nice), but inbound requests are unbounded.

**Fix:** Set `MaxHeaderBytes`, add a body-size cap middleware (e.g. `http.MaxBytesReader` at the gateway and/or each service), and set `ReadHeaderTimeout`/`ReadTimeout`/`WriteTimeout`/`IdleTimeout` on the `http.Server` (currently all unset, which also leaves Slowloris-style attacks open).

### 9. 🟡 Gin runs in debug mode in production

**Where:** No `gin.SetMode(gin.ReleaseMode)` in any `cmd/server/main.go` or router; only test files set `TestMode`.

Debug mode emits verbose startup/route logging and a runtime warning, and is not the intended production configuration. Low direct risk but leaks route/framework detail and is noisy.

**Fix:** Call `gin.SetMode(gin.ReleaseMode)` (or set `GIN_MODE=release`) in production for every service.

### 10. 🟡 No security response headers

**Where:** Gateway middleware chain is `RequestID → Logging → Recovery → CORS`; no header-hardening middleware.

Missing `Strict-Transport-Security`, `X-Content-Type-Options: nosniff`, `X-Frame-Options`/frame-ancestors, `Referrer-Policy`, and a `Content-Security-Policy`. CSP in particular is the main structural defense against the XSS that would defeat #7.

**Fix:** Add a security-headers middleware at the gateway (and set CSP on the frontend responses). HSTS only after TLS (#4) is in place.

---

## Low findings

### 11. 🟢 Validation errors echo raw binder strings
`Register`/`Login`/transaction handlers return `err.Error()` from gin binding directly in the response (`httpx.Fail(..., err.Error(), ...)`). Not sensitive today, but it leaks internal field/validation structure and is inconsistent with the otherwise-clean generic messaging. Prefer mapping to a stable, curated validation message.

### 12. 🟢 Public health endpoints expose dependency state
`/live`, `/ready`, `/health` are unauthenticated on every service (including the gateway). `/ready` reflects MongoDB reachability. Standard for probes, but in production restrict `/ready`/`/health` to the internal network / probe source so dependency health isn't publicly observable.

### 13. 🟢 Minimal password policy
`registerRequest` enforces only `min=8`. No complexity, no breached-password check. Consider a longer minimum and a check against known-breached passwords (e.g. HIBP k-anonymity API) rather than complexity rules.

### 14. 🟢 Toolchain currency
Go is pinned to `1.21.3` (an older patch of an older minor). Dependencies (`gin v1.10.0`, `golang-jwt/jwt/v5 v5.2.1`, `mongo-driver v1.17.1`, `x/crypto v0.26.0`) are reasonably current. Establish a routine to bump the Go patch/minor and run `govulncheck` + `npm audit` in CI (neither is present in `.github/workflows/ci.yml` today).

---

## What this codebase gets right (positives)

These are worth preserving as the platform hardens:

- **Password hashing:** bcrypt at default cost via `golang.org/x/crypto/bcrypt`; plaintext passwords are never stored or logged.
- **Refresh-token rotation with theft detection:** `auth_service.go`'s `Refresh` rotates tokens on every use and, on replay of an already-revoked token, revokes **all** of the user's tokens (`RevokeAllForUser`) — a strong response to token theft.
- **Refresh tokens stored hashed:** only a SHA-256 hash of the JTI is persisted (`RefreshToken.JTIHash`, `hashJTI`), so a database leak alone can't reconstruct a usable token.
- **Strict per-user data ownership:** every repository query is scoped by `user_id` (`{"_id": oid, "user_id": userID}` on get/update/delete), so one user cannot reach another's records even with a valid session. IDOR surface is well contained.
- **No password-hash leakage:** `User.PasswordHash` is tagged `json:"-"`, defended at the struct level rather than relying on handler discipline.
- **No injection surface:** MongoDB access uses typed `bson.M` filters with driver parameterization — no string-concatenated queries. Mongo `ObjectIDFromHex` failures map cleanly to `NOT_FOUND`.
- **Account-enumeration resistance:** login returns a uniform `invalid credentials`; password-reset always returns 202 regardless of whether the email exists (`RequestPasswordReset` is a deliberate no-op stub).
- **Client-supplied `X-User-Id` is never trusted at the gateway:** `authmw` always *overwrites* the header with the value from the validated token, so a client can't smuggle an identity *through* the gateway (the exposure in #1 is about reaching the backends *around* the gateway).
- **Panic containment:** `middleware.Recovery` converts panics into clean 500 envelopes, so stack traces don't leak to clients.
- **Graceful shutdown** and consistent structured logging (no `fmt.Println`) across services.

---

## Recommended remediation order (for a production go/no-go)

1. **Network isolation + secrets (blockers):** stop publishing backend ports; put backends on an internal-only network with NetworkPolicies; replace all JWT/secret placeholder values with high-entropy secrets from a real secret store and remove the compose fallbacks. (Findings #1, #2)
2. **Enable MongoDB auth + least-privilege users; add TLS everywhere (ingress termination).** (Findings #3, #4)
3. **Add rate limiting / lockout on auth; add body-size and server timeouts.** (Findings #5, #8)
4. **Harden defaults:** Gin release mode, security headers + CSP, JWT algorithm pinning, move frontend tokens to httpOnly cookies. (Findings #6, #7, #9, #10)
5. **Hygiene:** curate validation messages, restrict health endpoints, strengthen password policy, add `govulncheck`/`npm audit` to CI and keep the toolchain patched. (Findings #11–#14)

**Confidence to deploy to production today: Low.** The application logic would earn a solid pass in a code review, but the surrounding platform (auth boundary enforcement, secrets, transport, database auth, abuse protection) is at local-development maturity. Items 1–2 above are hard blockers; 3–4 should land before any real user data is involved.
