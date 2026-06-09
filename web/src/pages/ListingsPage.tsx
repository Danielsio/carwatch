import { useState, useEffect, useCallback, useRef, memo, useMemo } from "react";
import { useParams, Link } from "react-router";
import {
  ExternalLink,
  Bookmark,
  Car,
  Eye,
  EyeOff,
  RefreshCw,
  ArrowUp,
  Loader2,
} from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useInfiniteListings } from "@/hooks/useInfiniteListings";
import { useListingActions } from "@/hooks/useListingActions";
import { safeHref, cn, formatPrice } from "@/lib/utils";
import { api } from "@/lib/api";
import type { Listing, RefreshResponse } from "@/lib/api";
import { ListingCardBody } from "@/components/ListingCardBody";
import { ListingCardSkeleton } from "@/components/ListingCardSkeleton";
import { PageHeader } from "@/components/ui/PageHeader";
import { PageShell } from "@/components/ui/PageShell";
import { Button } from "@/components/ui/Button";
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
            <span className="inline-block h-4 w-24 rounded shimmer-skeleton align-middle" />
          ) : total > 0 ? (
            `${total.toLocaleString("he-IL")} מודעות`
          ) : undefined
        }
      />

      {/* Sort pills + refresh */}
      <div className="sticky top-0 z-30 -mx-4 bg-background/90 px-4 py-2 backdrop-blur-lg border-b border-border/30 will-change-transform md:static md:mx-0 md:px-0 md:bg-transparent md:backdrop-blur-none md:border-0 md:will-change-auto">
        <div className="flex items-center gap-2 dir-rtl">
          <div className="flex flex-wrap gap-1.5 flex-1">
            {SORT_OPTIONS.map((opt) => (
              <Button
                key={opt.value}
                type="button"
                size="sm"
                variant={sort === opt.value ? "primary" : "secondary"}
                onClick={() => setSort(opt.value)}
              >
                {opt.label}
              </Button>
            ))}
          </div>
          <RefreshButton searchId={searchId} />
        </div>
      </div>

      {/* Enrichment progress banner */}
      {!showSkeletons && allListings.length > 0 && (() => {
        const unenriched = allListings.filter(l => l.km <= 0 || !l.horse_power || !l.gear_box || !l.engine_type).length;
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
        <div className="grid gap-5 sm:gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <motion.div
              key={`skel-${i}`}
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: i * 0.06, duration: 0.4, ease: "easeOut" }}
            >
              <ListingCardSkeleton />
            </motion.div>
          ))}
        </div>
      ) : allListings.length === 0 ? (
        <EmptyState
          icon={Car}
          title="אין תוצאות עדיין"
          description="רכבים חדשים יופיעו כאן כשהם ימצאו"
          action={
            <Button asChild variant="secondary" size="sm">
              <Link to="/dashboard">חזרה ללוח הבקרה</Link>
            </Button>
          }
        />
      ) : (
        <>
          <div className="grid gap-5 sm:gap-4 sm:grid-cols-2 xl:grid-cols-3">
            <AnimatePresence mode="popLayout">
              {allListings.map((listing, i) => (
                <motion.div
                  key={listing.token}
                  layout
                  initial={{ opacity: 0, y: 20 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, scale: 0.95 }}
                  transition={{
                    delay: Math.min(i, 6) * 0.04,
                    duration: 0.3,
                    ease: "easeOut",
                    layout: { duration: 0.25 },
                  }}
                >
                  <ListingCard listing={listing} />
                </motion.div>
              ))}
            </AnimatePresence>
          </div>

          {/* Infinite scroll trigger */}
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
            ) : allListings.length > 0 ? (
              <p className="text-xs text-muted-foreground">
                הצגת כל {total.toLocaleString("he-IL")} המודעות
              </p>
            ) : null}
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
    <Link
      to={`/listings/${listing.token}`}
      state={{ listing }}
      aria-label={`${listing.manufacturer} ${listing.model} ${listing.year} - ${formatPrice(listing.price)}`}
      className="group block rounded-2xl border border-border/40 bg-card overflow-hidden shadow-sm transition-all duration-300 ease-out hover:border-primary/20 hover:shadow-[0_16px_48px_-12px_var(--color-glow-primary)] hover:-translate-y-1 dir-rtl"
    >
      <ListingCardBody
        listing={listing}
        hoverScale
        showBookmarkOverlay={saved}
        actions={
          <>
            <button
              type="button"
              aria-label={seen ? "החזר לרשימת החדשות" : "סמן כנצפה"}
              aria-pressed={seen}
              onClick={(e) => {
                e.stopPropagation();
                toggleSeen();
              }}
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
              onClick={(e) => {
                e.stopPropagation();
                toggleSaved({
                  onSuccess: (next) => toast(next ? "נשמר בהצלחה" : "הוסר מהשמורים", next ? "success" : "info"),
                });
              }}
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
                onClick={(e) => e.stopPropagation()}
                className="rounded-lg p-2.5 min-h-[44px] min-w-[44px] flex items-center justify-center text-muted-foreground transition-all duration-150 hover:bg-primary/5 hover:text-primary"
              >
                <ExternalLink className="h-4 w-4" />
              </a>
            ) : null}
          </>
        }
      />
    </Link>
  );
});
