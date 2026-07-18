"use client";

// Overview page: a real dashboard, not just a "signed in as" card. Computed
// client-side from endpoints every other page already uses (accounts,
// transactions, budgets, reports/summary, goals) — no new backend endpoints,
// no new API contract. See architecture/frontend-design-system.md §7.

import Link from "next/link";
import { useEffect, useState } from "react";
import { apiFetch, ApiError } from "@/lib/api";
import type { Account, Goal, ReportSummary, Transaction, User } from "@/lib/types";
import { StatTile } from "@/components/ui/StatTile";
import { Card } from "@/components/ui/Card";
import { SkeletonRows } from "@/components/ui/Skeleton";
import { EmptyState } from "@/components/ui/EmptyState";
import { TransactionTypeBadge } from "@/components/ui/Badge";
import { FlagIcon, InboxIcon, ListIcon, TargetIcon, WalletIcon } from "@/components/icons";

function currentMonthRange(): { from: string; to: string } {
  const now = new Date();
  const first = new Date(now.getFullYear(), now.getMonth(), 1);
  const last = new Date(now.getFullYear(), now.getMonth() + 1, 0);
  return { from: first.toISOString().slice(0, 10), to: last.toISOString().slice(0, 10) };
}

interface OverviewData {
  user: User;
  totalBalance: number;
  currency: string;
  monthSpend: number;
  budgetsOnTrack: number;
  budgetsTotal: number;
  nearestGoal: Goal | null;
  recentTransactions: Transaction[];
}

