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
import { safeHref, cn } from "@/lib/utils";
import type { Listing } from "@/lib/api";
import { ListingCardBody } from "@/components/ListingCardBody";
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

function isInteractiveTarget(target: EventTarget | null): boolean {
  if (!(target instanceof Element)) return false;
  return (
    target.closest(
      "button,a,input,select,textarea,[role='button']",
    ) != null
  );
}

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
        <PageHeader backTo="/dashboard" title="חיפוש לא נמצא" />
      </div>
    );
  }

  if (isError) {
    return (
      <div className="space-y-4">
        <PageHeader backTo="/dashboard" title="תוצאות" />
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
        <PageHeader backTo="/dashboard" title="תוצאות" />
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
        backTo="/dashboard"
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
      onClick={(e) => {
        if (isInteractiveTarget(e.target)) return;
        navigate(`/listings/${listing.token}`, { state: { listing } });
      }}
      onKeyDown={(e) => {
        if (isInteractiveTarget(e.target)) return;
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          navigate(`/listings/${listing.token}`, { state: { listing } });
        }
      }}
      className="group block cursor-pointer rounded-2xl border border-border/50 bg-card overflow-hidden transition-all duration-300 hover:border-border hover:shadow-[0_8px_32px_rgba(0,0,0,0.4)] hover:-translate-y-0.5 dir-rtl"
    >
      <ListingCardBody
        listing={listing}
        hoverScale
        showBookmarkOverlay={saved}
        actions={
          <>
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
                "rounded-lg p-1.5 transition-all duration-200",
                saved
                  ? "bg-primary/10 text-primary"
                  : "text-muted-foreground hover:bg-primary/5 hover:text-primary",
              )}
            >
              <Bookmark
                className={cn("h-3.5 w-3.5", saved && "fill-current")}
              />
            </button>
            {safeHref(listing.page_link) ? (
              <a
                href={safeHref(listing.page_link)!}
                target="_blank"
                rel="noopener noreferrer"
                aria-label="פתח מודעה באתר חיצוני"
                onClick={(e) => e.stopPropagation()}
                className="rounded-lg p-1.5 text-muted-foreground transition-all duration-200 hover:bg-primary/5 hover:text-primary"
              >
                <ExternalLink className="h-3.5 w-3.5" />
              </a>
            ) : null}
          </>
        }
      />
    </div>
  );
}
