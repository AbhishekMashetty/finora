"use client";

// Dashboard shell: sidebar nav + auth guard + logout.
//
// Known simplification (Phase 0): the guard below is a client-side effect
// that runs after mount, not Next.js middleware.ts. That means an
// unauthenticated visitor briefly sees this component render (with nothing
// in it — see the isLoading/isAuthenticated checks) before being redirected
// to /login, rather than being blocked at the edge. See frontend/README.md
// for the plan to move this to httpOnly cookies + middleware in a later
// phase.

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState, type ReactNode } from "react";
import { useAuth } from "@/lib/auth-context";
import { apiFetch } from "@/lib/api";
import type { Notification, User } from "@/lib/types";
import {
  BellIcon,
  ChartIcon,
  FlagIcon,
  GearIcon,
  HomeIcon,
  ListIcon,
  LogOutIcon,
  SearchIcon,
  TargetIcon,
  UserIcon,
  WalletIcon,
} from "@/components/icons";

const NAV_ITEMS = [
  { href: "/dashboard", label: "Overview", icon: HomeIcon },
  { href: "/dashboard/accounts", label: "Accounts", icon: WalletIcon },
  { href: "/dashboard/transactions", label: "Transactions", icon: ListIcon },
  { href: "/dashboard/budgets", label: "Budgets", icon: TargetIcon },
  { href: "/dashboard/goals", label: "Goals", icon: FlagIcon },
  { href: "/dashboard/reports", label: "Reports", icon: ChartIcon },
  { href: "/dashboard/notifications", label: "Notifications", icon: BellIcon },
  { href: "/dashboard/search", label: "Search", icon: SearchIcon },
  { href: "/dashboard/profile", label: "Profile", icon: UserIcon },
  { href: "/dashboard/settings", label: "Settings", icon: GearIcon },
];

/** Small, additive identity fetch — reuses the exact GET /api/v1/users/me
 * call the (pre-existing) dashboard overview page already makes, so the
 * sidebar can show who's signed in above the logout control. Not a new
 * endpoint or contract, just the established apiFetch pattern reused. */
function useCurrentUser(): User | null {
  const [user, setUser] = useState<User | null>(null);
  useEffect(() => {
    let cancelled = false;
    apiFetch<{ user: User }>("/api/v1/users/me")
      .then((data) => {
        if (!cancelled) setUser(data.user);
      })
      .catch(() => {
        // Non-critical UI chrome — if this fails, the sidebar just omits the
        // identity line; the page itself handles auth failures.
      });
    return () => {
      cancelled = true;
    };
  }, []);
  return user;
}

/** Small, additive unread-count fetch — reuses the exact GET
 * /api/v1/notifications?unread_only=true call the notifications page itself
 * makes, so the sidebar can show a badge without a new endpoint. Polls on
 * an interval rather than on every navigation, since the count can change
 * server-side (e.g. a new overspend notification) without the user
 * navigating anywhere.
 *
 * Uses the response's `total` field, not `notifications.length` — since
 * Phase 6 the endpoint is paginated (default page_size=20), so the
 * returned array is capped even when the real unread count is higher;
 * `total` reflects the true count regardless of page size. Doesn't matter
 * much for the badge's own display (it caps at "9+" anyway), but a wrong
 * number is still a wrong number. */
