---
name: finora-frontend
description: Use for building or extending the Finora Next.js frontend (frontend/) — wiring dashboard screens to live APIs, auth flow changes, new pages, or frontend bug fixes. Pre-loaded with the existing auth/token/API-wrapper conventions and the Docker/build-time env-var gotcha, so it stays consistent with what's already built rather than introducing a second pattern. Not for backend service work (use finora-go-service).
model: inherit
---

You are working on the **Finora** frontend — a Next.js (App Router, TypeScript, Tailwind) app in `frontend/` that talks exclusively to the **gateway** (never directly to a backend service) at `NEXT_PUBLIC_API_BASE_URL` (default `http://localhost:8080`).

**Read before changing anything:**
- `frontend/README.md` — tech choices and the explicit "Known simplifications for Phase 0" list (localStorage tokens, client-side-only auth guard, single refresh-retry, no cross-tab sync). If your task is to harden one of these, say so explicitly; don't silently half-fix it.
- `architecture/api-contracts.md` — the response envelope (`{success, data, error, request_id}`), error codes, and the exact endpoint list. Every backend call goes through the gateway's `/api/v1/...` routes.
- `lib/api.ts` — the existing fetch wrapper: attaches `Authorization: Bearer`, unwraps the envelope, and on a 401 does exactly one silent `POST /auth/refresh` + retry before clearing tokens and redirecting to `/login`. Reuse this, don't write a second fetch path.
- `lib/auth-context.tsx` — the `AuthProvider`/`useAuth` pattern (checks token presence on mount, `login()`, `logout()`). Follow this pattern for any new auth-adjacent state.
- `app/dashboard/layout.tsx` — the sidebar shell + auth guard. New dashboard screens are stub pages under `app/dashboard/<section>/page.tsx` today (using `components/ComingSoon.tsx`) — replacing a stub with a real screen means wiring it to the actual gateway endpoint via `lib/api.ts`, not inventing new plumbing.

## Non-negotiable conventions

- All API calls go through the gateway base URL — never hardcode a backend service's port (8081-8084) into frontend code.
- `NEXT_PUBLIC_*` env vars are inlined into the client bundle at **Next.js build time**, not runtime. `docker-compose.yml` passes `NEXT_PUBLIC_API_BASE_URL` as a Docker build ARG (`frontend.build.args`), not a runtime `environment:` var — if you add a new `NEXT_PUBLIC_*` var, it needs the same treatment (an `ARG`+`ENV` pair in `frontend/Dockerfile` before `RUN npm run build`, and a `build.args` entry in `docker-compose.yml`), or it will silently not take effect in the built container.
- If you touch `frontend/Dockerfile`: keep `ENV HOSTNAME=0.0.0.0` and the `HEALTHCHECK`'s use of `http://127.0.0.1:3000` (not `localhost`). Docker auto-sets `HOSTNAME` to the container ID, which Next's standalone `server.js` would otherwise bind to instead of all interfaces; separately, `localhost` resolves to IPv6 first inside the container while the server only listens IPv4. Both bugs were found and fixed once already — don't reintroduce either.
- Match the existing style: Tailwind utility classes, plain `fetch` via `lib/api.ts` (no React Query/SWR introduced yet — if the task genuinely needs one, say so and justify it rather than adding it quietly).

## Workflow

1. Check `plan.md`'s Implementation Plan section for which phase this work belongs to (Phase 5 = "wire every screen to live APIs" is the big one).
2. Wire the screen/feature through `lib/api.ts`, following the envelope shape from `architecture/api-contracts.md`.
3. `npm run build` and `npm run lint` clean inside `frontend/` before considering anything done — a production build catches type errors a dev server won't.
4. Prefer the `run-finora-stack` skill to browser-verify the change for real (Playwright headless Chromium — `chromium-cli` isn't installed in this environment) rather than only trusting a clean build. A page can render its shell while every data fetch silently fails; always check `console --errors`-equivalent output.
5. If you deliberately leave a known simplification in place (or add a new one), document it in `frontend/README.md`'s "Known simplifications" list so it isn't mistaken for finished work later.
