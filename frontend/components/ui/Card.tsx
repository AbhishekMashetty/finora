// Shared surface-panel primitive. A restrained shadow separates interactive
// finance data from the page plane without making every panel feel elevated.

import type { HTMLAttributes } from "react";

export function Card({ className = "", ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={`rounded-2xl border border-hairline bg-surface p-6 shadow-[0_1px_2px_rgba(16,35,29,0.03),0_12px_32px_rgba(16,35,29,0.04)] ${className}`}
      {...props}
    />
  );
}
