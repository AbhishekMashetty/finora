# Frontend Design System

This document is the visual/UX constitution for `frontend/` — the same role `CLAUDE.md` plays for backend architecture. It describes the **editorial data-product** revamp (see `frontend/UI-REVAMP-CHECKLIST.md` for the original audit and decision record), which replaced the Phase 0–3 UI in full: every design token, every shared component, every one of the app's 13 pages, and the shell they sit in.

Every rule below is chosen the same way `CLAUDE.md` chooses backend rules: pick the option that's easiest to keep consistent as the app grows, document why, document what was rejected. **When this document and the code disagree, the code is wrong** — the same standard `CLAUDE.md` holds itself to.

---

## 1. Design philosophy

**Editorial data-product.** A confident, near-neutral ink/surface system carries the UI; a validated violet accent is reserved for actions and identity; status color is reserved exclusively for state. A display serif (Fraunces) is used for headlines and hero moments — page titles, the landing page, empty-state and error copy — paired with Geist Sans for every interactive/dense surface (forms, tables, nav, and — per the data-viz method — everything inside a chart). This is the "editorial" half of the direction: confident typography and generous whitespace on top of the same flat, no-shadow, no-gradient surface system the app already had. Depth still comes from spacing and typography, not elevation effects, except where Radix's overlay components (`Dialog`, `DropdownMenu`, `Tooltip`) need a popover to visually separate from the page — those get the `--shadow-popover` token, the one place true elevation is used.

**Previously rejected, now adopted: Radix UI, an icon library, and a charting library.** The original Phase 0–3 doc rejected all three for a hand-rolled equivalent, on the grounds that the app's surface area didn't justify the dependency. That calculus changed with the editorial revamp's scope — 13 pages' worth of consistent, accessible interactive components (dialogs, dropdowns, tooltips) and a real chart (see §6) are a different order of complexity than the dozen form/table patterns the original doc was scoped against, and CLAUDE.md §10 was satisfied the normal way: each dependency was checked for React 19 peer-dependency compatibility before installing, and is used for exactly the problem it solves (Radix for unstyled-but-accessible primitives — focus trap, `Escape`/outside-click, ARIA wiring, all real correctness work a hand-rolled `<div>` would have to reimplement; `lucide-react` for a consistent, larger glyph set than 19 hand-drawn SVGs could offer; `recharts` for the one page that now needs a real chart). The old hand-rolled `components/icons.tsx` and its `ComingSoon` placeholder were deleted in the same phase that finished migrating every page off them — see §5.

**Data visualization method.** Built using Claude Code's `dataviz` skill (form → color → validate → marks → interaction → accessibility), not eyeballed. The palette below is the skill's validated reference instance (`references/palette.md`) — categorical, sequential, diverging, and status ramps all pass the skill's `validate_palette.js` script in both light and dark mode (re-run whenever a palette value changes). Brand reuses the palette's validated "violet" categorical slot 7 rather than an unvalidated hex.

## 2. Design tokens

Defined as CSS custom properties in `frontend/app/globals.css`, mapped into Tailwind v4's `@theme inline` block. Three states, not two — see §3 for the theming model.

### Neutrals & chrome

| Role | Light | Dark | Tailwind utility |
|---|---|---|---|
| Page plane | `#f9f9f7` | `#0d0d0d` | `bg-plane` |
| Surface (cards, table rows) | `#fcfcfb` | `#1a1a19` | `bg-surface` |
| Surface, raised (popovers, dialogs) | `#ffffff` | `#232322` | `bg-surface-raised` |
| Primary ink | `#0b0b0b` | `#ffffff` | `text-ink-primary` |
| Secondary ink | `#52514e` | `#c3c2b7` | `text-ink-secondary` |
| Muted ink | `#898781` | `#898781` | `text-ink-muted` |
| Hairline | `rgba(11,11,11,.10)` | `rgba(255,255,255,.10)` | `border-hairline` |
| Hairline, strong | `rgba(11,11,11,.18)` | `rgba(255,255,255,.18)` | `border-hairline-strong` |
| Gridline | `#e1e0d9` | `#2c2c2a` | `border-grid` |
| Baseline / axis | `#c3c2b7` | `#383835` | n/a — chart use only |

### Brand (violet — categorical slot 7)

