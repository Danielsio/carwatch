import { useEffect, useState } from "react";
import { Clock, ExternalLink } from "lucide-react";
import { useHistory } from "@/hooks/useBookmarks";
import { safeHref } from "@/lib/utils";
import { ListingCardBody } from "@/components/ListingCardBody";
import { Button, EmptyState, PageHeader, Skeleton } from "@/components/ui";
import type { Listing } from "@/lib/api";

const PAGE_SIZE = 20;

export function HistoryPage() {
  const [offset, setOffset] = useState(0);
  const { data, isLoading, isError } = useHistory(PAGE_SIZE, offset);

  useEffect(() => {
    if (!data || data.total === 0) return;
    if (offset >= data.total) {
      setOffset(Math.floor((data.total - 1) / PAGE_SIZE) * PAGE_SIZE);
    }
  }, [data, offset]);

  if (isLoading) {
    return (
      <div className="space-y-6 pb-20 md:pb-4">
        <PageHeader title="היסטוריה" />
        <div className="grid gap-4 sm:grid-cols-2">
          {[1, 2, 3, 4].map((i) => (
            <Skeleton key={i} className="h-72 rounded-2xl" />
          ))}
        </div>
      </div>
    );
  }

  if (isError) {
    return (
      <div className="space-y-6 pb-20 md:pb-4">
        <PageHeader title="היסטוריה" />
        <div className="rounded-2xl border border-destructive/20 bg-destructive/5 p-8 text-center">
          <p className="text-destructive font-medium">
            שגיאה בטעינת ההיסטוריה
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6 pb-20 md:pb-4">
      <PageHeader
        title="היסטוריה"
        action={
          data ? (
            <span className="text-sm text-muted-foreground tabular-nums">
              ({data.total} מודעות)
            </span>
          ) : null
        }
      />

      {!data || data.total === 0 ? (
        <EmptyState
          icon={Clock}
          title="אין מודעות בהיסטוריה"
          description="מודעות שנמצאו יופיעו כאן"
        />
      ) : (
        <>
          <div className="grid gap-4 sm:grid-cols-2">
            {data.items.map((listing) => (
              <HistoryCard key={listing.token} listing={listing} />
            ))}
          </div>

          {(data.total > PAGE_SIZE || offset > 0) && (
            <div className="flex items-center justify-center gap-3 pt-4">
              {offset > 0 && (
                <Button
                  variant="secondary"
                  size="sm"
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
                  variant="primary"
                  size="sm"
                  onClick={() => setOffset(offset + PAGE_SIZE)}
                >
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

function HistoryCard({ listing }: { listing: Listing }) {
  const href = safeHref(listing.page_link);

  const body = (
    <ListingCardBody
      listing={listing}
      hoverScale={!!href}
      actions={
        href ? (
          <ExternalLink className="h-3.5 w-3.5 text-muted-foreground group-hover:text-primary transition-colors duration-200" />
        ) : undefined
      }
    />
  );

  if (href) {
    return (
      <a
        href={href}
        target="_blank"
        rel="noopener noreferrer"
        aria-label={`פתח מודעה: ${listing.manufacturer} ${listing.model}`}
        className="group block rounded-2xl border border-border/50 bg-card overflow-hidden transition-all duration-300 hover:border-border hover:shadow-[0_8px_32px_rgba(0,0,0,0.4)] hover:-translate-y-0.5"
      >
        {body}
      </a>
    );
  }

  return (
    <div className="rounded-2xl border border-border/50 bg-card overflow-hidden">
      {body}
    </div>
  );
}
