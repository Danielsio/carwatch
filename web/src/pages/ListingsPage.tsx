import { useState } from "react";
import { useParams, Link, useNavigate } from "react-router";
import { ArrowRight, ExternalLink, ChevronDown, Bookmark } from "lucide-react";
import { useListings } from "@/hooks/useListings";
import { useSaveBookmark, useRemoveBookmark } from "@/hooks/useBookmarks";
import { formatPrice, formatKm, relativeTime, cn } from "@/lib/utils";
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

  const { data, isLoading, isError } = useListings(searchId, sort, PAGE_SIZE, offset);

  if (!searchId || Number.isNaN(searchId)) {
    return (
      <div className="space-y-4">
        <div className="flex items-center gap-3">
          <Link
            to="/"
            className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            <ArrowRight className="h-4 w-4" />
            חזרה
          </Link>
          <h1 className="text-2xl font-bold">חיפוש לא נמצא</h1>
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
            className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            <ArrowRight className="h-4 w-4" />
            חזרה
          </Link>
          <h1 className="text-2xl font-bold">תוצאות</h1>
        </div>
        <div className="rounded-xl border border-destructive/50 bg-destructive/10 p-6 text-center">
          <p className="text-destructive font-medium">שגיאה בטעינת התוצאות</p>
          <p className="text-sm text-muted-foreground mt-1">נסה לרענן את הדף</p>
        </div>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="space-y-4">
        <div className="h-8 w-40 animate-pulse rounded bg-muted" />
        <div className="grid gap-4 sm:grid-cols-2">
          {[1, 2, 3, 4].map((i) => (
            <div
              key={i}
              className="h-64 animate-pulse rounded-xl bg-muted"
            />
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-4 pb-20 md:pb-4">
      <div className="flex items-center gap-3">
        <Link
          to="/"
          className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground transition-colors"
        >
          <ArrowRight className="h-4 w-4" />
          חזרה
        </Link>
        <h1 className="text-2xl font-bold">תוצאות</h1>
        {data && (
          <span className="text-sm text-muted-foreground">
            ({data.total} מודעות)
          </span>
        )}
      </div>

      {/* Sort controls */}
      <div className="flex flex-wrap gap-2">
        {SORT_OPTIONS.map((opt) => (
          <button
            key={opt.value}
            onClick={() => {
              setSort(opt.value);
              setOffset(0);
            }}
            className={cn(
              "rounded-lg px-3 py-1.5 text-sm font-medium transition-colors",
              sort === opt.value
                ? "bg-primary text-primary-foreground"
                : "bg-secondary text-secondary-foreground hover:bg-secondary/80",
            )}
          >
            {opt.label}
          </button>
        ))}
      </div>

      {/* Listings grid */}
      {!data || data.items.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border py-16">
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

          {/* Pagination */}
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
                  className="inline-flex items-center gap-1 rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 transition-colors"
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
      className="group block cursor-pointer rounded-xl border border-border bg-card overflow-hidden shadow-sm hover:shadow-md transition-shadow"
    >
      {/* Image */}
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
        {/* Title + Price */}
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

        {/* Specs */}
        <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground mb-3">
          <span>{formatKm(listing.km)}</span>
          <span>יד {listing.hand}</span>
          {listing.city && <span>{listing.city}</span>}
        </div>

        {/* Badges + link */}
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
              "p-1 rounded transition-colors",
              saved
                ? "text-primary"
                : "text-muted-foreground hover:text-primary",
            )}
          >
            <Bookmark className={cn("h-3.5 w-3.5", saved && "fill-current")} />
          </button>
          <a
            href={listing.page_link}
            target="_blank"
            rel="noopener noreferrer"
            aria-label="פתח מודעה באתר חיצוני"
            onClick={(e) => e.stopPropagation()}
            className="p-1 rounded text-muted-foreground hover:text-primary transition-colors"
          >
            <ExternalLink className="h-3.5 w-3.5" />
          </a>
        </div>
      </div>
    </div>
  );
}

function ScoreBadge({ score }: { score: number }) {
  let color: string;
  let label: string;

  if (score >= 7) {
    color = "bg-green-100 text-green-800";
    label = `${score.toFixed(1)} ⭐`;
  } else if (score >= 5) {
    color = "bg-yellow-100 text-yellow-800";
    label = `${score.toFixed(1)}`;
  } else {
    color = "bg-gray-100 text-gray-600";
    label = `${score.toFixed(1)}`;
  }

  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium",
        color,
      )}
    >
      {label}
    </span>
  );
}
