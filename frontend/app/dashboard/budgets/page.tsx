"use client";

// Budgets screen: full CRUD against budget-service via the gateway.
// GET/POST /api/v1/budgets, PUT/DELETE /api/v1/budgets/:id.
//
// `category` here is a free-text name (matched case-insensitively by
// budget-service against expense-service categories for reporting), NOT an
// expense-service category id — a <datalist> of existing category names is
// offered as a soft suggestion, not a strict dropdown, per the contract.

import { useEffect, useState, type FormEvent } from "react";
import { apiFetch, ApiError } from "@/lib/api";
import type { Budget, Category, ReportSummary } from "@/lib/types";
import { Button } from "@/components/ui/Button";
import { Input, Select } from "@/components/ui/Input";
import { Card } from "@/components/ui/Card";
import { EmptyState } from "@/components/ui/EmptyState";
import { SkeletonRows } from "@/components/ui/Skeleton";
import { PencilIcon, PlusIcon, TargetIcon, TrashIcon } from "@/components/icons";

const PERIODS = ["weekly", "monthly", "yearly"] as const;

function currentMonthRange(): { from: string; to: string } {
  const now = new Date();
  return {
    from: new Date(now.getFullYear(), now.getMonth(), 1).toISOString().slice(0, 10),
    to: new Date(now.getFullYear(), now.getMonth() + 1, 0).toISOString().slice(0, 10),
  };
}

