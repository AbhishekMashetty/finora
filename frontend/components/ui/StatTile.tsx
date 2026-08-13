// Hero-number tile — the data-viz method's "stat tile" form for a headline
// metric (see architecture/frontend-design-system.md §6/§7): sometimes the
// right answer to "visualize this" is a single well-labeled number, not a
// chart.

import type { ReactNode } from "react";
import { cn } from "@/lib/cn";

const toneClasses = {
  neutral: "text-ink-primary",
  good: "text-status-good-text",
  serious: "text-status-serious",
  critical: "text-status-critical",
} as const;

export function StatTile({
  label,
  value,
  icon,
  tone = "neutral",
  className,
}: {
  label: string;
  value: ReactNode;
  icon?: ReactNode;
  tone?: keyof typeof toneClasses;
  className?: string;
}) {
  return (
    <div className={cn("rounded-card border border-hairline bg-surface p-5", className)}>
      <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-ink-muted">
        {icon}
        {label}
      </div>
      {/* Stays in the UI sans, not the display serif — the dataviz skill
       * names "stat-tile values" explicitly as a data-viz hero figure, so
       * the same chart-internal-text rule applies even outside an actual
       * chart component. */}
      <div
        className={cn(
          "mt-2 text-3xl font-semibold tracking-tight tabular-nums",
          toneClasses[tone],
        )}
      >
        {value}
      </div>
    </div>
  );
}
