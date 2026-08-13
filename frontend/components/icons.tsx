// Icon set — backed by lucide-react (see architecture decision in
// architecture/frontend-design-system.md §1: the earlier "14 fixed glyphs,
// not worth a dependency" call was revisited for the revamp — a real icon
// library gives a far larger, more consistent glyph set for the pages still
// to come, and the dependency cost is no longer a real objection at this
// app's size).
//
// This file stays as a thin re-export/wrapper layer rather than switching
// every call site to `import { Home } from "lucide-react"` directly: it
// preserves the exact prior API (component name, default size 20,
// hand-drawn-weight 1.75 stroke) so none of the 13 pages importing from
// "@/components/icons" need to change until they're rebuilt in Phase 4.

import {
  ArrowDown,
  ArrowUp,
  Bell,
  Check,
  Flag,
  Home,
  Inbox,
  List,
  BarChart3,
  LogOut,
  Pencil,
  Plus,
  Search,
  Settings,
  Target,
  Trash2,
  Upload,
  User,
  Wallet,
  type LucideIcon,
  type LucideProps,
} from "lucide-react";

export interface IconProps extends Omit<LucideProps, "size"> {
  size?: number;
}

function wrap(LucideGlyph: LucideIcon) {
  function WrappedIcon({ size = 20, strokeWidth = 1.75, ...props }: IconProps) {
    return <LucideGlyph size={size} strokeWidth={strokeWidth} aria-hidden="true" {...props} />;
  }
  return WrappedIcon;
}

export const HomeIcon = wrap(Home);
export const WalletIcon = wrap(Wallet);
export const ListIcon = wrap(List);
export const TargetIcon = wrap(Target);
export const FlagIcon = wrap(Flag);
export const ChartIcon = wrap(BarChart3);
export const SearchIcon = wrap(Search);
export const UserIcon = wrap(User);
export const GearIcon = wrap(Settings);
export const LogOutIcon = wrap(LogOut);
export const PlusIcon = wrap(Plus);
export const TrashIcon = wrap(Trash2);
export const PencilIcon = wrap(Pencil);
export const CheckIcon = wrap(Check);
export const ArrowUpIcon = wrap(ArrowUp);
export const ArrowDownIcon = wrap(ArrowDown);
export const InboxIcon = wrap(Inbox);
export const BellIcon = wrap(Bell);
export const UploadIcon = wrap(Upload);
