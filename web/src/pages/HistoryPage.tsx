import { useState } from "react";
import { Clock, ExternalLink } from "lucide-react";
import { useHistory } from "@/hooks/useBookmarks";
import { formatPrice, formatKm, relativeTime, cn } from "@/lib/utils";
import type { Listing } from "@/lib/api";

const PAGE_SIZE = 20;

export function HistoryPage() {
  const [offset, setOffset] = useState(0);
  const { data, isLoading, isError } = useHistory(PAGE_SIZE, offset);

  if (isLoading) {
    return (
      <div className="space-y-4">
        <h1 className="text-2xl font-bold">היסטוריה</h1>
        <div className="grid gap-4 sm:grid-cols-2">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="h-48 animate-pulse rounded-xl bg-muted" />
          ))}
        </div>
      </div>
    );
  }

  if (isError) {
    return (
      <div className="space-y-4">
        <h1 className="text-2xl font-bold">היסטוריה</h1>
        <div className="rounded-xl border border-destructive/50 bg-destructive/10 p-6 text-center">
          <p className="text-destructive font-medium">שגיאה בטעינת ההיסטוריה</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-4 pb-20 md:pb-4">
      <div className="flex items-center gap-2">
        <Clock className="h-5 w-5 text-primary" />
        <h1 className="text-2xl font-bold">היסטוריה</h1>
        {data && (
          <span className="text-sm text-muted-foreground">
            ({data.total} מודעות)
          </span>
        )}
      </div>

      {!data || data.items.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border py-16">
          <Clock className="h-8 w-8 text-muted-foreground/30 mb-3" />
          <p className="text-muted-foreground">אין מודעות בהיסטוריה</p>
          <p className="text-sm text-muted-foreground mt-1">
            מודעות שנמצאו יופיעו כאן
          </p>
        </div>
      ) : (
        <>
          <div className="grid gap-4 sm:grid-cols-2">
            {data.items.map((listing) => (
              <HistoryCard key={listing.token} listing={listing} />
            ))}
          </div>

          {data.total > PAGE_SIZE && (
            <div className="flex items-center justify-center gap-3 pt-4">
              {offset > 0 && (
                <button
                  onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
                  className="rounded-lg bg-secondary px-4 py-2 text-sm font-medium text-secondary-foreground hover:bg-secondary/80 transition-colors"
                >
                  הקודם
                </button>
              )}
              <span className="text-sm text-muted-foreground">
                {offset + 1}–{Math.min(offset + PAGE_SIZE, data.total)} מתוך{" "}
                {data.total}
              </span>
              {offset + PAGE_SIZE < data.total && (
                <button
                  onClick={() => setOffset(offset + PAGE_SIZE)}
                  className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 transition-colors"
                >
                  הבא
                </button>
              )}
            </div>
          )}
        </>
      )}
    </div>
  );
}

function HistoryCard({ listing }: { listing: Listing }) {
  return (
    <a
      href={listing.page_link}
      target="_blank"
      rel="noopener noreferrer"
      className="group block rounded-xl border border-border bg-card overflow-hidden shadow-sm hover:shadow-md transition-shadow"
    >
      {listing.image_url ? (
        <div className="aspect-video w-full overflow-hidden bg-muted">
          <img
            src={listing.image_url}
            alt={`${listing.manufacturer} ${listing.model}`}
            className="h-full w-full object-cover group-hover:scale-105 transition-transform duration-300"
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
          <ExternalLink className="h-3.5 w-3.5 text-muted-foreground group-hover:text-primary transition-colors" />
        </div>
      </div>
    </a>
  );
}
