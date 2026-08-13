import {
  BarChart3,
  Bell,
  Flag,
  Home,
  List,
  Search,
  Settings,
  Target,
  User,
  Wallet,
  type LucideIcon,
} from "lucide-react";

export interface NavItem {
  href: string;
  label: string;
  icon: LucideIcon;
}

export interface NavGroup {
  label: string | null;
  items: NavItem[];
}

// Grouped by task, not alphabetically or by build order — three shallow
// groups read faster in a sidebar than ten flat items at the same level.
export const NAV_GROUPS: NavGroup[] = [
  {
    label: null,
    items: [{ href: "/dashboard", label: "Overview", icon: Home }],
  },
  {
    label: "Money",
    items: [
      { href: "/dashboard/accounts", label: "Accounts", icon: Wallet },
      { href: "/dashboard/transactions", label: "Transactions", icon: List },
    ],
  },
  {
    label: "Planning",
    items: [
      { href: "/dashboard/budgets", label: "Budgets", icon: Target },
      { href: "/dashboard/goals", label: "Goals", icon: Flag },
      { href: "/dashboard/reports", label: "Reports", icon: BarChart3 },
    ],
  },
  {
    label: "Account",
    items: [
      { href: "/dashboard/notifications", label: "Notifications", icon: Bell },
      { href: "/dashboard/search", label: "Search", icon: Search },
      { href: "/dashboard/profile", label: "Profile", icon: User },
      { href: "/dashboard/settings", label: "Settings", icon: Settings },
    ],
  },
];

export const ALL_NAV_ITEMS: NavItem[] = NAV_GROUPS.flatMap((g) => g.items);
