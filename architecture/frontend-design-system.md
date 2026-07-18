# Frontend Design System

This document is the visual/UX constitution for `frontend/` — the same role `CLAUDE.md` plays for backend architecture. It exists because the original Phase 0–3 UI (functional, but a bare Tailwind starter aesthetic: unthemed zinc grayscale, ad hoc class strings repeated per-file, no icons, no data visualization, plain "Loading…" text) was never actually designed, just assembled to prove the API wiring worked. This is the pass that makes it look and feel like a real product, without touching any data-fetching logic, API contracts, or backend code.

Every rule below is chosen the same way `CLAUDE.md` chooses backend rules: pick the option that's easiest to keep consistent as the app grows, document why, document what was rejected.

---

## 1. Design philosophy

**Flat, confident, one accent color, real data density.** Fintech products people trust (Mercury, Ramp, Linear) share a look: near-neutral ink/surface system, a single restrained brand hue reserved for actions and identity, and status color used *only* for status — never decoration. No shadows-for-depth, no gradients, no illustration. Depth comes from spacing and typography, not elevation effects.

**Rejected alternative: a UI kit (shadcn/ui, Radix Themes, Chakra).** Would give faster scaffolding, but adds a real dependency surface (CLAUDE.md §10 requires justifying every new package) for a component set this app's actual surface area (a dozen form/table/card patterns) doesn't need. Hand-rolled Tailwind primitives in `frontend/components/ui/` cost nothing to audit, patch, or delete, and match how the rest of this codebase already prefers explicit code over framework magic (CLAUDE.md §1, "no clever code").

**Rejected alternative: an icon library (lucide-react, heroicons).** ~14 fixed, simple glyphs are needed across the whole app (nav items + a few action icons). Hand-rolling them as inline SVG in `frontend/components/icons.tsx` avoids a dependency for what amounts to a dozen `<path>` elements, keeps them trivially themeable via `currentColor`, and adds effectively zero bundle weight versus importing a whole icon package (even tree-shaken).

