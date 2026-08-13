import Link from "next/link";
import { Landmark, ShieldCheck, Target, Wallet } from "lucide-react";
import { Button } from "@/components/ui/Button";

const FEATURES = [
  {
    icon: Wallet,
    title: "Every account, one place",
    description: "Connect checking, savings, and credit accounts and see the full picture at a glance.",
  },
  {
    icon: Target,
    title: "Budgets that catch you early",
    description: "Set a budget per category and get flagged before you overspend, not after.",
  },
  {
    icon: Landmark,
    title: "Goals with real progress",
    description: "Track savings goals against actual balances, not a spreadsheet you forgot to update.",
  },
];

export default function Home() {
  return (
    <div className="relative flex flex-1 flex-col overflow-hidden bg-plane">
      <div
        aria-hidden="true"
        className="pointer-events-none absolute left-1/2 top-0 h-[36rem] w-[36rem] -translate-x-1/2 -translate-y-1/3 rounded-full bg-brand/10 blur-3xl"
      />

      <div className="relative flex flex-1 flex-col items-center justify-center px-6 py-24 text-center">
        <div className="mb-4 flex items-center gap-1.5 rounded-pill border border-hairline bg-surface px-3 py-1 text-xs font-medium text-ink-secondary">
          <ShieldCheck size={14} strokeWidth={1.75} className="text-brand" />
          Your data, your accounts — nothing shared
        </div>
        <h1 className="font-display text-5xl font-medium tracking-tight text-ink-primary sm:text-6xl">
          Finora
        </h1>
        <p className="mt-4 max-w-md text-lg text-ink-secondary">
          Personal finance, kept simple — track spending, budgets, and savings
          goals in one place.
        </p>
        <div className="mt-8 flex justify-center gap-3">
          <Button asChild size="md">
            <Link href="/login">Log in</Link>
          </Button>
          <Button asChild size="md" variant="secondary">
            <Link href="/register">Create account</Link>
          </Button>
        </div>
      </div>

      <div className="relative border-t border-hairline bg-surface px-6 py-16">
        <div className="mx-auto grid max-w-4xl gap-8 sm:grid-cols-3">
          {FEATURES.map(({ icon: Icon, title, description }) => (
            <div key={title} className="flex flex-col items-start gap-3 text-left">
              <span className="flex h-9 w-9 items-center justify-center rounded-control bg-brand-subtle text-brand">
                <Icon size={18} strokeWidth={1.75} />
              </span>
              <h2 className="text-sm font-semibold text-ink-primary">{title}</h2>
              <p className="text-sm text-ink-secondary">{description}</p>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
