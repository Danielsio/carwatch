import { useState } from "react";
import { useParams, Link, useNavigate } from "react-router";
import { ArrowRight, ExternalLink, ChevronLeft, ChevronRight, Bookmark, Inbox } from "lucide-react";
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

  const { data, isLoading, isError } = useListings(
    searchId,
    sort,
    PAGE_SIZE,
    offset,
  );

  if (!searchId || Number.isNaN(searchId)) {
    return (
      <div className="space-y-4 animate-fade-in">
        <BackLink />
        <h1 className="text-2xl font-bold">חיפוש לא נמצא</h1>
      </div>
    );
  }

  if (isError) {
    return (
      <div className="space-y-4 animate-fade-in">
        <BackLink />
        <div className="rounded-2xl border border-destructive/30 bg-destructive/5 p-8 text-center">
          <p className="text-destructive font-semibold text-lg">
            שגיאה בטעינת התוצאות
          </p>
          <p className="text-sm text-muted-foreground mt-2">נסה לרענן את הדף</p>
        </div>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="space-y-4 animate-fade-in">
        <div className="h-6 w-32 rounded-lg skeleton" />
        <div className="flex gap-2">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="h-9 w-16 rounded-xl skeleton" />
          ))}
        </div>
        <div className="grid gap-4 sm:grid-cols-2">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="h-72 rounded-2xl skeleton" />
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-5 pb-20 md:pb-4 animate-fade-in">
      <div className="flex items-center gap-3">
        <BackLink />
        <h1 className="text-2xl font-bold tracking-tight">תוצאות</h1>
        {data && (
          <span className="rounded-full bg-primary/10 px-3 py-0.5 text-sm font-semibold text-primary">
            {data.total}
          </span>
        )}
      </div>

      {/* Sort pills */}
      <div className="flex flex-wrap gap-1.5">
        {SORT_OPTIONS.map((opt) => (
          <button
            key={opt.value}
            onClick={() => {
              setSort(opt.value);
              setOffset(0);
            }}
            className={cn(
              "rounded-xl px-3.5 py-1.5 text-sm font-medium transition-all duration-200",
              sort === opt.value
                ? "gradient-primary text-white shadow-sm"
                : "bg-secondary text-secondary-foreground hover:bg-secondary/80",
            )}
          >
            {opt.label}
          </button>
        ))}
      </div>

      {/* Grid */}
      {!data || data.items.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-2xl border-2 border-dashed border-border py-20">
          <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-muted mb-4">
            <Inbox className="h-7 w-7 text-muted-foreground/50" />
          </div>
          <p className="text-muted-foreground font-medium">אין תוצאות עדיין</p>
          <p className="text-sm text-muted-foreground mt-1">
            רכבים חדשים יופיעו כאן כשהם ימצאו
          </p>
        </div>
      ) : (
        <>
          <div className="grid gap-4 sm:grid-cols-2">
            {data.items.map((listing, i) => (
              <ListingCard
                key={listing.token}
                listing={listing}
                className={`stagger-${Math.min(i + 1, 6)} animate-slide-up`}
              />
            ))}
          </div>

          {/* Pagination */}
          {data.total > PAGE_SIZE && (
            <Pagination
              offset={offset}
              total={data.total}
              pageSize={PAGE_SIZE}
              onPrev={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
              onNext={() => setOffset(offset + PAGE_SIZE)}
            />
          )}
        </>
      )}
    </div>
  );
}

function BackLink() {
  return (
    <Link
      to="/"
      className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors font-medium"
    >
      <ArrowRight className="h-4 w-4" />
      חזרה
    </Link>
  );
}

function ListingCard({
  listing,
  className,
}: {
  listing: Listing;
  className?: string;
}) {
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
      className={cn(
        "group block cursor-pointer rounded-2xl border border-border bg-card overflow-hidden shadow-sm hover-lift gradient-card",
        className,
      )}
    >
      {/* Image */}
      {listing.image_url ? (
        <div className="aspect-video w-full overflow-hidden bg-muted relative">
          <img
            src={listing.image_url}
            alt={`${listing.manufacturer} ${listing.model}`}
            className="h-full w-full object-cover group-hover:scale-105 transition-transform duration-500 ease-out"
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

        <div className="flex flex-wrap gap-1.5 text-xs text-muted-foreground mb-3">
          <span className="inline-flex items-center rounded-md bg-secondary px-1.5 py-0.5">
            {formatKm(listing.km)}
          </span>
          <span className="inline-flex items-center rounded-md bg-secondary px-1.5 py-0.5">
            יד {listing.hand}
          </span>
          {listing.city && (
            <span className="inline-flex items-center rounded-md bg-secondary px-1.5 py-0.5">
              {listing.city}
            </span>
          )}
        </div>

        <div className="flex items-center gap-2">
          {listing.fitness_score != null && (
            <span
              className={cn(
                "inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-bold tabular-nums",
                listing.fitness_score >= 7
                  ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-400"
                  : listing.fitness_score >= 5
                    ? "bg-amber-100 text-amber-700 dark:bg-amber-500/20 dark:text-amber-400"
                    : "bg-secondary text-muted-foreground",
              )}
            >
              {listing.fitness_score.toFixed(1)}
              {listing.fitness_score >= 7 && " ★"}
            </span>
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
              className={cn("h-4 w-4", saved && "fill-current")}
            />
          </button>
          <a
            href={listing.page_link}
            target="_blank"
            rel="noopener noreferrer"
            aria-label="פתח מודעה באתר חיצוני"
            onClick={(e) => e.stopPropagation()}
            className="p-1.5 rounded-lg text-muted-foreground hover:text-primary hover:bg-primary/5 transition-all duration-200"
          >
            <ExternalLink className="h-4 w-4" />
          </a>
        </div>
      </div>
    </div>
  );
}

export function Pagination({
  offset,
  total,
  pageSize,
  onPrev,
  onNext,
}: {
  offset: number;
  total: number;
  pageSize: number;
  onPrev: () => void;
  onNext: () => void;
}) {
  return (
    <div className="flex items-center justify-center gap-3 pt-4">
      {offset > 0 && (
        <button
          onClick={onPrev}
          className="inline-flex items-center gap-1.5 rounded-xl bg-secondary px-4 py-2 text-sm font-medium text-secondary-foreground hover:bg-secondary/80 transition-colors"
        >
          <ChevronRight className="h-4 w-4" />
          הקודם
        </button>
      )}
      <span className="text-sm text-muted-foreground font-medium tabular-nums">
        {offset + 1}–{Math.min(offset + pageSize, total)} מתוך {total}
      </span>
      {offset + pageSize < total && (
        <button
          onClick={onNext}
          className="inline-flex items-center gap-1.5 rounded-xl gradient-primary px-4 py-2 text-sm font-semibold text-white shadow-sm hover:shadow-md hover:brightness-110 transition-all"
        >
          הבא
          <ChevronLeft className="h-4 w-4" />
        </button>
      )}
    </div>
  );
}
