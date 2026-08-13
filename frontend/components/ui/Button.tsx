// Shared button primitive. See architecture/frontend-design-system.md §5 —
// replaces the near-identical `primaryButtonClasses`/`secondaryButtonClasses`
// string constants that used to be redefined at the top of every page file.

import type { ButtonHTMLAttributes } from "react";
import { Slot } from "radix-ui";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/cn";

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-1.5 whitespace-nowrap rounded-full font-medium transition-colors duration-[var(--duration-fast)] disabled:cursor-not-allowed disabled:opacity-50 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand",
  {
    variants: {
      variant: {
        // text-brand-foreground (not a hardcoded text-white): --brand
        // flips to a light violet in dark mode, so a fixed white label
        // would drop to near-unreadable contrast there.
        primary: "bg-brand text-brand-foreground hover:bg-brand-hover active:bg-brand-active",
        secondary: "border border-hairline text-ink-primary hover:bg-plane",
        danger:
          "border border-status-critical/40 text-status-critical hover:bg-status-critical-subtle",
        ghost: "text-ink-secondary hover:text-ink-primary hover:bg-plane",
      },
      size: {
        // `md` is `h-10` — an explicit height, not padding-derived — so it
        // is pixel-identical to Input/Select's own `h-10`
        // (components/ui/Input.tsx). This matters anywhere a `md` button
        // sits in the same row as a labeled field: with padding alone
        // determining height, a button measures shorter than a field
        // (36px vs 40px) and, bottom-aligned via `items-end`, visibly
        // floats a few pixels proud of the field it sits beside —
        // confirmed by measuring real bounding boxes, not by eyeballing.
        sm: "px-3 py-1.5 text-xs",
        md: "h-10 px-5 text-sm",
        icon: "h-10 w-10 p-0",
      },
    },
    defaultVariants: {
      variant: "primary",
      size: "md",
    },
  },
);

export interface ButtonProps
  extends ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  /** Render as the single child element instead of a <button> — for a Link styled as a button. */
  asChild?: boolean;
}

export function Button({ variant, size, className, asChild, ...props }: ButtonProps) {
  const Comp = asChild ? Slot.Root : "button";
  return <Comp className={cn(buttonVariants({ variant, size }), className)} {...props} />;
}
