import { useInfiniteQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";

const PAGE_SIZE = 20;

export function useInfiniteListings(searchId: number, sort: string = "newest") {
  return useInfiniteQuery({
    queryKey: ["listings-infinite", searchId, sort],
    queryFn: ({ pageParam = 0 }) =>
      api.listings(searchId, { sort, limit: PAGE_SIZE, offset: pageParam }),
    initialPageParam: 0,
    getNextPageParam: (lastPage) => {
      const nextOffset = lastPage.offset + lastPage.limit;
      return nextOffset < lastPage.total ? nextOffset : undefined;
    },
    enabled: searchId > 0,
  });
}
