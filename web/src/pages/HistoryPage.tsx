import { useEffect, useState } from "react";
import { Clock, ExternalLink } from "lucide-react";
import { useHistory } from "@/hooks/useBookmarks";
import { safeHref, cn } from "@/lib/utils";
import { ListingCardBody } from "@/components/ListingCardBody";
import { Pagination } from "@/pages/ListingsPage";
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
      <div className="space-y-5 animate-fade-in">
        <div className="flex items-center gap-2">
          <div className="h-7 w-7 rounded-lg skeleton" />
          <div className="h-8 w-28 rounded-lg skeleton" />
        </div>
        <div className="grid gap-4 sm:grid-cols-2">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="h-52 rounded-2xl skeleton" />
          ))}
        </div>
      </div>
    );
  }

  if (isError) {
    return (
      <div className="space-y-5 animate-fade-in">
        <h1 className="text-2xl font-bold">היסטוריה</h1>
        <div className="rounded-2xl border border-destructive/30 bg-destructive/5 p-8 text-center">
          <p className="text-destructive font-semibold text-lg">
            שגיאה בטעינת ההיסטוריה
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-5 pb-20 md:pb-4 animate-fade-in">
      <div className="flex items-center gap-2.5">
        <div className="flex h-8 w-8 items-center justify-center rounded-xl bg-primary/10">
          <Clock className="h-4 w-4 text-primary" />
        </div>
        <h1 className="text-2xl font-bold tracking-tight">היסטוריה</h1>
        {data && (
          <span className="rounded-full bg-primary/10 px-3 py-0.5 text-sm font-semibold text-primary">
            {data.total} מודעות
          </span>
        )}
      </div>

      {!data || data.total === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-2xl border-2 border-dashed border-border py-20">
          <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-muted mb-4">
            <Clock className="h-7 w-7 text-muted-foreground/30" />
          </div>
          <p className="text-muted-foreground font-medium">
            אין מודעות בהיסטוריה
          </p>
          <p className="text-sm text-muted-foreground mt-1">
            מודעות שנמצאו יופיעו כאן
          </p>
        </div>
      ) : (
        <>
          <div className="grid gap-4 sm:grid-cols-2">
            {data.items.map((listing, i) => (
              <HistoryCard
                key={listing.token}
                listing={listing}
                className={`stagger-${Math.min(i + 1, 6)} animate-slide-up`}
              />
            ))}
          </div>

          {(data.total > PAGE_SIZE || offset > 0) && (
            <Pagination
              offset={offset}
              total={data.total}
              pageSize={PAGE_SIZE}
              onPrev={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
              onNext={() => setOffset(offset + PAGE_SIZE)}
            />
          )}
        </>
      )}
    </div>
  );
}

function HistoryCard({
  listing,
  className,
}: {
  listing: Listing;
  className?: string;
}) {
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
        className={cn(
          "group block rounded-2xl border border-border bg-card overflow-hidden shadow-sm hover-lift gradient-card",
          className,
        )}
      >
        {body}
      </a>
    );
  }

  return (
    <div
      className={cn(
        "rounded-2xl border border-border bg-card overflow-hidden shadow-sm gradient-card",
        className,
      )}
    >
      {body}
    </div>
  );
}
