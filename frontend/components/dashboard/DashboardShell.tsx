"use client";

// Dashboard shell: sidebar nav + auth guard + logout + responsive drawer.
//
// Known simplification (Phase 0, still true): the guard below is a
// client-side effect that runs after mount, not Next.js middleware.ts. That
// means an unauthenticated visitor briefly sees this component render
// (with nothing in it — see the isLoading/isAuthenticated checks) before
// being redirected to /login, rather than being blocked at the edge. See
// frontend/README.md for the plan to move this to httpOnly cookies +
// middleware in a later phase.
//
// This is a Client Component so the auth guard, nav-item active-path
// highlighting, and notification polling can run — but `children` is
// passed through untouched from the Server Component layout.tsx that
// renders this shell, so route content itself still renders on the server.

import { useEffect, useState, type ReactNode } from "react";
import { useRouter } from "next/navigation";
import { LogOut, Menu, X } from "lucide-react";
import { Dialog as DialogPrimitive } from "radix-ui";
import { useAuth } from "@/lib/auth-context";
import { apiFetch } from "@/lib/api";
import type { Notification, User } from "@/lib/types";
import { SidebarNav } from "./SidebarNav";
import { Avatar } from "@/components/ui/Avatar";
import { ThemeToggle } from "@/components/theme/ThemeToggle";
import { Skeleton } from "@/components/ui/Skeleton";

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
 * `total` reflects the true count regardless of page size. */
function useUnreadNotificationCount(isAuthenticated: boolean): number {
  const [count, setCount] = useState(0);
  useEffect(() => {
    if (!isAuthenticated) return;
    let cancelled = false;
    async function poll() {
      try {
        const data = await apiFetch<{ notifications: Notification[]; total: number }>(
          "/api/v1/notifications?unread_only=true&page=1&page_size=1",
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

function Wordmark() {
  return (
    <div className="flex items-center gap-2 px-2.5 py-2">
      <span className="flex h-6 w-6 items-center justify-center rounded-control bg-brand text-xs font-bold text-brand-foreground">
        F
      </span>
      <span className="font-display text-lg font-medium tracking-tight text-ink-primary">Finora</span>
    </div>
  );
}

function SidebarFooter({ user, onLogout }: { user: User | null; onLogout: () => void }) {
  return (
    <div className="mt-4 flex flex-col gap-3 border-t border-hairline pt-4">
      {user && (
        <div className="flex items-center gap-2.5 px-1">
          <Avatar name={user.name} size={30} />
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-medium text-ink-primary">{user.name}</p>
            <p className="truncate text-xs text-ink-muted">{user.email}</p>
          </div>
        </div>
      )}
      <div className="flex items-center justify-between px-1">
        <ThemeToggle />
        <button
          onClick={onLogout}
          className="flex items-center gap-1.5 rounded-control px-2 py-1.5 text-sm font-medium text-ink-secondary transition-colors hover:bg-plane hover:text-ink-primary"
        >
          <LogOut size={16} strokeWidth={1.75} />
          Log out
        </button>
      </div>
    </div>
  );
}

export function DashboardShell({ children }: { children: ReactNode }) {
  const { isAuthenticated, isLoading, logout } = useAuth();
  const router = useRouter();
  const user = useCurrentUser();
  const unreadCount = useUnreadNotificationCount(isAuthenticated);
  const [drawerOpen, setDrawerOpen] = useState(false);

  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      router.replace("/login");
    }
  }, [isLoading, isAuthenticated, router]);

  if (isLoading || !isAuthenticated) {
    return (
      <div className="flex flex-1 flex-col gap-3 bg-plane p-8">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-32 w-full" />
        <Skeleton className="h-32 w-full" />
      </div>
    );
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col bg-plane lg:flex-row">
      {/* Mobile topbar — hidden at lg and up, where the fixed sidebar takes over */}
      <div className="flex items-center justify-between border-b border-hairline bg-surface px-4 py-3 lg:hidden">
        <Wordmark />
        <button
          onClick={() => setDrawerOpen(true)}
          aria-label="Open menu"
          className="flex h-9 w-9 items-center justify-center rounded-control text-ink-secondary hover:bg-plane hover:text-ink-primary"
        >
          <Menu size={20} strokeWidth={1.75} />
        </button>
      </div>

      {/* Mobile drawer */}
      <DialogPrimitive.Root open={drawerOpen} onOpenChange={setDrawerOpen}>
        <DialogPrimitive.Portal>
          <DialogPrimitive.Overlay
            className="fixed inset-0 z-40 bg-ink-primary/40 opacity-0 transition-opacity duration-[var(--duration-base)] data-[state=open]:opacity-100 lg:hidden"
          />
          <DialogPrimitive.Content
            className="fixed inset-y-0 left-0 z-50 flex w-72 -translate-x-full flex-col bg-surface p-4 shadow-[var(--shadow-popover)] transition-transform duration-[var(--duration-base)] ease-[var(--ease-out)] data-[state=open]:translate-x-0 lg:hidden"
            aria-describedby={undefined}
          >
            <div className="flex items-center justify-between">
              <DialogPrimitive.Title asChild>
                <div>
                  <Wordmark />
                </div>
              </DialogPrimitive.Title>
              <DialogPrimitive.Close
                aria-label="Close menu"
                className="flex h-9 w-9 items-center justify-center rounded-control text-ink-muted hover:bg-plane hover:text-ink-primary"
              >
                <X size={18} strokeWidth={1.75} />
              </DialogPrimitive.Close>
            </div>
            <div className="mt-2 flex flex-1 flex-col overflow-y-auto">
              <SidebarNav unreadCount={unreadCount} onNavigate={() => setDrawerOpen(false)} />
              <SidebarFooter user={user} onLogout={logout} />
            </div>
          </DialogPrimitive.Content>
        </DialogPrimitive.Portal>
      </DialogPrimitive.Root>

      {/* Desktop sidebar — fixed, always visible at lg and up */}
      <aside className="hidden w-64 shrink-0 flex-col border-r border-hairline bg-surface p-4 lg:flex">
        <Wordmark />
        <div className="mt-4 flex flex-1 flex-col overflow-y-auto">
          <SidebarNav unreadCount={unreadCount} />
        </div>
        <SidebarFooter user={user} onLogout={logout} />
      </aside>

      <main className="flex flex-1 flex-col overflow-y-auto p-5 sm:p-8">{children}</main>
    </div>
  );
}
