import { useState } from "react";
import { useParams, Link, useNavigate } from "react-router";
import { ArrowRight, ExternalLink, ChevronDown, Bookmark } from "lucide-react";
import { useListings } from "@/hooks/useListings";
import { useSaveBookmark, useRemoveBookmark } from "@/hooks/useBookmarks";
import { formatPrice, formatKm, relativeTime, safeHref, cn } from "@/lib/utils";
import type { Listing } from "@/lib/api";

const SORT_OPTIONS = [
  { value: "newest", label: "חדשים" },
  { value: "price_asc", label: "מחיר ↑" },
  { value: "price_desc", label: "מחיר ↓" },
  { value: "score", label: "ציון" },
  { value: "km", label: 'ק"מ' },
  { value: "year", label: "שנה" },
];

const PAGE_SIZE = 20;

export function ListingsPage() {
  const { id } = useParams();
  const searchId = Number(id);
  const [sort, setSort] = useState("newest");
  const [offset, setOffset] = useState(0);

  const { data, isLoading, isError } = useListings(
    searchId,
    sort,
    PAGE_SIZE,
    offset,
  );

  if (!searchId || Number.isNaN(searchId)) {
    return (
      <div className="space-y-4">
        <div className="flex items-center gap-3">
          <Link
            to="/"
            className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground transition-colors duration-200"
          >
            <ArrowRight className="h-4 w-4" />
            חזרה
          </Link>
          <h1 className="text-2xl font-semibold tracking-tight">
            חיפוש לא נמצא
          </h1>
        </div>
      </div>
    );
  }

  if (isError) {
    return (
      <div className="space-y-4">
        <div className="flex items-center gap-3">
          <Link
            to="/"
            className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground transition-colors duration-200"
          >
            <ArrowRight className="h-4 w-4" />
            חזרה
          </Link>
          <h1 className="text-2xl font-semibold tracking-tight">תוצאות</h1>
        </div>
        <div className="rounded-2xl border border-destructive/20 bg-destructive/5 p-8 text-center">
          <p className="text-destructive font-medium">שגיאה בטעינת התוצאות</p>
          <p className="text-sm text-muted-foreground mt-1">נסה לרענן את הדף</p>
        </div>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="space-y-4">
        <div className="h-8 w-40 shimmer-skeleton rounded-lg" />
        <div className="flex gap-2">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="h-9 w-16 shimmer-skeleton rounded-lg" />
          ))}
        </div>
        <div className="grid gap-4 sm:grid-cols-2">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="h-72 shimmer-skeleton rounded-2xl" />
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-5 pb-20 md:pb-4">
      {/* Header */}
      <div className="flex items-center gap-3">
        <Link
          to="/"
          className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground transition-colors duration-200"
        >
          <ArrowRight className="h-4 w-4" />
          חזרה
        </Link>
        <h1 className="text-2xl font-semibold tracking-tight">תוצאות</h1>
        {data && (
          <span className="text-sm text-muted-foreground tabular-nums">
            ({data.total} מודעות)
          </span>
        )}
      </div>

      {/* Sort pills */}
      <div className="flex flex-wrap gap-2">
        {SORT_OPTIONS.map((opt) => (
          <button
            key={opt.value}
            onClick={() => {
              setSort(opt.value);
              setOffset(0);
            }}
            className={cn(
              "rounded-lg px-3.5 py-1.5 text-sm font-medium transition-all duration-200",
              sort === opt.value
                ? "bg-primary text-primary-foreground shadow-[0_0_12px_rgba(59,130,246,0.3)]"
                : "bg-secondary text-secondary-foreground hover:bg-accent",
            )}
          >
            {opt.label}
          </button>
        ))}
      </div>

      {/* Grid */}
      {!data || data.items.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-2xl border border-border/50 bg-card/50 py-20">
          <p className="text-muted-foreground">אין תוצאות עדיין</p>
          <p className="text-sm text-muted-foreground mt-1">
            רכבים חדשים יופיעו כאן כשהם ימצאו
          </p>
        </div>
      ) : (
        <>
          <div className="grid gap-4 sm:grid-cols-2">
            {data.items.map((listing) => (
              <ListingCard key={listing.token} listing={listing} />
            ))}
          </div>

          {data.total > PAGE_SIZE && (
            <div className="flex items-center justify-center gap-3 pt-4">
              {offset > 0 && (
                <button
                  onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
                  className="rounded-xl bg-secondary px-4 py-2 text-sm font-medium text-secondary-foreground transition-all duration-200 hover:bg-accent active:scale-[0.97]"
                >
                  הקודם
                </button>
              )}
              <span className="text-sm text-muted-foreground tabular-nums">
                {offset + 1}–{Math.min(offset + PAGE_SIZE, data.total)} מתוך{" "}
                {data.total}
              </span>
              {offset + PAGE_SIZE < data.total && (
                <button
                  onClick={() => setOffset(offset + PAGE_SIZE)}
                  className="inline-flex items-center gap-1 rounded-xl bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-all duration-200 hover:bg-primary/90 active:scale-[0.97]"
                >
                  <ChevronDown className="h-4 w-4" />
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

function ListingCard({ listing }: { listing: Listing }) {
  const navigate = useNavigate();
  const saveBookmark = useSaveBookmark();
  const removeBookmark = useRemoveBookmark();
  const [saved, setSaved] = useState(false);

  return (
    <div
      role="button"
      tabIndex={0}
      onClick={() =>
        navigate(`/listings/${listing.token}`, { state: { listing } })
      }
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          navigate(`/listings/${listing.token}`, { state: { listing } });
        }
      }}
      className="group block cursor-pointer rounded-2xl border border-border/50 bg-card overflow-hidden transition-all duration-300 hover:border-border hover:shadow-[0_8px_32px_rgba(0,0,0,0.4)] hover:-translate-y-0.5"
    >
      {/* Image */}
      {listing.image_url ? (
        <div className="aspect-video w-full overflow-hidden bg-secondary">
          <img
            src={listing.image_url}
            alt={`${listing.manufacturer} ${listing.model}`}
            className="h-full w-full object-cover transition-transform duration-500 ease-out group-hover:scale-105"
            loading="lazy"
          />
        </div>
      ) : (
        <div className="aspect-video w-full bg-secondary flex items-center justify-center">
          <span className="text-4xl opacity-20">🚗</span>
        </div>
      )}

      <div className="p-4">
        {/* Title + Price */}
        <div className="flex items-start justify-between mb-2">
          <div>
            <h3 className="font-semibold text-card-foreground">
              {listing.manufacturer} {listing.model}
            </h3>
            <p className="text-xs text-muted-foreground mt-0.5">
              {listing.year}
            </p>
          </div>
          <span className="text-lg font-bold text-primary tabular-nums">
            {formatPrice(listing.price)}
          </span>
        </div>

        {/* Specs */}
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

        {/* Badges + actions */}
        <div className="flex items-center gap-2">
          {listing.fitness_score != null && (
            <ScoreBadge score={listing.fitness_score} />
          )}
          <span className="text-xs text-muted-foreground mr-auto">
            {relativeTime(listing.first_seen_at)}
          </span>
          <button
            type="button"
            aria-label={saved ? "הסר משמורים" : "שמור מודעה"}
            aria-pressed={saved}
            onClick={(e) => {
              e.stopPropagation();
              const next = !saved;
              setSaved(next);
              const mutation = next ? saveBookmark : removeBookmark;
              mutation.mutate(listing.token, {
                onError: () => setSaved(!next),
              });
            }}
            className={cn(
              "p-1.5 rounded-lg transition-all duration-200",
              saved
                ? "text-primary bg-primary/10"
                : "text-muted-foreground hover:text-primary hover:bg-primary/5",
            )}
          >
            <Bookmark
              className={cn("h-3.5 w-3.5", saved && "fill-current")}
            />
          </button>
          {safeHref(listing.page_link) && (
            <a
              href={safeHref(listing.page_link)!}
              target="_blank"
              rel="noopener noreferrer"
              aria-label="פתח מודעה באתר חיצוני"
              onClick={(e) => e.stopPropagation()}
              className="p-1.5 rounded-lg text-muted-foreground hover:text-primary hover:bg-primary/5 transition-all duration-200"
            >
              <ExternalLink className="h-3.5 w-3.5" />
            </a>
          )}
        </div>
      </div>
    </div>
  );
}

function ScoreBadge({ score }: { score: number }) {
  const tier = score >= 7 ? "great" : score >= 5 ? "good" : "low";

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