function useUnreadNotificationCount(isAuthenticated: boolean): number {
  const [count, setCount] = useState(0);
  useEffect(() => {
    if (!isAuthenticated) return;
    let cancelled = false;
    async function poll() {
      try {
        const data = await apiFetch<{ notifications: Notification[]; total: number }>(
          "/api/v1/notifications?unread_only=true&page=1&page_size=1"
        );
        if (!cancelled) setCount(data.total ?? 0);
      } catch {
        // Non-critical UI chrome — leave the last known count on failure.
      }
    }
    poll();
    const interval = setInterval(poll, 30_000);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [isAuthenticated]);
  return count;
}

export default function DashboardLayout({ children }: { children: ReactNode }) {
  const { isAuthenticated, isLoading, logout } = useAuth();
  const router = useRouter();
  const pathname = usePathname();
  const user = useCurrentUser();
  const unreadCount = useUnreadNotificationCount(isAuthenticated);

  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      router.replace("/login");
    }
  }, [isLoading, isAuthenticated, router]);

  if (isLoading || !isAuthenticated) {
    // Avoid flashing dashboard content before the redirect effect runs.
    return (
      <div className="flex flex-1 items-center justify-center bg-plane">
        <p className="text-sm text-ink-muted">Loading…</p>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex-1 bg-plane lg:flex">
      <aside className="hidden w-64 shrink-0 flex-col border-r border-white/10 bg-[#0c241d] p-5 text-white lg:sticky lg:top-0 lg:flex lg:h-screen">
        <div className="flex items-center gap-3 px-2 py-2">
          <span className="flex h-9 w-9 items-center justify-center rounded-xl bg-[#4be0ad] text-sm font-black text-[#08271d] shadow-lg shadow-emerald-950/20">
            F
          </span>
          <div>
            <span className="block text-lg font-semibold tracking-tight">Finora</span>
            <span className="block text-[10px] uppercase tracking-[0.18em] text-emerald-100/55">Money, clarified</span>
          </div>
        </div>

        <p className="mb-2 mt-8 px-2.5 text-[10px] font-semibold uppercase tracking-[0.18em] text-emerald-100/40">Workspace</p>
        <nav className="flex flex-1 flex-col gap-1">
          {NAV_ITEMS.map((item) => {
            const isActive =
              item.href === "/dashboard"
                ? pathname === "/dashboard"
                : pathname.startsWith(item.href);
            const Icon = item.icon;
            return (
              <Link
                key={item.href}
                href={item.href}
                className={`flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium transition-all ${
                  isActive
                    ? "bg-white/12 text-white shadow-sm"
                    : "text-emerald-50/65 hover:bg-white/[0.07] hover:text-white"
                }`}
              >
                <Icon size={18} />
                <span className="flex-1">{item.label}</span>
                {item.href === "/dashboard/notifications" && unreadCount > 0 && (
                  <span className="flex h-5 min-w-5 items-center justify-center rounded-full bg-[#ff756f] px-1 text-[10px] font-semibold text-white">
                    {unreadCount > 9 ? "9+" : unreadCount}
                  </span>
                )}
              </Link>
            );
          })}
        </nav>

        <div className="mt-4 rounded-2xl border border-white/10 bg-white/[0.06] p-2">
          {user && (
            <div className="mb-2 px-2.5">
              <p className="truncate text-sm font-medium text-white">{user.name}</p>
              <p className="truncate text-xs text-emerald-50/50">{user.email}</p>
            </div>
          )}
          <button
            onClick={() => logout()}
            className="flex w-full items-center gap-2.5 rounded-xl px-2.5 py-2 text-sm font-medium text-emerald-50/65 transition-colors hover:bg-white/10 hover:text-white"
          >
            <LogOutIcon size={18} />
            Log out
          </button>
        </div>
      </aside>

      <div className="border-b border-hairline bg-surface/90 px-4 py-3 backdrop-blur lg:hidden">
        <div className="flex items-center justify-between">
          <Link href="/dashboard" className="flex items-center gap-2.5">
            <span className="flex h-8 w-8 items-center justify-center rounded-xl bg-brand text-sm font-black text-white">F</span>
            <span className="font-semibold tracking-tight text-ink-primary">Finora</span>
          </Link>
          <details className="relative">
            <summary className="cursor-pointer list-none rounded-lg border border-hairline px-3 py-2 text-sm font-medium text-ink-secondary">Menu</summary>
            <nav className="absolute right-0 z-50 mt-2 grid w-64 grid-cols-2 gap-1 rounded-2xl border border-hairline bg-surface p-2 shadow-2xl">
              {NAV_ITEMS.map((item) => {
                const Icon = item.icon;
                const isActive = item.href === "/dashboard" ? pathname === "/dashboard" : pathname.startsWith(item.href);
                return (
                  <Link key={item.href} href={item.href} className={`flex items-center gap-2 rounded-xl px-3 py-2.5 text-sm ${isActive ? "bg-brand/10 font-medium text-brand" : "text-ink-secondary hover:bg-plane"}`}>
                    <Icon size={16} /> {item.label}
                  </Link>
                );
              })}
            </nav>
          </details>
        </div>
      </div>

      <main className="min-w-0 flex-1 px-4 py-6 sm:px-6 lg:px-10 lg:py-9">
        <div className="mx-auto w-full max-w-[1440px]">{children}</div>
      </main>
    </div>
  );
}
