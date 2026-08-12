# Finora UI/UX Revamp — Audit & Checklist

**Status:** Proposal, awaiting sign-off. Nothing here is implemented yet.
**Scope:** `frontend/` — 13 routes, 8 UI primitives, 19 icons, ~4,600 LOC.
**Companion doc:** `architecture/frontend-design-system.md` is the frontend's constitution. It must be rewritten in the same change that implements this (CLAUDE.md §6), because several of its current rules are explicitly what this proposal changes.

---

## How to read this

Every finding below was verified against the code, not assumed. Where I claim something is missing, I grepped for it — those claims are marked **[verified]** with what I ran. I've flagged the handful of decisions that are genuinely yours, not mine, in [§0 Decisions](#0-decisions-i-need-from-you-blocking) — **those block the start of work**, because two of them (dependency policy, rendering architecture) change what half this list even looks like.

Effort key: **XS** <1h · **S** <1d · **M** 2–4d · **L** 1–2wk · **XL** 3wk+

---

## The honest assessment

The current frontend is **competently built and genuinely under-designed** — and those are two different problems with two different fixes.

What's genuinely good, and should survive the revamp: the token architecture in `globals.css` (CSS vars → `@theme inline` → utilities is exactly right, and means a palette change is a one-file change); the discipline of `tabular-nums` on every money figure; the `h-10` field/button alignment work (that's a real bug someone found by measuring bounding boxes, not eyeballing — that instinct is worth keeping); and the fact that every design decision in the design-system doc has a written rationale and a rejected alternative.

What isn't working, at the level of "this is a product someone pays for":

1. **It does not work on a phone.** The dashboard sidebar is `w-60 shrink-0` with zero breakpoints — on a 375px viewport it consumes 64% of the screen and there is no drawer, no hamburger, no bottom nav. This is not a polish issue; it's a "the product is unusable on the most common device class" issue. **[verified: no `sm:`/`md:`/`lg:` anywhere in `app/dashboard/layout.tsx`]**

2. **Destructive actions delete immediately, with no confirmation.** Accounts, transactions, budgets and goals all call `handleDelete` straight from an icon button. There is no dialog primitive in the codebase to build one with. **[verified: no `confirm(`, `<dialog`, `role="dialog"`, or `Modal` anywhere in `app/` or `components/`]**

3. **Text inputs have no focus ring.** `fieldClasses` sets `outline-none focus:border-brand` — a 1px border color change is the entire focus affordance, which fails WCAG 2.4.11 and is nearly invisible for keyboard users. Buttons do this correctly; fields don't. **[verified]**

4. **The `Input` component's `error` prop is dead code.** It's designed, documented, and rendered — and not one of the 13 pages passes it. Every validation error in the app is instead a page-level or card-level `<p>` far from the field that caused it. **[verified: zero call sites]**

5. **Money is `.toFixed(2)` + string concatenation, 18 times.** No `Intl.NumberFormat`. `"1234.5"` renders as `1234.50 USD`, not `$1,234.50`. There is a Settings page that stores the user's currency *and timezone* — and nothing in the app reads either one for formatting. **[verified: zero `Intl.` usages]**

6. **The whole app is client-rendered.** 13 of 26 component files are `"use client"`, including every single page. There are no `loading.tsx`, `error.tsx`, or `not-found.tsx` files anywhere — so there's no streaming, no route-level error recovery, and every page waterfalls its data *after* JS hydration. On Next 16 with RSC available, this is the single biggest structural miss. **[verified]**

7. **Dark mode exists but the user can't choose it.** It's `@media (prefers-color-scheme: dark)` only — no toggle, no persistence, no override. **[verified]**

8. **There is no motion system.** Four `transition-colors` and one `animate-pulse` in the entire codebase, and no `prefers-reduced-motion` handling at all. **[verified]**

The through-line: this was built as *"prove the API wiring works, then restyle it"* — and the design-system doc says so in its own opening paragraph. It succeeded at that. What it never got was a design pass that started from **what the user is trying to do** rather than from what the endpoints return.

---

## 0. Decisions I need from you (blocking)

These change the shape of the work. I'm not starting until these are answered.

### D1 — Visual direction

"Out of this world" can mean three quite different things. I'd like you to pick one, and I have a recommendation.

