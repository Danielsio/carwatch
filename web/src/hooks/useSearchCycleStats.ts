import { useQuery } from "@tanstack/react-query";
import { cycleStatsApi } from "@/lib/api";

export function useSearchCycleStats(enabled = true) {
  return useQuery({
    queryKey: ["searches", "cycle-stats"],
    queryFn: async () => {
      const res = await cycleStatsApi.list();
      return res.items;
    },
    enabled,
    refetchInterval: 30_000,
  });
}
