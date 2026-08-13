"use client";

import { useState, type MouseEvent, type ReactNode } from "react";
import { AlertDialog as AlertDialogPrimitive } from "radix-ui";
import { cn } from "@/lib/cn";
import { Button } from "@/components/ui/Button";

const overlayClasses =
  "fixed inset-0 z-50 bg-ink-primary/40 backdrop-blur-[2px] " +
  "opacity-0 transition-opacity duration-[var(--duration-base)] data-[state=open]:opacity-100";

const contentClasses =
  "fixed left-1/2 top-1/2 z-50 w-[calc(100%-2rem)] max-w-sm -translate-x-1/2 -translate-y-1/2 rounded-card border border-hairline bg-surface-raised p-6 shadow-[var(--shadow-popover)] " +
  "opacity-0 scale-95 transition-[opacity,transform] duration-[var(--duration-base)] ease-[var(--ease-out)] " +
  "data-[state=open]:opacity-100 data-[state=open]:scale-100";

/**
 * Confirmation gate for a destructive action — built on Radix AlertDialog
 * (not Dialog: it doesn't dismiss on an outside click, matching the higher
 * bar a delete/revoke action needs). Either pass `trigger` to render this as
 * a self-contained button+dialog, or omit it and drive `open`/`onOpenChange`
 * yourself (e.g. from a dropdown menu item, which can't itself be an
 * AlertDialogTrigger without breaking the menu's own close behavior).
 */
export function ConfirmDialog({
  trigger,
  open,
  onOpenChange,
  title,
  description,
  confirmLabel = "Delete",
  cancelLabel = "Cancel",
  variant = "danger",
  onConfirm,
}: {
  trigger?: ReactNode;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  title: string;
  description?: ReactNode;
  confirmLabel?: string;
  cancelLabel?: string;
  variant?: "danger" | "primary";
  onConfirm: () => void | Promise<void>;
}) {
  const [pending, setPending] = useState(false);
  const [internalOpen, setInternalOpen] = useState(false);
  // Merge controlled/uncontrolled so the dialog can always be closed
  // programmatically after an async confirm resolves, regardless of
  // whether the caller drives `open` or just passes `trigger`.
  const isOpen = open ?? internalOpen;
  const setOpen = onOpenChange ?? setInternalOpen;

  async function handleConfirm(e: MouseEvent) {
    e.preventDefault();
    setPending(true);
    try {
      await onConfirm();
      setOpen(false);
    } finally {
      setPending(false);
    }
  }

  return (
    <AlertDialogPrimitive.Root open={isOpen} onOpenChange={setOpen}>
      {trigger && <AlertDialogPrimitive.Trigger asChild>{trigger}</AlertDialogPrimitive.Trigger>}
      <AlertDialogPrimitive.Portal>
        <AlertDialogPrimitive.Overlay className={overlayClasses} />
        <AlertDialogPrimitive.Content className={contentClasses}>
          <AlertDialogPrimitive.Title className="text-lg font-semibold text-ink-primary">
            {title}
          </AlertDialogPrimitive.Title>
          {description && (
            <AlertDialogPrimitive.Description className="mt-2 text-sm text-ink-secondary">
              {description}
            </AlertDialogPrimitive.Description>
          )}
          <div className="mt-6 flex justify-end gap-2">
            <AlertDialogPrimitive.Cancel asChild>
              <Button variant="secondary" disabled={pending}>
                {cancelLabel}
              </Button>
            </AlertDialogPrimitive.Cancel>
            <AlertDialogPrimitive.Action asChild onClick={handleConfirm}>
              <Button
                variant={variant}
                disabled={pending}
                className={cn(variant === "danger" && "border-transparent bg-status-critical text-white hover:bg-status-critical/90")}
              >
                {pending ? "Working…" : confirmLabel}
              </Button>
            </AlertDialogPrimitive.Action>
          </div>
        </AlertDialogPrimitive.Content>
      </AlertDialogPrimitive.Portal>
    </AlertDialogPrimitive.Root>
  );
}
