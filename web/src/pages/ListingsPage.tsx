import { useState, useEffect, useCallback, useRef, memo, useMemo } from "react";
import { useParams, Link, useNavigate } from "react-router";
import {
  ExternalLink,
  Bookmark,
  Car,
  Eye,
  EyeOff,
  RefreshCw,
  ArrowUp,
  Loader2,
  FilterX,
  LayoutGrid,
  Rows3,
} from "lucide-react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useInfiniteListings } from "@/hooks/useInfiniteListings";
import { useListingActions } from "@/hooks/useListingActions";
import {
  useListingFilters,
  deriveFacets,
  activeFilterCount,
  EMPTY_FILTERS,
  type ListingFilters,
} from "@/hooks/useListingFilters";
import { ListingsFilterBar } from "@/components/ListingsFilterBar";
import { useListingDensity } from "@/hooks/useListingDensity";
import { useGridKeyboardNav } from "@/hooks/useGridKeyboardNav";
import { useSaveBookmark, useRemoveBookmark } from "@/hooks/useBookmarks";
import { useMarkListingSeen, useUnmarkListingSeen } from "@/hooks/useListingSeen";
import { CompactListingCard } from "@/components/CompactListingCard";
import { safeHref, cn, formatPrice } from "@/lib/utils";
import { api } from "@/lib/api";
import type { Listing, RefreshResponse } from "@/lib/api";
import { ListingCardBody } from "@/components/ListingCardBody";
import { ListingCardSkeleton } from "@/components/ListingCardSkeleton";
import { PageHeader } from "@/components/ui/PageHeader";
import { PageShell } from "@/components/ui/PageShell";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/EmptyState";
import { ErrorState } from "@/components/ui/ErrorState";
import { useToast } from "@/components/ui/Toast";

const SORT_OPTIONS = [
  { value: "newest", label: "חדשים" },
  { value: "price_asc", label: "מחיר ↑" },
  { value: "price_desc", label: "מחיר ↓" },
  { value: "score", label: "ציון" },
  { value: "km", label: 'ק"מ' },
  { value: "year", label: "שנה" },
];

const REFRESH_COOLDOWN_S = 60;

function RefreshButton({ searchId }: { searchId: number }) {
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const [cooldown, setCooldown] = useState(0);
  const cooldownRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const startCooldown = useCallback(() => {
    setCooldown(REFRESH_COOLDOWN_S);
    if (cooldownRef.current) clearInterval(cooldownRef.current);
    cooldownRef.current = setInterval(() => {
      setCooldown((prev) => {
        if (prev <= 1) {
          clearInterval(cooldownRef.current!);
          cooldownRef.current = null;
          return 0;
        }
        return prev - 1;
      });
    }, 1000);
  }, []);

  useEffect(() => {
    return () => {
      if (cooldownRef.current) clearInterval(cooldownRef.current);
    };
  }, []);

  const refreshMutation = useMutation<RefreshResponse>({
    mutationFn: () => api.refreshListings(searchId),
    onSuccess: (result) => {
      void queryClient.invalidateQueries({ queryKey: ["listings-infinite", searchId] });
      void queryClient.invalidateQueries({ queryKey: ["listings", searchId] });
      startCooldown();
      if (result.items.length > 0) {
        toast(`${result.items.length} מודעות חדשות נמצאו!`, "success");
      } else {
        toast("אין מודעות חדשות כרגע", "info");
      }
    },
    onError: () => {
      toast("שגיאה ברענון, נסה שוב", "error");
    },
  });

  const isRefreshing = refreshMutation.isPending;

  return (
    <button
      type="button"
      onClick={() => refreshMutation.mutate()}
      disabled={isRefreshing || cooldown > 0}
      aria-label={cooldown > 0 ? `רענון זמין בעוד ${cooldown} שניות` : "רענן מודעות"}
      className={cn(
        "relative flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-sm font-medium transition-all duration-150",
        "bg-primary/10 text-primary hover:bg-primary/20",
        "disabled:opacity-50 disabled:cursor-not-allowed",
        "focus-visible:outline-2 focus-visible:outline-primary focus-visible:outline-offset-2",
      )}
    >
      <RefreshCw
        className={cn(
          "h-4 w-4 transition-transform duration-500",
          isRefreshing && "animate-spin",
        )}
      />
      {cooldown > 0 ? (
        <span className="tabular-nums text-xs">{cooldown}s</span>
      ) : (
        <span className="hidden sm:inline">רענן</span>
      )}
    </button>
  );
}

