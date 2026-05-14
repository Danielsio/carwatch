import { cn } from "@/lib/utils";

export function ListingCardSkeleton({ className }: { className?: string }) {
  return (
    <div
      className={cn(
        "rounded-2xl border border-border/50 bg-card overflow-hidden dir-rtl",
        className,
      )}
    >
      {/* Image area */}
      <div className="aspect-video w-full shimmer-skeleton" />

      <div className="p-4">
        {/* Title row: logo + score + name/year + price */}
        <div className="mb-2 flex items-start gap-3">
          <div className="h-10 w-10 shrink-0 rounded-lg shimmer-skeleton" />
          <div className="h-10 w-10 shrink-0 rounded-xl shimmer-skeleton" />
          <div className="min-w-0 flex-1 space-y-1.5">
            <div className="h-4 w-3/4 rounded shimmer-skeleton" />
            <div className="h-3 w-12 rounded shimmer-skeleton" />
          </div>
          <div className="shrink-0 space-y-1.5 text-end">
            <div className="h-5 w-20 rounded shimmer-skeleton ms-auto" />
            <div className="h-3 w-14 rounded shimmer-skeleton ms-auto" />
          </div>
        </div>

        {/* Meta line: km · hand · city */}
        <div className="flex gap-3 mb-3">
          <div className="h-3.5 w-16 rounded shimmer-skeleton" />
          <div className="h-3.5 w-10 rounded shimmer-skeleton" />
          <div className="h-3.5 w-14 rounded shimmer-skeleton" />
        </div>

        {/* Footer: time + action buttons */}
        <div className="flex items-center gap-2">
          <div className="h-3 w-20 rounded shimmer-skeleton me-auto" />
          <div className="h-7 w-7 rounded-lg shimmer-skeleton" />
          <div className="h-7 w-7 rounded-lg shimmer-skeleton" />
          <div className="h-7 w-7 rounded-lg shimmer-skeleton" />
        </div>
      </div>
    </div>
  );
}
