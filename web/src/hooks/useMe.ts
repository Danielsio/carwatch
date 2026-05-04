import { useQuery } from "@tanstack/react-query";
import { userApi } from "@/lib/api";

export function useMe() {
  return useQuery({
    queryKey: ["me"],
    queryFn: userApi.me,
    staleTime: 5 * 60 * 1000,
  });
}
