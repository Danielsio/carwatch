import { useQuery } from "@tanstack/react-query";
import { adminApi } from "@/lib/api";

export function useAdminStats() {
  return useQuery({
    queryKey: ["admin", "stats"],
    queryFn: adminApi.stats,
    refetchInterval: 30_000,
  });
}