- [ ] **A. Refined institutional** — evolve the current Mercury/Ramp lineage. Tighter type scale, real elevation, a proper neutral ramp, restrained motion. Lowest risk, reads "trustworthy fintech," but will not make anyone say *whoa*.
- [ ] **B. Editorial data-product ⭐ recommended** — treat money as *content*. A real display typeface paired with a precise UI face, generous whitespace, oversized numerals as the visual anchor, charts as first-class page furniture rather than afterthoughts, deliberate use of one accent. This is where products like Linear, Arc, and Monzo's newer work live. It's the direction that makes a personal-finance app feel like something you'd *want* to open.
- [ ] **C. Expressive / high-craft** — the above plus signature moments: an animated net-worth hero, scroll-linked transitions, a genuinely custom empty-state illustration set, physics-based interactions. Highest ceiling, highest cost, and the most likely to age badly if not maintained.

My recommendation is **B**, with two or three deliberate "moments" borrowed from C (the overview hero, the goal-completion celebration, the command palette) rather than expressiveness applied uniformly. Spending flourish everywhere is the same as spending it nowhere.

### D2 — Dependency policy ⚠️ the important one

`architecture/frontend-design-system.md` §1 currently rejects, in writing and with rationale, **UI kits, icon libraries, and charting libraries**. Those rejections were correct for the app as it was. Several are no longer correct for the app you're asking for, and I won't quietly violate your own constitution — CLAUDE.md §10 requires a justification per dependency, so here they are:

| Package | Why it's now justified | Alternative if you say no |
|---|---|---|
| **Radix UI** or **Base UI** (primitives) | Dialog, dropdown, tooltip, popover, tabs, switch, combobox — all need focus trapping, `aria-*` wiring, typeahead, collision detection, scroll locking. Hand-rolling accessible versions of these is *weeks* and is the single most common source of a11y bugs in hand-built design systems. Unstyled, so it costs us zero visual control. | Hand-roll ~6 overlay primitives. Adds ~1.5wk and I can't promise the same a11y quality. |
| **Recharts** or **visx** | You'll want trend lines, category breakdowns, sparklines, and stacked cashflow. The existing hand-rolled `BudgetBar` is genuinely the right call for a bullet chart — but axes, scales, tooltips, and responsive containers are not worth rebuilding. | Keep charts to bars/rings only; no time-series. Meaningfully narrows what Reports can be. |
| **sonner** (toasts) | Small, excellent, solves stacking/timing/a11y announcements. | Hand-roll (~S). Genuinely fine to skip the dep here. |
| **lucide-react** | Current set is 19 hand-drawn glyphs; the revamp needs ~45 (chevrons, close, filter, sort, calendar, external-link, eye/eye-off, menu, sun/moon, info, alert-triangle…). Tree-shakes to roughly the same bytes as hand-rolling that many. | Hand-draw ~26 more icons (~M) and accept inconsistent optical weight. |
| **motion** (Framer Motion) | Layout animations, shared-element transitions, gesture-driven sheets. | CSS transitions only — rules out the signature moments in D1-C. |

- [ ] **Approve all** (recommended for direction B or C)
- [ ] **Approve primitives + charts only** — hand-roll toasts, icons, motion
- [ ] **Approve none** — fully hand-rolled; add ~2–3wk and I'd steer you toward direction A

### D3 — Rendering architecture

- [ ] **Migrate to RSC properly** — server-render page shells and initial data, `"use client"` only for genuinely interactive leaves, add `loading.tsx`/`error.tsx` per route, stream with Suspense. Real perceived-performance win, and it's the *reason to be on Next 16*. **(+L, touches every page)**
- [ ] **Stay client-rendered** — pure visual revamp, much smaller diff, keeps the current data-fetching untouched.

### D4 — Backend contract

The current design-system doc §8 says the last pass changed **no** API contracts. Some things on this list can't hold that line:

- A **net-worth / balance-over-time chart** needs historical balance data. No endpoint returns it.
- **Spend vs. last month** on the overview needs a second period's aggregate (cheap now — the `/transactions/aggregate` endpoint added for F01 can serve it).
- **Keyset pagination** for infinite-scroll transaction lists is finding **F06** in the scale review, currently unimplemented.

