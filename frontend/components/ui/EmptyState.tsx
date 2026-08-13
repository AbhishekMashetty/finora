// Empty-state primitive — replaces bare "No X found." text and the old
// ComingSoon component's plain dashed box. See
// architecture/frontend-design-system.md §5/§7.

import type { ReactNode } from "react";
import { cn } from "@/lib/cn";

export function EmptyState({
  icon,
  title,
  description,
  action,
  className,
}: {
  icon?: ReactNode;
  title: string;
  description?: string;
  action?: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center gap-3 rounded-card border border-dashed border-hairline p-12 text-center",
        className,
      )}
    >
      {icon && <div className="text-ink-muted">{icon}</div>}
      <div>
        <h3 className="text-sm font-medium text-ink-primary">{title}</h3>
        {description && <p className="mt-1 text-sm text-ink-muted">{description}</p>}
      </div>
      {action}
    </div>
  );
}