export default function BudgetsPage() {
  const [budgets, setBudgets] = useState<Budget[]>([]);
  const [categoryNames, setCategoryNames] = useState<string[]>([]);
  const [summary, setSummary] = useState<ReportSummary | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);

  // create form
  const [isAddOpen, setIsAddOpen] = useState(false);
  const [category, setCategory] = useState("");
  const [amount, setAmount] = useState("");
  const [period, setPeriod] = useState<(typeof PERIODS)[number]>("monthly");
  const [isCreating, setIsCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  // per-row edit
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editCategory, setEditCategory] = useState("");
  const [editAmount, setEditAmount] = useState("");
  const [editPeriod, setEditPeriod] = useState<(typeof PERIODS)[number]>("monthly");
  const [isSavingEdit, setIsSavingEdit] = useState(false);
  const [editError, setEditError] = useState<string | null>(null);

  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [rowError, setRowError] = useState<string | null>(null);

  async function loadBudgets() {
    try {
      const { from, to } = currentMonthRange();
      const [data, report] = await Promise.all([
        apiFetch<{ budgets: Budget[] }>("/api/v1/budgets"),
        apiFetch<{ summary: ReportSummary }>(`/api/v1/reports/summary?from=${from}&to=${to}`),
      ]);
      setBudgets(data.budgets ?? []);
      setSummary(report.summary);
      setLoadError(null);
    } catch (err) {
      setLoadError(err instanceof ApiError ? err.message : "Could not load budgets.");
    } finally {
      setIsLoading(false);
    }
  }

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const { from, to } = currentMonthRange();
        const [budgetsData, categoriesData, reportData] = await Promise.all([
          apiFetch<{ budgets: Budget[] }>("/api/v1/budgets"),
          apiFetch<{ categories: Category[] }>("/api/v1/categories"),
          apiFetch<{ summary: ReportSummary }>(`/api/v1/reports/summary?from=${from}&to=${to}`),
        ]);
        if (!cancelled) {
          setBudgets(budgetsData.budgets ?? []);
          setCategoryNames((categoriesData.categories ?? []).map((c) => c.name));
          setSummary(reportData.summary);
        }
      } catch (err) {
        if (!cancelled) {
          setLoadError(err instanceof ApiError ? err.message : "Could not load budgets.");
        }
      } finally {
        if (!cancelled) setIsLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  async function handleCreate(event: FormEvent) {
    event.preventDefault();
    setCreateError(null);
    setIsCreating(true);
    try {
      await apiFetch<{ budget: Budget }>("/api/v1/budgets", {
        method: "POST",
        body: JSON.stringify({ category, amount: Number(amount), period }),
      });
      setCategory("");
      setAmount("");
      setPeriod("monthly");
      setIsAddOpen(false);
      await loadBudgets();
    } catch (err) {
      setCreateError(err instanceof ApiError ? err.message : "Could not create budget.");
    } finally {
      setIsCreating(false);
    }
  }

  function startEdit(budget: Budget) {
    setEditingId(budget.id);
    setEditCategory(budget.category);
    setEditAmount(String(budget.amount));
    setEditPeriod(budget.period);
    setEditError(null);
  }

  function cancelEdit() {
    setEditingId(null);
    setEditError(null);
  }

  async function handleSaveEdit(id: string) {
    setEditError(null);
    setIsSavingEdit(true);
    try {
      await apiFetch<{ budget: Budget }>(`/api/v1/budgets/${id}`, {
        method: "PUT",
        body: JSON.stringify({
          category: editCategory,
          amount: Number(editAmount),
          period: editPeriod,
        }),
      });
      setEditingId(null);
      await loadBudgets();
    } catch (err) {
      setEditError(err instanceof ApiError ? err.message : "Could not update budget.");
    } finally {
      setIsSavingEdit(false);
    }
  }

  async function handleDelete(id: string) {
    setRowError(null);
    setDeletingId(id);
    try {
      await apiFetch<null>(`/api/v1/budgets/${id}`, { method: "DELETE" });
      await loadBudgets();
    } catch (err) {
      setRowError(err instanceof ApiError ? err.message : "Could not delete budget.");
    } finally {
      setDeletingId(null);
    }
  }

  const plannedMonthly = budgets.reduce((total, budget) => {
    if (budget.period === "weekly") return total + budget.amount * 4.33;
    if (budget.period === "yearly") return total + budget.amount / 12;
    return total + budget.amount;
  }, 0);
  const actualMonthly = summary?.total_actual ?? 0;
  const remainingMonthly = plannedMonthly - actualMonthly;

  function categoryActual(category: string): number {
    return summary?.categories.find((item) => item.category.toLowerCase() === category.toLowerCase())?.actual ?? 0;
  }

  return (
    <div>
      <div className="flex items-center justify-between">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.16em] text-brand">Plan</p>
          <h1 className="mt-1 text-3xl font-semibold tracking-tight text-ink-primary">Budgets</h1>
          <p className="mt-1 text-sm text-ink-secondary">See the plan and this month&apos;s real spending in one place.</p>
        </div>
        <Button size="sm" variant={isAddOpen ? "secondary" : "primary"} onClick={() => setIsAddOpen((v) => !v)}>
          <PlusIcon size={16} />
          {isAddOpen ? "Cancel" : "Add budget"}
        </Button>
      </div>

      {!isLoading && budgets.length > 0 && (
        <div className="mt-7 grid gap-4 sm:grid-cols-3">
          <Card className="p-5">
            <p className="text-xs font-medium uppercase tracking-wide text-ink-muted">Monthly plan</p>
            <p className="mt-2 text-2xl font-semibold tabular-nums text-ink-primary">${plannedMonthly.toFixed(0)}</p>
            <p className="mt-1 text-xs text-ink-muted">Weekly and yearly budgets normalized</p>
          </Card>
          <Card className="p-5">
            <p className="text-xs font-medium uppercase tracking-wide text-ink-muted">Spent this month</p>
            <p className="mt-2 text-2xl font-semibold tabular-nums text-ink-primary">${actualMonthly.toFixed(0)}</p>
            <div className="mt-3 h-2 overflow-hidden rounded-full bg-grid">
              <div className={`h-full rounded-full ${actualMonthly > plannedMonthly ? "bg-status-critical" : "bg-brand"}`} style={{ width: `${Math.min(100, plannedMonthly > 0 ? (actualMonthly / plannedMonthly) * 100 : 0)}%` }} />
            </div>
          </Card>
          <Card className="p-5">
            <p className="text-xs font-medium uppercase tracking-wide text-ink-muted">Available</p>
            <p className={`mt-2 text-2xl font-semibold tabular-nums ${remainingMonthly < 0 ? "text-status-critical" : "text-status-good-text"}`}>
              {remainingMonthly < 0 ? "−" : ""}${Math.abs(remainingMonthly).toFixed(0)}
            </p>
            <p className="mt-1 text-xs text-ink-muted">Across all active budgets</p>
          </Card>
        </div>
      )}

      <datalist id="category-suggestions">
        {categoryNames.map((name) => (
          <option key={name} value={name} />
        ))}
      </datalist>

      {isAddOpen && (
        <Card className="mt-6 max-w-xl">
          <h2 className="text-xs font-semibold uppercase tracking-wide text-ink-muted">New budget</h2>
          <form onSubmit={handleCreate} className="mt-3 flex flex-col gap-3 sm:flex-row sm:items-end">
            <div className="flex-1">
              <Input
                id="budget-category"
                label="Category"
                required
                list="category-suggestions"
                value={category}
                onChange={(e) => setCategory(e.target.value)}
                placeholder="groceries"
              />
            </div>
            <div className="w-32">
              <Input
                id="budget-amount"
                label="Amount"
                type="number"
                step="0.01"
                min="0.01"
                required
                value={amount}
                onChange={(e) => setAmount(e.target.value)}
              />
            </div>
            <div>
              <Select
                id="budget-period"
                label="Period"
                value={period}
                onChange={(e) => setPeriod(e.target.value as (typeof PERIODS)[number])}
              >
                {PERIODS.map((p) => (
                  <option key={p} value={p}>
                    {p}
                  </option>
                ))}
              </Select>
            </div>
            <Button type="submit" disabled={isCreating}>
              {isCreating ? "Adding…" : "Add budget"}
            </Button>
          </form>
          {createError && (
            <p className="mt-3 rounded-md bg-status-critical/10 px-3 py-2 text-sm text-status-critical">
              {createError}
            </p>
          )}
        </Card>
      )}

      <Card className="mt-6 overflow-hidden">
        {isLoading && <SkeletonRows rows={4} />}
        {!isLoading && loadError && <p className="text-sm text-status-critical">{loadError}</p>}
        {!isLoading && !loadError && budgets.length === 0 && (
          <EmptyState
            icon={<TargetIcon size={24} />}
            title="No budgets yet"
            description="Add a budget to start tracking your spending."
            action={
              <Button size="sm" onClick={() => setIsAddOpen(true)}>
                <PlusIcon size={16} />
                Add budget
              </Button>
            }
          />
        )}
        {!isLoading && !loadError && budgets.length > 0 && (
          <div className="-m-6 overflow-x-auto">
            {rowError && (
              <p className="m-4 rounded-md bg-status-critical/10 px-3 py-2 text-sm text-status-critical">
                {rowError}
              </p>
            )}
            <table className="w-full min-w-[640px] text-left text-sm">
              <thead className="border-b border-hairline text-xs uppercase text-ink-muted">
                <tr>
                  <th className="px-6 py-3 font-medium">Category</th>
                  <th className="px-6 py-3 font-medium">Amount</th>
                  <th className="px-6 py-3 font-medium">This month</th>
                  <th className="px-6 py-3 font-medium">Progress</th>
                  <th className="px-6 py-3 font-medium">Period</th>
                  <th className="px-6 py-3 font-medium text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-hairline">
                {budgets.map((budget) =>
                  editingId === budget.id ? (
                    <tr key={budget.id} className="bg-plane">
                      <td className="px-6 py-3">
                        <Input
                          list="category-suggestions"
                          value={editCategory}
                          onChange={(e) => setEditCategory(e.target.value)}
                          className="w-full"
                        />
                      </td>
                      <td className="px-6 py-3 text-ink-muted">—</td>
                      <td className="px-6 py-3 text-ink-muted">—</td>
                      <td className="px-6 py-3">
                        <Input
                          type="number"
                          step="0.01"
                          value={editAmount}
                          onChange={(e) => setEditAmount(e.target.value)}
                          className="w-28"
                        />
                      </td>
                      <td className="px-6 py-3">
                        <Select
                          value={editPeriod}
                          onChange={(e) => setEditPeriod(e.target.value as (typeof PERIODS)[number])}
                        >
                          {PERIODS.map((p) => (
                            <option key={p} value={p}>
                              {p}
                            </option>
                          ))}
                        </Select>
                      </td>
                      <td className="px-6 py-3 text-right">
                        <div className="flex justify-end gap-2">
                          <Button size="sm" onClick={() => handleSaveEdit(budget.id)} disabled={isSavingEdit}>
                            {isSavingEdit ? "Saving…" : "Save"}
                          </Button>
                          <Button size="sm" variant="secondary" onClick={cancelEdit}>
                            Cancel
                          </Button>
                        </div>
                        {editError && <p className="mt-2 text-xs text-status-critical">{editError}</p>}
                      </td>
                    </tr>
                  ) : (
                    <tr key={budget.id}>
                      <td className="px-6 py-3 font-medium capitalize text-ink-primary">{budget.category}</td>
                      <td className="px-6 py-3 tabular-nums text-ink-secondary">{budget.amount.toFixed(2)}</td>
                      <td className="px-6 py-3 tabular-nums text-ink-secondary">${categoryActual(budget.category).toFixed(2)}</td>
                      <td className="px-6 py-3">
                        <div className="w-28">
                          <div className="h-2 overflow-hidden rounded-full bg-grid">
                            <div
                              className={`h-full rounded-full ${categoryActual(budget.category) > budget.amount ? "bg-status-critical" : "bg-brand"}`}
                              style={{ width: `${Math.min(100, (categoryActual(budget.category) / budget.amount) * 100)}%` }}
                            />
                          </div>
                          <p className="mt-1 text-[11px] tabular-nums text-ink-muted">
                            {Math.round((categoryActual(budget.category) / budget.amount) * 100)}%
                          </p>
                        </div>
                      </td>
                      <td className="px-6 py-3 capitalize text-ink-secondary">{budget.period}</td>
                      <td className="px-6 py-3 text-right">
                        <div className="flex justify-end gap-1">
                          <Button
                            variant="ghost"
                            size="sm"
                            aria-label="Edit budget"
                            onClick={() => startEdit(budget)}
                          >
                            <PencilIcon size={16} />
                          </Button>
                          <Button
                            variant="ghost"
                            size="sm"
                            aria-label="Delete budget"
                            disabled={deletingId === budget.id}
                            onClick={() => handleDelete(budget.id)}
                            className="text-status-critical hover:bg-status-critical/10"
                          >
                            <TrashIcon size={16} />
                          </Button>
                        </div>
                      </td>
                    </tr>
                  )
                )}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </div>
  );
}
