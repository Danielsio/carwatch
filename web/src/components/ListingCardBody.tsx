import type { ReactNode } from "react";
import { Bookmark } from "lucide-react";
import { formatPrice, formatKm, relativeTime, cn } from "@/lib/utils";
import type { Listing } from "@/lib/api";
import { MatchScoreBox } from "@/components/ui/MatchScoreBox";
import { scoreColor, scoreLabel } from "@/lib/scoringAlgorithm";

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
            referrerPolicy="no-referrer"
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
        <div className="mb-2 flex items-start gap-3">
          {listing.fitness_score != null ? (
            <MatchScoreBox score={listing.fitness_score} size="md" />
          ) : null}
          <div className="min-w-0 flex-1">
            <h3 className="font-semibold text-card-foreground">
              {listing.manufacturer} {listing.model}
            </h3>
            <p className="mt-0.5 text-xs text-muted-foreground">{listing.year}</p>
            {listing.fitness_score != null ? (
              <p
                className={cn(
                  "mt-0.5 text-xs font-medium",
                  scoreColor(listing.fitness_score),
                )}
              >
                {scoreLabel(listing.fitness_score)}
              </p>
            ) : null}
          </div>
          <span className="shrink-0 text-lg font-bold tabular-nums text-primary">
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
          <span className="mr-auto text-xs text-muted-foreground">
            {relativeTime(listing.first_seen_at)}
          </span>
          {actions}
        </div>
      </div>
    </>
  );
}
