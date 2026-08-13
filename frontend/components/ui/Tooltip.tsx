"use client";

import type { ReactNode } from "react";
import { Tooltip as TooltipPrimitive } from "radix-ui";
import { cn } from "@/lib/cn";

export const TooltipProvider = TooltipPrimitive.Provider;

const contentClasses =
  "z-50 rounded-control bg-ink-primary px-2.5 py-1.5 text-xs font-medium text-plane shadow-[var(--shadow-popover)] " +
  "opacity-0 scale-95 transition-[opacity,transform] duration-[var(--duration-fast)] " +
  "data-[state=delayed-open]:opacity-100 data-[state=delayed-open]:scale-100 " +
  "data-[state=instant-open]:opacity-100 data-[state=instant-open]:scale-100";

/** Hover/focus tooltip — wrap a single focusable child (button, icon). */
export function Tooltip({
  content,
  children,
  side = "top",
  className,
}: {
  content: ReactNode;
  children: ReactNode;
  side?: "top" | "right" | "bottom" | "left";
  className?: string;
}) {
  return (
    <TooltipPrimitive.Root delayDuration={200}>
      <TooltipPrimitive.Trigger asChild>{children}</TooltipPrimitive.Trigger>
      <TooltipPrimitive.Portal>
        <TooltipPrimitive.Content side={side} sideOffset={6} className={cn(contentClasses, className)}>
          {content}
          <TooltipPrimitive.Arrow className="fill-ink-primary" />
        </TooltipPrimitive.Content>
      </TooltipPrimitive.Portal>
    </TooltipPrimitive.Root>
  );
}
