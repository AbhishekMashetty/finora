"use client";

import type { CSSProperties } from "react";
import { Toaster as SonnerToaster } from "sonner";
import { useTheme } from "./ThemeProvider";

const toasterStyle = {
  "--border-radius": "var(--radius-control)",
} as CSSProperties;

/** Global toast host — mounted once in AppProviders. Call `toast(...)` from "sonner" anywhere. */
export function AppToaster() {
  const { resolvedTheme } = useTheme();
  return (
    <SonnerToaster theme={resolvedTheme} position="bottom-right" closeButton style={toasterStyle} />
  );
}
