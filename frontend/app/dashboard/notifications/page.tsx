"use client";

// Notifications screen: list + mark-read against notification-service via
// the gateway (GET /api/v1/notifications?unread_only=, PATCH
// /api/v1/notifications/:id/read). Notifications are produced server-side —
// today, exclusively by budget-service's overspend trigger (see
// architecture/api-contracts.md's budget-service -> notification-service
// subsection) — there is no user-facing "create notification" UI, by
// design: this feed is read/acknowledge only.

import { useEffect, useState } from "react";
import { apiFetch, ApiError } from "@/lib/api";
import type { Notification } from "@/lib/types";
import { Card } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { EmptyState } from "@/components/ui/EmptyState";
import { SkeletonRows } from "@/components/ui/Skeleton";
import { BellIcon, CheckIcon } from "@/components/icons";

function formatTimestamp(iso: string): string {
  const date = new Date(iso);
  return date.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

export default function NotificationsPage() {
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [unreadOnly, setUnreadOnly] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [markingId, setMarkingId] = useState<string | null>(null);
  const [rowError, setRowError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const data = await apiFetch<{ notifications: Notification[] }>(
          `/api/v1/notifications${unreadOnly ? "?unread_only=true" : ""}`
        );
        if (!cancelled) {
          setNotifications(data.notifications ?? []);
          setLoadError(null);
        }
      } catch (err) {
        if (!cancelled) {
          setLoadError(err instanceof ApiError ? err.message : "Could not load notifications.");
        }
      } finally {
        if (!cancelled) setIsLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [unreadOnly]);

  async function handleMarkRead(id: string) {
    setRowError(null);
    setMarkingId(id);
    try {
      await apiFetch<{ notification: Notification }>(`/api/v1/notifications/${id}/read`, {
        method: "PATCH",
      });
      if (unreadOnly) {
        setNotifications((prev) => prev.filter((n) => n.id !== id));
      } else {
        setNotifications((prev) => prev.map((n) => (n.id === id ? { ...n, read: true } : n)));
      }
    } catch (err) {
      setRowError(err instanceof ApiError ? err.message : "Could not mark notification as read.");
    } finally {
      setMarkingId(null);
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-ink-primary">Notifications</h1>
        <Button
          size="sm"
          variant={unreadOnly ? "primary" : "secondary"}
          onClick={() => setUnreadOnly((v) => !v)}
        >
          {unreadOnly ? "Showing unread only" : "Show unread only"}
        </Button>
      </div>

      <Card className="mt-6 overflow-hidden p-0">
        {isLoading && <SkeletonRows rows={4} />}
        {!isLoading && loadError && <p className="p-6 text-sm text-status-critical">{loadError}</p>}
        {!isLoading && !loadError && notifications.length === 0 && (
          <EmptyState
            icon={<BellIcon size={24} />}
            title={unreadOnly ? "No unread notifications" : "No notifications yet"}
            description={
              unreadOnly
                ? "You're all caught up."
                : "You'll see alerts here — for example, when you go over a budget."
            }
          />
        )}
        {!isLoading && !loadError && notifications.length > 0 && (
          <ul className="flex flex-col divide-y divide-hairline">
            {rowError && (
              <li className="p-4">
                <p className="rounded-md bg-status-critical/10 px-3 py-2 text-sm text-status-critical">
                  {rowError}
                </p>
              </li>
            )}
            {notifications.map((n) => (
              <li key={n.id} className="flex items-start justify-between gap-4 p-6">
                <div className="flex items-start gap-3">
                  <div
                    className={`mt-1 h-2 w-2 shrink-0 rounded-full ${
                      n.read ? "bg-ink-muted/30" : "bg-brand"
                    }`}
                    aria-hidden="true"
                  />
                  <div>
                    <div className="flex items-center gap-2">
                      <p className="font-medium text-ink-primary">{n.title}</p>
                      {!n.read && <Badge status="warning">New</Badge>}
                    </div>
                    <p className="mt-1 text-sm text-ink-secondary">{n.message}</p>
                    <p className="mt-1 text-xs text-ink-muted">{formatTimestamp(n.created_at)}</p>
                  </div>
                </div>
                {!n.read && (
                  <Button
                    variant="ghost"
                    size="sm"
                    aria-label="Mark as read"
                    disabled={markingId === n.id}
                    onClick={() => handleMarkRead(n.id)}
                  >
                    <CheckIcon size={16} />
                    {markingId === n.id ? "Marking…" : "Mark read"}
                  </Button>
                )}
              </li>
            ))}
          </ul>
        )}
      </Card>
    </div>
  );
}
