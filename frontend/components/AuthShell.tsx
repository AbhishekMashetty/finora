// Two-pane auth layout shared by /login and /register. See
// architecture/frontend-design-system.md §7 — replaces the previous bare
// centered card, which said nothing about the product and wasted the full
// viewport on larger screens.

import type { ReactNode } from "react";
import { Check } from "lucide-react";

const VALUE_BULLETS = [
  "See every account and transaction in one place",
  "Set budgets and catch overspending before it happens",
  "Track savings goals with real progress, not guesswork",
];

export function AuthShell({ children }: { children: ReactNode }) {
  return (
    <div className="flex flex-1">
      {/* Fixed dark panel, deliberately independent of the site's light/dark
          theme (a `bg-ink-primary` token would invert to white in dark mode,
          which is not the intent — this pane is always dark). */}
      <div className="hidden w-[40%] flex-col justify-between bg-[#0b0b0b] p-10 text-[#f9f9f7] md:flex">
        <div className="font-display text-2xl font-medium tracking-tight">Finora</div>
        <ul className="flex flex-col gap-4 text-sm text-[#f9f9f7]/80">
          {VALUE_BULLETS.map((bullet) => (
            <li key={bullet} className="flex gap-2.5">
              <span className="mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded-full bg-[#9085e9]/20 text-[#9085e9]">
                <Check size={11} strokeWidth={2.5} aria-hidden="true" />
              </span>
              {bullet}
            </li>
          ))}
        </ul>
        <p className="text-xs text-[#f9f9f7]/50">
          Personal finance, kept simple.
        </p>
      </div>
      <div className="flex flex-1 items-center justify-center bg-plane px-6 py-12">
        <div className="w-full max-w-sm">{children}</div>
      </div>
    </div>
  );
}
