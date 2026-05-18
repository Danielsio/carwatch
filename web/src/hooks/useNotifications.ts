import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { notificationsApi } from "@/lib/api";

export function useNotificationCount(enabled = true) {
  return useQuery({
    queryKey: ["notification-count"],
    queryFn: () => notificationsApi.count(),
    enabled,
    refetchInterval: enabled ? 60_000 : false,
    refetchIntervalInBackground: false,
    refetchOnWindowFocus: enabled,
  });
}

export function useNotifications(limit: number, offset: number, enabled = true) {
  return useQuery({
    queryKey: ["notifications", limit, offset],
    queryFn: () => notificationsApi.list({ limit, offset }),
    enabled,
  });
}

export function useMarkNotificationsSeen() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => notificationsApi.markSeen(),
    onSuccess: () => {
      queryClient.setQueryData(["notification-count"], { count: 0 });
      void queryClient.invalidateQueries({ queryKey: ["notifications"] });
      void queryClient.invalidateQueries({ queryKey: ["listings"] });
      void queryClient.invalidateQueries({ queryKey: ["saved"] });
      void queryClient.invalidateQueries({ queryKey: ["history"] });
    },
  });
}
