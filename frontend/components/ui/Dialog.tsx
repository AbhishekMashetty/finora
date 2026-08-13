"use client";

import type { ComponentProps, ReactNode } from "react";
import { X } from "lucide-react";
import { Dialog as DialogPrimitive } from "radix-ui";
import { cn } from "@/lib/cn";

export const Dialog = DialogPrimitive.Root;
export const DialogTrigger = DialogPrimitive.Trigger;
export const DialogClose = DialogPrimitive.Close;

const overlayClasses =
  "fixed inset-0 z-50 bg-ink-primary/40 backdrop-blur-[2px] " +
  "opacity-0 transition-opacity duration-[var(--duration-base)] data-[state=open]:opacity-100";

const contentClasses =
  "fixed left-1/2 top-1/2 z-50 w-[calc(100%-2rem)] max-w-md -translate-x-1/2 -translate-y-1/2 rounded-card border border-hairline bg-surface-raised p-6 shadow-[var(--shadow-popover)] " +
  "opacity-0 scale-95 transition-[opacity,transform] duration-[var(--duration-base)] ease-[var(--ease-out)] " +
  "data-[state=open]:opacity-100 data-[state=open]:scale-100";

export function DialogContent({
  children,
  className,
  ...props
}: ComponentProps<typeof DialogPrimitive.Content>) {
  return (
    <DialogPrimitive.Portal>
      <DialogPrimitive.Overlay className={overlayClasses} />
      <DialogPrimitive.Content className={cn(contentClasses, className)} {...props}>
        {children}
        <DialogPrimitive.Close
          className="absolute right-4 top-4 rounded-control p-1 text-ink-muted transition-colors hover:bg-plane hover:text-ink-primary focus-visible:outline-2 focus-visible:outline-brand"
          aria-label="Close"
        >
          <X size={16} strokeWidth={1.75} aria-hidden="true" />
        </DialogPrimitive.Close>
      </DialogPrimitive.Content>
    </DialogPrimitive.Portal>
  );
}

export function DialogHeader({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn("mb-4 flex flex-col gap-1 pr-6", className)}>{children}</div>;
}

export function DialogTitle({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <DialogPrimitive.Title className={cn("text-lg font-semibold text-ink-primary", className)}>
      {children}
    </DialogPrimitive.Title>
  );
}

export function DialogDescription({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <DialogPrimitive.Description className={cn("text-sm text-ink-secondary", className)}>
      {children}
    </DialogPrimitive.Description>
  );
}

export function DialogFooter({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn("mt-6 flex justify-end gap-2", className)}>{children}</div>;
}
