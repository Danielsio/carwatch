import { useEffect, useState } from "react";
import { Bookmark, ExternalLink, Trash2 } from "lucide-react";
import { useSavedListings, useRemoveBookmark } from "@/hooks/useBookmarks";
import { safeHref, cn } from "@/lib/utils";
import { ListingCardBody } from "@/components/ListingCardBody";
import { Pagination } from "@/pages/ListingsPage";
import type { Listing } from "@/lib/api";

const PAGE_SIZE = 20;

export function SavedPage() {
  const [offset, setOffset] = useState(0);
  const [removingTokens, setRemovingTokens] = useState<Set<string>>(new Set());
  const { data, isLoading, isError } = useSavedListings(PAGE_SIZE, offset);
  const removeBookmark = useRemoveBookmark();

  useEffect(() => {
    if (!data || data.total === 0) return;
    if (offset > 0 && offset >= data.total) {
      setOffset(Math.floor((data.total - 1) / PAGE_SIZE) * PAGE_SIZE);
    }
  }, [data, offset]);

  if (isLoading) {
    return (
      <div className="space-y-5 animate-fade-in">
        <div className="flex items-center gap-2">
          <div className="h-7 w-7 rounded-lg skeleton" />
          <div className="h-8 w-24 rounded-lg skeleton" />
        </div>
        <div className="grid gap-4 sm:grid-cols-2">
          {[1, 2].map((i) => (
            <div key={i} className="h-52 rounded-2xl skeleton" />
          ))}
        </div>
      </div>
    );
  }

  if (isError) {
    return (
      <div className="space-y-5 animate-fade-in">
        <h1 className="text-2xl font-bold">שמורים</h1>
        <div className="rounded-2xl border border-destructive/30 bg-destructive/5 p-8 text-center">
          <p className="text-destructive font-semibold text-lg">
            שגיאה בטעינת המודעות
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-5 pb-20 md:pb-4 animate-fade-in">
      <div className="flex items-center gap-2.5">
        <div className="flex h-8 w-8 items-center justify-center rounded-xl bg-primary/10">
          <Bookmark className="h-4 w-4 text-primary" />
        </div>
        <h1 className="text-2xl font-bold tracking-tight">שמורים</h1>
        {data && (
          <span className="rounded-full bg-primary/10 px-3 py-0.5 text-sm font-semibold text-primary">
            {data.total}
          </span>
        )}
      </div>

      {!data || data.items.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-2xl border-2 border-dashed border-border py-20">
          <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-muted mb-4">
            <Bookmark className="h-7 w-7 text-muted-foreground/30" />
          </div>
          <p className="text-muted-foreground font-medium">
            אין מודעות שמורות
          </p>
          <p className="text-sm text-muted-foreground mt-1">
            לחץ על סמל השמירה במודעה כדי לשמור אותה
          </p>
        </div>
      ) : (
        <>
          <div className="grid gap-4 sm:grid-cols-2">
            {data.items.map((listing, i) => (
              <SavedCard
                key={listing.token}
                listing={listing}
                onRemove={() => {
                  setRemovingTokens((prev) =>
                    new Set(prev).add(listing.token),
                  );
                  removeBookmark.mutate(listing.token, {
                    onSettled: () =>
                      setRemovingTokens((prev) => {
                        const next = new Set(prev);
                        next.delete(listing.token);
                        return next;
                      }),
                  });
                }}
                removing={removingTokens.has(listing.token)}
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

function SavedCard({
  listing,
  onRemove,
  removing,
  className,
}: {
  listing: Listing;
  onRemove: () => void;
  removing: boolean;
  className?: string;
}) {
  const externalHref = safeHref(listing.page_link);

  return (
    <div
      className={cn(
        "rounded-2xl border border-border bg-card overflow-hidden shadow-sm hover-lift gradient-card",
        className,
      )}
    >
      <ListingCardBody
        listing={listing}
        actions={
          <>
            <button
              onClick={onRemove}
              disabled={removing}
              className="inline-flex items-center gap-1 rounded-lg px-2 py-1 text-xs font-medium text-destructive hover:bg-destructive/10 transition-colors disabled:opacity-50"
            >
              <Trash2 className="h-3.5 w-3.5" />
              הסר
            </button>
            {externalHref && (
              <a
                href={externalHref}
                target="_blank"
                rel="noopener noreferrer"
                aria-label={`פתח מודעה: ${listing.manufacturer} ${listing.model}`}
                className="inline-flex items-center gap-1 rounded-lg px-2 py-1 text-xs text-muted-foreground hover:text-primary transition-colors"
              >
                <ExternalLink className="h-3.5 w-3.5" />
              </a>
            )}
          </>
        }
      />
    </div>
  );
}