function ScrollToTopButton() {
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    const main = document.querySelector("main");
    if (!main) return;
    const onScroll = () => setVisible(main.scrollTop > 600);
    main.addEventListener("scroll", onScroll, { passive: true });
    return () => main.removeEventListener("scroll", onScroll);
  }, []);

  if (!visible) return null;

  return (
    <button
      type="button"
      onClick={() => document.querySelector("main")?.scrollTo({ top: 0, behavior: "smooth" })}
      className="fixed bottom-[calc(var(--bottom-nav-h)+1rem)] start-4 z-40 flex h-10 w-10 items-center justify-center rounded-full bg-primary text-white shadow-lg transition-all duration-150 hover:opacity-90 active:scale-95 md:bottom-6"
      aria-label="חזרה למעלה"
    >
      <ArrowUp className="h-4 w-4" />
    </button>
  );
}

export function ListingsPage() {
  const { id } = useParams();
  const searchId = Number(id);
  const [sort, setSort] = useState("newest");

  const {
    data,
    isLoading,
    isError,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useInfiniteListings(searchId, sort);

  const loadMoreRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = loadMoreRef.current;
    if (!el) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting && hasNextPage && !isFetchingNextPage) {
          void fetchNextPage();
        }
      },
      { rootMargin: "200px" },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [fetchNextPage, hasNextPage, isFetchingNextPage]);

  const allListings = useMemo(
    () => data?.pages.flatMap((p) => p.items) ?? [],
    [data],
  );
  const total = data?.pages[0]?.total ?? 0;

  const [filters, setFilters] = useState<ListingFilters>(EMPTY_FILTERS);
  const facets = useMemo(() => deriveFacets(allListings), [allListings]);
  const visible = useListingFilters(allListings, filters);
  const hasFilters = activeFilterCount(filters) > 0;
  const [density, setDensity] = useListingDensity();

  // Keyboard navigation (j/k move, o open, s save, e toggle seen)
  const navigate = useNavigate();
  const { toast } = useToast();
  const saveBookmark = useSaveBookmark();
  const removeBookmark = useRemoveBookmark();
  const markSeen = useMarkListingSeen();
  const unmarkSeen = useUnmarkListingSeen();

  const openListing = useCallback(
    (i: number) => {
      const l = visible[i];
      if (l) navigate(`/listings/${l.token}`, { state: { listing: l } });
    },
    [visible, navigate],
  );
  const saveListing = useCallback(
    (i: number) => {
      const l = visible[i];
      if (!l) return;
      const wasSaved = l.saved ?? false;
      (wasSaved ? removeBookmark : saveBookmark).mutate(l.token, {
        onSuccess: () =>
          toast(wasSaved ? "הוסר מהשמורים" : "נשמר בהצלחה", wasSaved ? "info" : "success"),
      });
    },
    [visible, saveBookmark, removeBookmark, toast],
  );
  const seenListing = useCallback(
    (i: number) => {
      const l = visible[i];
      if (!l) return;
      ((l.seen ?? false) ? unmarkSeen : markSeen).mutate(l.token);
    },
    [visible, markSeen, unmarkSeen],
  );

  const { activeIndex } = useGridKeyboardNav({
    count: visible.length,
    onOpen: openListing,
    onSave: saveListing,
    onSeen: seenListing,
  });

  useEffect(() => {
    if (activeIndex < 0) return;
    document
      .querySelector(`[data-kbd="${activeIndex}"]`)
      ?.scrollIntoView({ block: "nearest" });
  }, [activeIndex]);

  const unenrichedCount = useMemo(
    () =>
      allListings.filter(
        (l) => l.km <= 0 || !l.horse_power || !l.gear_box || !l.engine_type,
      ).length,
    [allListings],
  );

  if (!searchId || Number.isNaN(searchId)) {
    return (
      <PageShell gap="sm">
        <PageHeader backTo="/dashboard" title="חיפוש לא נמצא" />
      </PageShell>
    );
  }

  if (isError) {
    return (
      <PageShell gap="sm">
        <PageHeader backTo="/dashboard" title="תוצאות" />
        <ErrorState
          title="שגיאה בטעינת התוצאות"
          description="נסה לרענן את הדף"
          onRetry={() => window.location.reload()}
        />
      </PageShell>
    );
  }

  const showSkeletons = isLoading && !data;

  return (
    <PageShell gap="sm">
      <PageHeader
        backTo="/dashboard"
        title="תוצאות"
        subtitle={
          showSkeletons ? (
            <span className="inline-block h-4 w-24 rounded skeleton align-middle" />
          ) : total > 0 ? (
            `${total.toLocaleString("he-IL")} מודעות`
          ) : undefined
        }
      />

      {/* Sort pills + refresh */}
      <div className="sticky top-0 z-30 -mx-4 bg-background/90 px-4 py-2 border-b border-border/30 will-change-transform md:static md:mx-0 md:px-0 md:bg-transparent md:border-0 md:will-change-auto">
        <div className="flex items-center gap-2 dir-rtl">
          <div className="flex min-w-0 flex-1 flex-nowrap gap-1 overflow-x-auto scrollbar-hide" role="radiogroup" aria-label="מיון תוצאות">
            {SORT_OPTIONS.map((opt) => (
              <Button
                key={opt.value}
                type="button"
                role="radio"
                aria-checked={sort === opt.value}
                size="sm"
                variant={sort === opt.value ? "default" : "ghost"}
                onClick={() => setSort(opt.value)}
              >
                {opt.label}
              </Button>
            ))}
          </div>
          <div className="flex shrink-0 items-center gap-0.5 rounded-lg border border-border/50 bg-secondary/40 p-0.5">
            <Button
              variant={density === "comfortable" ? "secondary" : "ghost"}
              size="icon-sm"
              onClick={() => setDensity("comfortable")}
              aria-label="תצוגה מרווחת"
            >
              <LayoutGrid className="h-4 w-4" />
            </Button>
            <Button
              variant={density === "compact" ? "secondary" : "ghost"}
              size="icon-sm"
              onClick={() => setDensity("compact")}
              aria-label="תצוגה צפופה"
            >
              <Rows3 className="h-4 w-4" />
            </Button>
          </div>
          <RefreshButton searchId={searchId} />
        </div>
      </div>

      {/* Filter bar */}
      {!showSkeletons && allListings.length > 0 ? (
        <ListingsFilterBar
          filters={filters}
          onChange={setFilters}
          onClear={() => setFilters(EMPTY_FILTERS)}
          facets={facets}
          resultCount={visible.length}
          totalCount={allListings.length}
        />
      ) : null}

      {/* Enrichment progress banner */}
      {!showSkeletons && allListings.length > 0 && (() => {
        const unenriched = unenrichedCount;
        if (unenriched === 0) return null;
        const pct = Math.round(((allListings.length - unenriched) / allListings.length) * 100);
        return (
          <div className="rounded-xl border border-amber-500/20 bg-amber-500/5 px-4 py-3 flex items-center gap-3">
            <div className="flex gap-0.5 shrink-0">
              <span className="h-1.5 w-1.5 rounded-full bg-amber-400 animate-pulse" />
              <span className="h-1.5 w-1.5 rounded-full bg-amber-400 animate-pulse [animation-delay:150ms]" />
              <span className="h-1.5 w-1.5 rounded-full bg-amber-400 animate-pulse [animation-delay:300ms]" />
            </div>
            <div className="flex-1 min-w-0">
              <p className="text-xs font-medium text-amber-700 dark:text-amber-400">
                מעשיר נתונים עבור {unenriched} מודעות ({pct}% הושלם)
              </p>
              <p className="text-[10px] text-amber-600/70 dark:text-amber-500/60">
                קילומטראז׳, כ״ס, הילוכים ודלק יתעדכנו תוך דקות
              </p>
            </div>
            <div className="h-1.5 w-20 rounded-full bg-amber-500/20 overflow-hidden shrink-0">
              <div className="h-full rounded-full bg-amber-400 transition-all duration-500" style={{ width: `${pct}%` }} />
            </div>
          </div>
        );
      })()}

      {/* Grid */}
      {showSkeletons ? (
        <div className="grid grid-cols-1 gap-5 sm:gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <div
              key={`skel-${i}`}
              className="animate-slide-up motion-reduce:animate-none"
              style={{ animationDelay: `${i * 60}ms`, animationFillMode: "backwards" }}
            >
              <ListingCardSkeleton />
            </div>
          ))}
        </div>
      ) : allListings.length === 0 ? (
        <EmptyState
          icon={Car}
          title="אין תוצאות עדיין"
          description="רכבים חדשים יופיעו כאן כשהם ימצאו. נסה להרחיב את טווח המחיר או השנים, או להסיר מסננים כמו ק״מ מקסימלי."
          action={
            <div className="flex flex-wrap gap-2 justify-center">
              <Button asChild variant="secondary" size="sm">
                <Link to={`/searches/${searchId}/edit`}>ערוך מסננים</Link>
              </Button>
              <Button asChild variant="ghost" size="sm">
                <Link to="/dashboard">חזרה ללוח הבקרה</Link>
              </Button>
            </div>
          }
        />
      ) : visible.length === 0 ? (
        <EmptyState
          icon={FilterX}
          title="אין תוצאות לסינון"
          description="אף מודעה לא תואמת את הסינון הנוכחי. נסה להרחיב או לאפס את המסננים."
          action={
            <Button variant="secondary" size="sm" onClick={() => setFilters(EMPTY_FILTERS)}>
              נקה מסננים
            </Button>
          }
        />
      ) : (
        <>
          <div
            className={cn(
              density === "compact"
                ? "space-y-2"
                : // grid-cols-1 is required: a bare `grid` falls back to an implicit
                  // `auto` track on mobile, which WebKit/iOS Safari sizes to the card
                  // image's intrinsic width — overflowing the viewport and clipping
                  // the card. minmax(0,1fr) from grid-cols-1 keeps it clamped.
                  "grid grid-cols-1 gap-5 sm:gap-4 sm:grid-cols-2 xl:grid-cols-3",
            )}
          >
            {visible.map((listing, i) => (
              <div
                key={listing.token}
                data-kbd={i}
                className={cn(
                  "cv-auto animate-slide-up motion-reduce:animate-none rounded-2xl",
                  activeIndex === i &&
                    "ring-2 ring-primary ring-offset-2 ring-offset-background",
                )}
                style={{
                  animationDelay: `${Math.min(i, 8) * 40}ms`,
                  animationFillMode: "backwards",
                }}
              >
                {density === "compact" ? (
                  <CompactListingCard listing={listing} />
                ) : (
                  <ListingCard listing={listing} />
                )}
              </div>
            ))}
          </div>

          {/* Infinite scroll trigger — keeps loading more raw results to filter */}
          <div ref={loadMoreRef} className="flex justify-center py-6">
            {isFetchingNextPage ? (
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            ) : hasNextPage ? (
              <button
                type="button"
                onClick={() => void fetchNextPage()}
                className="text-sm font-medium text-primary hover:underline"
              >
                טען עוד
              </button>
            ) : (
              <p className="text-xs text-muted-foreground">
                {hasFilters
                  ? `${visible.length.toLocaleString("he-IL")} תוצאות מסוננות`
                  : `הצגת כל ${total.toLocaleString("he-IL")} המודעות`}
              </p>
            )}
          </div>
        </>
      )}

      <ScrollToTopButton />
    </PageShell>
  );
}

