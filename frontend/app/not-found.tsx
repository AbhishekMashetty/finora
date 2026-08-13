import Link from "next/link";
import { Compass } from "lucide-react";
import { Button } from "@/components/ui/Button";

export default function NotFound() {
  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-3 bg-plane px-6 py-24 text-center">
      <Compass size={28} strokeWidth={1.75} className="text-ink-muted" />
      <div>
        <h1 className="font-display text-2xl font-medium text-ink-primary">Page not found</h1>
        <p className="mt-1 text-sm text-ink-muted">
          The page you&rsquo;re looking for doesn&rsquo;t exist or has moved.
        </p>
      </div>
      <Button asChild variant="primary" size="sm">
        <Link href="/dashboard">Back to dashboard</Link>
      </Button>
    </div>
  );
}
