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
        <div className="aspect-video w-full overflow-hidden bg-muted">
          <img
            src={listing.image_url}
            alt={`${listing.manufacturer} ${listing.model}`}
            className={cn(
              "h-full w-full object-cover",
              hoverScale && "group-hover:scale-105 transition-transform duration-300",
            )}
            loading="lazy"
          />
        </div>
      ) : (
        <div className="aspect-video w-full bg-muted flex items-center justify-center">
          <span className="text-4xl text-muted-foreground/30">🚗</span>
        </div>
      )}

      <div className="p-4">
        <div className="flex items-start justify-between mb-2">
          <div>
            <h3 className="font-semibold">
              {listing.manufacturer} {listing.model}
            </h3>
            <p className="text-xs text-muted-foreground">{listing.year}</p>
          </div>
          <span className="text-lg font-bold text-primary">
            {formatPrice(listing.price)}
          </span>
        </div>

        <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground mb-3">
          <span>{formatKm(listing.km)}</span>
          <span>יד {listing.hand}</span>
          {listing.city && <span>{listing.city}</span>}
        </div>

        <div className="flex items-center gap-2">
          {listing.fitness_score != null && (
            <span
              className={cn(
                "inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium",
                listing.fitness_score >= 7
                  ? "bg-green-100 text-green-800"
                  : listing.fitness_score >= 5
                    ? "bg-yellow-100 text-yellow-800"
                    : "bg-gray-100 text-gray-600",
              )}
            >
              {listing.fitness_score.toFixed(1)}
            </span>
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
