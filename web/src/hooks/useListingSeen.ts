import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";

function invalidateListingSeen(qc: ReturnType<typeof useQueryClient>, token: string) {
  void qc.invalidateQueries({ queryKey: ["notifications"] });
  void qc.invalidateQueries({ queryKey: ["notification-count"] });
  void qc.invalidateQueries({ queryKey: ["listings"] });
  void qc.invalidateQueries({ queryKey: ["saved"] });
  void qc.invalidateQueries({ queryKey: ["history"] });
  void qc.invalidateQueries({ queryKey: ["listing", token] });
}

export function useMarkListingSeen() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (token: string) => api.markListingSeen(token),
    onSuccess: (_data, token) => {
      invalidateListingSeen(qc, token);
    },
  });
}

export function useUnmarkListingSeen() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (token: string) => api.unmarkListingSeen(token),
    onSuccess: (_data, token) => {
      invalidateListingSeen(qc, token);
    },
  });
}
