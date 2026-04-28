import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api, type CreateSearchRequest } from "@/lib/api";

export function useSearches() {
  return useQuery({
    queryKey: ["searches"],
    queryFn: api.searches.list,
  });
}

export function useCreateSearch() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateSearchRequest) => api.searches.create(data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["searches"] }),
  });
}

export function useDeleteSearch() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => api.searches.delete(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["searches"] }),
  });
}

export function usePauseSearch() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => api.searches.pause(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["searches"] }),
  });
}

export function useResumeSearch() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => api.searches.resume(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["searches"] }),
  });
}
