// Server Component shell: the interactive parts (auth guard, nav
// highlighting, notification polling, mobile drawer) live in
// DashboardShell, a Client Component. `children` is handed to it
// untouched, so route segments underneath keep rendering on the server as
// they're migrated to RSC page-by-page in the phases that follow.

import type { ReactNode } from "react";
import { DashboardShell } from "@/components/dashboard/DashboardShell";

export default function DashboardLayout({ children }: { children: ReactNode }) {
  return <DashboardShell>{children}</DashboardShell>;
}
