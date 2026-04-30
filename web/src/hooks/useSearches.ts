import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api, type CreateSearchRequest } from "@/lib/api";

export function useSearch(id: number) {
  return useQuery({
    queryKey: ["searches", id],
    queryFn: () => api.searches.get(id),
    enabled: id > 0,
  });
}

export function useUpdateSearch() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: number; data: Partial<CreateSearchRequest> }) =>
      api.searches.update(id, data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["searches"] }),
  });
}

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