const ListingCard = memo(function ListingCard({ listing }: { listing: Listing }) {
  const { saved, seen, toggleSaved, toggleSeen } = useListingActions(listing);
  const { toast } = useToast();

  return (
    <Card className="group hover-tint relative overflow-hidden dir-rtl">
      {/* Stretched navigation link — covers entire card */}
      <Link
        to={`/listings/${listing.token}`}
        state={{ listing }}
        aria-label={`${listing.manufacturer} ${listing.model} ${listing.year} - ${formatPrice(listing.price)}`}
        className="absolute inset-0 z-0"
        tabIndex={-1}
      />

      <ListingCardBody
        listing={listing}
        hoverScale
        showBookmarkOverlay={saved}
        actions={
          <div className="relative z-10 flex items-center">
            <button
              type="button"
              aria-label={seen ? "החזר לרשימת החדשות" : "סמן כנצפה"}
              aria-pressed={seen}
              onClick={() => toggleSeen()}
              className={cn(
                "rounded-lg p-2.5 min-h-[44px] min-w-[44px] flex items-center justify-center transition-all duration-150",
                seen
                  ? "bg-primary/10 text-primary"
                  : "text-muted-foreground hover:bg-primary/5 hover:text-primary",
              )}
            >
              {seen ? (
                <EyeOff className="h-4 w-4" />
              ) : (
                <Eye className="h-4 w-4" />
              )}
            </button>
            <button
              type="button"
              aria-label={saved ? "הסר משמורים" : "שמור מודעה"}
              aria-pressed={saved}
              onClick={() =>
                toggleSaved({
                  onSuccess: (next) => toast(next ? "נשמר בהצלחה" : "הוסר מהשמורים", next ? "success" : "info"),
                })
              }
              className={cn(
                "rounded-lg p-2.5 min-h-[44px] min-w-[44px] flex items-center justify-center transition-all duration-150",
                saved
                  ? "bg-primary/10 text-primary"
                  : "text-muted-foreground hover:bg-primary/5 hover:text-primary",
              )}
            >
              <Bookmark
                className={cn("h-4 w-4", saved && "fill-current")}
              />
            </button>
            {safeHref(listing.page_link) ? (
              <a
                href={safeHref(listing.page_link)!}
                target="_blank"
                rel="noopener noreferrer"
                aria-label="פתח מודעה באתר חיצוני"
                className="rounded-lg p-2.5 min-h-[44px] min-w-[44px] flex items-center justify-center text-muted-foreground transition-all duration-150 hover:bg-primary/5 hover:text-primary"
              >
                <ExternalLink className="h-4 w-4" />
              </a>
            ) : null}
          </div>
        }
      />
    </Card>
  );
});
