import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";

export function useListings(
  searchId: number,
  sort: string = "newest",
  limit: number = 20,
  offset: number = 0,
) {
  return useQuery({
    queryKey: ["listings", searchId, { sort, limit, offset }],
    queryFn: () => api.listings(searchId, { sort, limit, offset }),
    enabled: searchId > 0,
  });
}