export default function DashboardOverviewPage() {
  const [data, setData] = useState<OverviewData | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;

    async function load() {
      try {
        const { from, to } = currentMonthRange();
        const [userRes, accountsRes, goalsRes, recentRes, reportRes] = await Promise.all([
          apiFetch<{ user: User }>("/api/v1/users/me"),
          apiFetch<{ accounts: Account[] }>("/api/v1/accounts"),
          apiFetch<{ goals: Goal[] }>("/api/v1/goals"),
          apiFetch<{ transactions: Transaction[] }>("/api/v1/transactions?page=1&page_size=5"),
          // reports/summary needs at least one budget to return any rows —
          // an empty categories list (no budgets yet) is a normal, valid
          // response here, not an error.
          apiFetch<{ summary: ReportSummary }>(
            `/api/v1/reports/summary?from=${from}&to=${to}`
          ),
        ]);

        if (cancelled) return;

        const totalBalance = accountsRes.accounts.reduce((sum, a) => sum + a.balance, 0);
        const currency = accountsRes.accounts[0]?.currency ?? "USD";

        // "This month's spend" from the report's total_actual across all
        // budgeted categories — a reasonable proxy without a second,
        // separate transactions-summing pass. Uncategorized/unbudgeted
        // spend isn't reflected, same as the reports page itself.
        const monthSpend = reportRes.summary.total_actual;
        const budgetsTotal = reportRes.summary.categories.length;
        const budgetsOnTrack = reportRes.summary.categories.filter((c) => c.remaining >= 0).length;

        const upcomingGoals = goalsRes.goals
          .filter((g) => new Date(g.target_date).getTime() >= Date.now())
          .sort((a, b) => new Date(a.target_date).getTime() - new Date(b.target_date).getTime());

        setData({
          user: userRes.user,
          totalBalance,
          currency,
          monthSpend,
          budgetsOnTrack,
          budgetsTotal,
          nearestGoal: upcomingGoals[0] ?? null,
          recentTransactions: recentRes.transactions,
        });
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof ApiError ? err.message : "Could not load your overview.");
        }
      } finally {
        if (!cancelled) setIsLoading(false);
      }
    }

    load();
    return () => {
      cancelled = true;
    };
  }, []);

  if (isLoading) {
    return (
      <div>
        <h1 className="text-2xl font-semibold text-ink-primary">Overview</h1>
        <div className="mt-6 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="rounded-xl border border-hairline bg-surface p-5">
              <SkeletonRows rows={2} />
            </div>
          ))}
        </div>
      </div>
    );
  }

  if (error || !data) {
    return (
      <div>
        <h1 className="text-2xl font-semibold text-ink-primary">Overview</h1>
        <p className="mt-6 rounded-md bg-status-critical/10 px-3 py-2 text-sm text-status-critical">
          {error ?? "Something went wrong."}
        </p>
      </div>
    );
  }

  const { user, totalBalance, currency, monthSpend, budgetsOnTrack, budgetsTotal, nearestGoal, recentTransactions } = data;

  return (
    <div>
      <h1 className="text-2xl font-semibold text-ink-primary">
        Welcome back, {user.name.split(" ")[0]}
      </h1>

      <div className="mt-6 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatTile
          label="Total balance"
          icon={<WalletIcon size={16} />}
          value={`${totalBalance.toFixed(2)} ${currency}`}
        />
        <StatTile
          label="This month's spend"
          icon={<ListIcon size={16} />}
          value={`${monthSpend.toFixed(2)} ${currency}`}
          tone={monthSpend > 0 ? "critical" : "neutral"}
        />
        <StatTile
          label="Budgets on track"
          icon={<TargetIcon size={16} />}
          value={budgetsTotal > 0 ? `${budgetsOnTrack} / ${budgetsTotal}` : "—"}
          tone={budgetsTotal > 0 && budgetsOnTrack < budgetsTotal ? "critical" : "good"}
        />
        <StatTile
          label="Next goal"
          icon={<FlagIcon size={16} />}
          value={
            nearestGoal
              ? `${Math.round((nearestGoal.current_amount / nearestGoal.target_amount) * 100)}%`
              : "—"
          }
        />
      </div>

      <div className="mt-6 grid grid-cols-1 gap-6 lg:grid-cols-2">
        <Card>
          <h2 className="text-xs font-semibold uppercase tracking-wide text-ink-muted">
            Recent activity
          </h2>
          {recentTransactions.length === 0 ? (
            <div className="mt-4">
              <EmptyState
                icon={<InboxIcon size={24} />}
                title="No transactions yet"
                description="Log your first transaction to see it here."
              />
            </div>
          ) : (
            <ul className="mt-4 flex flex-col divide-y divide-hairline">
              {recentTransactions.map((tx) => (
                <li key={tx.id} className="flex items-center justify-between py-3 text-sm">
                  <div>
                    <p className="text-ink-primary">{tx.note || "—"}</p>
                    <p className="text-xs text-ink-muted">{tx.date.slice(0, 10)}</p>
                  </div>
                  <div className="flex items-center gap-3">
                    <TransactionTypeBadge type={tx.type} />
                    <span className="tabular-nums text-ink-primary">
                      {tx.type === "income" ? "+" : "-"}
                      {tx.amount.toFixed(2)} {tx.currency}
                    </span>
                  </div>
                </li>
              ))}
            </ul>
          )}
          <Link
            href="/dashboard/transactions"
            className="mt-4 inline-block text-xs font-medium text-brand"
          >
            View all transactions →
          </Link>
        </Card>

        <Card>
          <h2 className="text-xs font-semibold uppercase tracking-wide text-ink-muted">
            Next goal
          </h2>
          {nearestGoal ? (
            <div className="mt-4">
              <p className="text-lg font-medium text-ink-primary">{nearestGoal.name}</p>
              <p className="mt-1 text-sm text-ink-muted">
                Due {nearestGoal.target_date.slice(0, 10)}
              </p>
              <div className="mt-4 h-2 w-full overflow-hidden rounded-full bg-ink-muted/15">
                <div
                  className="h-full rounded-full bg-brand"
                  style={{
                    width: `${Math.min(100, (nearestGoal.current_amount / nearestGoal.target_amount) * 100)}%`,
                  }}
                />
              </div>
              <p className="mt-2 text-sm tabular-nums text-ink-secondary">
                {nearestGoal.current_amount.toFixed(2)} / {nearestGoal.target_amount.toFixed(2)}
              </p>
            </div>
          ) : (
            <div className="mt-4">
              <EmptyState
                icon={<FlagIcon size={24} />}
                title="No goals yet"
                description="Set a savings goal to track your progress."
              />
            </div>
          )}
          <Link href="/dashboard/goals" className="mt-4 inline-block text-xs font-medium text-brand">
            View all goals →
          </Link>
        </Card>
      </div>
    </div>
  );
}
