"use client";

// Search screen: there is no dedicated search endpoint anywhere in the
// fleet (no service owns "search" per architecture/service-boundaries.md),
// and building one would be real, unjustified complexity for a personal-
// finance app's data volumes — see CLAUDE.md §10/plan.md's "don't add
// complexity before there's a legitimate reason." Instead this composes a
// client-side search over the same list endpoints every other page already
// uses: accounts, categories (to resolve transaction category names),
// transactions, budgets, and goals — fetched once per query, matched
// case-insensitively against the fields a user would actually recall
// (name/note/category/type/amount).
//
// Known limitation: transactions are fetched up to expense-service's own
// page_size cap (100, see transaction_service.go's maxPageSize) — search
// only covers a user's most recent 100 transactions, not their full
// history. That's an existing server-side ceiling this page inherits, not
// a new one invented here.

import Link from "next/link";
import { useState, type FormEvent } from "react";
import { apiFetch, ApiError } from "@/lib/api";
import type { Account, Budget, Category, Goal, Transaction } from "@/lib/types";
import { Card } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { EmptyState } from "@/components/ui/EmptyState";
import { TransactionTypeBadge } from "@/components/ui/Badge";
import {
  FlagIcon,
  ListIcon,
  SearchIcon,
  TargetIcon,
  WalletIcon,
} from "@/components/icons";

interface SearchResults {
  accounts: Account[];
  transactions: Transaction[];
  budgets: Budget[];
  goals: Goal[];
}

function matches(query: string, ...fields: (string | undefined | null)[]): boolean {
  const q = query.trim().toLowerCase();
  if (!q) return false;
  return fields.some((f) => f && f.toLowerCase().includes(q));
}

export default function SearchPage() {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SearchResults | null>(null);
  const [hasSearched, setHasSearched] = useState(false);
  const [isSearching, setIsSearching] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    const q = query.trim();
    if (!q) return;

    setIsSearching(true);
    setError(null);
    try {
      const [accountsRes, categoriesRes, transactionsRes, budgetsRes, goalsRes] = await Promise.all([
        apiFetch<{ accounts: Account[] }>("/api/v1/accounts"),
        apiFetch<{ categories: Category[] }>("/api/v1/categories"),
        apiFetch<{ transactions: Transaction[] }>("/api/v1/transactions?page=1&page_size=100"),
        apiFetch<{ budgets: Budget[] }>("/api/v1/budgets"),
        apiFetch<{ goals: Goal[] }>("/api/v1/goals"),
      ]);

      const categoryNameById = new Map(categoriesRes.categories.map((c) => [c.id, c.name]));

      setResults({
        accounts: accountsRes.accounts.filter((a) => matches(q, a.name, a.type)),
        transactions: transactionsRes.transactions.filter((t) =>
          matches(
            q,
            t.note,
            t.type,
            String(t.amount),
            t.category_id ? categoryNameById.get(t.category_id) : undefined
          )
        ),
        budgets: budgetsRes.budgets.filter((b) => matches(q, b.category, b.period)),
        goals: goalsRes.goals.filter((g) => matches(q, g.name)),
      });
      setHasSearched(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Search failed.");
      setResults(null);
    } finally {
      setIsSearching(false);
    }
  }

  const totalMatches = results
    ? results.accounts.length + results.transactions.length + results.budgets.length + results.goals.length
    : 0;

  return (
    <div>
      <h1 className="text-2xl font-semibold text-ink-primary">Search</h1>

      <Card className="mt-6">
        <form onSubmit={handleSubmit} className="flex items-end gap-3">
          <div className="flex-1">
            <Input
              id="search-query"
              label="Search accounts, transactions, budgets, and goals"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="e.g. groceries, checking, vacation"
              autoFocus
            />
          </div>
          <Button type="submit" size="md" disabled={isSearching || query.trim() === ""}>
            <SearchIcon size={16} />
            {isSearching ? "Searching…" : "Search"}
          </Button>
        </form>
      </Card>

      {error && (
        <p className="mt-6 rounded-md bg-status-critical/10 px-3 py-2 text-sm text-status-critical">
          {error}
        </p>
      )}

      {!error && !hasSearched && (
        <Card className="mt-6">
          <EmptyState
            icon={<SearchIcon size={24} />}
            title="Search your finances"
            description="Find an account, transaction, budget, or goal by name."
          />
        </Card>
      )}

      {!error && hasSearched && results && totalMatches === 0 && (
        <Card className="mt-6">
          <EmptyState
            icon={<SearchIcon size={24} />}
            title="No matches"
            description={`Nothing found for "${query}".`}
          />
        </Card>
      )}

      {!error && hasSearched && results && totalMatches > 0 && (
        <div className="mt-6 flex flex-col gap-6">
          {results.accounts.length > 0 && (
            <ResultSection title="Accounts" icon={<WalletIcon size={16} />} href="/dashboard/accounts">
              {results.accounts.map((a) => (
                <li key={a.id} className="flex items-center justify-between px-6 py-3 text-sm">
                  <span className="text-ink-primary">{a.name}</span>
                  <span className="capitalize text-ink-muted">{a.type}</span>
                </li>
              ))}
            </ResultSection>
          )}

          {results.transactions.length > 0 && (
            <ResultSection title="Transactions" icon={<ListIcon size={16} />} href="/dashboard/transactions">
              {results.transactions.map((t) => (
                <li key={t.id} className="flex items-center justify-between px-6 py-3 text-sm">
                  <div>
                    <span className="text-ink-primary">{t.note || "—"}</span>
                    <span className="ml-2 text-xs text-ink-muted">{t.date.slice(0, 10)}</span>
                  </div>
                  <div className="flex items-center gap-3">
                    <TransactionTypeBadge type={t.type} />
                    <span className="tabular-nums text-ink-primary">
                      {t.amount.toFixed(2)} {t.currency}
                    </span>
                  </div>
                </li>
              ))}
            </ResultSection>
          )}

          {results.budgets.length > 0 && (
            <ResultSection title="Budgets" icon={<TargetIcon size={16} />} href="/dashboard/budgets">
              {results.budgets.map((b) => (
                <li key={b.id} className="flex items-center justify-between px-6 py-3 text-sm">
                  <span className="capitalize text-ink-primary">{b.category}</span>
                  <span className="tabular-nums text-ink-muted">
                    {b.amount.toFixed(2)} / {b.period}
                  </span>
                </li>
              ))}
            </ResultSection>
          )}

          {results.goals.length > 0 && (
            <ResultSection title="Goals" icon={<FlagIcon size={16} />} href="/dashboard/goals">
              {results.goals.map((g) => (
                <li key={g.id} className="flex items-center justify-between px-6 py-3 text-sm">
                  <span className="text-ink-primary">{g.name}</span>
                  <span className="tabular-nums text-ink-muted">
                    {g.current_amount.toFixed(2)} / {g.target_amount.toFixed(2)}
                  </span>
                </li>
              ))}
            </ResultSection>
          )}
        </div>
      )}
    </div>
  );
}

function ResultSection({
  title,
  icon,
  href,
  children,
}: {
  title: string;
  icon: React.ReactNode;
  href: string;
  children: React.ReactNode;
}) {
  return (
    <Card className="overflow-hidden p-0">
      <div className="flex items-center justify-between border-b border-hairline px-6 py-3">
        <span className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-ink-muted">
          {icon}
          {title}
        </span>
        <Link href={href} className="text-xs font-medium text-brand">
          View all →
        </Link>
      </div>
      <ul className="flex flex-col divide-y divide-hairline">{children}</ul>
    </Card>
  );
}
