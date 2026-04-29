import { useEffect, useState } from "react";
import { Clock, ExternalLink } from "lucide-react";
import { useHistory } from "@/hooks/useBookmarks";
import { safeHref } from "@/lib/utils";
import { ListingCardBody } from "@/components/ListingCardBody";
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
      <div className="space-y-4">
        <h1 className="text-2xl font-bold">היסטוריה</h1>
        <div className="grid gap-4 sm:grid-cols-2">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="h-48 animate-pulse rounded-xl bg-muted" />
          ))}
        </div>
      </div>
    );
  }

  if (isError) {
    return (
      <div className="space-y-4">
        <h1 className="text-2xl font-bold">היסטוריה</h1>
        <div className="rounded-xl border border-destructive/50 bg-destructive/10 p-6 text-center">
          <p className="text-destructive font-medium">שגיאה בטעינת ההיסטוריה</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-4 pb-20 md:pb-4">
      <div className="flex items-center gap-2">
        <Clock className="h-5 w-5 text-primary" />
        <h1 className="text-2xl font-bold">היסטוריה</h1>
        {data && (
          <span className="text-sm text-muted-foreground">
            ({data.total} מודעות)
          </span>
        )}
      </div>

      {!data || data.total === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border py-16">
          <Clock className="h-8 w-8 text-muted-foreground/30 mb-3" />
          <p className="text-muted-foreground">אין מודעות בהיסטוריה</p>
          <p className="text-sm text-muted-foreground mt-1">
            מודעות שנמצאו יופיעו כאן
          </p>
        </div>
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
                <button
                  onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
                  className="rounded-lg bg-secondary px-4 py-2 text-sm font-medium text-secondary-foreground hover:bg-secondary/80 transition-colors"
                >
                  הקודם
                </button>
              )}
              <span className="text-sm text-muted-foreground">
                {offset + 1}–{Math.min(offset + PAGE_SIZE, data.total)} מתוך{" "}
                {data.total}
              </span>
              {offset + PAGE_SIZE < data.total && (
                <button
                  onClick={() => setOffset(offset + PAGE_SIZE)}
                  className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 transition-colors"
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

function HistoryCard({ listing }: { listing: Listing }) {
  const href = safeHref(listing.page_link);

  const body = (
    <ListingCardBody
      listing={listing}
      hoverScale={!!href}
      actions={
        href ? (
          <ExternalLink className="h-3.5 w-3.5 text-muted-foreground group-hover:text-primary transition-colors" />
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
        className="group block rounded-xl border border-border bg-card overflow-hidden shadow-sm hover:shadow-md transition-shadow"
      >
        {body}
      </a>
    );
  }

  return (
    <div className="rounded-xl border border-border bg-card overflow-hidden shadow-sm">
      {body}
    </div>
  );
}
