// Status pill. See architecture/frontend-design-system.md §2 — status color
// is reserved exclusively for state (never a category/brand color) and is
// never the only signal: the label text always carries the meaning too.

import type { ReactNode } from "react";
import { ArrowDown, ArrowUp } from "lucide-react";
import { cn } from "@/lib/cn";

export type Status = "good" | "warning" | "serious" | "critical" | "neutral";

const statusClasses: Record<Status, string> = {
  good: "bg-status-good-subtle text-status-good-text",
  // warning/serious sit below 3:1 text contrast on a light surface by
  // design (dataviz skill reference palette) — always render the label in
  // ink, never in the status color itself, and lean on the subtle fill +
  // icon for the color signal instead.
  warning: "bg-status-warning-subtle text-ink-primary",
  serious: "bg-status-serious-subtle text-ink-primary",
  critical: "bg-status-critical-subtle text-status-critical",
  neutral: "bg-ink-muted/10 text-ink-secondary",
};

export function Badge({
  status = "neutral",
  className,
  children,
}: {
  status?: Status;
  className?: string;
  children: ReactNode;
}) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 rounded-pill px-2 py-0.5 text-xs font-medium",
        statusClasses[status],
        className,
      )}
    >
      {children}
    </span>
  );
}

/** Specialization for a transaction's income/expense polarity. */
export function TransactionTypeBadge({ type }: { type: "income" | "expense" }) {
  return (
    <Badge status={type === "income" ? "good" : "critical"}>
      {type === "income" ? <ArrowUp size={12} strokeWidth={2} /> : <ArrowDown size={12} strokeWidth={2} />}
      {type === "income" ? "Income" : "Expense"}
    </Badge>
  );
}
