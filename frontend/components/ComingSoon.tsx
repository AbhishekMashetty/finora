import { EmptyState } from "@/components/ui/EmptyState";
import { InboxIcon } from "@/components/icons";

export function ComingSoon({
  title,
  description,
}: {
  title: string;
  description?: string;
}) {
  return (
    <div className="flex flex-1 flex-col">
      <h1 className="text-2xl font-semibold text-ink-primary">{title}</h1>
      <div className="mt-6 flex flex-1">
        <EmptyState
          icon={<InboxIcon size={28} />}
          title="Coming soon"
          description={description ?? "This screen isn't built yet."}
        />
      </div>
    </div>
  );
}
