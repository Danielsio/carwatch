import type { ReactNode } from "react";
import { Bookmark } from "lucide-react";
import { formatPrice, formatKm, relativeTime, cn } from "@/lib/utils";
import type { Listing } from "@/lib/api";

/**
 * Shared card body for listing display. When `hoverScale` is true the image
 * zooms on hover via `group-hover:scale-105` -- the caller must wrap this
 * component in an element with the Tailwind `group` class for the effect to
 * work.
 */
export function ListingCardBody({
  listing,
  actions,
  hoverScale,
  showBookmarkOverlay,
}: {
  listing: Listing;
  actions?: ReactNode;
  hoverScale?: boolean;
  /** When true, show a bookmark icon on the image (e.g. saved listing). */
  showBookmarkOverlay?: boolean;
}) {
  return (
    <>
      {listing.image_url ? (
        <div className="relative aspect-video w-full overflow-hidden bg-secondary">
          {showBookmarkOverlay ? (
            <div
              className="absolute top-2 start-2 z-10 flex h-8 w-8 items-center justify-center rounded-full bg-background/85 shadow-sm ring-1 ring-border/60 backdrop-blur-[2px]"
              aria-hidden
            >
              <Bookmark className="h-4 w-4 fill-amber-500 text-amber-600 dark:text-amber-400 dark:fill-amber-400" />
            </div>
          ) : null}
          <img
            src={listing.image_url}
            alt={`${listing.manufacturer} ${listing.model}`}
            className={cn(
              "h-full w-full object-cover transition-transform duration-500 ease-out",
              hoverScale && "group-hover:scale-105",
            )}
            loading="lazy"
          />
        </div>
      ) : (
        <div className="aspect-video w-full bg-secondary flex items-center justify-center">
          <span className="text-4xl opacity-20">🚗</span>
        </div>
      )}

      <div className="p-4">
        <div className="flex items-start justify-between mb-2">
          <div>
            <h3 className="font-semibold text-card-foreground">
              {listing.manufacturer} {listing.model}
            </h3>
            <p className="text-xs text-muted-foreground mt-0.5">
              {listing.year}
            </p>
          </div>
          <span className="text-lg font-bold text-amber-500 dark:text-amber-400 tabular-nums">
            {formatPrice(listing.price)}
          </span>
        </div>

        <div className="flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground mb-3">
          <span className="tabular-nums">{formatKm(listing.km)}</span>
          <span className="text-border">·</span>
          <span>יד {listing.hand}</span>
          {listing.city && (
            <>
              <span className="text-border">·</span>
              <span>{listing.city}</span>
            </>
          )}
        </div>

        <div className="flex items-center gap-2">
          {listing.fitness_score != null && (
            <FitnessChip score={listing.fitness_score} />
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

function FitnessChip({ score }: { score: number }) {
  const tier =
    score >= 7 ? "great" : score >= 5 ? "good" : "low";

  const styles = {
    great: "bg-score-great/15 text-score-great",
    good: "bg-score-good/15 text-score-good",
    low: "bg-score-low/15 text-score-low",
  };

  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-semibold tabular-nums",
        styles[tier],
      )}
    >
      {score.toFixed(1)}
      {tier === "great" && " ⭐"}
    </span>
  );
}
