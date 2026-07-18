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
 * navigating anywhere. */
function useUnreadNotificationCount(isAuthenticated: boolean): number {
  const [count, setCount] = useState(0);
  useEffect(() => {
    if (!isAuthenticated) return;
    let cancelled = false;
    async function poll() {
      try {
        const data = await apiFetch<{ notifications: Notification[] }>(
          "/api/v1/notifications?unread_only=true"
        );
        if (!cancelled) setCount(data.notifications?.length ?? 0);
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
    <div className="flex flex-1 bg-plane">
      <aside className="flex w-60 shrink-0 flex-col border-r border-hairline bg-surface p-4">
        <div className="flex items-center gap-2 px-2 py-2">
          <span className="flex h-6 w-6 items-center justify-center rounded-md bg-brand text-xs font-bold text-white">
            F
          </span>
          <span className="text-lg font-semibold tracking-tight text-ink-primary">
            Finora
          </span>
        </div>

        <nav className="mt-4 flex flex-1 flex-col gap-0.5">
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
                className={`flex items-center gap-2.5 rounded-md border-l-2 px-2.5 py-2 text-sm font-medium transition-colors ${
                  isActive
                    ? "border-brand bg-brand/10 text-brand"
                    : "border-transparent text-ink-secondary hover:bg-plane hover:text-ink-primary"
                }`}
              >
                <Icon size={18} />
                <span className="flex-1">{item.label}</span>
                {item.href === "/dashboard/notifications" && unreadCount > 0 && (
                  <span className="flex h-5 min-w-5 items-center justify-center rounded-full bg-status-critical px-1 text-[10px] font-semibold text-white">
                    {unreadCount > 9 ? "9+" : unreadCount}
                  </span>
                )}
              </Link>
            );
          })}
        </nav>

        <div className="mt-4 border-t border-hairline pt-4">
          {user && (
            <div className="mb-2 px-2.5">
              <p className="truncate text-sm font-medium text-ink-primary">{user.name}</p>
              <p className="truncate text-xs text-ink-muted">{user.email}</p>
            </div>
          )}
          <button
            onClick={() => logout()}
            className="flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-sm font-medium text-ink-secondary transition-colors hover:bg-plane hover:text-ink-primary"
          >
            <LogOutIcon size={18} />
            Log out
          </button>
        </div>
      </aside>
      <main className="flex flex-1 flex-col p-8">{children}</main>
    </div>
  );
}
