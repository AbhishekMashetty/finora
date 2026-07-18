// Shared form-field primitives. See architecture/frontend-design-system.md §5.

import type { InputHTMLAttributes, ReactNode, SelectHTMLAttributes } from "react";

const fieldClasses =
  "rounded-md border border-hairline bg-surface px-3 py-2 text-sm text-ink-primary outline-none focus:border-brand disabled:opacity-50";

function Field({
  label,
  htmlFor,
  error,
  children,
}: {
  label?: string;
  htmlFor?: string;
  error?: string;
  children: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-1">
      {label && (
        <label htmlFor={htmlFor} className="text-xs font-medium text-ink-secondary">
          {label}
        </label>
      )}
      {children}
      {error && <p className="text-xs text-status-critical">{error}</p>}
    </div>
  );
}

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
}

export function Input({ label, error, className = "", id, ...props }: InputProps) {
  return (
    <Field label={label} htmlFor={id} error={error}>
      <input id={id} className={`${fieldClasses} ${className}`} {...props} />
    </Field>
  );
}

export interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  label?: string;
  error?: string;
}

export function Select({ label, error, className = "", id, children, ...props }: SelectProps) {
  return (
    <Field label={label} htmlFor={id} error={error}>
      <select id={id} className={`${fieldClasses} ${className}`} {...props}>
        {children}
      </select>
    </Field>
  );
}
