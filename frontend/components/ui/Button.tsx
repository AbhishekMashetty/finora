// Shared button primitive. See architecture/frontend-design-system.md §5 —
// replaces the near-identical `primaryButtonClasses`/`secondaryButtonClasses`
// string constants that used to be redefined at the top of every page file.

import type { ButtonHTMLAttributes } from "react";

type Variant = "primary" | "secondary" | "danger" | "ghost";
type Size = "sm" | "md";

const base =
  "inline-flex items-center justify-center gap-1.5 rounded-full font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand";

const variantClasses: Record<Variant, string> = {
  primary: "bg-brand text-white hover:opacity-90",
  secondary: "border border-hairline text-ink-primary hover:bg-plane",
  danger: "border border-status-critical/40 text-status-critical hover:bg-status-critical/10",
  ghost: "text-ink-secondary hover:text-ink-primary hover:bg-plane",
};

const sizeClasses: Record<Size, string> = {
  sm: "px-3 py-1.5 text-xs",
  md: "px-5 py-2 text-sm",
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
