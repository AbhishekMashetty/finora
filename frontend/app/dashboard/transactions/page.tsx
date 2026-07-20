"use client";

// Transactions screen: full CRUD + pagination/filtering against
// expense-service via the gateway (GET/POST /api/v1/transactions,
// PUT/DELETE /api/v1/transactions/:id), plus inline category management
// (GET/POST /api/v1/categories — categories are create+list only, no
// update/delete, by deliberate backend design).
//
// Wrinkle from architecture/api-contracts.md: the query param is named
// `category` but its value is a category **id**.

import Link from "next/link";
import { useEffect, useState, type ChangeEvent, type FormEvent } from "react";
import { apiFetch, ApiError } from "@/lib/api";
import type { Account, Category, ImportResult, Transaction } from "@/lib/types";
import { Button } from "@/components/ui/Button";
import { Input, Select } from "@/components/ui/Input";
import { Card } from "@/components/ui/Card";
import { TransactionTypeBadge } from "@/components/ui/Badge";
import { EmptyState } from "@/components/ui/EmptyState";
import { SkeletonRows } from "@/components/ui/Skeleton";
import { ListIcon, PencilIcon, PlusIcon, TrashIcon, UploadIcon } from "@/components/icons";

const PAGE_SIZE = 20;

function todayISO(): string {
  return new Date().toISOString().slice(0, 10);
}

interface TxFormState {
  accountId: string;
  categoryId: string;
  type: "income" | "expense";
  amount: string;
  currency: string;
  date: string;
  note: string;
}

function emptyForm(defaultAccountId: string): TxFormState {
  return {
    accountId: defaultAccountId,
    categoryId: "",
    type: "expense",
    amount: "",
    currency: "USD",
    date: todayISO(),
    note: "",
  };
}

