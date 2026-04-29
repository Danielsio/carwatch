import type { ReactNode } from "react";
import { formatPrice, formatKm, relativeTime, cn } from "@/lib/utils";
import type { Listing } from "@/lib/api";

export function ListingCardBody({
  listing,
  actions,
  hoverScale,
}: {
  listing: Listing;
  actions?: ReactNode;
  hoverScale?: boolean;
}) {
  return (
    <>
      {listing.image_url ? (
        <div className="aspect-video w-full overflow-hidden bg-muted relative">
          <img
            src={listing.image_url}
            alt={`${listing.manufacturer} ${listing.model}`}
            className={cn(
              "h-full w-full object-cover",
              hoverScale &&
                "group-hover:scale-105 transition-transform duration-500 ease-out",
            )}
            loading="lazy"
          />
          <div className="absolute inset-0 bg-gradient-to-t from-black/20 to-transparent pointer-events-none" />
        </div>
      ) : (
        <div className="aspect-video w-full bg-gradient-to-br from-muted to-muted/50 flex items-center justify-center">
          <span className="text-5xl opacity-20">🚗</span>
        </div>
      )}

      <div className="p-4">
        <div className="flex items-start justify-between mb-2">
          <div>
            <h3 className="font-bold tracking-tight">
              {listing.manufacturer} {listing.model}
            </h3>
            <p className="text-xs text-muted-foreground font-medium">
              {listing.year}
            </p>
          </div>
          <span className="text-lg font-bold text-gradient">
            {formatPrice(listing.price)}
          </span>
        </div>

        <div className="flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground mb-3">
          <span className="inline-flex items-center gap-1 rounded-md bg-secondary px-1.5 py-0.5">
            {formatKm(listing.km)}
          </span>
          <span className="inline-flex items-center gap-1 rounded-md bg-secondary px-1.5 py-0.5">
            יד {listing.hand}
          </span>
          {listing.city && (
            <span className="inline-flex items-center gap-1 rounded-md bg-secondary px-1.5 py-0.5">
              {listing.city}
            </span>
          )}
        </div>

        <div className="flex items-center gap-2">
          {listing.fitness_score != null && (
            <ScoreBadge score={listing.fitness_score} />
          )}
          <span className="text-xs text-muted-foreground mr-auto">
            {relativeTime(listing.first_seen_at)}
          </span>
          {actions}
        </div>
      </div>
    </>
  );
}

function ScoreBadge({ score }: { score: number }) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-bold tabular-nums",
        score >= 7
          ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-400"
          : score >= 5
            ? "bg-amber-100 text-amber-700 dark:bg-amber-500/20 dark:text-amber-400"
            : "bg-secondary text-muted-foreground",
      )}
    >
      {score.toFixed(1)}
      {score >= 7 && " ★"}
    </span>
  );
}
