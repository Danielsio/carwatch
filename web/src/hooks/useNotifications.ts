import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { notificationsApi } from "@/lib/api";

export function useNotificationCount() {
  return useQuery({
    queryKey: ["notifications", "count"],
    queryFn: () => notificationsApi.count(),
    refetchInterval: 60_000,
  });
}

export function useNotifications(limit = 20, offset = 0) {
  return useQuery({
    queryKey: ["notifications", { limit, offset }],
    queryFn: () => notificationsApi.list({ limit, offset }),
  });
}

export function useMarkNotificationsSeen() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => notificationsApi.markSeen(),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["notifications"] }),
  });
}
