import { useEffect, useState } from "react";
import { Bookmark, ExternalLink, Trash2 } from "lucide-react";
import { useSavedListings, useRemoveBookmark } from "@/hooks/useBookmarks";
import { safeHref } from "@/lib/utils";
import { ListingCardBody } from "@/components/ListingCardBody";
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
      <div className="space-y-4">
        <h1 className="text-2xl font-bold">שמורים</h1>
        <div className="grid gap-4 sm:grid-cols-2">
          {[1, 2].map((i) => (
            <div key={i} className="h-48 animate-pulse rounded-xl bg-muted" />
          ))}
        </div>
      </div>
    );
  }

  if (isError) {
    return (
      <div className="space-y-4">
        <h1 className="text-2xl font-bold">שמורים</h1>
        <div className="rounded-xl border border-destructive/50 bg-destructive/10 p-6 text-center">
          <p className="text-destructive font-medium">שגיאה בטעינת המודעות</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-4 pb-20 md:pb-4">
      <div className="flex items-center gap-2">
        <Bookmark className="h-5 w-5 text-primary" />
        <h1 className="text-2xl font-bold">שמורים</h1>
        {data && (
          <span className="text-sm text-muted-foreground">
            ({data.total})
          </span>
        )}
      </div>

      {!data || data.items.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border py-16">
          <Bookmark className="h-8 w-8 text-muted-foreground/30 mb-3" />
          <p className="text-muted-foreground">אין מודעות שמורות</p>
          <p className="text-sm text-muted-foreground mt-1">
            לחץ על סמל השמירה במודעה כדי לשמור אותה
          </p>
        </div>
      ) : (
        <>
          <div className="grid gap-4 sm:grid-cols-2">
            {data.items.map((listing) => (
              <SavedCard
                key={listing.token}
                listing={listing}
                onRemove={() => {
                  setRemovingTokens((prev) => new Set(prev).add(listing.token));
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
              />
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

function SavedCard({
  listing,
  onRemove,
  removing,
}: {
  listing: Listing;
  onRemove: () => void;
  removing: boolean;
}) {
  const externalHref = safeHref(listing.page_link);

  return (
    <div className="rounded-xl border border-border bg-card overflow-hidden shadow-sm">
      <ListingCardBody
        listing={listing}
        actions={
          <>
            <button
              onClick={onRemove}
              disabled={removing}
              className="inline-flex items-center gap-1 rounded-lg px-2 py-1 text-xs text-destructive hover:bg-destructive/10 transition-colors disabled:opacity-50"
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
