"use client";

// Accounts screen: full CRUD against expense-service via the gateway.
// GET/POST /api/v1/accounts, PUT/DELETE /api/v1/accounts/:id.

import { useEffect, useState, type FormEvent } from "react";
import { apiFetch, ApiError } from "@/lib/api";
import type { Account } from "@/lib/types";
import { Button } from "@/components/ui/Button";
import { Input, Select } from "@/components/ui/Input";
import { Card } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";
import { EmptyState } from "@/components/ui/EmptyState";
import { SkeletonRows } from "@/components/ui/Skeleton";
import { PencilIcon, PlusIcon, TrashIcon, WalletIcon } from "@/components/icons";

// Must match expense-service's domain.ValidAccountType exactly
// (services/expense-service/internal/domain/account.go) — anything else is a
// guaranteed 400 VALIDATION_ERROR from the backend.
const ACCOUNT_TYPES = ["checking", "savings", "credit", "cash"];

export default function AccountsPage() {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);

  // create form
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [name, setName] = useState("");
  const [type, setType] = useState(ACCOUNT_TYPES[0]);
  const [currency, setCurrency] = useState("USD");
  const [isCreating, setIsCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  // per-row edit state
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editName, setEditName] = useState("");
  const [editType, setEditType] = useState("");
  const [editCurrency, setEditCurrency] = useState("");
  const [editBalance, setEditBalance] = useState("");
  const [isSavingEdit, setIsSavingEdit] = useState(false);
  const [editError, setEditError] = useState<string | null>(null);

  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [rowError, setRowError] = useState<string | null>(null);

  async function loadAccounts() {
    try {
      const data = await apiFetch<{ accounts: Account[] }>("/api/v1/accounts");
      setAccounts(data.accounts ?? []);
      setLoadError(null);
    } catch (err) {
      setLoadError(err instanceof ApiError ? err.message : "Could not load accounts.");
    } finally {
      setIsLoading(false);
    }
  }

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const data = await apiFetch<{ accounts: Account[] }>("/api/v1/accounts");
        if (!cancelled) setAccounts(data.accounts ?? []);
      } catch (err) {
        if (!cancelled) {
          setLoadError(err instanceof ApiError ? err.message : "Could not load accounts.");
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
      await apiFetch<{ account: Account }>("/api/v1/accounts", {
        method: "POST",
        body: JSON.stringify({ name, type, currency }),
      });
      setName("");
      setType(ACCOUNT_TYPES[0]);
      setCurrency("USD");
      setShowCreateForm(false);
      await loadAccounts();
    } catch (err) {
      setCreateError(err instanceof ApiError ? err.message : "Could not create account.");
    } finally {
      setIsCreating(false);
    }
  }

  function startEdit(account: Account) {
    setEditingId(account.id);
    setEditName(account.name);
    setEditType(account.type);
    setEditCurrency(account.currency);
    setEditBalance(String(account.balance));
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
      const parsedBalance = Number(editBalance);
      await apiFetch<{ account: Account }>(`/api/v1/accounts/${id}`, {
        method: "PUT",
        body: JSON.stringify({
          name: editName,
          type: editType,
          currency: editCurrency,
          balance: Number.isNaN(parsedBalance) ? undefined : parsedBalance,
        }),
      });
      setEditingId(null);
      await loadAccounts();
    } catch (err) {
      setEditError(err instanceof ApiError ? err.message : "Could not update account.");
    } finally {
      setIsSavingEdit(false);
    }
  }

  async function handleDelete(id: string) {
    setRowError(null);
    setDeletingId(id);
    try {
      await apiFetch<null>(`/api/v1/accounts/${id}`, { method: "DELETE" });
      setAccounts((prev) => prev.filter((a) => a.id !== id));
    } catch (err) {
      setRowError(err instanceof ApiError ? err.message : "Could not delete account.");
    } finally {
      setDeletingId(null);
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-ink-primary">Accounts</h1>
        <Button
          type="button"
          variant={showCreateForm ? "secondary" : "primary"}
          size="sm"
          onClick={() => setShowCreateForm((prev) => !prev)}
        >
          <PlusIcon size={16} />
          {showCreateForm ? "Cancel" : "Add account"}
        </Button>
      </div>

      {showCreateForm && (
        <Card className="mt-6 max-w-xl">
          <h2 className="text-xs font-semibold uppercase tracking-wide text-ink-muted">New account</h2>
          <form onSubmit={handleCreate} className="mt-3 flex flex-col gap-3 sm:flex-row sm:items-end">
            <div className="flex-1">
              <Input
                id="acc-name"
                label="Name"
                required
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Chase Checking"
              />
            </div>
            <Select id="acc-type" label="Type" value={type} onChange={(e) => setType(e.target.value)}>
              {ACCOUNT_TYPES.map((t) => (
                <option key={t} value={t}>
                  {t}
                </option>
              ))}
            </Select>
            <Input
              id="acc-currency"
              label="Currency"
              wrapperClassName="w-24"
              required
              value={currency}
              onChange={(e) => setCurrency(e.target.value.toUpperCase())}
              maxLength={3}
            />
            <Button type="submit" disabled={isCreating}>
              {isCreating ? "Adding…" : "Add account"}
            </Button>
          </form>

          {createError && (
            <p className="mt-3 rounded-md bg-status-critical/10 px-3 py-2 text-sm text-status-critical">
              {createError}
            </p>
          )}
        </Card>
      )}

      <Card className="mt-6 p-0">
        {isLoading && <SkeletonRows rows={4} />}
        {!isLoading && loadError && (
          <p className="p-6 text-sm text-status-critical">{loadError}</p>
        )}
        {!isLoading && !loadError && accounts.length === 0 && (
          <EmptyState
            icon={<WalletIcon size={24} />}
            title="No accounts yet"
            description="Add an account above to start tracking balances and transactions."
          />
        )}
        {!isLoading && !loadError && accounts.length > 0 && (
          <div className="overflow-x-auto">
            {rowError && (
              <p className="m-4 rounded-md bg-status-critical/10 px-3 py-2 text-sm text-status-critical">
                {rowError}
              </p>
            )}
            <table className="w-full min-w-[640px] text-left text-sm">
              <thead className="border-b border-hairline text-xs uppercase text-ink-muted">
                <tr>
                  <th className="px-6 py-3 font-medium">Name</th>
                  <th className="px-6 py-3 font-medium">Type</th>
                  <th className="px-6 py-3 font-medium">Currency</th>
                  <th className="px-6 py-3 font-medium">Balance</th>
                  <th className="px-6 py-3 font-medium text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-hairline">
                {accounts.map((account) =>
                  editingId === account.id ? (
                    <tr key={account.id} className="bg-plane">
                      <td className="px-6 py-3">
                        <Input
                          value={editName}
                          onChange={(e) => setEditName(e.target.value)}
                          className="w-full"
                        />
                      </td>
                      <td className="px-6 py-3">
                        <Select value={editType} onChange={(e) => setEditType(e.target.value)}>
                          {ACCOUNT_TYPES.map((t) => (
                            <option key={t} value={t}>
                              {t}
                            </option>
                          ))}
                        </Select>
                      </td>
                      <td className="px-6 py-3">
                        <Input
                          value={editCurrency}
                          onChange={(e) => setEditCurrency(e.target.value.toUpperCase())}
                          className="w-20"
                          maxLength={3}
                        />
                      </td>
                      <td className="px-6 py-3">
                        <Input
                          type="number"
                          step="0.01"
                          value={editBalance}
                          onChange={(e) => setEditBalance(e.target.value)}
                          className="w-28 tabular-nums"
                        />
                      </td>
                      <td className="px-6 py-3 text-right">
                        <div className="flex justify-end gap-2">
                          <Button size="sm" onClick={() => handleSaveEdit(account.id)} disabled={isSavingEdit}>
                            {isSavingEdit ? "Saving…" : "Save"}
                          </Button>
                          <Button variant="secondary" size="sm" onClick={cancelEdit}>
                            Cancel
                          </Button>
                        </div>
                        {editError && (
                          <p className="mt-2 text-xs text-status-critical">{editError}</p>
                        )}
                      </td>
                    </tr>
                  ) : (
                    <tr key={account.id} className="hover:bg-plane">
                      <td className="px-6 py-3 font-medium text-ink-primary">{account.name}</td>
                      <td className="px-6 py-3">
                        <Badge status="neutral">{account.type}</Badge>
                      </td>
                      <td className="px-6 py-3 text-ink-secondary">{account.currency}</td>
                      <td className="px-6 py-3 tabular-nums text-ink-primary">
                        {account.balance.toFixed(2)}
                      </td>
                      <td className="px-6 py-3 text-right">
                        <div className="flex justify-end gap-1">
                          <Button
                            variant="ghost"
                            size="sm"
                            aria-label="Edit account"
                            onClick={() => startEdit(account)}
                          >
                            <PencilIcon size={16} />
                          </Button>
                          <Button
                            variant="ghost"
                            size="sm"
                            aria-label="Delete account"
                            onClick={() => handleDelete(account.id)}
                            disabled={deletingId === account.id}
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
