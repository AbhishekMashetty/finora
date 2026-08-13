// Shared surface-panel primitive. See architecture/frontend-design-system.md
// §4 — flat, no shadow; depth comes from the plane/surface contrast plus a
// hairline border, not elevation effects.

import type { HTMLAttributes } from "react";
import { cn } from "@/lib/cn";

export function Card({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn("rounded-card border border-hairline bg-surface p-6", className)}
      {...props}
    />
  );
}
