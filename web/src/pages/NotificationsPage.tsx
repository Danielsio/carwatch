import { useEffect, useRef, useState } from "react";
import { Link } from "react-router";
import { Bell } from "lucide-react";
import {
  useNotifications,
  useMarkNotificationsSeen,
} from "@/hooks/useNotifications";
import { usePageTitle } from "@/hooks/usePageTitle";
import { ListingCardBody } from "@/components/ListingCardBody";
import {
  Button,
  EmptyState,
  ErrorState,
  PageHeader,
  PageShell,
  Pagination,
  Skeleton,
} from "@/components/ui";
import type { Listing } from "@/lib/api";

const PAGE_SIZE = 20;

export function NotificationsPage() {
  usePageTitle("התראות");
  const [offset, setOffset] = useState(0);
  const { data, isLoading, isSuccess, isFetching, isError } =
    useNotifications(PAGE_SIZE, offset);
  const markSeen = useMarkNotificationsSeen();
  const markedRef = useRef(false);

  useEffect(() => {
    if (!markedRef.current && isSuccess && !isFetching) {
      markedRef.current = true;
      markSeen.mutate();
    }
  }, [isSuccess, isFetching, markSeen]);

  useEffect(() => {
    if (!data || data.total === 0) return;
    if (offset >= data.total) {
      setOffset(Math.floor((data.total - 1) / PAGE_SIZE) * PAGE_SIZE);
    }
  }, [data, offset]);

  if (isLoading) {
    return (
      <PageShell>
        <PageHeader title="התראות" />
        <div className="grid gap-4 sm:grid-cols-2">
          {[1, 2, 3, 4].map((i) => (
            <Skeleton key={i} className="h-72 rounded-2xl" />
          ))}
        </div>
      </PageShell>
    );
  }

  if (isError) {
    return (
      <PageShell>
        <PageHeader title="התראות" />
        <ErrorState
          title="שגיאה בטעינת ההתראות"
          description="נסה לרענן את הדף"
          onRetry={() => window.location.reload()}
        />
      </PageShell>
    );
  }

  return (
    <PageShell>
      <PageHeader
        title="התראות"
        action={
          data && data.total > 0 ? (
            <span className="text-sm text-muted-foreground tabular-nums">
              ({data.total} חדשות)
            </span>
          ) : null
        }
      />

      {!data || data.total === 0 ? (
        <EmptyState
          icon={Bell}
          title="אין התראות חדשות"
          description="מודעות חדשות שימצאו יופיעו כאן"
          action={
            <Button asChild variant="secondary" size="sm">
              <Link to="/dashboard">חזרה ללוח הבקרה</Link>
            </Button>
          }
        />
      ) : (
        <>
          <div className="grid gap-4 sm:grid-cols-2">
            {data.items.map((listing) => (
              <NotificationCard key={listing.token} listing={listing} />
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

function NotificationCard({ listing }: { listing: Listing }) {
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
