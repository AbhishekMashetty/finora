"use client";

// Goals screen: full CRUD against budget-service via the gateway.
// GET/POST /api/v1/goals, PUT/DELETE /api/v1/goals/:id.
//
// `current_amount` is manual progress logging (not computed from
// transactions) — the "Log progress" control below just PUTs a new
// absolute current_amount value, per the contract.

import { useEffect, useState, type FormEvent } from "react";
import { Flag, Pencil, Plus, Trash2 } from "lucide-react";
import { apiFetch, ApiError } from "@/lib/api";
import { formatDate, formatNumber } from "@/lib/format";
import type { Goal } from "@/lib/types";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Card } from "@/components/ui/Card";
import { EmptyState } from "@/components/ui/EmptyState";
import { SkeletonRows } from "@/components/ui/Skeleton";
import { Alert } from "@/components/ui/Alert";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";

function defaultTargetDate(): string {
  const d = new Date();
  d.setMonth(d.getMonth() + 6);
  return d.toISOString().slice(0, 10);
}

export default function GoalsPage() {
  const [goals, setGoals] = useState<Goal[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);

  // create form
  const [isAddOpen, setIsAddOpen] = useState(false);
  const [name, setName] = useState("");
  const [targetAmount, setTargetAmount] = useState("");
  const [targetDate, setTargetDate] = useState(defaultTargetDate());
  const [isCreating, setIsCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  // edit-the-rest (name/target_amount/target_date)
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editName, setEditName] = useState("");
  const [editTargetAmount, setEditTargetAmount] = useState("");
  const [editTargetDate, setEditTargetDate] = useState("");
  const [isSavingEdit, setIsSavingEdit] = useState(false);
  const [editError, setEditError] = useState<string | null>(null);

  // log-progress inline input, per goal
  const [progressInputs, setProgressInputs] = useState<Record<string, string>>({});
  const [savingProgressId, setSavingProgressId] = useState<string | null>(null);
  const [progressError, setProgressError] = useState<Record<string, string>>({});

  const [deletingId, setDeletingId] = useState<string | null>(null);

  async function loadGoals() {
    try {
      const data = await apiFetch<{ goals: Goal[] }>("/api/v1/goals");
      setGoals(data.goals ?? []);
      setLoadError(null);
    } catch (err) {
      setLoadError(err instanceof ApiError ? err.message : "Could not load goals.");
    } finally {
      setIsLoading(false);
    }
  }

  useEffect(() => {
    loadGoals();
  }, []);

  async function handleCreate(event: FormEvent) {
    event.preventDefault();
    setCreateError(null);
    setIsCreating(true);
    try {
      await apiFetch<{ goal: Goal }>("/api/v1/goals", {
        method: "POST",
        body: JSON.stringify({
          name,
          target_amount: Number(targetAmount),
          target_date: targetDate,
        }),
      });
      setName("");
      setTargetAmount("");
      setTargetDate(defaultTargetDate());
      setIsAddOpen(false);
      await loadGoals();
    } catch (err) {
      setCreateError(err instanceof ApiError ? err.message : "Could not create goal.");
    } finally {
      setIsCreating(false);
    }
  }

  function startEdit(goal: Goal) {
    setEditingId(goal.id);
    setEditName(goal.name);
    setEditTargetAmount(String(goal.target_amount));
    setEditTargetDate(goal.target_date.slice(0, 10));
    setEditError(null);
  }

  function cancelEdit() {
    setEditingId(null);
    setEditError(null);
  }

  async function handleSaveEdit(goal: Goal) {
    setEditError(null);
    setIsSavingEdit(true);
    try {
      await apiFetch<{ goal: Goal }>(`/api/v1/goals/${goal.id}`, {
        method: "PUT",
        body: JSON.stringify({
          name: editName,
          target_amount: Number(editTargetAmount),
          target_date: editTargetDate,
          current_amount: goal.current_amount,
        }),
      });
      setEditingId(null);
      await loadGoals();
    } catch (err) {
      setEditError(err instanceof ApiError ? err.message : "Could not update goal.");
    } finally {
      setIsSavingEdit(false);
    }
  }

  async function handleLogProgress(goal: Goal) {
    const raw = progressInputs[goal.id];
    const value = Number(raw);
    if (raw === undefined || raw === "" || Number.isNaN(value)) {
      setProgressError((prev) => ({ ...prev, [goal.id]: "Enter a valid amount." }));
      return;
    }
    setProgressError((prev) => ({ ...prev, [goal.id]: "" }));
    setSavingProgressId(goal.id);
    try {
      await apiFetch<{ goal: Goal }>(`/api/v1/goals/${goal.id}`, {
        method: "PUT",
        body: JSON.stringify({
          name: goal.name,
          target_amount: goal.target_amount,
          target_date: goal.target_date.slice(0, 10),
          current_amount: value,
        }),
      });
      setProgressInputs((prev) => ({ ...prev, [goal.id]: "" }));
      await loadGoals();
    } catch (err) {
      setProgressError((prev) => ({
        ...prev,
        [goal.id]: err instanceof ApiError ? err.message : "Could not log progress.",
      }));
    } finally {
      setSavingProgressId(null);
    }
  }

  async function handleDelete(id: string) {
    setDeletingId(id);
    try {
      await apiFetch<null>(`/api/v1/goals/${id}`, { method: "DELETE" });
      setGoals((prev) => prev.filter((g) => g.id !== id));
    } catch (err) {
      setProgressError((prev) => ({
        ...prev,
        [id]: err instanceof ApiError ? err.message : "Could not delete goal.",
      }));
    } finally {
      setDeletingId(null);
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between">
        <h1 className="font-display text-2xl font-medium text-ink-primary">Goals</h1>
        <Button size="sm" variant={isAddOpen ? "secondary" : "primary"} onClick={() => setIsAddOpen((v) => !v)}>
          <Plus size={16} strokeWidth={1.75} />
          {isAddOpen ? "Cancel" : "Add goal"}
        </Button>
      </div>

      {isAddOpen && (
        <Card className="mt-6 max-w-xl">
          <h2 className="text-xs font-semibold uppercase tracking-wide text-ink-muted">New goal</h2>
          <form onSubmit={handleCreate} className="mt-3 flex flex-col gap-3 sm:flex-row sm:items-end">
            <div className="flex-1">
              <Input
                id="goal-name"
                label="Name"
                required
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Emergency fund"
              />
            </div>
            <div className="w-32">
              <Input
                id="goal-amount"
                label="Target amount"
                type="number"
                step="0.01"
                min="0.01"
                required
                value={targetAmount}
                onChange={(e) => setTargetAmount(e.target.value)}
              />
            </div>
            <div>
              <Input
                id="goal-date"
                label="Target date"
                type="date"
                required
                value={targetDate}
                onChange={(e) => setTargetDate(e.target.value)}
              />
            </div>
            <Button type="submit" disabled={isCreating}>
              {isCreating ? "Adding…" : "Add goal"}
            </Button>
          </form>
          {createError && <Alert variant="error" className="mt-3">{createError}</Alert>}
        </Card>
      )}

      <div className="mt-6">
        {isLoading && (
          <Card>
            <SkeletonRows rows={4} />
          </Card>
        )}
        {!isLoading && loadError && (
          <Card>
            <Alert variant="error">{loadError}</Alert>
          </Card>
        )}
        {!isLoading && !loadError && goals.length === 0 && (
          <Card>
            <EmptyState
              icon={<Flag size={24} strokeWidth={1.75} />}
              title="No goals yet"
              description="Set a savings goal to track your progress."
              action={
                <Button size="sm" onClick={() => setIsAddOpen(true)}>
                  <Plus size={16} strokeWidth={1.75} />
                  Add goal
                </Button>
              }
            />
          </Card>
        )}

        {!isLoading && !loadError && goals.length > 0 && (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            {goals.map((goal) => {
              const pct =
                goal.target_amount > 0
                  ? Math.min(100, Math.round((goal.current_amount / goal.target_amount) * 100))
                  : 0;
              const isEditing = editingId === goal.id;
              return (
                <Card key={goal.id}>
                  {isEditing ? (
                    <div className="flex flex-col gap-2">
                      <Input value={editName} onChange={(e) => setEditName(e.target.value)} />
                      <Input
                        type="number"
                        step="0.01"
                        value={editTargetAmount}
                        onChange={(e) => setEditTargetAmount(e.target.value)}
                      />
                      <Input
                        type="date"
                        value={editTargetDate}
                        onChange={(e) => setEditTargetDate(e.target.value)}
                      />
                      <div className="flex gap-2">
                        <Button size="sm" onClick={() => handleSaveEdit(goal)} disabled={isSavingEdit}>
                          {isSavingEdit ? "Saving…" : "Save"}
                        </Button>
                        <Button size="sm" variant="secondary" onClick={cancelEdit}>
                          Cancel
                        </Button>
                      </div>
                      {editError && <p className="text-xs text-status-critical">{editError}</p>}
                    </div>
                  ) : (
                    <>
                      <div className="flex items-start justify-between">
                        <h3 className="text-lg font-medium text-ink-primary">{goal.name}</h3>
                        <div className="flex gap-1">
                          <Button
                            variant="ghost"
                            size="sm"
                            aria-label="Edit goal"
                            onClick={() => startEdit(goal)}
                          >
                            <Pencil size={16} strokeWidth={1.75} />
                          </Button>
                          <ConfirmDialog
                            trigger={
                              <Button
                                variant="ghost"
                                size="sm"
                                aria-label="Delete goal"
                                disabled={deletingId === goal.id}
                              >
                                <Trash2 size={16} strokeWidth={1.75} />
                              </Button>
                            }
                            title={`Delete "${goal.name}"?`}
                            description="This can't be undone."
                            onConfirm={() => handleDelete(goal.id)}
                          />
                        </div>
                      </div>
                      <p className="mt-1 text-sm text-ink-muted">
                        Target: <span className="tabular-nums">{formatNumber(goal.target_amount)}</span> by{" "}
                        {formatDate(goal.target_date)}
                      </p>

                      <div className="mt-3">
                        <div className="h-2 w-full overflow-hidden rounded-pill bg-ink-muted/15">
                          <div
                            className="h-full rounded-pill bg-brand transition-[width] duration-[var(--duration-slow)]"
                            style={{ width: `${pct}%` }}
                          />
                        </div>
                        <p className="mt-1 text-xs tabular-nums text-ink-muted">
                          {formatNumber(goal.current_amount)} / {formatNumber(goal.target_amount)} ({pct}%)
                        </p>
                      </div>

                      <div className="mt-3 flex items-end gap-2">
                        <div className="flex-1">
                          <Input
                            label="Log progress (new saved total)"
                            type="number"
                            step="0.01"
                            value={progressInputs[goal.id] ?? ""}
                            onChange={(e) =>
                              setProgressInputs((prev) => ({ ...prev, [goal.id]: e.target.value }))
                            }
                            placeholder={String(goal.current_amount)}
                          />
                        </div>
                        <Button
                          size="sm"
                          variant="secondary"
                          className="h-10"
                          onClick={() => handleLogProgress(goal)}
                          disabled={savingProgressId === goal.id}
                        >
                          {savingProgressId === goal.id ? "Saving…" : "Update"}
                        </Button>
                      </div>
                      {progressError[goal.id] && (
                        <p className="mt-1 text-xs text-status-critical">{progressError[goal.id]}</p>
                      )}
                    </>
                  )}
                </Card>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