export default function TransactionsPage() {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [refDataLoaded, setRefDataLoaded] = useState(false);
  const [refDataError, setRefDataError] = useState<string | null>(null);

  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);

  // filters
  const [filterAccountId, setFilterAccountId] = useState("");
  const [filterCategoryId, setFilterCategoryId] = useState("");
  const [filterFrom, setFilterFrom] = useState("");
  const [filterTo, setFilterTo] = useState("");

  // create form
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [form, setForm] = useState<TxFormState>(emptyForm(""));
  const [isCreating, setIsCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  // inline "+ new category"
  const [showNewCategory, setShowNewCategory] = useState(false);
  const [newCategoryName, setNewCategoryName] = useState("");
  const [newCategoryType, setNewCategoryType] = useState<"income" | "expense">("expense");
  const [isCreatingCategory, setIsCreatingCategory] = useState(false);
  const [categoryError, setCategoryError] = useState<string | null>(null);

  // CSV import (e.g. a downloaded credit card statement)
  const [showImportForm, setShowImportForm] = useState(false);
  const [importAccountId, setImportAccountId] = useState("");
  const [importFile, setImportFile] = useState<File | null>(null);
  const [isImporting, setIsImporting] = useState(false);
  const [importError, setImportError] = useState<string | null>(null);
  const [importResult, setImportResult] = useState<ImportResult | null>(null);

  // per-row edit
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editForm, setEditForm] = useState<TxFormState>(emptyForm(""));
  const [isSavingEdit, setIsSavingEdit] = useState(false);
  const [editError, setEditError] = useState<string | null>(null);

  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [rowError, setRowError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const [accountsData, categoriesData] = await Promise.all([
          apiFetch<{ accounts: Account[] }>("/api/v1/accounts"),
          apiFetch<{ categories: Category[] }>("/api/v1/categories"),
        ]);
        if (!cancelled) {
          setAccounts(accountsData.accounts ?? []);
          setCategories(categoriesData.categories ?? []);
          setForm((prev) => ({
            ...prev,
            accountId: prev.accountId || accountsData.accounts?.[0]?.id || "",
          }));
          setImportAccountId((prev) => prev || accountsData.accounts?.[0]?.id || "");
        }
      } catch (err) {
        if (!cancelled) {
          setRefDataError(err instanceof ApiError ? err.message : "Could not load accounts/categories.");
        }
      } finally {
        if (!cancelled) setRefDataLoaded(true);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setIsLoading(true);
      try {
        const params = new URLSearchParams();
        params.set("page", String(page));
        params.set("page_size", String(PAGE_SIZE));
        if (filterAccountId) params.set("account_id", filterAccountId);
        if (filterCategoryId) params.set("category", filterCategoryId);
        if (filterFrom) params.set("from", filterFrom);
        if (filterTo) params.set("to", filterTo);

        const data = await apiFetch<{ transactions: Transaction[]; page: number; total: number }>(
          `/api/v1/transactions?${params.toString()}`
        );
        if (!cancelled) {
          setTransactions(data.transactions ?? []);
          setTotal(data.total ?? 0);
          setLoadError(null);
        }
      } catch (err) {
        if (!cancelled) {
          setLoadError(err instanceof ApiError ? err.message : "Could not load transactions.");
        }
      } finally {
        if (!cancelled) setIsLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [page, filterAccountId, filterCategoryId, filterFrom, filterTo]);

  function categoryLabel(id: string | null): string {
    if (!id) return "—";
    const cat = categories.find((c) => c.id === id);
    return cat ? cat.name : "(deleted category)";
  }

  function accountLabel(id: string): string {
    const acc = accounts.find((a) => a.id === id);
    return acc ? acc.name : id;
  }

  async function refreshCurrentPage() {
    // Re-run the same query by nudging page state via a no-op filter change
    // isn't clean; simplest is to just re-fetch directly.
    try {
      const params = new URLSearchParams();
      params.set("page", String(page));
      params.set("page_size", String(PAGE_SIZE));
      if (filterAccountId) params.set("account_id", filterAccountId);
      if (filterCategoryId) params.set("category", filterCategoryId);
      if (filterFrom) params.set("from", filterFrom);
      if (filterTo) params.set("to", filterTo);
      const data = await apiFetch<{ transactions: Transaction[]; page: number; total: number }>(
        `/api/v1/transactions?${params.toString()}`
      );
      setTransactions(data.transactions ?? []);
      setTotal(data.total ?? 0);
    } catch (err) {
      setLoadError(err instanceof ApiError ? err.message : "Could not reload transactions.");
    }
  }

  async function handleCreate(event: FormEvent) {
    event.preventDefault();
    setCreateError(null);
    setIsCreating(true);
    try {
      const amountNum = Number(form.amount);
      await apiFetch<{ transaction: Transaction }>("/api/v1/transactions", {
        method: "POST",
        body: JSON.stringify({
          account_id: form.accountId,
          category_id: form.categoryId || null,
          type: form.type,
          amount: amountNum,
          currency: form.currency,
          date: form.date,
          note: form.note || undefined,
        }),
      });
      setForm(emptyForm(form.accountId));
      setShowCreateForm(false);
      if (page !== 1) {
        setPage(1);
      } else {
        await refreshCurrentPage();
      }
    } catch (err) {
      setCreateError(err instanceof ApiError ? err.message : "Could not create transaction.");
    } finally {
      setIsCreating(false);
    }
  }

  function handleImportFileChange(event: ChangeEvent<HTMLInputElement>) {
    setImportFile(event.target.files?.[0] ?? null);
  }

  async function handleImport(event: FormEvent) {
    event.preventDefault();
    setImportError(null);
    setImportResult(null);
    if (!importFile) {
      setImportError("Choose a CSV file first.");
      return;
    }
    setIsImporting(true);
    try {
      const body = new FormData();
      body.append("account_id", importAccountId);
      body.append("file", importFile);
      const result = await apiFetch<ImportResult>("/api/v1/transactions/import", {
        method: "POST",
        body,
      });
      setImportResult(result);
      setImportFile(null);
      if (page !== 1) {
        setPage(1);
      } else {
        await refreshCurrentPage();
      }
    } catch (err) {
      setImportError(err instanceof ApiError ? err.message : "Could not import transactions.");
    } finally {
      setIsImporting(false);
    }
  }

  async function handleCreateCategory(event: FormEvent) {
    event.preventDefault();
    setCategoryError(null);
    setIsCreatingCategory(true);
    try {
      const data = await apiFetch<{ category: Category }>("/api/v1/categories", {
        method: "POST",
        body: JSON.stringify({ name: newCategoryName, type: newCategoryType }),
      });
      setCategories((prev) => [...prev, data.category]);
      setForm((prev) => ({ ...prev, categoryId: data.category.id }));
      setNewCategoryName("");
      setShowNewCategory(false);
    } catch (err) {
      setCategoryError(err instanceof ApiError ? err.message : "Could not create category.");
    } finally {
      setIsCreatingCategory(false);
    }
  }

  function startEdit(tx: Transaction) {
    setEditingId(tx.id);
    setEditForm({
      accountId: tx.account_id,
      categoryId: tx.category_id ?? "",
      type: tx.type,
      amount: String(tx.amount),
      currency: tx.currency,
      date: tx.date.slice(0, 10),
      note: tx.note ?? "",
    });
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
      const amountNum = Number(editForm.amount);
      await apiFetch<{ transaction: Transaction }>(`/api/v1/transactions/${id}`, {
        method: "PUT",
        body: JSON.stringify({
          account_id: editForm.accountId,
          category_id: editForm.categoryId || null,
          type: editForm.type,
          amount: amountNum,
          currency: editForm.currency,
          date: editForm.date,
          note: editForm.note || undefined,
        }),
      });
      setEditingId(null);
      await refreshCurrentPage();
    } catch (err) {
      setEditError(err instanceof ApiError ? err.message : "Could not update transaction.");
    } finally {
      setIsSavingEdit(false);
    }
  }

  async function handleDelete(id: string) {
    setRowError(null);
    setDeletingId(id);
    try {
      await apiFetch<null>(`/api/v1/transactions/${id}`, { method: "DELETE" });
      await refreshCurrentPage();
    } catch (err) {
      setRowError(err instanceof ApiError ? err.message : "Could not delete transaction.");
    } finally {
      setDeletingId(null);
    }
  }

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const hasAccounts = accounts.length > 0;

  return (
    <div>
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-ink-primary">Transactions</h1>
        {refDataLoaded && hasAccounts && (
          <div className="flex gap-2">
            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={() => {
                setShowImportForm((prev) => !prev);
                setShowCreateForm(false);
              }}
            >
              <UploadIcon size={16} />
              {showImportForm ? "Cancel" : "Import CSV"}
            </Button>
            <Button
              type="button"
              variant={showCreateForm ? "secondary" : "primary"}
              size="sm"
              onClick={() => {
                setShowCreateForm((prev) => !prev);
                setShowImportForm(false);
              }}
            >
              <PlusIcon size={16} />
              {showCreateForm ? "Cancel" : "Add transaction"}
            </Button>
          </div>
        )}
      </div>

      {/* Filters */}
      <Card className="mt-6">
        <h2 className="text-xs font-semibold uppercase tracking-wide text-ink-muted">Filters</h2>
        {/* A grid, not a flex-wrap row: account/category names vary a lot in
            length, and a native date input has a different intrinsic width
            than a <select> — left to their own content width, the four
            controls never lined up. Fixed columns + w-full on each control
            keeps every field (and its label) aligned in a clean row, and
            wraps predictably (2x2, then stacked) on smaller screens instead
            of raggedly. */}
        <div className="mt-3 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <Select
            label="Account"
            className="w-full"
            value={filterAccountId}
            onChange={(e) => {
              setFilterAccountId(e.target.value);
              setPage(1);
            }}
          >
            <option value="">All accounts</option>
            {accounts.map((a) => (
              <option key={a.id} value={a.id}>
                {a.name}
              </option>
            ))}
          </Select>
          <Select
            label="Category"
            className="w-full"
            value={filterCategoryId}
            onChange={(e) => {
              setFilterCategoryId(e.target.value);
              setPage(1);
            }}
          >
            <option value="">All categories</option>
            {categories.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name} ({c.type})
              </option>
            ))}
          </Select>
          <Input
            type="date"
            label="From"
            className="w-full"
            value={filterFrom}
            onChange={(e) => {
              setFilterFrom(e.target.value);
              setPage(1);
            }}
          />
          <Input
            type="date"
            label="To"
            className="w-full"
            value={filterTo}
            onChange={(e) => {
              setFilterTo(e.target.value);
              setPage(1);
            }}
          />
        </div>
        {(filterAccountId || filterCategoryId || filterFrom || filterTo) && (
          <div className="mt-3 flex justify-end">
            <Button
              variant="secondary"
              size="sm"
              onClick={() => {
                setFilterAccountId("");
                setFilterCategoryId("");
                setFilterFrom("");
                setFilterTo("");
                setPage(1);
              }}
            >
              Clear filters
            </Button>
          </div>
        )}
      </Card>

      {/* Create form */}
      {refDataLoaded && !hasAccounts && (
        <Card className="mt-6">
          <p className="text-sm text-ink-secondary">
            You need an account before you can log a transaction —{" "}
            <Link href="/dashboard/accounts" className="font-medium text-brand underline underline-offset-2">
              create an account first
            </Link>
            .
          </p>
        </Card>
      )}

      {refDataError && (
        <Card className="mt-6">
          <p className="text-sm text-status-critical">{refDataError}</p>
        </Card>
      )}

      {refDataLoaded && hasAccounts && showCreateForm && (
        <Card className="mt-6">
          <h2 className="text-xs font-semibold uppercase tracking-wide text-ink-muted">New transaction</h2>
          <form onSubmit={handleCreate} className="mt-4 flex flex-col gap-4">
            {/* A grid, not a flex-wrap row: Account's option text, a
                2-letter Type, a number, a 3-letter currency code, and a
                date all have wildly different intrinsic widths — a
                fixed-span grid keeps every field's label and control
                aligned instead of raggedly sized to its own content.
                Column spans go on wrapperClassName, not className: the
                Field wrapper div is the actual grid item, not the inner
                <select>/<input> — see Input.tsx's doc comment. */}
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-12">
              <Select
                label="Account"
                wrapperClassName="lg:col-span-3"
                value={form.accountId}
                onChange={(e) => setForm({ ...form, accountId: e.target.value })}
                required
              >
                {accounts.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name}
                  </option>
                ))}
              </Select>
              <Select
                label="Type"
                wrapperClassName="lg:col-span-2"
                value={form.type}
                onChange={(e) => setForm({ ...form, type: e.target.value as "income" | "expense" })}
              >
                <option value="expense">Expense</option>
                <option value="income">Income</option>
              </Select>
              <Input
                type="number"
                step="0.01"
                min="0.01"
                label="Amount"
                placeholder="0.00"
                className="tabular-nums"
                wrapperClassName="lg:col-span-2"
                required
                value={form.amount}
                onChange={(e) => setForm({ ...form, amount: e.target.value })}
              />
              <Input
                label="Currency"
                wrapperClassName="lg:col-span-2"
                required
                value={form.currency}
                onChange={(e) => setForm({ ...form, currency: e.target.value.toUpperCase() })}
                maxLength={3}
              />
              <Input
                type="date"
                label="Date"
                wrapperClassName="lg:col-span-3"
                required
                value={form.date}
                onChange={(e) => setForm({ ...form, date: e.target.value })}
              />
            </div>

            <div className="flex flex-wrap items-end gap-3">
              <Select
                label="Category"
                wrapperClassName="w-48"
                value={form.categoryId}
                onChange={(e) => setForm({ ...form, categoryId: e.target.value })}
              >
                <option value="">None</option>
                {categories.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name} ({c.type})
                  </option>
                ))}
              </Select>
              <Button
                type="button"
                variant="secondary"
                size="sm"
                className="h-10"
                onClick={() => setShowNewCategory((prev) => !prev)}
              >
                <PlusIcon size={14} />
                {showNewCategory ? "Cancel" : "New category"}
              </Button>
              <div className="min-w-[200px] flex-1">
                <Input
                  label="Note"
                  value={form.note}
                  onChange={(e) => setForm({ ...form, note: e.target.value })}
                  placeholder="Optional"
                />
              </div>
            </div>

            {showNewCategory && (
              <div className="flex flex-wrap items-end gap-3 rounded-md border border-dashed border-hairline p-3">
                <Input
                  label="New category name"
                  wrapperClassName="w-56"
                  value={newCategoryName}
                  onChange={(e) => setNewCategoryName(e.target.value)}
                  placeholder="Groceries"
                />
                <Select
                  label="Type"
                  value={newCategoryType}
                  onChange={(e) => setNewCategoryType(e.target.value as "income" | "expense")}
                >
                  <option value="expense">Expense</option>
                  <option value="income">Income</option>
                </Select>
                <Button
                  type="button"
                  size="sm"
                  className="h-10"
                  onClick={handleCreateCategory}
                  disabled={isCreatingCategory || !newCategoryName}
                >
                  {isCreatingCategory ? "Adding…" : "Add category"}
                </Button>
                {categoryError && <p className="text-sm text-status-critical">{categoryError}</p>}
              </div>
            )}

            <div className="flex items-center gap-3">
              <Button type="submit" disabled={isCreating}>
                {isCreating ? "Adding…" : "Add transaction"}
              </Button>
              {createError && <p className="text-sm text-status-critical">{createError}</p>}
            </div>
          </form>
        </Card>
      )}

      {/* CSV import */}
      {refDataLoaded && hasAccounts && showImportForm && (
        <Card className="mt-6">
          <h2 className="text-xs font-semibold uppercase tracking-wide text-ink-muted">
            Import from CSV
          </h2>
          <form onSubmit={handleImport} className="mt-4 flex flex-col gap-3">
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-12">
              <Select
                label="Account"
                wrapperClassName="lg:col-span-4"
                value={importAccountId}
                onChange={(e) => setImportAccountId(e.target.value)}
                required
              >
                {accounts.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name}
                  </option>
                ))}
              </Select>
              <div className="flex flex-col gap-1 lg:col-span-8">
                <label htmlFor="import-file" className="text-xs font-medium text-ink-secondary">
                  Statement CSV
                </label>
                <input
                  id="import-file"
                  type="file"
                  accept=".csv,text/csv"
                  onChange={handleImportFileChange}
                  className="h-10 rounded-md border border-hairline bg-surface px-3 text-sm text-ink-primary outline-none file:mr-3 file:h-full file:rounded-md file:border-0 file:bg-plane file:px-3 file:text-xs file:font-medium file:text-ink-primary focus:border-brand"
                />
              </div>
            </div>

            <p className="text-xs text-ink-muted">
              Needs a Date column (e.g. &quot;Date&quot;/&quot;Transaction Date&quot;) and either an
              &quot;Amount&quot; column (signed — negative for a charge) or separate
              &quot;Debit&quot;/&quot;Credit&quot; columns (the shape most bank/card exports actually use).
              Amounts accept &quot;$&quot;, thousands commas, and parenthesized negatives. Rows that
              don&apos;t parse are skipped, not fatal to the rest of the file.
            </p>

            <div className="flex items-center gap-3">
              <Button type="submit" disabled={isImporting || !importFile || !importAccountId}>
                <UploadIcon size={16} />
                {isImporting ? "Importing…" : "Import"}
              </Button>
              {importError && <p className="text-sm text-status-critical">{importError}</p>}
            </div>
          </form>

          {importResult && (
            <div className="mt-4 rounded-md border border-hairline p-3">
              <p className="text-sm text-ink-primary">
                Imported <span className="tabular-nums font-medium">{importResult.imported}</span>,
                skipped <span className="tabular-nums font-medium">{importResult.skipped}</span>.
              </p>
              {importResult.errors.length > 0 && (
                <ul className="mt-2 space-y-1 text-xs text-status-critical">
                  {importResult.errors.map((e, i) => (
                    <li key={i}>
                      Row {e.row}: {e.message}
                    </li>
                  ))}
                </ul>
              )}
            </div>
          )}
        </Card>
      )}

      {/* List */}
      <Card className="mt-6 p-0">
        {isLoading && <SkeletonRows rows={6} />}
        {!isLoading && loadError && <p className="p-6 text-sm text-status-critical">{loadError}</p>}
        {!isLoading && !loadError && transactions.length === 0 && (
          <EmptyState
            icon={<ListIcon size={24} />}
            title="No transactions found"
            description="Log a transaction above, or adjust your filters."
          />
        )}
        {!isLoading && !loadError && transactions.length > 0 && (
          <div className="overflow-x-auto">
            {rowError && (
              <p className="m-4 rounded-md bg-status-critical/10 px-3 py-2 text-sm text-status-critical">
                {rowError}
              </p>
            )}
            <table className="w-full min-w-[720px] text-left text-sm">
              <thead className="border-b border-hairline text-xs uppercase text-ink-muted">
                <tr>
                  <th className="px-6 py-3 font-medium">Date</th>
                  <th className="px-6 py-3 font-medium">Account</th>
                  <th className="px-6 py-3 font-medium">Category</th>
                  <th className="px-6 py-3 font-medium">Note</th>
                  <th className="px-6 py-3 font-medium">Amount</th>
                  <th className="px-6 py-3 font-medium text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-hairline">
                {transactions.map((tx) =>
                  editingId === tx.id ? (
                    <tr key={tx.id} className="bg-plane">
                      <td className="px-6 py-3">
                        <Input
                          type="date"
                          value={editForm.date}
                          onChange={(e) => setEditForm({ ...editForm, date: e.target.value })}
                        />
                      </td>
                      <td className="px-6 py-3">
                        <Select
                          value={editForm.accountId}
                          onChange={(e) => setEditForm({ ...editForm, accountId: e.target.value })}
                        >
                          {accounts.map((a) => (
                            <option key={a.id} value={a.id}>
                              {a.name}
                            </option>
                          ))}
                        </Select>
                      </td>
                      <td className="px-6 py-3">
                        <Select
                          value={editForm.categoryId}
                          onChange={(e) => setEditForm({ ...editForm, categoryId: e.target.value })}
                        >
                          <option value="">None</option>
                          {categories.map((c) => (
                            <option key={c.id} value={c.id}>
                              {c.name}
                            </option>
                          ))}
                        </Select>
                      </td>
                      <td className="px-6 py-3">
                        <Input
                          value={editForm.note}
                          onChange={(e) => setEditForm({ ...editForm, note: e.target.value })}
                        />
                      </td>
                      <td className="px-6 py-3">
                        <div className="flex items-center gap-2">
                          <Select
                            value={editForm.type}
                            onChange={(e) =>
                              setEditForm({ ...editForm, type: e.target.value as "income" | "expense" })
                            }
                          >
                            <option value="expense">Expense</option>
                            <option value="income">Income</option>
                          </Select>
                          <Input
                            type="number"
                            step="0.01"
                            value={editForm.amount}
                            onChange={(e) => setEditForm({ ...editForm, amount: e.target.value })}
                            className="w-24 tabular-nums"
                          />
                        </div>
                      </td>
                      <td className="px-6 py-3 text-right">
                        <div className="flex justify-end gap-2">
                          <Button size="sm" onClick={() => handleSaveEdit(tx.id)} disabled={isSavingEdit}>
                            {isSavingEdit ? "Saving…" : "Save"}
                          </Button>
                          <Button variant="secondary" size="sm" onClick={cancelEdit}>
                            Cancel
                          </Button>
                        </div>
                        {editError && <p className="mt-2 text-xs text-status-critical">{editError}</p>}
                      </td>
                    </tr>
                  ) : (
                    <tr key={tx.id} className="hover:bg-plane">
                      <td className="px-6 py-3 text-ink-secondary">{tx.date.slice(0, 10)}</td>
                      <td className="px-6 py-3 text-ink-secondary">{accountLabel(tx.account_id)}</td>
                      <td className="px-6 py-3 text-ink-secondary">{categoryLabel(tx.category_id)}</td>
                      <td className="px-6 py-3 text-ink-muted">{tx.note || "—"}</td>
                      <td className="px-6 py-3">
                        <div className="flex items-center gap-2">
                          <TransactionTypeBadge type={tx.type} />
                          <span className="tabular-nums text-ink-primary">
                            {tx.type === "income" ? "+" : "-"}
                            {tx.amount.toFixed(2)} {tx.currency}
                          </span>
                        </div>
                      </td>
                      <td className="px-6 py-3 text-right">
                        <div className="flex justify-end gap-1">
                          <Button
                            variant="ghost"
                            size="sm"
                            aria-label="Edit transaction"
                            onClick={() => startEdit(tx)}
                          >
                            <PencilIcon size={16} />
                          </Button>
                          <Button
                            variant="ghost"
                            size="sm"
                            aria-label="Delete transaction"
                            onClick={() => handleDelete(tx.id)}
                            disabled={deletingId === tx.id}
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

            <div className="flex items-center justify-between border-t border-hairline px-6 py-4">
              <p className="text-sm text-ink-muted">
                Page {page} of {totalPages} ({total} total)
              </p>
              <div className="flex gap-2">
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                  disabled={page <= 1}
                >
                  Previous
                </Button>
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => setPage((p) => p + 1)}
                  disabled={page >= totalPages}
                >
                  Next
                </Button>
              </div>
            </div>
          </div>
        )}
      </Card>
    </div>
  );
}
