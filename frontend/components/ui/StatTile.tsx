// Hero-number tile — the data-viz method's "stat tile" form for a headline
// metric (see architecture/frontend-design-system.md §6/§7): sometimes the
// right answer to "visualize this" is a single well-labeled number, not a
// chart.

import type { ReactNode } from "react";

export function StatTile({
  label,
  value,
  icon,
  tone = "neutral",
}: {
  label: string;
  value: ReactNode;
  icon?: ReactNode;
  tone?: "neutral" | "good" | "critical";
}) {
  const valueClasses =
    tone === "good"
      ? "text-status-good-text"
      : tone === "critical"
        ? "text-status-critical"
        : "text-ink-primary";

  return (
    <div className="rounded-xl border border-hairline bg-surface p-5">
      <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-ink-muted">
        {icon}
        {label}
      </div>
      <div className={`mt-2 text-3xl font-semibold tracking-tight tabular-nums ${valueClasses}`}>
        {value}
      </div>
    </div>
  );
}
