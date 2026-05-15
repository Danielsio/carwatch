import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";

export function useSearchStats(searchId: number) {
  return useQuery({
    queryKey: ["search-stats", searchId],
    queryFn: () => api.searches.stats(searchId),
    enabled: searchId > 0,
    staleTime: 5 * 60_000,
  });
}