| Role | Light | Dark |
|---|---|---|
| `brand` | `#4a3aa7` | `#9085e9` |
| `brand-hover` | `#3d2f8c` | `#a89fee` |
| `brand-active` | `#302470` | `#bab3f2` |
| `brand-subtle` (selected-nav, tinted backgrounds) | `#efecfa` | `#2b2557` |
| `brand-foreground` (text/icon on a solid `brand` fill) | `#ffffff` | `#1a1230` |

`brand-foreground`, not a hardcoded `text-white`, is load-bearing: `brand` flips to a *light* violet in dark mode, and a fixed white label on it drops to near-unreadable contrast. This was a real bug in `Button`'s `primary` variant caught during the Phase 2 primitive rebuild, fixed by introducing this token rather than a `dark:` override.

### Status (fixed — never themed)

| Role | Hex | Light-surface contrast | Notes |
|---|---|---|---|
| `status-good` | `#0ca30c` | 3.27:1 | text-safe as `status-good-text` (`#006300` light / `#0ca30c` dark) |
| `status-warning` | `#fab219` | 1.79:1 | icon + label only, never bare text |
| `status-serious` | `#ec835a` | 2.57:1 | icon + label only, never bare text |
| `status-critical` | `#d03b3b` | 4.68:1 | text-safe |

Four tiers, not three — `serious` was added in this revamp (the reference palette always had it; the original doc only adopted three). Sub-3:1 contrast on `warning`/`serious` against a light surface is **by design**, per the dataviz skill's own reference: the mitigation is the icon + label pairing (see `Badge.tsx`), never darkening the hex. Each status also has a `-subtle` background-wash token (e.g. `status-warning-subtle`) for badge fills.

### Chart palettes

Full detail and validation results live in `frontend/app/styleguide` (a dev-only route rendering every token as a live swatch) and were re-confirmed via the dataviz skill's `validate_palette.js` in both modes before adoption. Categorical: 8 fixed slots (`chart-1`…`chart-8`), never cycled — a 9th category folds into "Other" (see `CategorySpendChart.tsx`). Sequential: single-hue blue ramp, 13 steps. Diverging: blue↔red poles with a neutral gray midpoint (poles only implemented today; per-chart interpolation is left to the chart that needs it, not pre-baked as unused tokens).

### Elevation, radius, motion

- **Radius:** `--radius-control` (0.5rem, inputs/buttons/small chrome), `--radius-card` (0.875rem, panels/dialogs), `--radius-pill` (999px, pills/badges/toggle tracks) — as Tailwind utilities `rounded-control`/`rounded-card`/`rounded-pill`.
- **Shadow:** `--shadow-sm/md/lg/popover` — used only on Radix overlay content (`Dialog`, `DropdownMenu`, `Tooltip`) and the styleguide's elevation demo. Everything else stays flat per §1.
- **Motion:** `--duration-fast` (120ms, hover/focus states), `--duration-base` (200ms, dialogs/drawers), `--duration-slow` (320ms, progress-bar fills); `--ease-out`/`--ease-in-out`. A global `@media (prefers-reduced-motion: reduce)` rule in `globals.css` collapses every transition/animation to near-zero duration app-wide — no per-component opt-in required.

## 3. Theming — three states, not two

`globals.css` defines light values on a bare `:root`, dark values under `@media (prefers-color-scheme: dark)` guarded by `:root:not([data-theme="light"])`, and dark values again under `:root[data-theme="dark"]` for an explicit user choice. The toggle wins over the OS setting either way. `ThemeProvider` (`components/theme/ThemeProvider.tsx`) persists the choice to `localStorage` (`finora-theme`); a synchronous inline `<script>` in `app/layout.tsx` (`components/theme/no-flash-script.ts`) applies the stored `data-theme` attribute before hydration, so there's no flash of the wrong theme on first paint. `ThemeToggle` lives in the dashboard sidebar footer.

## 4. Typography

