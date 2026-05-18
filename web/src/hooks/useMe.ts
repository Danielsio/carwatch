import { useQuery } from "@tanstack/react-query";
import { userApi } from "@/lib/api";

export function useMe(enabled = true) {
  return useQuery({
    queryKey: ["me"],
    queryFn: userApi.me,
    enabled,
    staleTime: 5 * 60 * 1000,
  });
}
