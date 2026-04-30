import { useEffect, useState } from "react";
import { Bookmark, ExternalLink, Trash2 } from "lucide-react";
import { useSavedListings, useRemoveBookmark } from "@/hooks/useBookmarks";
import { safeHref } from "@/lib/utils";
import { ListingCardBody } from "@/components/ListingCardBody";
import {
  Button,
  EmptyState,
  PageHeader,
  Skeleton,
  useToast,
} from "@/components/ui";
import type { Listing } from "@/lib/api";

const PAGE_SIZE = 20;

export function SavedPage() {
  const { toast } = useToast();
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
      <div className="space-y-6 pb-20 md:pb-4">
        <PageHeader title="שמורים" />
        <div className="grid gap-4 sm:grid-cols-2">
          {[1, 2].map((i) => (
            <Skeleton key={i} className="h-72 rounded-2xl" />
          ))}
        </div>
      </div>
    );
  }

  if (isError) {
    return (
      <div className="space-y-6 pb-20 md:pb-4">
        <PageHeader title="שמורים" />
        <div className="rounded-2xl border border-destructive/20 bg-destructive/5 p-8 text-center">
          <p className="text-destructive font-medium">שגיאה בטעינת המודעות</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6 pb-20 md:pb-4">
      <PageHeader
        title="שמורים"
        action={
          data ? (
            <span className="text-sm text-muted-foreground tabular-nums">
              ({data.total})
            </span>
          ) : null
        }
      />

      {!data || data.items.length === 0 ? (
        <EmptyState
          icon={Bookmark}
          title="אין מודעות שמורות"
          description="לחץ על סמל השמירה במודעה כדי לשמור אותה"
        />
      ) : (
        <>
          <div className="grid gap-4 sm:grid-cols-2">
            {data.items.map((listing) => (
              <SavedCard
                key={listing.token}
                listing={listing}
                onRemove={() => {
                  setRemovingTokens((prev) =>
                    new Set(prev).add(listing.token),
                  );
                  removeBookmark.mutate(listing.token, {
                    onSuccess: () => {
                      toast("הוסר מהשמורים", "info");
                    },
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
}: {
  listing: Listing;
  onRemove: () => void;
  removing: boolean;
}) {
  const externalHref = safeHref(listing.page_link);

  return (
    <div className="rounded-2xl border border-border/50 bg-card overflow-hidden transition-all duration-300 hover:border-border hover:shadow-[0_8px_32px_rgba(0,0,0,0.4)]">
    <ListingCardBody
      listing={listing}
      showBookmarkOverlay
      actions={
          <>
            <button
              onClick={onRemove}
              disabled={removing}
              className="inline-flex items-center gap-1 rounded-lg px-2 py-1 text-xs text-destructive hover:bg-destructive/10 transition-all duration-200 disabled:opacity-50 active:scale-[0.97]"
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
                className="inline-flex items-center gap-1 rounded-lg px-2 py-1 text-xs text-muted-foreground hover:text-primary hover:bg-primary/5 transition-all duration-200"
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

function Pagination({
  offset,
  total,
  pageSize,
  onPrev,
  onNext,
}: {
  offset: number;
  total: number;
  pageSize: number;
  onPrev: () => void;
  onNext: () => void;
}) {
  return (
    <div className="flex items-center justify-center gap-3 pt-4">
      {offset > 0 && (
        <Button variant="secondary" size="sm" onClick={onPrev}>
          הקודם
        </Button>
      )}
      <span className="text-sm text-muted-foreground tabular-nums">
        {offset + 1}–{Math.min(offset + pageSize, total)} מתוך {total}
      </span>
      {offset + pageSize < total && (
        <Button variant="primary" size="sm" onClick={onNext}>
          הבא
        </Button>
      )}
    </div>
  );
}
