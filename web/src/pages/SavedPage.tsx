import { useEffect, useState } from "react";
import { Link } from "react-router";
import { Bookmark, ExternalLink, Trash2 } from "lucide-react";
import {
  useSavedListings,
  useRemoveBookmark,
  useSaveBookmark,
} from "@/hooks/useBookmarks";
import { usePageTitle } from "@/hooks/usePageTitle";
import { safeHref } from "@/lib/utils";
import { ListingCardBody } from "@/components/ListingCardBody";
import {
  Button,
  EmptyState,
  ErrorState,
  PageShell,
  OffsetPagination as Pagination,
  useToast,
} from "@/components/ui";
import { InboxTabs } from "@/components/InboxTabs";
import { ListingCardSkeleton } from "@/components/ListingCardSkeleton";
import type { Listing } from "@/lib/api";

const PAGE_SIZE = 20;

export function SavedPage() {
  usePageTitle("שמורים");
  const { toast } = useToast();
  const [offset, setOffset] = useState(0);
  const [removingTokens, setRemovingTokens] = useState<Set<string>>(new Set());
  const { data, isLoading, isError } = useSavedListings(PAGE_SIZE, offset);
  const removeBookmark = useRemoveBookmark();
  const saveBookmark = useSaveBookmark();

  useEffect(() => {
    if (!data || data.total === 0) return;
    if (offset > 0 && offset >= data.total) {
      setOffset(Math.floor((data.total - 1) / PAGE_SIZE) * PAGE_SIZE);
    }
  }, [data, offset]);

  if (isLoading) {
    return (
      <PageShell>
        <InboxTabs />
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          {[1, 2].map((i) => (
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
          title="שגיאה בטעינת המודעות"
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
          action={
            <Button asChild variant="secondary" size="sm">
              <Link to="/dashboard">חזרה ללוח הבקרה</Link>
            </Button>
          }
        />
      ) : (
        <>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
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
                      toast("הוסר מהשמורים", "info", {
                        action: {
                          label: "בטל",
                          onClick: () => saveBookmark.mutate(listing.token),
                        },
                      });
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

