// Inline form/page banner — success, error, or informational. Replaces the
// repeated `<p className="rounded-md bg-status-*/10 ...">` pattern that used
// to be hand-copied into every page with a form.

import type { ReactNode } from "react";
import { AlertTriangle, CheckCircle2, Info } from "lucide-react";
import { cn } from "@/lib/cn";

export type AlertVariant = "success" | "error" | "info";

const VARIANT_STYLES: Record<AlertVariant, { classes: string; icon: typeof Info }> = {
  success: { classes: "bg-status-good-subtle text-status-good-text", icon: CheckCircle2 },
  error: { classes: "bg-status-critical-subtle text-status-critical", icon: AlertTriangle },
  info: { classes: "bg-brand-subtle text-brand", icon: Info },
};

export function Alert({
  variant = "info",
  children,
  className,
}: {
  variant?: AlertVariant;
  children: ReactNode;
  className?: string;
}) {
  const { classes, icon: Icon } = VARIANT_STYLES[variant];
  return (
    <div
      role={variant === "error" ? "alert" : "status"}
      className={cn(
        "flex items-start gap-2 rounded-control px-3 py-2.5 text-sm",
        classes,
        className,
      )}
    >
      <Icon size={16} strokeWidth={1.75} className="mt-0.5 shrink-0" aria-hidden="true" />
      <span>{children}</span>
    </div>
  );
}
