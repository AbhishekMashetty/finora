"use client";

// Reports screen: real cross-service budget-vs-actual computation from
// budget-service via the gateway. GET /api/v1/reports/summary?from&to
// (both required — no implicit default range server-side, so this page
// supplies one: the current calendar month).
//
// budget-service's report figures carry no currency field (see lib/types.ts
// ReportSummary/CategorySummary) — formatNumber (plain grouped decimal) is
// used throughout instead of formatCurrency, rather than guessing a
// currency that could misrepresent a non-USD user's data.

import { useEffect, useState, type FormEvent } from "react";
import { BarChart3 } from "lucide-react";
import { apiFetch, ApiError } from "@/lib/api";
import { formatNumber } from "@/lib/format";
import type { ReportSummary } from "@/lib/types";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Card } from "@/components/ui/Card";
import { EmptyState } from "@/components/ui/EmptyState";
import { SkeletonRows } from "@/components/ui/Skeleton";
import { StatTile } from "@/components/ui/StatTile";
import { Badge } from "@/components/ui/Badge";
import { Alert } from "@/components/ui/Alert";
import { BudgetBar, BudgetLegend, budgetStatus } from "@/components/ui/BudgetBar";
import { CategorySpendChart } from "@/components/dashboard/CategorySpendChart";

function currentMonthRange(): { from: string; to: string } {
  const now = new Date();
  const first = new Date(now.getFullYear(), now.getMonth(), 1);
  const last = new Date(now.getFullYear(), now.getMonth() + 1, 0);
  return { from: first.toISOString().slice(0, 10), to: last.toISOString().slice(0, 10) };
}

export default function ReportsPage() {
  const initialRange = currentMonthRange();
  const [from, setFrom] = useState(initialRange.from);
  const [to, setTo] = useState(initialRange.to);
  const [summary, setSummary] = useState<ReportSummary | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  async function runReport(fromValue: string, toValue: string) {
    setIsLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams({ from: fromValue, to: toValue });
      const data = await apiFetch<{ summary: ReportSummary }>(
        `/api/v1/reports/summary?${params.toString()}`
      );
      setSummary(data.summary);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not load report.");
      setSummary(null);
    } finally {
      setIsLoading(false);
    }
  }

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const params = new URLSearchParams({ from: initialRange.from, to: initialRange.to });
        const data = await apiFetch<{ summary: ReportSummary }>(
          `/api/v1/reports/summary?${params.toString()}`
        );
        if (!cancelled) setSummary(data.summary);
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof ApiError ? err.message : "Could not load report.");
          setSummary(null);
        }
      } finally {
        if (!cancelled) setIsLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function handleSubmit(event: FormEvent) {
    event.preventDefault();
    runReport(from, to);
  }

  const remaining = summary ? summary.total_budgeted - summary.total_actual : 0;

  return (
    <div>
      <h1 className="font-display text-2xl font-medium text-ink-primary">Reports</h1>

      <Card className="mt-6">
        <h2 className="text-xs font-semibold uppercase tracking-wide text-ink-muted">Budget vs. actual</h2>
        <form onSubmit={handleSubmit} className="mt-3 flex flex-wrap items-end gap-3">
          <Input
            id="report-from"
            label="From"
            type="date"
            required
            value={from}
            onChange={(e) => setFrom(e.target.value)}
          />
          <Input
            id="report-to"
            label="To"
            type="date"
            required
            value={to}
            onChange={(e) => setTo(e.target.value)}
          />
          <Button type="submit" disabled={isLoading}>
            {isLoading ? "Running…" : "Run report"}
          </Button>
        </form>
      </Card>

      {isLoading && (
        <Card className="mt-6">
          <SkeletonRows rows={4} />
        </Card>
      )}

      {!isLoading && error && (
        <Card className="mt-6">
          <Alert variant="error">{error}</Alert>
        </Card>
      )}

      {!isLoading && !error && summary && summary.categories.length === 0 && (
        <Card className="mt-6">
          <EmptyState
            icon={<BarChart3 size={24} strokeWidth={1.75} aria-hidden="true" />}
            title="No budgets found for this range"
            description="Add a budget to see a report."
          />
        </Card>
      )}

      {!isLoading && !error && summary && summary.categories.length > 0 && (
        <>
          <div className="mt-6 grid grid-cols-1 gap-4 sm:grid-cols-3">
            <StatTile label="Total budgeted" value={formatNumber(summary.total_budgeted)} />
            <StatTile label="Total actual" value={formatNumber(summary.total_actual)} />
            <StatTile
              label="Remaining"
              value={formatNumber(remaining)}
              tone={remaining < 0 ? "critical" : "good"}
            />
          </div>

          <Card className="mt-6">
            <h2 className="text-xs font-semibold uppercase tracking-wide text-ink-muted">
              Spend by category
            </h2>
            <div className="mt-4">
              <CategorySpendChart categories={summary.categories} />
            </div>
          </Card>

          <Card className="mt-6">
            <h2 className="text-xs font-semibold uppercase tracking-wide text-ink-muted">
              Budget vs. actual by category
            </h2>
            <div className="mt-4">
              <BudgetLegend />
            </div>
            <ul className="mt-4 flex flex-col divide-y divide-hairline">
              {summary.categories.map((cs) => {
                const status = budgetStatus(cs.budgeted, cs.actual);
                return (
                  <li key={cs.category} className="flex flex-col gap-3 py-4 sm:flex-row sm:items-center">
                    <div className="sm:w-40 sm:shrink-0">
                      <p className="font-medium capitalize text-ink-primary">{cs.category}</p>
                      <p className="text-xs capitalize text-ink-muted">{cs.period}</p>
                    </div>
                    <div className="flex-1">
                      <BudgetBar budgeted={cs.budgeted} actual={cs.actual} />
                    </div>
                    <div className="flex flex-wrap items-center gap-3 text-sm sm:w-auto sm:shrink-0 sm:justify-end">
                      <span className="tabular-nums text-ink-secondary">
                        Budgeted {formatNumber(cs.budgeted)}
                      </span>
                      <span className="tabular-nums text-ink-secondary">
                        Actual {formatNumber(cs.actual)}
                      </span>
                      <Badge status={status}>
                        <span className="tabular-nums">Remaining {formatNumber(cs.remaining)}</span>
                      </Badge>
                    </div>
                  </li>
                );
              })}
            </ul>
          </Card>
        </>
      )}
    </div>
  );
}
