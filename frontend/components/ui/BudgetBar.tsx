// Budget-vs-actual bullet bar. See architecture/frontend-design-system.md
// §6 for the full rationale: this is a magnitude-vs-target comparison, not a
// trend or distribution, so a bullet-style bar (a track + a target marker)
// is the right form — not a general-purpose chart library.
//
// Track represents up to 130% of the budgeted amount, so a real overspend
// is visible without letting a pathological outlier distort the scale. Fill
// color follows *status*, never a fixed brand/category hue, and the numeric
// figures are always rendered alongside by the caller — this component never
// carries meaning by color alone.

const TRACK_CAP = 1.3;

export type BudgetStatus = "good" | "warning" | "critical";

export function budgetStatus(budgeted: number, actual: number): BudgetStatus {
  if (budgeted <= 0) return actual > 0 ? "critical" : "good";
  const pct = actual / budgeted;
  if (pct > 1) return "critical";
  if (pct >= 0.8) return "warning";
  return "good";
}

const fillClasses: Record<BudgetStatus, string> = {
  good: "bg-status-good",
  warning: "bg-status-warning",
  critical: "bg-status-critical",
};

export function BudgetBar({ budgeted, actual }: { budgeted: number; actual: number }) {
  const status = budgetStatus(budgeted, actual);
  const pct = budgeted > 0 ? actual / budgeted : actual > 0 ? TRACK_CAP : 0;
  const fillPct = Math.min(pct, TRACK_CAP) / TRACK_CAP * 100;
  const markerPct = 100 / TRACK_CAP; // the 100%-of-budget target line

  return (
    <div
      className="relative h-2.5 w-full min-w-32 overflow-hidden rounded-full bg-ink-muted/15"
      role="img"
      aria-label={`${Math.round(pct * 100)}% of budget used`}
    >
      <div
        className={`h-full rounded-full transition-[width] ${fillClasses[status]}`}
        style={{ width: `${fillPct}%` }}
      />
      <div
        className="absolute top-0 h-full w-px bg-ink-primary/40"
        style={{ left: `${markerPct}%` }}
        title="Budgeted amount"
      />
    </div>
  );
}

/** One legend, shown once above a list of BudgetBars — not repeated per row. */
export function BudgetLegend() {
  const items: { status: BudgetStatus; label: string }[] = [
    { status: "good", label: "Under 80% of budget" },
    { status: "warning", label: "80–100% of budget" },
    { status: "critical", label: "Over budget" },
  ];
  return (
    <div className="flex flex-wrap items-center gap-4 text-xs text-ink-muted">
      {items.map((item) => (
        <span key={item.status} className="flex items-center gap-1.5">
          <span className={`h-2 w-2 rounded-full ${fillClasses[item.status]}`} />
          {item.label}
        </span>
      ))}
      <span className="flex items-center gap-1.5">
        <span className="h-2.5 w-px bg-ink-primary/40" />
        Budgeted target
      </span>
    </div>
  );
}
