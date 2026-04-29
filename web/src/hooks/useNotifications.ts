import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { notificationsApi } from "@/lib/api";

export function useNotificationCount() {
  return useQuery({
    queryKey: ["notification-count"],
    queryFn: () => notificationsApi.count(),
    refetchInterval: 60_000,
  });
}

export function useNotifications(limit: number, offset: number) {
  return useQuery({
    queryKey: ["notifications", limit, offset],
    queryFn: () => notificationsApi.list({ limit, offset }),
  });
}

export function useMarkNotificationsSeen() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => notificationsApi.markSeen(),
    onSuccess: () => {
      queryClient.setQueryData(["notification-count"], { count: 0 });
      queryClient.invalidateQueries({ queryKey: ["notifications"] });
    },
  });
}
