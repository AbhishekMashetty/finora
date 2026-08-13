"use client";

import { useEffect } from "react";
import { AlertTriangle } from "lucide-react";
import { Button } from "@/components/ui/Button";

export default function DashboardError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    // eslint-disable-next-line no-console
    console.error(error);
  }, [error]);

  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-3 py-24 text-center">
      <AlertTriangle size={28} strokeWidth={1.75} className="text-status-critical" />
      <div>
        <h2 className="text-sm font-medium text-ink-primary">Something went wrong</h2>
        <p className="mt-1 text-sm text-ink-muted">
          This page hit an unexpected error. You can try again.
        </p>
      </div>
      <Button variant="secondary" onClick={() => reset()}>
        Try again
      </Button>
    </div>
  );
}
