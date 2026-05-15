import { useState, useEffect, useCallback, useRef, memo } from "react";
import { useParams, useNavigate, Link } from "react-router";
import {
  ExternalLink,
  Bookmark,
  Car,
  Eye,
  EyeOff,
  RefreshCw,
} from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useListings } from "@/hooks/useListings";
import { useSaveBookmark, useRemoveBookmark } from "@/hooks/useBookmarks";
import { useMarkListingSeen, useUnmarkListingSeen } from "@/hooks/useListingSeen";
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
import { Pagination } from "@/components/ui/Pagination";
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
const REFRESH_COOLDOWN_S = 60;

function isInteractiveTarget(target: EventTarget | null): boolean {
  if (!(target instanceof Element)) return false;
  return target.closest("button,a,input,select,textarea") != null;
}

/** Self-contained refresh button that manages its own cooldown timer so ticks
 *  don't re-render the parent (and therefore every listing card). */
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
          if (cooldownRef.current) clearInterval(cooldownRef.current);
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

  const refreshMutation = useMutation<RefreshResponse, Error>({
    mutationFn: () => api.refreshListings(searchId),
    meta: { suppressToast: true },
    onSuccess: (res) => {
      queryClient.invalidateQueries({ queryKey: ["listings", searchId] });
      queryClient.invalidateQueries({ queryKey: ["searches"] });
      startCooldown();

      if (res.removed > 0) {
        toast(`הוסרו ${res.removed} מודעות שכבר לא זמינות`, "info");
      }
      toast(
        `נטענו ${res.total} מודעות מעודכנות`,
        "success",
      );
    },
    onError: (err) => {
      if (err.message.includes("wait")) {
        toast(err.message, "info");
        startCooldown();
      } else {
        toast("שגיאה בעת רענון המודעות", "error");
      }
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
        "relative flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-sm font-medium transition-all duration-300",
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

  const countSubtitle =
    !showSkeletons && data != null
      ? `${data.total.toLocaleString("he-IL")} מודעות`
      : undefined;

  return (
    <PageShell gap="sm">
      <PageHeader
        backTo="/dashboard"
        title="תוצאות"
        subtitle={
          showSkeletons ? (
            <span className="inline-block h-4 w-24 rounded shimmer-skeleton align-middle" />
          ) : (
            countSubtitle
          )
        }
      />

      {/* Sort pills + refresh button */}
      <div className="flex items-center gap-2 dir-rtl">
        <div className="flex flex-wrap gap-2 flex-1">
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

        <RefreshButton searchId={searchId} />
      </div>

      {/* Grid */}
      {showSkeletons ? (
        <div className="grid gap-5 sm:gap-4 sm:grid-cols-2">
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
      ) : !data || data.items.length === 0 ? (
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
          <div className="grid gap-5 sm:gap-4 sm:grid-cols-2">
            <AnimatePresence mode="popLayout">
              {data.items.map((listing, i) => (
                <motion.div
                  key={listing.token}
                  layout
                  initial={{ opacity: 0, y: 20 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, scale: 0.95 }}
                  transition={{
                    delay: i * 0.05,
                    duration: 0.35,
                    ease: "easeOut",
                    layout: { duration: 0.3 },
                  }}
                >
                  <ListingCard listing={listing} />
                </motion.div>
              ))}
            </AnimatePresence>
          </div>

          <Pagination
            offset={offset}
            total={data.total}
            pageSize={PAGE_SIZE}
            onPrev={() => {
              setOffset(Math.max(0, offset - PAGE_SIZE));
              window.scrollTo({ top: 0, behavior: "smooth" });
            }}
            onNext={() => {
              setOffset(offset + PAGE_SIZE);
              window.scrollTo({ top: 0, behavior: "smooth" });
            }}
          />
        </>
      )}
    </PageShell>
  );
}

const ListingCard = memo(function ListingCard({ listing }: { listing: Listing }) {
  const navigate = useNavigate();
  const saveBookmark = useSaveBookmark();
  const removeBookmark = useRemoveBookmark();
  const markListingSeen = useMarkListingSeen();
  const unmarkListingSeen = useUnmarkListingSeen();
  const { toast } = useToast();
  const [saved, setSaved] = useState(() => listing.saved ?? false);
  const [seen, setSeen] = useState(() => listing.seen ?? false);

  useEffect(() => {
    setSaved(listing.saved ?? false);
  }, [listing.token, listing.saved]);

  useEffect(() => {
    setSeen(listing.seen ?? false);
  }, [listing.token, listing.seen]);

  return (
    <div
      role="button"
      tabIndex={0}
      aria-label={`${listing.manufacturer} ${listing.model} ${listing.year} - ${formatPrice(listing.price)}`}
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
      className="group block cursor-pointer rounded-2xl border border-border/50 bg-card overflow-hidden shadow-sm transition-all duration-300 hover:border-border hover:shadow-[0_12px_40px_rgba(0,0,0,0.45)] hover:-translate-y-1 dir-rtl"
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
                const next = !seen;
                setSeen(next);
                const mutation = next ? markListingSeen : unmarkListingSeen;
                mutation.mutate(listing.token, {
                  onError: () => setSeen(!next),
                });
              }}
              className={cn(
                "rounded-lg p-2.5 min-h-[44px] min-w-[44px] flex items-center justify-center transition-all duration-200",
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
                "rounded-lg p-2.5 min-h-[44px] min-w-[44px] flex items-center justify-center transition-all duration-200",
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
                className="rounded-lg p-2.5 min-h-[44px] min-w-[44px] flex items-center justify-center text-muted-foreground transition-all duration-200 hover:bg-primary/5 hover:text-primary"
              >
                <ExternalLink className="h-4 w-4" />
              </a>
            ) : null}
          </>
        }
      />
    </div>
  );
});
