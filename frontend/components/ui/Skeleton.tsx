// Loading-placeholder primitive — replaces literal "Loading…" text with a
// standard perceived-performance shimmer block. See
// architecture/frontend-design-system.md §5.

export function Skeleton({ className = "" }: { className?: string }) {
  return <div className={`animate-pulse rounded-md bg-ink-muted/15 ${className}`} />;
}

/** A few stacked skeleton rows, for a list/table that's still loading. */
export function SkeletonRows({ rows = 3 }: { rows?: number }) {
  return (
    <div className="flex flex-col gap-3 p-6">
      {Array.from({ length: rows }).map((_, i) => (
        <Skeleton key={i} className="h-10 w-full" />
      ))}
    </div>
  );
}