- [ ] **Frontend-only** — I'll design around what exists and mark the rest "needs backend"
- [ ] **Frontend + small additive backend endpoints** where a screen genuinely needs one (I'd propose each separately)

### D5 — Brand

- [ ] Keep "Finora" wordmark, violet accent, current voice
- [ ] Open to a brand refresh (wordmark treatment, accent hue, tone of voice) as part of this

### D6 — Scope & sequencing

- [ ] **Big bang** — one branch, one review, everything lands together
- [ ] **Phased ⭐ recommended** — foundations → primitives → shell → pages → polish, reviewable at each gate, app stays shippable throughout

---

## 1. Foundations — design language

> Nothing else starts until this is settled. Every later section consumes these tokens.

- [ ] **Typography.** Currently one family (Geist Sans) and a 5-role scale; `--font-mono` is declared in the theme and never used once. Establish a display/UI pairing, a full 8–10 step scale with line-height and tracking per step, and a dedicated numeric treatment for money (the single most important text in this product). **[M]**
- [ ] **Color.** Currently 4 semantic + 3 status tokens. Build a full neutral ramp (12 steps), brand ramp, and semantic aliases (`surface-raised`, `surface-sunken`, `border-subtle`, `border-strong`, `text-on-brand`…). Add a **categorical data-viz scale** — none exists today, so a multi-series chart has no legal colors. **[M]**
- [ ] **Contrast audit.** The design doc knowingly ships `--status-warning` at 1.79:1 and compensates with a required adjacent label. Re-derive the status trio to hit 4.5:1 as text where used as text, keeping the label rule as defense-in-depth rather than the only defense. **[S]**
- [ ] **Elevation.** Currently *zero* by explicit decision ("flat, no shadow"). That's defensible for cards and indefensible for overlays — a dialog, dropdown, or popover with no elevation cannot be read as floating. Define a 4-step elevation scale that works on both near-white and near-black grounds. **[S]**
- [ ] **Shape.** Three unrelated radii currently collide in a single row: `rounded-md` (6px) inputs, `rounded-xl` (12px) cards, `rounded-full` pill buttons. Pick one coherent language and apply it. **[S]**
- [ ] **Spacing & density.** Formalize the scale; define comfortable vs. compact density (financial tables want compact, overview wants comfortable). **[S]**
- [ ] **Motion.** Define duration/easing tokens, entrance/exit patterns, and what is *never* animated. Add a global `prefers-reduced-motion` kill-switch — currently absent entirely. **[M]**
- [ ] **Theming.** Add explicit light/dark/system with a toggle, `localStorage` persistence, and an SSR-safe no-flash script. Restructure tokens to the three-state pattern (`:root`, `@media` guarded by `:not([data-theme="light"])`, `[data-theme="dark"]`) so an explicit choice beats the OS in both directions. **[M]**
- [ ] **Iconography.** Standardize grid, optical weight, and sizes; expand 19 → ~45 glyphs (or adopt a library per D2). **[S–M]**
- [ ] **Focus & selection.** One focus-ring treatment applied to *every* interactive element, including fields. **[S]**
- [ ] **Publish a living styleguide** at `/styleguide` (dev-only route) rendering every token, primitive, state, and density — so drift is visible instead of theoretical. **[M]**

---

## 2. Primitives — rebuild & expand

Current inventory is 8: `Button`, `Input`/`Select`, `Card`, `Badge`, `EmptyState`, `Skeleton`, `StatTile`, `BudgetBar`.

### Rebuild existing
- [ ] **Button** — add `loading` (spinner + disabled + preserved width), icon-only variant with required `aria-label`, `xs`/`lg` sizes, link variant, `asChild` so `<Link>` wrapping stops producing nested-interactive markup (the landing page does this today). **[S]**
- [ ] **Input / Select** — real focus ring; **wire up the dead `error` prop** and add `aria-invalid`/`aria-describedby`; prefix/suffix adornments (currency symbol, unit); size variants; success/warning states. **[M]**
- [ ] **Card** — header/body/footer slots, interactive/linked variant, density prop. Today it's a single padded div, so every page hand-builds its own header row. **[S]**
- [ ] **Badge** — size variants, dot variant, removable variant. **[XS]**
- [ ] **Skeleton** — layout-matched skeletons per surface. Today `SkeletonRows` is always `h-10 w-full ×N`, which doesn't resemble any real layout and guarantees a visible content jump. **[S]**
- [ ] **EmptyState** — distinct treatments for *first-run* (encouraging, with a primary CTA), *no-results* (neutral, with a clear reset), and *error* (recovery action). All three currently render identically. **[S]**
- [ ] **StatTile** — the design doc promises "a labeled hero number **+ optional delta**"; the delta was never built. Add trend delta, sparkline slot, comparison label, loading state. **[M]**
- [ ] **BudgetBar** — keep the bullet form (it's the right call), restyle, animate on mount, add a hover/focus tooltip. **[S]**

### New primitives (none of these exist)
- [ ] **Dialog / AlertDialog** — unblocks destructive confirmation, the most serious interaction gap in the app **[M]**
- [ ] **Sheet / Drawer** — mobile nav, mobile filters, record detail **[M]**
- [ ] **DropdownMenu** — row actions, user menu **[M]**
- [ ] **Toast** — every mutation currently reports success via an inline `<p>` that never disappears **[S]**
- [ ] **Tooltip** & **Popover** — chart tooltips, truncated-content reveal, help affordances **[S]**
- [ ] **Tabs**, **SegmentedControl** — period switchers, filter groups **[S]**
- [ ] **Switch**, **Checkbox**, **RadioGroup** — no boolean/multi-select control exists anywhere today **[S]**
- [ ] **Combobox / Autocomplete** — replaces `<datalist>` (used for categories and timezones), which is unstylable and inconsistent across browsers **[M]**
- [ ] **DatePicker / DateRangePicker** — replaces bare `<input type="date">`; a range picker with presets ("This month", "Last 30 days") is essential for Reports **[M]**
- [ ] **Table** — sortable headers, sticky header, column alignment (numeric right + `tabular-nums`), row hover/selection, per-row loading, and a **card-collapse mode under `md`** rather than today's horizontal scroll **[L]**
- [ ] **Pagination** — page numbers, jump-to, page-size selector (currently Prev/Next only) **[S]**
- [ ] **CommandPalette (⌘K)** — global nav + entity search + quick actions **[L]**
- [ ] **Avatar**, **Progress / ProgressRing**, **Separator**, **Kbd**, **Alert/Banner**, **Breadcrumb**, **ScrollArea**, **Spinner** **[M total]**
- [ ] **CurrencyInput** — masked, locale-aware, correct caret behaviour **[M]**

---

## 3. App shell & navigation

- [ ] **Mobile navigation.** The blocking gap. Slide-over drawer under `lg`, plus a bottom tab bar for the 4 primary destinations. **[L]**
- [ ] **Responsive dashboard shell.** `main` is a fixed `p-8` with no max-width — cramped on mobile, sprawling on ultrawide. Fluid padding + container max-width + a proper breakpoint strategy. **[M]**
- [ ] **Collapsible desktop sidebar** — icon-rail mode with tooltips, persisted. **[M]**
- [ ] **Top bar** — breadcrumbs, global search trigger, notification bell with unread badge, user menu, theme toggle. Currently there is no top bar at all; the notification badge lives in the sidebar and identity is a plain text block above Log out. **[M]**
- [ ] **Nav polish** — `aria-current="page"` (absent), animated active indicator, section grouping (10 flat items today), keyboard nav. **[S]**
- [ ] **Skip-to-content link** and correct landmark roles — absent. **[XS]**
- [ ] **Route transitions** — coordinated enter/exit, respecting reduced-motion. **[M]**
- [ ] **Command palette wiring** — ⌘K from anywhere. **[S, after the primitive]**

---

## 4. Page-by-page

Each gets: responsive layout, real loading/empty/error states, keyboard support, and motion.

- [ ] **Landing** (`/`) — currently a wordmark, one line of copy, two buttons, and a blur circle. Needs an actual hero, a product screenshot or live demo, value props, and social proof. It's the only page whose entire job is a first impression. **[L]**
- [ ] **Login / Register** — the two-pane shell is a good bone. Needs: password reveal, password-strength meter on register, inline field validation, better error surfacing, loading states, and a real **forgot-password** flow (the `/auth/password-reset` endpoint exists and nothing in the UI calls it). **[M]**
- [ ] **Overview** (`/dashboard`) — the most important screen, currently 4 flat stat tiles + a 5-row list. Rebuild around a hero net-worth figure with trend, period comparison on every stat, spend-by-category viz, budget health at a glance, goal progress rings, and a **first-run onboarding state** (a new user currently sees four `—` characters and two empty boxes). **[XL]**
- [ ] **Accounts** — card grid with per-type iconography and colour, balance sparklines, sensible ordering, inline edit that doesn't reflow the row. **[L]**
- [ ] **Transactions** — the big one: **864 lines in a single file** holding filters, create form, inline category creation, CSV import, per-row edit, and pagination all at once. Decompose into components, move create/edit into a Sheet, add a real filter bar with active-filter chips, bulk selection + bulk actions, sticky column headers, running balance, date grouping, and mobile card layout. **[XL]**
- [ ] **Budgets** — visual budget cards over a table, period switcher, over-budget emphasis, progress against time-elapsed-in-period (a budget 90% spent on day 3 is a different story than on day 28 — the UI currently can't tell them apart). **[L]**
- [ ] **Goals** — progress rings, projected completion date, contribution history, celebration on completion. **[L]**
- [ ] **Reports** — currently one bullet-bar list. Add period presets + comparison, category breakdown, trend over time, and export. **[XL]**
- [ ] **Notifications** — grouping by date, type-specific iconography, mark-all-read, filter tabs, unread emphasis, empty state that isn't a dead end. **[M]**
- [ ] **Search** — currently requires typing then clicking a button. Make it instant/debounced, add result highlighting, recent searches, keyboard result navigation, and fold it into the command palette. **[L]**
- [ ] **Profile** — avatar (initials fallback), account metadata, danger zone, session management. **[M]**
- [ ] **Settings** — sectioned layout, currency picker with live preview, timezone combobox with current-time preview, theme control, notification preferences. **[M]**
- [ ] **404 / 500 / offline** — none exist. **[S]**

---

## 5. Data visualization

Today the app has exactly one chart form (`BudgetBar`) and no chart infrastructure.

- [ ] Chart foundations — responsive container, axes, gridlines, tooltip, legend, and **loading/empty/error states for charts specifically** (an empty chart is its own design problem) **[L]**
- [ ] Net worth / balance over time **[M]** *(needs D4)*
- [ ] Spend by category — donut or treemap **[M]**
- [ ] Cashflow in vs. out **[M]**
- [ ] Sparklines for stat tiles and account cards **[S]**
- [ ] Budget bullet bars — refined **[S]**
- [ ] Goal progress rings **[S]**
- [ ] Colorblind-safe categorical palette + verification pass **[S]**
- [ ] Every chart gets an accessible table or text equivalent **[M]**

---

## 6. Interaction & feedback

- [ ] **Confirmation for destructive actions** — 4 pages currently delete on single click, no undo, no dialog **[M]**
- [ ] **Toast notifications** for every mutation outcome **[S]**
- [ ] **Undo** for deletes (soft-delete window client-side) **[M]**
- [ ] **Optimistic updates** — every mutation currently triggers a full refetch **[L]**
- [ ] **Inline field validation** — wire the dead `error` prop, validate on blur, show errors adjacent to the field **[M]**
- [ ] **Complete state coverage** — hover, active, focus, disabled, loading, error, empty on every interactive element **[M]**
- [ ] **Keyboard shortcuts** + a discoverable shortcut sheet **[M]**
- [ ] **Focus management** — trap in overlays, restore on close, logical order **[M]**
- [ ] **Form UX** — unsaved-changes guard, autosave where sensible, `Cmd+Enter` submit **[M]**
- [ ] **First-run onboarding** — a guided path from empty account to first transaction **[L]**

---

## 7. Accessibility (target: WCAG 2.2 AA)

- [ ] Focus rings on **all** interactive elements — fields currently have none **[S]**
- [ ] Full contrast audit across both themes, including the known-failing warning token **[M]**
- [ ] Screen-reader pass — labels, descriptions, `aria-live` for async results, meaningful heading order **[L]**
- [ ] Keyboard-only pass on every flow, including tables, menus, and dialogs **[L]**
- [ ] `prefers-reduced-motion` honoured globally — absent today **[S]**
- [ ] Touch targets ≥44px — several `sm` icon buttons are ~30px **[S]**
- [ ] `eslint-plugin-jsx-a11y` in CI **[XS]**
- [ ] Axe automated pass + manual VoiceOver/NVDA spot-check **[M]**

---

## 8. Performance & architecture

- [ ] **RSC migration** per D3 — server components, `loading.tsx`/`error.tsx`/`not-found.tsx` per route, Suspense streaming **[XL]**
- [ ] **Data layer** — caching, request dedup, revalidation, mutation invalidation. Today every page refetches everything from scratch on mount **[L]**
- [ ] **Visibility-aware polling** — the notification poller runs every 30s forever regardless of whether the tab is visible **[XS]**
- [ ] **Decompose oversized components** — Transactions is 864 lines **[L]**
- [ ] Bundle budget + analysis in CI **[S]**
- [ ] Font loading strategy — subsetting, `display: swap`, preload **[S]**
- [ ] Image/OG asset pipeline **[S]**
- [ ] Route prefetching strategy **[S]**
- [ ] **Metadata** — no `viewport` export, no `theme-color`, no OpenGraph, no favicon set beyond the default, no web manifest **[S]**

---

## 9. Formatting, i18n & content

- [ ] **`Intl.NumberFormat` for all currency** — replaces 18 instances of `.toFixed(2)` + string concat **[M]**
- [ ] **`Intl.DateTimeFormat` + relative time** — replaces `.slice(0, 10)` raw-ISO rendering **[S]**
- [ ] **Actually use the user's saved currency and timezone.** Settings persists both; nothing reads either. **[M]**
- [ ] Compact notation for large figures (`$1.2M`) **[XS]**
- [ ] Microcopy pass — "Add budget" / "Add account" / "New transaction" / "Create one" are four conventions for one action **[M]**
- [ ] Error-message pass — every message should say what went wrong *and* what to do next **[M]**
- [ ] i18n-ready string extraction (not translating yet, just not hardcoding) **[L]** *(optional)*

---

## 10. Quality gates

- [ ] Component tests (Vitest + Testing Library) — **zero frontend tests exist today** **[L]**
- [ ] Playwright E2E for critical flows: register → login → add account → add transaction → see overview **[L]**
- [ ] Visual regression on the styleguide route **[M]**
- [ ] Extend `.github/workflows/ci.yml` — the frontend job currently runs `build` + `lint` only **[S]**
- [ ] **Rewrite `architecture/frontend-design-system.md`** to match what's actually built, including new rejected-alternative rationale (CLAUDE.md §6 requires this in the same change) **[M]**

---

## Proposed sequencing

| Phase | Contents | Gate |
|---|---|---|
| **1. Foundations** | §0 decisions, §1 tokens, styleguide route | You review the styleguide before any page is touched |
| **2. Primitives** | §2 — rebuild 8, add ~20 | Styleguide shows every primitive in every state |
| **3. Shell** | §3 — mobile nav, responsive shell, top bar, ⌘K | App is usable on a phone for the first time |
| **4. Pages** | §4 — highest-traffic first: Overview → Transactions → Budgets → Goals → Accounts → Reports → rest | Reviewable per page |
| **5. Depth** | §5 charts, §6 interaction, §7 a11y | Axe + keyboard pass clean |
| **6. Hardening** | §8 perf, §9 formatting, §10 tests + doc rewrite | CI green, design-system doc matches reality |

Phases 1–3 are the ones that change how the product *feels*. If budget is limited, stopping after 3 still leaves you dramatically better off than today.

---

## What I explicitly recommend against

Saying no to things is part of the job:

- **A full component library of our own.** Build only what these 13 screens need. A generic `<DataGrid>` nobody uses is waste.
- **Uniform expressiveness.** Animation everywhere reads as noise and as AI-generated design. Pick 2–3 moments and make those excellent.
- **Redesigning the API to suit the UI.** Where a screen wants data that doesn't exist, I'll flag it and propose an additive endpoint — not reshape existing contracts.
- **Dropping the `h-10` field/button alignment rule.** It looks fussy in the current doc and it is load-bearing; it stays, restated in the new system.
- **Touching backend code.** Out of scope entirely.

---

## Open risks

1. **Dependency approval (D2) gates roughly a third of this list.** A "no" doesn't stop the work, it changes the estimate materially — most of all for accessible overlays.
2. **RSC migration (D3) touches all 13 pages.** Best done *before* the page redesigns, not after; doing it second means editing every page twice.
3. **`architecture/frontend-design-system.md` will contradict reality** the moment phase 1 lands. It must be rewritten as part of the work, not afterward.
4. **The scale-review findings and this work overlap** on the frontend: F19 (build-time-baked `NEXT_PUBLIC_API_BASE_URL`, no CDN, tokens in `localStorage`) is unaddressed and intersects §8. Worth deciding whether it folds into this effort or stays separate.

---

*Prepared from a full read of all 13 routes, 8 primitives, `lib/`, `globals.css`, and `architecture/frontend-design-system.md`. Every "missing"/"absent" claim above was verified by grep against the codebase, not inferred.*