**Rejected alternative: a charting library (recharts, visx, chart.js).** The one real chart need — budget-vs-actual per category — is a bullet-style comparison (a target line plus a filled bar), which is a few `<div>`s and a `<svg>` marker line, not a general-purpose charting problem. Hand-rolled per the data-viz method below. If Phase 5+ ever needs genuinely complex charts (multi-series time trends, stacked area), revisit then — don't pay the dependency cost now for a need that doesn't exist yet (`plan.md`'s "don't introduce complexity before there's a legitimate reason," applied to the frontend).

**Data visualization method.** Built using Claude Code's `dataviz` skill (form → color → validate → marks → interaction → accessibility), not eyeballed. Concretely: colors below are the skill's validated default palette (`references/palette.md`), chosen because rolling a fresh brand palette and hand-checking colorblind-safety/contrast per shade is exactly the kind of "reasoning about ΔE" the skill says never to do — the reference palette is already validated (CVD ΔE, contrast, both color-scheme modes) so adopting it outright is strictly safer than inventing new hex values and re-deriving that work. The one deviation: **brand/accent** below reuses the palette's validated "violet" categorical slot rather than introducing an unvalidated hex, since violet doesn't collide with the two hues this app actually needs for financial polarity (green/red, reserved below).

---

## 2. Design tokens

Defined as CSS custom properties in `frontend/app/globals.css`, mapped into Tailwind v4's `@theme inline` block so pages use utility classes (`bg-surface`, `text-ink-primary`, `border-hairline`, `bg-brand`, …) instead of raw `zinc-*`/`red-*` values. This is the single place the whole app's palette can be retuned later — the same rationale `shared/config` gives for centralizing env var reads on the backend.

| Role | Light | Dark | Tailwind utility |
|---|---|---|---|
| Page plane (app background, outside cards) | `#f9f9f7` | `#0d0d0d` | `bg-plane` |
| Surface (cards, inputs, popovers) | `#fcfcfb` | `#1a1a19` | `bg-surface` |
| Primary ink (headings, values) | `#0b0b0b` | `#ffffff` | `text-ink-primary` |
| Secondary ink (body, labels) | `#52514e` | `#c3c2b7` | `text-ink-secondary` |
| Muted ink (placeholders, axis, timestamps) | `#898781` | `#898781` | `text-ink-muted` |
| Hairline border | `rgba(11,11,11,.10)` | `rgba(255,255,255,.10)` | `border-hairline` |
| Gridline (table dividers, chart baselines) | `#e1e0d9` | `#2c2c2a` | `border-grid` |
| **Brand** (primary actions, links, active nav, focus ring) | `#4a3aa7` | `#9085e9` | `bg-brand` / `text-brand` |
| Status — good (under budget, income) | `#0ca30c` | `#0ca30c` | `text-status-good` / `bg-status-good` |
| Status — warning (approaching a limit) | `#fab219` | `#fab219` | `text-status-warning` / `bg-status-warning` |
| Status — critical (over budget, expense emphasis) | `#d03b3b` | `#d03b3b` | `text-status-critical` / `bg-status-critical` |

Status colors are **fixed, never themed** (same hex both modes, per the data-viz method) and are reserved exclusively for state — never reused as a fourth "brand" color or a chart series color, so a status pill is never mistaken for a category. Every status color ships with an icon or label, never carries meaning by hue alone (light-mode warning is only 1.79:1 contrast by design — it leans on the accompanying label/icon, not on being readable as text by itself).

**Financial polarity** (income vs. expense, under vs. over budget) always maps to good/critical status roles above — never ad hoc `red-500`/`green-500` Tailwind defaults, so a color always means the same thing everywhere in the app.

## 3. Typography

Keep **Geist Sans** (already wired via `next/font/google` in `app/layout.tsx`) — it's self-hosted at build time (no runtime webfont request, no extra network hop), already integrated, and a deliberate, good choice; no reason to replace it with a system-font stack.

Fixed scale (use these, don't invent one-off sizes):

| Role | Classes |
|---|---|
| Hero figure / stat-tile value | `text-3xl font-semibold tracking-tight tabular-nums` |
| Page title | `text-2xl font-semibold tracking-tight` |
| Section label | `text-xs font-semibold uppercase tracking-wide text-ink-muted` |
| Body | `text-sm text-ink-secondary` |
| Micro (timestamps, helper text) | `text-xs text-ink-muted` |

`tabular-nums` is required on every rendered money amount and every table column of numbers, so digits align vertically — a detail generic Tailwind starters never set and real finance products always do.

## 4. Spacing & shape

- **Radius:** `rounded-md` (6px) for inputs/badges/small controls, `rounded-xl` (12px) for cards/panels, `rounded-full` for pill buttons and status chips. Never mix `rounded-lg`/`rounded-md` for the same role across pages (the pre-redesign code did this inconsistently — normalize it as pages are touched).
- **Card padding:** `p-6` standard panel, `p-5` for a compact stat tile.
- **No shadows.** Depth comes from the plane/surface contrast plus a 1px hairline border — not `shadow-sm`/`shadow-md`. Flat is the deliberate look (see §1); shadows on a near-black dark surface read muddy anyway.

## 5. Component inventory (`frontend/components/ui/`)

New shared primitives, replacing the copy-pasted class-string constants that used to live at the top of every page file (`inputClasses`, `primaryButtonClasses`, etc., redefined near-identically five times):

- `Button.tsx` — variants `primary` (brand fill), `secondary` (hairline outline), `danger` (critical outline, for destructive actions), `ghost` (text-only, for table-row inline actions); sizes `sm`/`md`.
- `Input.tsx`, `Select.tsx` — consistent field chrome, built-in label + inline error slot.
- `Card.tsx` — the one surface-panel wrapper every page's sections use.
- `Badge.tsx` — status pill (takes a `status: "good" | "warning" | "critical" | "neutral"` prop, renders the right color + accessible text, never color alone).
- `EmptyState.tsx` — icon + message + optional CTA button, replacing bare "No transactions found." text and the old `ComingSoon` component's plain dashed box.
- `Skeleton.tsx` — loading placeholder blocks (a shimmering gray bar), replacing literal "Loading…" text — standard perceived-performance pattern in real products.
- `StatTile.tsx` — a labeled hero number + optional delta, the data-viz method's "stat tile" form for headline metrics (total balance, this month's spend, etc.).
- `BudgetBar.tsx` — the bullet-style budget-vs-actual mark described in §6.
- `../icons.tsx` — the hand-rolled icon set (wallet, arrows, list, target, chart, gear, search, user, plus, trash, pencil, log-out, check) as named exports of small inline-SVG components, `currentColor`-themed, `16`/`20`px.

## 6. Reports chart: budget-vs-actual bullet bar

Per the data-viz method's form step: this is a **magnitude-vs-target comparison**, not a trend or a distribution — the right form is a bullet-style bar, not a line/donut/multi-series chart. Per category: a track (the full width = 100%+ of budgeted amount, capped visually at say 130% so a large overspend doesn't distort the bar), a filled bar for actual spend, and a vertical marker line at the 100%-of-budgeted point (the target). Fill color follows **status**, not a fixed brand or category hue: `good` under 80% of budget used, `warning` 80–100%, `critical` over 100% — with the numeric budgeted/actual/remaining figures always shown as text alongside (never color-alone, per the method's accessibility rule), and a three-item legend (a small swatch + label per status) shown once above the list, not repeated per row. Headline totals (total budgeted/actual/remaining) render as `StatTile`s above the per-category list, so the page reads "totals first, detail below" rather than forcing a scan of every row to find the overall picture.

## 7. Page-level layout decisions

- **Landing (`app/page.tsx`):** wordmark + a short, confident value line, primary/secondary CTA using `Button`. A restrained radial brand-tinted glow behind the wordmark (low-opacity, no animation) — the only decorative flourish in the whole app, deliberately spent on the one page whose entire job is a first impression.
- **Auth (`login`/`register`):** two-pane layout on `md:` and up — a dark, brand-tinted left panel (wordmark + 2–3 short value bullets) and the form on the right; stacks to form-only on mobile. Replaces the previous bare centered card, which wasted the entire viewport on small screens and said nothing about the product.
- **Dashboard shell (`layout.tsx`):** sidebar nav items get icons; the active item gets a left accent bar + tinted brand background instead of a full invert; the signed-in user's name/email now renders above the logout control (previously logout was the only identity cue in the entire shell).
- **Dashboard overview (`dashboard/page.tsx`):** was a single "signed in as {name}" card and nothing else. Replaced with a real overview: a short greeting, four `StatTile`s (total balance across accounts, this calendar month's spend, budgets on track vs. total this month, nearest goal's progress), and a five-row "recent activity" list. All computed client-side from **existing** endpoints already used elsewhere (`/accounts`, `/transactions?page_size=5`, `/budgets`, `/reports/summary` for the current month, `/goals`) — no new backend endpoints, no new API contract.
- **Accounts/Transactions/Budgets/Goals:** the previous pattern kept a full create-form permanently open above every list, which is a lot of visual weight for an action used occasionally. Each list now leads with a compact header + a "+ Add" `Button` that reveals the create form in place; everything else (tables, filters, per-row edit/delete) keeps its existing behavior, just restyled with the new primitives, `Badge` for type/status, and `Skeleton`/`EmptyState` for loading/empty states.

## 8. What this pass does **not** touch

No changes to `lib/api.ts`, `lib/auth-context.tsx`, any data-fetching logic, query params, request/response shapes, or backend code. This is presentation-layer only — every page's actual CRUD behavior, validation, and API contract usage from Phase 2/3 stays exactly as verified then. `profile`/`settings`/`search` remain `ComingSoon` (unstarted Phase 5 scope, restyled with the new `EmptyState` component but not built out).
