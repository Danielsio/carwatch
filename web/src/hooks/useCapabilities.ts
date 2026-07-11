import { useQuery } from "@tanstack/react-query";

import { guestApi, type Capabilities } from "@/lib/api";

/**
 * Which optional features this deployment can perform. Used to hide entry
 * points that would only ever error — see the Capabilities docs in lib/api.
 *
 * While the answer is unknown (loading, or the request failed) `live_search` is
 * false, so callers never offer a feature we have not confirmed. Callers that
 * would flash a hidden-then-shown form should branch on `isLoading` first.
 */
export function useCapabilities(): Capabilities & { isLoading: boolean } {
  const { data, isPending } = useQuery({
    queryKey: ["capabilities"],
    queryFn: () => guestApi.capabilities(),
    staleTime: Infinity,
    retry: false,
  });

  return {
    live_search: data?.live_search ?? false,
    isLoading: isPending,
  };
}
