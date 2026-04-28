import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";

export function useManufacturers(query?: string) {
  return useQuery({
    queryKey: ["manufacturers", query],
    queryFn: () => api.catalog.manufacturers(query),
    staleTime: 5 * 60_000,
  });
}

export function useModels(manufacturerId: number, query?: string) {
  return useQuery({
    queryKey: ["models", manufacturerId, query],
    queryFn: () => api.catalog.models(manufacturerId, query),
    enabled: manufacturerId > 0,
    staleTime: 5 * 60_000,
  });
}
