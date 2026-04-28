import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";

export function useSavedListings(limit = 20, offset = 0) {
  return useQuery({
    queryKey: ["saved", { limit, offset }],
    queryFn: () => api.saved.list({ limit, offset }),
  });
}

export function useSaveBookmark() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (token: string) => api.saved.save(token),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["saved"] }),
  });
}

export function useRemoveBookmark() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (token: string) => api.saved.remove(token),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["saved"] }),
  });
}

export function useHistory(limit = 20, offset = 0) {
  return useQuery({
    queryKey: ["history", { limit, offset }],
    queryFn: () => api.history({ limit, offset }),
  });
}
