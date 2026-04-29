import { useEffect, useRef, useState } from "react";
import { Bell, ExternalLink } from "lucide-react";
import {
  useNotifications,
  useMarkNotificationsSeen,
} from "@/hooks/useNotifications";
import { safeHref } from "@/lib/utils";
import { ListingCardBody } from "@/components/ListingCardBody";
import type { Listing } from "@/lib/api";

const PAGE_SIZE = 20;

export function NotificationsPage() {
  const [offset, setOffset] = useState(0);
  const { data, isLoading, isError } = useNotifications(PAGE_SIZE, offset);
  const markSeen = useMarkNotificationsSeen();
  const markedRef = useRef(false);

  useEffect(() => {
    if (!markedRef.current) {
      markedRef.current = true;
      markSeen.mutate();
    }
  }, [markSeen]);

  useEffect(() => {
    if (!data || data.total === 0) return;
    if (offset >= data.total) {
      setOffset(Math.floor((data.total - 1) / PAGE_SIZE) * PAGE_SIZE);
    }
  }, [data, offset]);

  if (isLoading) {
    return (
      <div className="space-y-6">
        <div className="h-8 w-36 shimmer-skeleton rounded-lg" />
        <div className="grid gap-4 sm:grid-cols-2">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="h-72 shimmer-skeleton rounded-2xl" />
          ))}
        </div>
      </div>
    );
  }

  if (isError) {
    return (
      <div className="space-y-6">
        <h1 className="text-2xl font-semibold tracking-tight">התראות</h1>
        <div className="rounded-2xl border border-destructive/20 bg-destructive/5 p-8 text-center">
          <p className="text-destructive font-medium">
            שגיאה בטעינת ההתראות
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6 pb-20 md:pb-4">
      <div className="flex items-center gap-2.5">
        <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary/10">
          <Bell className="h-4 w-4 text-primary" />
        </div>
        <h1 className="text-2xl font-semibold tracking-tight">התראות</h1>
        {data && (
          <span className="text-sm text-muted-foreground tabular-nums">
            ({data.total} חדשות)
          </span>
        )}
      </div>

      {!data || data.total === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-2xl border border-border/50 bg-card/50 py-20">
          <Bell className="h-10 w-10 text-muted-foreground/20 mb-4" />
          <p className="text-muted-foreground">אין התראות חדשות</p>
          <p className="text-sm text-muted-foreground mt-1">
            מודעות חדשות שימצאו יופיעו כאן
          </p>
        </div>
      ) : (
        <>
          <div className="grid gap-4 sm:grid-cols-2">
            {data.items.map((listing) => (
              <NotificationCard key={listing.token} listing={listing} />
            ))}
          </div>

          {(data.total > PAGE_SIZE || offset > 0) && (
            <div className="flex items-center justify-center gap-3 pt-4">
              {offset > 0 && (
                <button
                  onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
                  className="rounded-xl bg-secondary px-4 py-2 text-sm font-medium text-secondary-foreground transition-all duration-200 hover:bg-accent active:scale-[0.97]"
                >
                  הקודם
                </button>
              )}
              <span className="text-sm text-muted-foreground tabular-nums">
                {offset + 1}–{Math.min(offset + PAGE_SIZE, data.total)} מתוך{" "}
                {data.total}
              </span>
              {offset + PAGE_SIZE < data.total && (
                <button
                  onClick={() => setOffset(offset + PAGE_SIZE)}
                  className="rounded-xl bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-all duration-200 hover:bg-primary/90 active:scale-[0.97]"
                >
                  הבא
                </button>
              )}
            </div>
          )}
        </>
      )}
    </div>
  );
}

function NotificationCard({ listing }: { listing: Listing }) {
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
