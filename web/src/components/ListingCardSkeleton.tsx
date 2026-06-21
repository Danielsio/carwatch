import { cn } from "@/lib/utils";

export function ListingCardSkeleton({ className }: { className?: string }) {
  return (
    <div
      className={cn(
        "rounded-2xl border border-border/50 bg-card overflow-hidden dir-rtl",
        className,
      )}
    >
      {/* Image */}
      <div className="aspect-video w-full skeleton" />

      <div className="p-4">
        {/* Title row: logo + name + score + price */}
        <div className="mb-2.5 flex items-start gap-3">
          <div className="h-9 w-9 shrink-0 rounded-lg skeleton" />
          <div className="min-w-0 flex-1 space-y-1.5">
            <div className="h-4 w-3/4 rounded skeleton" />
            <div className="h-3 w-20 rounded skeleton" />
          </div>
          <div className="flex items-start gap-2">
            <div className="h-9 w-9 shrink-0 rounded-lg skeleton" />
            <div className="space-y-1.5">
              <div className="h-5 w-20 rounded skeleton" />
              <div className="h-3 w-14 rounded skeleton" />
            </div>
          </div>
        </div>

        {/* Meta pills */}
        <div className="flex gap-1.5 mb-3">
          <div className="h-5 w-16 rounded-md skeleton" />
          <div className="h-5 w-12 rounded-md skeleton" />
          <div className="h-5 w-14 rounded-md skeleton" />
        </div>

        {/* Footer */}
        <div className="flex items-center gap-2">
          <div className="h-3 w-16 rounded skeleton me-auto" />
          <div className="h-7 w-7 rounded-md skeleton" />
          <div className="h-7 w-7 rounded-md skeleton" />
        </div>
      </div>
    </div>
  );
}
