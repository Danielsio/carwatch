import { useQuery } from "@tanstack/react-query";
import { schedulerApi } from "@/lib/api";

export function useSchedulerStatus(enabled = true) {
  return useQuery({
    queryKey: ["scheduler", "status"],
    queryFn: schedulerApi.status,
    enabled,
    refetchInterval: 30_000,
  });
}