- **Geist Sans** — UI workhorse: body copy, forms, tables, nav, and (per the dataviz skill's own rule) everything inside a chart, including hero figures like `StatTile` values. Loaded via `next/font/google` in `app/layout.tsx`.
- **Geist Mono** — tabular figures where columns must align (money in tables, timestamps).
- **Fraunces** (new) — display serif for page titles (`font-display` utility, mapped from `--font-fraunces`), the landing page, and empty/error-state headings. Never used inside a chart component — that's the one place the dataviz skill's "system sans only" rule is enforced as written; everywhere else, "editorial" means the serif is available for headline moments.

`tabular-nums` is required on every rendered money amount and every numeric table column.

## 5. Dependencies

Installed for the revamp (all version-checked for React 19 peer compatibility before install, per CLAUDE.md §10):

| Package | Used for |
|---|---|
| `radix-ui` (unified package) | `Dialog`, `AlertDialog` (via `ConfirmDialog`), `DropdownMenu`, `Tooltip`, `Slot` (`Button`'s `asChild`) |
| `lucide-react` | Every icon in the app — direct imports per file, no wrapper layer (see below) |
| `recharts` | `CategorySpendChart` (§6) |
| `sonner` | Global toast host (`AppToaster`, mounted once in `AppProviders`) — not yet consumed by any page's mutation flow, ready for the next page that needs a non-blocking success/error toast instead of an inline `Alert` |
| `class-variance-authority`, `clsx` + `tailwind-merge` (`lib/cn.ts`) | Component variant APIs (`Button`, others as they're extended) |
| `cmdk`, `react-day-picker`, `date-fns`, `motion` | Installed and approved, **not yet consumed** — reserved for a command palette (replacing the flat "Search" nav item) and native `<input type="date">` fields ever needing a richer picker. Don't remove these speculatively; don't add new usage without a real page that needs them either. |

`components/icons.tsx` (a lucide-react-backed compatibility layer preserving the old hand-rolled icon API) existed only to let pages migrate one at a time without touching every import path at once. It was deleted, along with the now-callerless `ComingSoon.tsx`, once all 13 pages were migrated to importing `lucide-react` directly — see git history for the exact commit. Any new page imports icons directly from `lucide-react`, not through a wrapper.

## 6. Component inventory (`frontend/components/ui/`)

- **`Button`** — `cva`-based variants `primary`/`secondary`/`danger`/`ghost`, sizes `sm`/`md`/`icon`. `size="md"` is `h-10`, pixel-identical to `Input`/`Select`'s own height (see below) for any row mixing a button with a labeled field. `asChild` (Radix `Slot`) renders the button's classes onto a single child element instead of a `<button>` — for a `Link` styled as a button, avoiding invalid `<button><a>` nesting.
- **`Input`/`Select`** — `fieldClasses` sets an explicit `h-10` (native date/number controls have different intrinsic heights than a `<select>`, most visible in Safari) and now carry a `focus:` ring (`focus:ring-2 focus:ring-brand/25`) plus an `error`-driven red border/ring — both were previously missing entirely (the `error` prop rendered a message but never restyled the field). `className` styles the control itself; `wrapperClassName` styles the field's outer wrapper `<div>` — the actual grid/flex item for `col-span-*`/`flex-1`.
- **`Card`, `Badge`, `StatTile`, `EmptyState`, `Skeleton`/`SkeletonRows`, `BudgetBar`/`BudgetLegend`, `Avatar`** — same roles as before, rebuilt on the current token set. `Badge` gained the `serious` status tier. `Avatar` (new) renders initials on a `brand-subtle` circle — used in the sidebar footer and the profile page, one implementation instead of two ad hoc ones.
- **`Alert`** — new. Replaces the hand-copied `bg-status-critical/10` banner markup that used to be redefined per page; `success`/`error`/`info` variants, each with an icon.
- **`Dialog`, `ConfirmDialog`, `DropdownMenu`, `Tooltip`** — new, all Radix-based (`components/ui/Dialog.tsx`, `ConfirmDialog.tsx`, `DropdownMenu.tsx`, `Tooltip.tsx`). `ConfirmDialog` is the fix for the audit's blocking finding that every destructive delete fired immediately on click with no confirmation — it's now wired into every delete action across accounts/transactions/budgets/goals. Entrance/exit animation uses Tailwind's built-in `data-[state=...]` variants against Radix's own state attributes, not an animation plugin.
- **`components/theme/`** — `ThemeProvider`, `ThemeToggle`, `AppToaster` (theme-aware `sonner` host), `no-flash-script.ts`.
- **`components/dashboard/`** — `DashboardShell` (the Client Component interactive shell — auth guard, nav polling, responsive drawer), `SidebarNav` (shared between the desktop `<aside>` and the mobile drawer, so they can't drift apart), `nav-items.ts` (grouped nav config), `CategorySpendChart.tsx` (§6).
- **`components/providers/AppProviders.tsx`** — single composition point for every app-wide client provider (`ThemeProvider` → Radix `TooltipProvider` → `AuthProvider` → `AppToaster`), replacing ad hoc provider nesting in `layout.tsx`.

## 7. Reports charts

Two complementary, non-redundant views on the reports page, per the dataviz skill's form step:

- **`CategorySpendChart`** (new) — a horizontal Recharts bar chart answering "where did the money go": actual spend per category, fixed 8-slot categorical color order (9th+ folds into "Other"), direct axis labels instead of a separate legend, recessive gridlines, rounded bar ends, a token-driven hover tooltip. Colors are wired as `fill="var(--color-chart-N)"` directly in the SVG, so the chart re-themes with the light/dark toggle for free — no re-render/recompute logic needed.
- **`BudgetBar`/`BudgetLegend`** (unchanged from the original doc) — answers "am I over budget per category": a bullet-style track + target marker, fill color following *status*, not entity. Kept as the right form for a magnitude-vs-target comparison; the bar chart doesn't replace it, it adds the composition view the bullet bars never provided.

Money values without a currency field in their API contract (`Budget`, `ReportSummary`/`CategorySummary` — see `lib/types.ts`) render via `lib/format.ts#formatNumber` (a plain grouped decimal), never `formatCurrency` with a guessed currency — guessing would misrepresent a non-USD user's numbers. `Account`/`Transaction` do carry a real `currency` field and use `formatCurrency` throughout.

## 8. RSC pattern

Server Component shell, Client Component leaf: a page/layout that has real static structure stays a Server Component and renders the interactive parts as a Client Component child, passed through untouched rather than re-rendered. Applied at the shell level (`app/dashboard/layout.tsx` is a Server Component; `DashboardShell` is the Client Component it renders) and demonstrated at the page level (`app/styleguide/page.tsx` is a Server Component; the interactive demos live in `ComponentsShowcase.tsx`).

This does **not** extend to server-side data fetching for authenticated pages: auth tokens live in `localStorage` (a documented Phase 0 simplification in `lib/auth-context.tsx`/`lib/api.ts`, deferred to a future httpOnly-cookie + `middleware.ts` migration), which is only readable client-side. Every dashboard page's actual data fetching therefore stays a `"use client"` component calling `apiFetch` in a `useEffect` — the RSC migration's real, deliverable scope for this app is confining `"use client"` to genuinely interactive/data-dependent code and hoisting everything else (layout chrome, static page structure) to the server, not fetching data server-side under a client-only auth model that hasn't itself been migrated. Every dashboard route also has `loading.tsx` (skeleton) and `error.tsx`; a root `app/not-found.tsx` covers unmatched routes.

## 9. Responsive shell

The dashboard sidebar was a fixed `w-60` at every viewport with no responsive behavior at all (a blocking audit finding). Now: a fixed 256px sidebar at `lg:` (1024px) and up; below that, a topbar with a hamburger opens a Radix `Dialog`-based slide-in drawer (focus trap, `Escape`-to-close, backdrop) rendering the same `SidebarNav` content. Page-level header rows (an `<h1>` plus one or two action buttons) use `flex-wrap` so a second button drops to its own line on narrow viewports instead of overflowing off-screen — found and fixed via an actual 390px-width screenshot, not by inspection.

## 10. Known, deliberate gaps

- **Command palette / `⌘K` search** — `cmdk` is installed and approved but not wired up; the sidebar still has a flat "Search" nav item pointing at `/dashboard/search`. A real command palette is future work, not silently dropped.
- **Native date fields** — `react-day-picker`/`date-fns` are installed but every date input in the app is still a plain `<input type="date">`. Revisit if a richer picker (range selection, keyboard nav beyond the native control) becomes a real need.
- **`sonner` toasts** — the host is mounted app-wide; no page has been converted from its inline `Alert` success/error pattern to a toast yet. Both are legitimate patterns for different cases (a toast for a transient confirmation, an inline `Alert` for a persistent form-validation state) — this isn't a to-do to eliminate `Alert`, just an unused capability.
- **Currency-less domains** — `Budget`/`Goal`/`ReportSummary` carry no currency field server-side (see §7). Fixing this properly is a backend contract change (adding a `currency` field to budget-service's domain types and migrating existing records), out of scope for a frontend-only revamp; `formatNumber` is the honest interim treatment, not a permanent design decision.
