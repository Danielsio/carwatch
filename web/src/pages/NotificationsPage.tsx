import { useEffect, useState } from "react";
import { Link } from "react-router";
import { Bell, Eye } from "lucide-react";
import {
  useNotifications,
  useMarkNotificationsSeen,
} from "@/hooks/useNotifications";
import { useMarkListingSeen } from "@/hooks/useListingSeen";
import { usePageTitle } from "@/hooks/usePageTitle";
import { ListingCardBody } from "@/components/ListingCardBody";
import {
  Button,
  EmptyState,
  ErrorState,
  PageShell,
  Pagination,
  Skeleton,
} from "@/components/ui";
import { InboxTabs } from "@/components/InboxTabs";
import type { Listing } from "@/lib/api";

const PAGE_SIZE = 20;

export function NotificationsPage() {
  usePageTitle("התראות");
  const [offset, setOffset] = useState(0);
  const { data, isLoading, isError } = useNotifications(PAGE_SIZE, offset);
  const markSeen = useMarkNotificationsSeen();

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
        <InboxTabs />
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
      <InboxTabs
        action={
          data && data.total > 0 ? (
            <div className="flex flex-wrap items-center gap-2 justify-end">
              <span className="text-sm text-muted-foreground tabular-nums">
                ({data.total} חדשות)
              </span>
              <Button
                type="button"
                variant="secondary"
                size="sm"
                disabled={markSeen.isPending}
                onClick={() => markSeen.mutate()}
              >
                סמן הכל כנקרא
              </Button>
            </div>
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
  const markOne = useMarkListingSeen();

  return (
    <div className="relative overflow-hidden rounded-2xl border border-border/50 bg-card transition-all duration-300 hover:border-border hover:shadow-[0_8px_32px_rgba(0,0,0,0.4)] hover:-translate-y-0.5">
      <button
        type="button"
        className="absolute top-2 end-2 z-20 flex h-9 w-9 items-center justify-center rounded-full bg-background/90 text-muted-foreground shadow-sm ring-1 ring-border/60 backdrop-blur-[2px] hover:bg-background hover:text-foreground"
        aria-label="סמן כנצפה והסר מהחדשות"
        disabled={markOne.isPending}
        onClick={() => markOne.mutate(listing.token)}
      >
        <Eye className="h-4 w-4" />
      </button>
      <Link
        to={`/listings/${listing.token}`}
        aria-label={`פתח מודעה: ${listing.manufacturer} ${listing.model}`}
        className="group block"
      >
        <ListingCardBody
          listing={listing}
          hoverScale
          showBookmarkOverlay={!!listing.saved}
        />
      </Link>
    </div>
  );
}
