import { ThemeToggle } from "@/components/theme/ThemeToggle";
import { ComponentsShowcase } from "./ComponentsShowcase";

export const metadata = {
  title: "Styleguide",
};

const NEUTRALS = [
  ["plane", "--color-plane"],
  ["surface", "--color-surface"],
  ["surface-raised", "--color-surface-raised"],
  ["ink-primary", "--color-ink-primary"],
  ["ink-secondary", "--color-ink-secondary"],
  ["ink-muted", "--color-ink-muted"],
] as const;

const BRAND = [
  ["brand", "--color-brand"],
  ["brand-hover", "--color-brand-hover"],
  ["brand-active", "--color-brand-active"],
  ["brand-subtle", "--color-brand-subtle"],
] as const;

const STATUS = [
  ["good", "--color-status-good"],
  ["warning", "--color-status-warning"],
  ["serious", "--color-status-serious"],
  ["critical", "--color-status-critical"],
] as const;

const CHART = [
  "--color-chart-1",
  "--color-chart-2",
  "--color-chart-3",
  "--color-chart-4",
  "--color-chart-5",
  "--color-chart-6",
  "--color-chart-7",
  "--color-chart-8",
];

const SEQ_STEPS = [
  "--seq-blue-100",
  "--seq-blue-150",
  "--seq-blue-200",
  "--seq-blue-250",
  "--seq-blue-300",
  "--seq-blue-350",
  "--seq-blue-400",
  "--seq-blue-450",
  "--seq-blue-500",
  "--seq-blue-550",
  "--seq-blue-600",
  "--seq-blue-650",
  "--seq-blue-700",
];

function Swatch({ name, varName }: { name: string; varName: string }) {
  return (
    <div className="flex flex-col gap-2">
      <div
        className="h-16 w-full rounded-card border border-hairline"
        style={{ background: `var(${varName})` }}
      />
      <div className="text-xs">
        <div className="font-mono text-ink-primary">{name}</div>
        <div className="font-mono text-ink-muted">{varName}</div>
      </div>
    </div>
  );
}

export default function StyleguidePage() {
  return (
    <main className="mx-auto max-w-5xl px-6 py-12">
      <div className="mb-10 flex items-center justify-between">
        <div>
          <h1 className="font-display text-4xl font-medium text-ink-primary">
            Styleguide
          </h1>
          <p className="mt-1 text-ink-secondary">
            Living reference for Finora&rsquo;s design tokens. Dev-only route.
          </p>
        </div>
        <ThemeToggle />
      </div>

      <section className="mb-12">
        <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-ink-muted">
          Typography
        </h2>
        <div className="flex flex-col gap-3 rounded-card border border-hairline bg-surface p-6">
          <p className="font-display text-5xl font-medium text-ink-primary">
            Editorial headline
          </p>
          <p className="font-display text-2xl italic text-ink-secondary">
            Fraunces, for moments that deserve weight
          </p>
          <p className="text-base text-ink-primary">
            Body copy renders in Geist Sans — the UI workhorse, and per the
            dataviz skill&rsquo;s rule, the only face ever used inside a chart.
          </p>
          <p className="font-mono text-sm text-ink-secondary">
            $1,284.30 — tabular figures in Geist Mono for aligned columns.
          </p>
        </div>
      </section>

      <section className="mb-12">
        <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-ink-muted">
          Neutrals
        </h2>
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-6">
          {NEUTRALS.map(([name, v]) => (
            <Swatch key={v} name={name} varName={v} />
          ))}
        </div>
      </section>

      <section className="mb-12">
        <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-ink-muted">
          Brand
        </h2>
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
          {BRAND.map(([name, v]) => (
            <Swatch key={v} name={name} varName={v} />
          ))}
        </div>
      </section>

      <section className="mb-12">
        <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-ink-muted">
          Status (fixed both themes — icon + label always required)
        </h2>
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
          {STATUS.map(([name, v]) => (
            <Swatch key={v} name={name} varName={v} />
          ))}
        </div>
      </section>

      <section className="mb-12">
        <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-ink-muted">
          Categorical chart palette (fixed order)
        </h2>
        <div className="grid grid-cols-4 gap-4 sm:grid-cols-8">
          {CHART.map((v, i) => (
            <Swatch key={v} name={`chart-${i + 1}`} varName={v} />
          ))}
        </div>
      </section>

      <section className="mb-12">
        <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-ink-muted">
          Sequential ramp (blue, magnitude encoding)
        </h2>
        <div className="flex overflow-hidden rounded-card border border-hairline">
          {SEQ_STEPS.map((v) => (
            <div key={v} className="h-12 flex-1" style={{ background: `var(${v})` }} />
          ))}
        </div>
      </section>

      <section className="mb-12">
        <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-ink-muted">
          Components
        </h2>
        <ComponentsShowcase />
      </section>

      <section className="mb-12">
        <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-ink-muted">
          Elevation
        </h2>
        <div className="flex flex-wrap gap-6">
          {(["sm", "md", "lg", "popover"] as const).map((size) => (
            <div
              key={size}
              className="flex h-20 w-32 items-center justify-center rounded-card bg-surface text-xs text-ink-muted"
              style={{ boxShadow: `var(--shadow-${size})` }}
            >
              shadow-{size}
            </div>
          ))}
        </div>
      </section>
    </main>
  );
}
