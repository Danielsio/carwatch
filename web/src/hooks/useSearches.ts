import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api, type CreateSearchRequest, type Search } from "@/lib/api";

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
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ["searches"] });
      queryClient.invalidateQueries({ queryKey: ["listings", variables.id] });
    },
  });
}

export function useSearches(enabled = true) {
  return useQuery({
    queryKey: ["searches"],
    queryFn: api.searches.list,
    enabled,
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
    onMutate: async (id) => {
      await queryClient.cancelQueries({ queryKey: ["searches"] });
      const previousList = queryClient.getQueryData<Search[]>(["searches"]);
      const previousDetail = queryClient.getQueriesData({
        queryKey: ["searches", id],
      });
      queryClient.setQueryData<Search[]>(["searches"], (old) =>
        old?.map((s) => (s.id === id ? { ...s, active: false } : s)),
      );
      queryClient.setQueryData<Search>(["searches", id], (old) =>
        old ? { ...old, active: false } : old,
      );
      return { previousList, previousDetail };
    },
    onError: (_err, _id, ctx) => {
      if (ctx?.previousList !== undefined) {
        queryClient.setQueryData(["searches"], ctx.previousList);
      }
      if (ctx?.previousDetail) {
        for (const [key, data] of ctx.previousDetail) {
          queryClient.setQueryData(key, data);
        }
      }
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["searches"] }),
  });
}

export function useResumeSearch() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => api.searches.resume(id),
    onMutate: async (id) => {
      await queryClient.cancelQueries({ queryKey: ["searches"] });
      const previousList = queryClient.getQueryData<Search[]>(["searches"]);
      const previousDetail = queryClient.getQueriesData({
        queryKey: ["searches", id],
      });
      queryClient.setQueryData<Search[]>(["searches"], (old) =>
        old?.map((s) => (s.id === id ? { ...s, active: true } : s)),
      );
      queryClient.setQueryData<Search>(["searches", id], (old) =>
        old ? { ...old, active: true } : old,
      );
      return { previousList, previousDetail };
    },
    onError: (_err, _id, ctx) => {
      if (ctx?.previousList !== undefined) {
        queryClient.setQueryData(["searches"], ctx.previousList);
      }
      if (ctx?.previousDetail) {
        for (const [key, data] of ctx.previousDetail) {
          queryClient.setQueryData(key, data);
        }
      }
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["searches"] }),
  });
}
