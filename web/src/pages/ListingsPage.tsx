import { useState, useEffect } from "react";
import { useParams, useNavigate } from "react-router";
import {
  ExternalLink,
  ChevronDown,
  Bookmark,
  Car,
} from "lucide-react";
import { useListings } from "@/hooks/useListings";
import { useSaveBookmark, useRemoveBookmark } from "@/hooks/useBookmarks";
import { formatPrice, formatKm, relativeTime, safeHref, cn } from "@/lib/utils";
import type { Listing } from "@/lib/api";
import { PageHeader } from "@/components/ui/PageHeader";
import { Button } from "@/components/ui/Button";
import { EmptyState } from "@/components/ui/EmptyState";
import { Skeleton } from "@/components/ui/Skeleton";
import { useToast } from "@/components/ui/Toast";

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
        <PageHeader backTo="/" title="חיפוש לא נמצא" />
      </div>
    );
  }

  if (isError) {
    return (
      <div className="space-y-4">
        <PageHeader backTo="/" title="תוצאות" />
        <div className="rounded-2xl border border-destructive/20 bg-destructive/5 p-8 text-center dir-rtl">
          <p className="text-destructive font-medium">שגיאה בטעינת התוצאות</p>
          <p className="text-sm text-muted-foreground mt-1">נסה לרענן את הדף</p>
        </div>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="space-y-5 pb-20 md:pb-4">
        <PageHeader backTo="/" title="תוצאות" />
        <div className="flex flex-wrap gap-2">
          {[1, 2, 3, 4, 5, 6].map((i) => (
            <Skeleton key={i} className="h-8 w-16 rounded-md" />
          ))}
        </div>
        <div className="grid gap-4 sm:grid-cols-2">
          {[1, 2, 3, 4].map((i) => (
            <Skeleton key={i} className="h-72 rounded-2xl" />
          ))}
        </div>
      </div>
    );
  }

  const countSubtitle =
    data != null ? `${data.total.toLocaleString("he-IL")} מודעות` : undefined;

  return (
    <div className="space-y-5 pb-20 md:pb-4">
      <PageHeader
        backTo="/"
        title="תוצאות"
        subtitle={countSubtitle}
      />

      {/* Sort pills */}
      <div className="flex flex-wrap gap-2 dir-rtl">
        {SORT_OPTIONS.map((opt) => (
          <Button
            key={opt.value}
            type="button"
            size="sm"
            variant={sort === opt.value ? "primary" : "secondary"}
            onClick={() => {
              setSort(opt.value);
              setOffset(0);
            }}
          >
            {opt.label}
          </Button>
        ))}
      </div>

      {/* Grid */}
      {!data || data.items.length === 0 ? (
        <EmptyState
          icon={Car}
          title="אין תוצאות עדיין"
          description="רכבים חדשים יופיעו כאן כשהם ימצאו"
        />
      ) : (
        <>
          <div className="grid gap-4 sm:grid-cols-2">
            {data.items.map((listing) => (
              <ListingCard key={listing.token} listing={listing} />
            ))}
          </div>

          {data.total > PAGE_SIZE && (
            <div className="flex items-center justify-center gap-3 pt-4 dir-rtl">
              {offset > 0 && (
                <Button
                  type="button"
                  variant="secondary"
                  size="md"
                  onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
                >
                  הקודם
                </Button>
              )}
              <span className="text-sm text-muted-foreground tabular-nums">
                {offset + 1}–{Math.min(offset + PAGE_SIZE, data.total)} מתוך{" "}
                {data.total}
              </span>
              {offset + PAGE_SIZE < data.total && (
                <Button
                  type="button"
                  variant="primary"
                  size="md"
                  onClick={() => setOffset(offset + PAGE_SIZE)}
                >
                  <ChevronDown className="h-4 w-4" />
                  הבא
                </Button>
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
  const { toast } = useToast();
  const [saved, setSaved] = useState(() => listing.saved ?? false);

  useEffect(() => {
    setSaved(listing.saved ?? false);
  }, [listing.token, listing.saved]);

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
      className="group block cursor-pointer rounded-2xl border border-border/50 bg-card overflow-hidden transition-all duration-300 hover:border-border hover:shadow-[0_8px_32px_rgba(0,0,0,0.4)] hover:-translate-y-0.5 dir-rtl"
    >
      {/* Image */}
      {listing.image_url ? (
        <div className="relative aspect-video w-full overflow-hidden bg-secondary">
          {saved ? (
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
          <span className="text-lg font-bold text-amber-500 dark:text-amber-400 tabular-nums">
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
                onSuccess: () => {
                  if (next) {
                    toast("נשמר בהצלחה", "success");
                  } else {
                    toast("הוסר מהשמורים", "info");
                  }
                },
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
