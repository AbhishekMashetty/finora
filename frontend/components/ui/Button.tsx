// Shared button primitive. See architecture/frontend-design-system.md §5 —
// replaces the near-identical `primaryButtonClasses`/`secondaryButtonClasses`
// string constants that used to be redefined at the top of every page file.

import type { ButtonHTMLAttributes } from "react";

type Variant = "primary" | "secondary" | "danger" | "ghost";
type Size = "sm" | "md";

const base =
  "inline-flex items-center justify-center gap-1.5 whitespace-nowrap rounded-full font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand";

const variantClasses: Record<Variant, string> = {
  primary: "bg-brand text-white hover:opacity-90",
  secondary: "border border-hairline text-ink-primary hover:bg-plane",
  danger: "border border-status-critical/40 text-status-critical hover:bg-status-critical/10",
  ghost: "text-ink-secondary hover:text-ink-primary hover:bg-plane",
};

// `md` is `h-10` — an explicit height, not padding-derived — so it is
// pixel-identical to Input/Select's own `h-10` (components/ui/Input.tsx).
// This matters anywhere a `md` button sits in the same row as a labeled
// field (every "Add X" form submit button in this app): with padding alone
// determining height, a button measures shorter than a field (36px vs
// 40px) and, bottom-aligned via `items-end`, visibly floats a few pixels
// proud of the field it sits beside — confirmed by measuring real
// bounding boxes on Accounts/Budgets/Goals/Reports/Search, not by
// eyeballing. `sm` is deliberately left padding-derived (not h-10): it's
// used for compact contexts with no adjacent field to match (table row
// icons, pagination, pill toggles) where a 40px box would look oversized.
// Where a `sm` button DOES sit beside a labeled field (transactions' "New
// category"/"Add category", goals' "Update"), the call site adds `h-10`
// via `className` rather than changing `sm` globally — see those pages.
const sizeClasses: Record<Size, string> = {
  sm: "px-3 py-1.5 text-xs",
  md: "h-10 px-5 text-sm",
};

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: Size;
}

export function Button({ variant = "primary", size = "md", className = "", ...props }: ButtonProps) {
  return (
    <button
      className={`${base} ${variantClasses[variant]} ${sizeClasses[size]} ${className}`}
      {...props}
    />
  );
}
