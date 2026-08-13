"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { NAV_GROUPS } from "./nav-items";
import { cn } from "@/lib/cn";

export function SidebarNav({
  unreadCount,
  onNavigate,
}: {
  unreadCount: number;
  onNavigate?: () => void;
}) {
  const pathname = usePathname();

  return (
    <nav className="flex flex-1 flex-col gap-4">
      {NAV_GROUPS.map((group, i) => (
        <div key={group.label ?? `top-${i}`} className="flex flex-col gap-0.5">
          {group.label && (
            <h2 className="px-2.5 pb-1 text-[11px] font-semibold uppercase tracking-wide text-ink-muted">
              {group.label}
            </h2>
          )}
          {group.items.map((item) => {
            const isActive =
              item.href === "/dashboard" ? pathname === "/dashboard" : pathname.startsWith(item.href);
            const Icon = item.icon;
            return (
              <Link
                key={item.href}
                href={item.href}
                onClick={onNavigate}
                aria-current={isActive ? "page" : undefined}
                className={cn(
                  "flex items-center gap-2.5 rounded-control px-2.5 py-2 text-sm font-medium transition-colors duration-[var(--duration-fast)]",
                  isActive
                    ? "bg-brand-subtle text-brand"
                    : "text-ink-secondary hover:bg-plane hover:text-ink-primary",
                )}
              >
                <Icon size={18} strokeWidth={1.75} />
                <span className="flex-1">{item.label}</span>
                {item.href === "/dashboard/notifications" && unreadCount > 0 && (
                  <span className="flex h-5 min-w-5 items-center justify-center rounded-pill bg-status-critical px-1 text-[10px] font-semibold text-white">
                    {unreadCount > 9 ? "9+" : unreadCount}
                  </span>
                )}
              </Link>
            );
          })}
        </div>
      ))}
    </nav>
  );
}
