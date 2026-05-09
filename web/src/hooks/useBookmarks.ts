import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api, type Listing, type ListingsResponse } from "@/lib/api";

export function useSavedListings(limit = 20, offset = 0) {
  return useQuery({
    queryKey: ["saved", { limit, offset }],
    queryFn: () => api.saved.list({ limit, offset }),
  });
}

function findListingByToken(
  qc: ReturnType<typeof useQueryClient>,
  token: string,
): Listing | undefined {
  for (const [, data] of qc.getQueriesData({ queryKey: ["listings"] })) {
    const r = data as ListingsResponse | undefined;
    const found = r?.items?.find((i) => i.token === token);
    if (found) return found;
  }
  for (const [, data] of qc.getQueriesData({ queryKey: ["history"] })) {
    const r = data as ListingsResponse | undefined;
    const found = r?.items?.find((i) => i.token === token);
    if (found) return found;
  }
  return undefined;
}

function restoreQueries(
  qc: ReturnType<typeof useQueryClient>,
  entries: [readonly unknown[], unknown][],
) {
  for (const [key, data] of entries) {
    qc.setQueryData(key, data);
  }
}

export function useSaveBookmark() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (token: string) => api.saved.save(token),
    onMutate: async (token) => {
      await qc.cancelQueries({ queryKey: ["saved"] });
      await qc.cancelQueries({ queryKey: ["listings"] });
      await qc.cancelQueries({ queryKey: ["history"] });
      await qc.cancelQueries({ queryKey: ["notifications"] });

      const previousSaved = qc.getQueriesData({ queryKey: ["saved"] });
      const previousListings = qc.getQueriesData({ queryKey: ["listings"] });
      const previousHistory = qc.getQueriesData({ queryKey: ["history"] });

      const base = findListingByToken(qc, token);
      const savedListing: Listing | undefined = base
        ? { ...base, saved: true }
        : undefined;

      qc.setQueriesData({ queryKey: ["listings"] }, (old) => {
        if (!old) return old;
        const d = old as ListingsResponse;
        return {
          ...d,
          items: d.items.map((item) =>
            item.token === token ? { ...item, saved: true } : item,
          ),
        };
      });

      qc.setQueriesData({ queryKey: ["history"] }, (old) => {
        if (!old) return old;
        const d = old as ListingsResponse;
        return {
          ...d,
          items: d.items.map((item) =>
            item.token === token ? { ...item, saved: true } : item,
          ),
        };
      });

      for (const [queryKey, data] of previousSaved) {
        if (data == null) continue;
        const d = data as ListingsResponse;
        const params = queryKey[1] as { limit: number; offset: number };
        const idx = d.items.findIndex((i) => i.token === token);
        if (idx !== -1) {
          qc.setQueryData(queryKey, {
            ...d,
            items: d.items.map((i) =>
              i.token === token ? { ...i, saved: true } : i,
            ),
          });
          continue;
        }
        if (params.offset === 0 && savedListing) {
          qc.setQueryData(queryKey, {
            ...d,
            items: [savedListing, ...d.items],
            total: d.total + 1,
          });
        } else {
          qc.setQueryData(queryKey, {
            ...d,
            total: d.total + 1,
          });
        }
      }

      return { previousSaved, previousListings, previousHistory };
    },
    onError: (_e, _token, ctx) => {
      if (!ctx) return;
      restoreQueries(qc, ctx.previousSaved);
      restoreQueries(qc, ctx.previousListings);
      restoreQueries(qc, ctx.previousHistory);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["saved"] });
      qc.invalidateQueries({ queryKey: ["listings"] });
      qc.invalidateQueries({ queryKey: ["history"] });
      qc.invalidateQueries({ queryKey: ["notifications"] });
    },
  });
}

export function useRemoveBookmark() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (token: string) => api.saved.remove(token),
    onMutate: async (token) => {
      await qc.cancelQueries({ queryKey: ["saved"] });
      await qc.cancelQueries({ queryKey: ["listings"] });
      await qc.cancelQueries({ queryKey: ["history"] });
      await qc.cancelQueries({ queryKey: ["notifications"] });

      const previousSaved = qc.getQueriesData({ queryKey: ["saved"] });
      const previousListings = qc.getQueriesData({ queryKey: ["listings"] });
      const previousHistory = qc.getQueriesData({ queryKey: ["history"] });

      qc.setQueriesData({ queryKey: ["listings"] }, (old) => {
        if (!old) return old;
        const d = old as ListingsResponse;
        return {
          ...d,
          items: d.items.map((item) =>
            item.token === token ? { ...item, saved: false } : item,
          ),
        };
      });

      qc.setQueriesData({ queryKey: ["history"] }, (old) => {
        if (!old) return old;
        const d = old as ListingsResponse;
        return {
          ...d,
          items: d.items.map((item) =>
            item.token === token ? { ...item, saved: false } : item,
          ),
        };
      });

      for (const [queryKey, data] of previousSaved) {
        if (data == null) continue;
        const d = data as ListingsResponse;
        qc.setQueryData(queryKey, {
          ...d,
          items: d.items.filter((i) => i.token !== token),
          total: Math.max(0, d.total - 1),
        });
      }

      return { previousSaved, previousListings, previousHistory };
    },
    onError: (_e, _token, ctx) => {
      if (!ctx) return;
      restoreQueries(qc, ctx.previousSaved);
      restoreQueries(qc, ctx.previousListings);
      restoreQueries(qc, ctx.previousHistory);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["saved"] });
      qc.invalidateQueries({ queryKey: ["listings"] });
      qc.invalidateQueries({ queryKey: ["history"] });
      qc.invalidateQueries({ queryKey: ["notifications"] });
    },
  });
}

export function useHistory(limit = 20, offset = 0) {
  return useQuery({
    queryKey: ["history", { limit, offset }],
    queryFn: () => api.history({ limit, offset }),
  });
}
