// Status pill. See architecture/frontend-design-system.md §2 — status color
// is reserved exclusively for state (never a category/brand color) and is
// never the only signal: the label text always carries the meaning too.

import type { ReactNode } from "react";
import { ArrowDownIcon, ArrowUpIcon } from "@/components/icons";

export type Status = "good" | "warning" | "critical" | "neutral";

const statusClasses: Record<Status, string> = {
  good: "bg-status-good/10 text-status-good-text",
  warning: "bg-status-warning/15 text-ink-primary",
  critical: "bg-status-critical/10 text-status-critical",
  neutral: "bg-ink-muted/10 text-ink-secondary",
};

export function Badge({ status = "neutral", children }: { status?: Status; children: ReactNode }) {
  return (
    <span
      className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium ${statusClasses[status]}`}
    >
      {children}
    </span>
  );
}

/** Specialization for a transaction's income/expense polarity. */
export function TransactionTypeBadge({ type }: { type: "income" | "expense" }) {
  return (
    <Badge status={type === "income" ? "good" : "critical"}>
      {type === "income" ? <ArrowUpIcon size={12} /> : <ArrowDownIcon size={12} />}
      {type === "income" ? "Income" : "Expense"}
    </Badge>
  );
}
