import { useState, useEffect, useCallback } from "react";
import type { Listing } from "@/lib/api";
import { useSaveBookmark, useRemoveBookmark } from "@/hooks/useBookmarks";
import { useMarkListingSeen, useUnmarkListingSeen } from "@/hooks/useListingSeen";

export function useListingActions(listing: Listing) {
  const saveBookmark = useSaveBookmark();
  const removeBookmark = useRemoveBookmark();
  const markSeen = useMarkListingSeen();
  const unmarkSeen = useUnmarkListingSeen();

  const [saved, setSaved] = useState(() => listing.saved ?? false);
  const [seen, setSeen] = useState(() => listing.seen ?? false);

  useEffect(() => {
    setSaved(listing.saved ?? false);
  }, [listing.token, listing.saved]);

  useEffect(() => {
    setSeen(listing.seen ?? false);
  }, [listing.token, listing.seen]);

  const toggleSaved = useCallback(
    (opts?: { onSuccess?: (next: boolean) => void }) => {
      const next = !saved;
      setSaved(next);
      const mutation = next ? saveBookmark : removeBookmark;
      mutation.mutate(listing.token, {
        onSuccess: () => opts?.onSuccess?.(next),
        onError: () => setSaved(!next),
      });
    },
    [saved, listing.token, saveBookmark, removeBookmark],
  );

  const toggleSeen = useCallback(
    (opts?: { onSuccess?: (next: boolean) => void }) => {
      const next = !seen;
      setSeen(next);
      const mutation = next ? markSeen : unmarkSeen;
      mutation.mutate(listing.token, {
        onSuccess: () => opts?.onSuccess?.(next),
        onError: () => setSeen(!next),
      });
    },
    [seen, listing.token, markSeen, unmarkSeen],
  );

  return {
    saved,
    seen,
    toggleSaved,
    toggleSeen,
    isSaving: saveBookmark.isPending || removeBookmark.isPending,
    isTogglingSeen: markSeen.isPending || unmarkSeen.isPending,
  };
}
