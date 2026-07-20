// Shared form-field primitives. See architecture/frontend-design-system.md §5.

import type { InputHTMLAttributes, ReactNode, SelectHTMLAttributes } from "react";

// h-10 is load-bearing, not decorative: native <input type="date">/
// type="number"> controls render with browser-supplied internal chrome
// (a calendar/stepper widget) that gives them a different intrinsic
// height than a <select> or plain text input even with identical padding
// classes — most visible in Safari/WebKit, where a date field renders
// visibly "deeper" than the account/category selects beside it in the
// same row. An explicit height overrides that per-control-type intrinsic
// sizing so every field in a row is pixel-identical regardless of type or
// browser, instead of leaving each native control to size itself.
const fieldClasses =
  "h-10 rounded-md border border-hairline bg-surface px-3 text-sm text-ink-primary outline-none focus:border-brand disabled:opacity-50";

function Field({
  label,
  htmlFor,
  error,
  wrapperClassName = "",
  children,
}: {
  label?: string;
  htmlFor?: string;
  error?: string;
  wrapperClassName?: string;
  children: ReactNode;
}) {
  return (
    <div className={`flex flex-col gap-1 ${wrapperClassName}`}>
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

// `className` styles the input/select element itself (width overrides like
// `w-24`, text utilities like `tabular-nums`). `wrapperClassName` is for
// layout classes that must land on the field's outer wrapper div instead —
// most importantly grid placement (`col-span-*`) and flex sizing
// (`flex-1`), since that wrapper (not the inner control) is the actual
// grid/flex item. Passing `col-span-*` via `className` is a no-op: it lands
// on the input, which isn't a grid child, so the span is silently ignored
// and the field shrinks to its own content width instead — this is exactly
// the bug that made the transactions page's "New transaction" row of
// fields collapse to a cramped, truncated cluster on the left instead of
// spanning the full card width (see architecture/frontend-design-system.md §5).
export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
  wrapperClassName?: string;
}

export function Input({ label, error, wrapperClassName, className = "", id, ...props }: InputProps) {
  return (
    <Field label={label} htmlFor={id} error={error} wrapperClassName={wrapperClassName}>
      <input id={id} className={`${fieldClasses} ${className}`} {...props} />
    </Field>
  );
}

export interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  label?: string;
  error?: string;
  wrapperClassName?: string;
}

export function Select({ label, error, wrapperClassName, className = "", id, children, ...props }: SelectProps) {
  return (
    <Field label={label} htmlFor={id} error={error} wrapperClassName={wrapperClassName}>
      <select id={id} className={`${fieldClasses} ${className}`} {...props}>
        {children}
      </select>
    </Field>
  );
}
