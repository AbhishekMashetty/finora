// Hand-rolled icon set — see architecture/frontend-design-system.md §1 for
// why this exists instead of an icon-library dependency (lucide-react,
// heroicons, etc): the app needs ~14 fixed glyphs total, so a dependency
// costs more (install, version, audit) than the paths themselves.
//
// Every icon is stroke-based, 24x24 viewBox, themed via `currentColor` so it
// inherits text color (including the design tokens in globals.css) with no
// per-icon color prop needed. Default size is 20px; pass `size` to override.

import type { SVGProps } from "react";

export interface IconProps extends SVGProps<SVGSVGElement> {
  size?: number;
}

function Icon({ size = 20, children, ...props }: IconProps & { children: React.ReactNode }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.75}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      {...props}
    >
      {children}
    </svg>
  );
}

export function HomeIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="M4 11.5 12 4l8 7.5" />
      <path d="M6 10v9a1 1 0 0 0 1 1h4v-6h2v6h4a1 1 0 0 0 1-1v-9" />
    </Icon>
  );
}

export function WalletIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <rect x="3" y="6" width="18" height="13" rx="2" />
      <path d="M3 10h18" />
      <path d="M16 14.5h2" />
    </Icon>
  );
}

export function ListIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="M8 6h12" />
      <path d="M8 12h12" />
      <path d="M8 18h12" />
      <path d="M4 6h.01" />
      <path d="M4 12h.01" />
      <path d="M4 18h.01" />
    </Icon>
  );
}

export function TargetIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <circle cx="12" cy="12" r="8" />
      <circle cx="12" cy="12" r="4.5" />
      <circle cx="12" cy="12" r="1" fill="currentColor" stroke="none" />
    </Icon>
  );
}

export function FlagIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="M5 4v16" />
      <path d="M5 5h11l-2.5 3.5L16 12H5" />
    </Icon>
  );
}

export function ChartIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="M4 20V10" />
      <path d="M11 20V4" />
      <path d="M18 20v-7" />
      <path d="M3 20h18" />
    </Icon>
  );
}

export function SearchIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <circle cx="11" cy="11" r="6.5" />
      <path d="m20 20-3.8-3.8" />
    </Icon>
  );
}

export function UserIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <circle cx="12" cy="8" r="3.5" />
      <path d="M5 20c0-3.5 3-6 7-6s7 2.5 7 6" />
    </Icon>
  );
}

export function GearIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <circle cx="12" cy="12" r="3" />
      <path d="M19.4 13.5a7.5 7.5 0 0 0 0-3l1.9-1.3-1.5-2.6-2.2.7a7.4 7.4 0 0 0-2.6-1.5L14.5 3h-3l-.5 2.3a7.4 7.4 0 0 0-2.6 1.5l-2.2-.7-1.5 2.6L6 10.5a7.5 7.5 0 0 0 0 3l-1.9 1.3 1.5 2.6 2.2-.7a7.4 7.4 0 0 0 2.6 1.5l.5 2.3h3l.5-2.3a7.4 7.4 0 0 0 2.6-1.5l2.2.7 1.5-2.6z" />
    </Icon>
  );
}

export function LogOutIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="M9 4H6a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h3" />
      <path d="M15 16l5-4-5-4" />
      <path d="M20 12H9" />
    </Icon>
  );
}

export function PlusIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="M12 5v14" />
      <path d="M5 12h14" />
    </Icon>
  );
}

export function TrashIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="M4 7h16" />
      <path d="M9 7V4h6v3" />
      <path d="M6 7l1 13h10l1-13" />
      <path d="M10 11v6" />
      <path d="M14 11v6" />
    </Icon>
  );
}

export function PencilIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="M4 20h4l10.5-10.5a2 2 0 0 0 0-2.8l-1.2-1.2a2 2 0 0 0-2.8 0L4 16v4" />
      <path d="M13 6.5 17.5 11" />
    </Icon>
  );
}

export function CheckIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="m5 12.5 4.5 4.5L19 7" />
    </Icon>
  );
}

export function ArrowUpIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="M12 19V5" />
      <path d="m6 10 6-6 6 6" />
    </Icon>
  );
}

export function ArrowDownIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="M12 5v14" />
      <path d="m18 14-6 6-6-6" />
    </Icon>
  );
}

export function InboxIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="M4 12h4l1.5 3h5L16 12h4" />
      <path d="M5.5 6h13l2 6.5V18a1.5 1.5 0 0 1-1.5 1.5H5a1.5 1.5 0 0 1-1.5-1.5v-5.5z" />
    </Icon>
  );
}

export function BellIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="M6 10a6 6 0 0 1 12 0c0 4 1.5 5.5 2 6.5H4c.5-1 2-2.5 2-6.5Z" />
      <path d="M10 19a2 2 0 0 0 4 0" />
    </Icon>
  );
}

export function UploadIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="M12 15V4" />
      <path d="M7.5 8.5 12 4l4.5 4.5" />
      <path d="M4 15v3a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-3" />
    </Icon>
  );
}
