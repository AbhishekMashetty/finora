"use client";

import type { ReactNode } from "react";
import { TooltipProvider } from "@/components/ui/Tooltip";
import { ThemeProvider } from "@/components/theme/ThemeProvider";
import { AppToaster } from "@/components/theme/AppToaster";
import { AuthProvider } from "@/lib/auth-context";

/** Single composition point for every app-wide client provider. */
export function AppProviders({ children }: { children: ReactNode }) {
  return (
    <ThemeProvider>
      <TooltipProvider delayDuration={200}>
        <AuthProvider>{children}</AuthProvider>
        <AppToaster />
      </TooltipProvider>
    </ThemeProvider>
  );
}
