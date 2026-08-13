# Finora Frontend

Next.js frontend for Finora, a personal-finance SaaS. This app is the single
client that talks to the backend, and it only ever talks to the **gateway**
(never individual services directly) — see
`architecture/api-contracts.md` at the repo root for the authoritative API
contract, response envelope shape, and per-service routes.

## Phase 0 scope

This pass builds:

- `/register` and `/login` — auth forms against the gateway's
  `/api/v1/auth/*` routes.
- `/dashboard` — a shell (sidebar nav + logout) with an overview page that
  fetches `GET /api/v1/users/me` to prove the auth + API path works
  end-to-end.
- Stub "Coming soon" pages for Accounts, Transactions, Budgets, Goals,
  Reports, Search, Profile, and Settings under `app/dashboard/<section>/` —
  navigable placeholders only, no real data fetching or CRUD UI yet.
- Client-side auth guarding and a small `fetch` wrapper with automatic
  token refresh (see below).

## Screens

All 13 routes are live, each talking to the gateway via `lib/api.ts`:

- **Landing** (`app/page.tsx`), **login**/**register** — public, unauthenticated.
- **Overview** (`app/dashboard/page.tsx`) — stat tiles + recent activity +
  nearest goal, computed client-side from endpoints every other page
  already uses (`/accounts`, `/transactions`, `/budgets`, `/reports/summary`,
  `/goals`) — no dedicated overview endpoint.
- **Accounts** — full CRUD against expense-service (`GET/POST
  /api/v1/accounts`, `PUT/DELETE /api/v1/accounts/:id`).
- **Transactions** — full CRUD + server-side pagination/filtering against
  expense-service, plus inline category management (`GET/POST
  /api/v1/categories`, create+list only by deliberate backend design) and
  CSV import.
- **Budgets** — full CRUD against budget-service. `category` is a free-text
  field (with a `<datalist>` suggestion from expense-service's real
  categories), not an id — matches the backend's loose, case-insensitive
  matching for reports.
- **Goals** — full CRUD against budget-service, manual progress logging
  (`current_amount` is a plain field the user sets, never derived from
  transactions).
- **Reports** — `GET /api/v1/reports/summary?from&to` against
  budget-service's cross-service budget-vs-actual computation, plus a
  Recharts bar chart (spend by category) and the bullet-bar budget-vs-actual
  list — see the design doc §7 for why both, not one or the other.
- **Notifications**, **search**, **profile**, **settings** — see the design
  doc for each; search composes a client-side filter over existing list
  endpoints rather than a dedicated search endpoint (none exists in the
  fleet, and building one isn't justified at this app's data volumes).

## Design system

The app went through a full editorial-data-product redesign — every token,
every shared component, every page. Full rationale, every rejected
alternative, and the current token/component inventory live in
**`architecture/frontend-design-system.md`** (read that file, not this
summary, before touching any page's visual layer).

## Tech choices

- **Next.js 16 (App Router), TypeScript** — scaffolded with
  `create-next-app@latest` (flags: `--typescript --app --no-src-dir --eslint
  --use-npm --tailwind`). "14+" from the brief resolved to 16, the current
  `latest` at scaffold time; App Router conventions used here (client
  components, `layout.tsx`, route folders) are unchanged across that range.
- **Tailwind CSS** over CSS Modules — picked for speed: utility classes let
  simple auth forms and a sidebar shell get built and re-styled quickly
  without juggling separate `.module.css` files for what is, in this phase,
  a small number of pages. The "revisit once a design-system layer pays
  off" note from earlier phases is now moot — see "Design system" above;
  Tailwind v4's `@theme inline` was enough to add real design tokens
  without switching away from utility classes.
- **Plain `fetch` + a small hand-rolled wrapper** (`lib/api.ts`) instead of
  a data-fetching library (React Query, SWR, etc.) — Phase 0 has exactly
  one real data fetch (`/users/me`). Revisit once more pages need caching,
  background refetch, or optimistic updates.

## Project layout

```
app/
  page.tsx                 landing page (Server Component)
  login/page.tsx           two-pane auth layout via components/AuthShell.tsx
  register/page.tsx
  not-found.tsx            root 404
  styleguide/page.tsx      dev-only live token/component reference (Server shell + Client demo)
  dashboard/
    layout.tsx              Server Component shell — renders DashboardShell
    loading.tsx              skeleton shown during route-segment navigation
    error.tsx                 route error boundary
    page.tsx                   overview: stat tiles, recent activity, nearest goal
    accounts/, transactions/, budgets/, goals/, reports/,
    notifications/, search/, profile/, settings/         one page.tsx each — all live
components/
  AuthShell.tsx              two-pane brand/form layout shared by login + register
  theme/                     ThemeProvider, ThemeToggle, AppToaster, no-flash-script.ts
  dashboard/                 DashboardShell, SidebarNav, nav-items.ts, CategorySpendChart.tsx
  providers/AppProviders.tsx  single composition point for every app-wide client provider
  ui/
    Button, Input/Select, Card, Badge, EmptyState, Skeleton, StatTile,
    BudgetBar, Alert, Avatar, Dialog, ConfirmDialog, DropdownMenu, Tooltip
    — see architecture/frontend-design-system.md §6
lib/
  api.ts                     fetch wrapper: base URL, auth header, 401→refresh→retry-once
  auth-context.tsx           AuthProvider/useAuth: token presence check, login(), logout()
  format.ts                  Intl-based currency/number/date formatting
  cn.ts                      clsx + tailwind-merge combinator
  types.ts                   User/auth shapes + Account, Category, Transaction, Budget,
                             Goal, ReportSummary, CategorySummary
```

## Environment variables

| Var | Default | Purpose |
|---|---|---|
| `NEXT_PUBLIC_API_BASE_URL` | `http://localhost:8080` | Base URL of the gateway. Every API call is `${NEXT_PUBLIC_API_BASE_URL}/api/v1/...`. |

Copy `.env.example` to `.env.local` for local dev — Next.js loads it
automatically (`npm run dev` / `npm run build` both pick it up).

## Running locally

```bash
npm install
npm run dev
```

Then open http://localhost:3000. Without a running gateway, the landing
page, `/login`, and `/register` render fine; submitting the forms will
fail (network error) until `docker compose up` (or the gateway + user-
service running some other way) is available.

## Running via Docker / docker-compose

This app is normally run as the `frontend` service in the root
`docker-compose.yml` (owned by a separate pass — not present in this repo
yet as of this writing). The `Dockerfile` here is a standard multi-stage
build using `next.config.ts`'s `output: 'standalone'` for a lean runtime
image (`node:20-alpine`, non-root `nextjs` user, healthcheck on `:3000`).
`NEXT_PUBLIC_API_BASE_URL` is a **build-time** env var for Next.js (it gets
inlined into the client bundle), so if the gateway's address differs
between environments, rebuild the image rather than only changing the
container's runtime env.

## Known simplifications for Phase 0

These are intentional shortcuts for this pass, called out so a future
engineer knows exactly what to harden next:

1. **Tokens stored in `localStorage`.** `lib/api.ts` reads/writes
   `access_token` and `refresh_token` directly to `localStorage`. This is
   fine for local dev but is not production-hardened: it's readable by any
   script on the page (XSS exposure) and isn't sent automatically as a
   cookie, so it can't be validated by Next.js middleware or a CDN edge.
   **Next step:** move both tokens to httpOnly, `Secure`, `SameSite=Strict`
   cookies set by a Next.js Route Handler that proxies
   `/api/v1/auth/login|refresh|logout`, so client JS never touches the raw
   token value.

2. **Client-side-only auth guarding.** `app/dashboard/layout.tsx` is a
   client component that checks for a token in `localStorage` in a
   `useEffect` after mount, and redirects to `/login` if absent. There is
   no `middleware.ts`. This means: (a) the guard only runs after the JS
   bundle loads and hydrates — there's a brief unstyled/empty flash before
   the redirect for a fully logged-out visitor hitting `/dashboard/*`
   directly, and (b) it only checks *presence* of a token, not its
   validity or expiry (an expired-but-present access token gets caught
   later, on the first real API call, via the 401→refresh flow in
   `lib/api.ts`, not by the guard itself). **Next step:** once tokens move
   to httpOnly cookies (#1), add `middleware.ts` that checks for the
   cookie and redirects server-side before any dashboard HTML is sent.

3. **No token expiry/validity check, only presence.** Related to #2 —
   the `AuthProvider` in `lib/auth-context.tsx` does not decode or validate
   the JWT client-side; it only checks that *a* token string exists.
   Actual validation happens server-side at the gateway. This is
   deliberate (per `architecture/api-contracts.md`, only the gateway
   verifies access tokens) but means a corrupted/garbage token value would
   still pass the client guard and only fail on the first real request.

4. **Single retry on 401, no multi-tab token sync.** `lib/api.ts` retries
   a failed request exactly once after a silent refresh. If refresh also
   fails, tokens are cleared and the browser is hard-redirected to
   `/login`. There is no `storage` event listener to sync logout/refresh
   across multiple open tabs — logging out in one tab will not immediately
   clear the session in another tab until that tab makes its own API call
   and gets a 401.

5. **No data-fetching library, still.** Every page (all 13, now live) uses
   the same plain `apiFetch` + `useState`/`useEffect` pattern — no React
   Query/SWR. This means: no shared cache between pages (each page
   refetches from scratch on mount/navigation), no background
   refetch/stale-while-revalidate, and no automatic retry beyond the single
   401-refresh retry already in `lib/api.ts`. Revisit if cross-page data
   sharing grows enough to justify the dependency.

6. **Simple page-based pagination, no prefetch.** The Transactions list
   uses `page`/`page_size` query params and refetches on every
   prev/next click — no prefetching the next page, no infinite scroll,
   no URL-encoded pagination state (a page refresh resets to page 1).

7. **Manual-only goal progress.** `Goal.current_amount` is updated by the
   user typing a new absolute total into the "Log progress" field — it is
   never derived from transactions or budgets, matching the backend
   contract (`PUT /api/v1/goals/:id` takes `current_amount` as a plain
   field, not a computed value).

8. **No optimistic updates.** Every create/edit/delete waits for the gateway response before updating local state (or
   re-fetches the current page/list) — there is no optimistic UI that
   assumes success and rolls back on failure. Simpler to reason about at
   this scale; revisit if perceived latency becomes a UX problem.

9. **Free-text budget categories, no cross-check against expense
    categories.** `Budget.category` is a plain string with a `<datalist>`
    suggestion drawn from `GET /api/v1/categories`, but nothing stops a
    user from typing a name that doesn't match any expense-service
    category (the budget simply reports `actual: 0` for it, per the
    backend's case-insensitive-match-or-zero behavior) — this mirrors the
    backend's own deliberately loose design, not a frontend gap.
