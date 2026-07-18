import Link from "next/link";
import { Button } from "@/components/ui/Button";

export default function Home() {
  return (
    <div className="relative flex flex-1 flex-col items-center justify-center overflow-hidden bg-plane px-6 text-center">
      {/* The one decorative flourish in the whole app — a restrained,
          low-opacity brand-tinted glow behind the wordmark. See
          architecture/frontend-design-system.md §7. */}
      <div
        aria-hidden="true"
        className="pointer-events-none absolute left-1/2 top-1/2 h-[36rem] w-[36rem] -translate-x-1/2 -translate-y-1/2 rounded-full bg-brand/10 blur-3xl"
      />

      <div className="relative">
        <h1 className="text-4xl font-semibold tracking-tight text-ink-primary">
          Finora
        </h1>
        <p className="mt-3 max-w-md text-lg text-ink-secondary">
          Personal finance, kept simple.
        </p>
        <div className="mt-8 flex justify-center gap-4">
          <Link href="/login">
            <Button size="md">Log in</Button>
          </Link>
          <Link href="/register">
            <Button size="md" variant="secondary">
              Create account
            </Button>
          </Link>
        </div>
      </div>
    </div>
  );
}
