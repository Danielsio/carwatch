import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";

const DAYS = 14;

/** Daily new-listing counts for a search over the last 14 days (dense series). */
export function useSearchActivity(searchId: number, enabled = true) {
  return useQuery({
    queryKey: ["search-activity", searchId, DAYS],
    queryFn: async () => (await api.searchActivity(searchId, DAYS)).days,
    enabled: enabled && searchId > 0,
    staleTime: 5 * 60_000,
  });
}
