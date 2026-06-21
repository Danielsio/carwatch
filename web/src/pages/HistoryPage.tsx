import { useEffect, useState } from "react";
import { Link } from "react-router";
import { Clock } from "lucide-react";
import { useHistory } from "@/hooks/useBookmarks";
import { usePageTitle } from "@/hooks/usePageTitle";
import { ListingCardBody } from "@/components/ListingCardBody";
import {
  Button,
  EmptyState,
  ErrorState,
  PageShell,
  OffsetPagination as Pagination,
} from "@/components/ui";
import { InboxTabs } from "@/components/InboxTabs";
import { ListingCardSkeleton } from "@/components/ListingCardSkeleton";
import { Select } from "@/components/ui/NativeSelect";
import type { Listing } from "@/lib/api";

const PAGE_SIZE = 20;

const SORT_OPTIONS = [
  { value: "newest", label: "חדש ביותר" },
  { value: "price_asc", label: "מחיר: נמוך לגבוה" },
  { value: "price_desc", label: "מחיר: גבוה לנמוך" },
];

export function HistoryPage() {
  usePageTitle("היסטוריה");
  const [offset, setOffset] = useState(0);
  const [sort, setSort] = useState("newest");
  const { data, isLoading, isError } = useHistory(PAGE_SIZE, offset, sort);

  useEffect(() => {
    if (!data || data.total === 0) return;
    if (offset >= data.total) {
      setOffset(Math.floor((data.total - 1) / PAGE_SIZE) * PAGE_SIZE);
    }
  }, [data, offset]);

  if (isLoading) {
    return (
      <PageShell>
        <InboxTabs />
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {[1, 2, 3, 4].map((i) => (
            <ListingCardSkeleton key={i} />
          ))}
        </div>
      </PageShell>
    );
  }

  if (isError) {
    return (
      <PageShell>
        <InboxTabs />
        <ErrorState
          title="שגיאה בטעינת ההיסטוריה"
          description="נסה לרענן את הדף"
          onRetry={() => window.location.reload()}
        />
      </PageShell>
    );
  }

  return (
    <PageShell>
      <InboxTabs
        action={
          data ? (
            <div className="flex items-center gap-3">
              <Select
                value={sort}
                onChange={(e) => {
                  setSort(e.target.value);
                  setOffset(0);
                }}
                className="w-auto text-sm"
                aria-label="מיון תוצאות"
              >
                {SORT_OPTIONS.map((opt) => (
                  <option key={opt.value} value={opt.value}>
                    {opt.label}
                  </option>
                ))}
              </Select>
              <span className="text-sm text-muted-foreground tabular-nums">
                ({data.total} מודעות)
              </span>
            </div>
          ) : null
        }
      />

      {!data || data.total === 0 ? (
        <EmptyState
          icon={Clock}
          title="אין מודעות בהיסטוריה"
          description="מודעות שנמצאו יופיעו כאן"
          action={
            <Button asChild variant="secondary" size="sm">
              <Link to="/dashboard">חזרה ללוח הבקרה</Link>
            </Button>
          }
        />
      ) : (
        <>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
            {data.items.map((listing) => (
              <HistoryCard key={listing.token} listing={listing} />
            ))}
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

function HistoryCard({ listing }: { listing: Listing }) {
  return (
    <Link
      to={`/listings/${listing.token}`}
      aria-label={`פתח מודעה: ${listing.manufacturer} ${listing.model}`}
      className="group block rounded-2xl border border-border/50 bg-card overflow-hidden transition-all duration-300 hover:border-border hover:shadow-[0_8px_32px_rgba(0,0,0,0.4)] hover:-translate-y-0.5"
    >
      <ListingCardBody
        listing={listing}
        hoverScale
        showBookmarkOverlay={!!listing.saved}
      />
    </Link>
  );
}
