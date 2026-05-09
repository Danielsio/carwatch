import React, { type ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor, act } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { CreateSearchRequest, Search } from "@/lib/api";
import { api } from "@/lib/api";
import { useCreateSearch, useDeleteSearch, useSearches } from "./useSearches";

vi.mock("@/lib/api", async (importOriginal) => {
  const mod = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...mod,
    api: {
      ...mod.api,
      searches: {
        ...mod.api.searches,
        list: vi.fn(),
        create: vi.fn(),
        delete: vi.fn(),
      },
    },
  };
});

function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
}

function withProviders(client: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return React.createElement(QueryClientProvider, { client }, children);
  };
}

describe("useSearches hooks", () => {
  const mockedSearches = vi.mocked(api.searches);

  let queryClient: QueryClient;

  beforeEach(() => {
    queryClient = createTestQueryClient();
    vi.clearAllMocks();
  });

  afterEach(() => {
    queryClient.clear();
  });

  it("useSearches returns list of searches", async () => {
    const searches: Search[] = [
      {
        id: 1,
        name: "My search",
        source: "yad2",
        manufacturer_id: 19,
        manufacturer_name: "Toyota",
        model_id: 10226,
        model_name: "Corolla",
        year_min: 2018,
        year_max: 2022,
        price_max: 150000,
        engine_min_cc: 0,
        max_km: 200000,
        max_hand: 4,
        keywords: "",
        exclude_keys: "",
        active: true,
        created_at: "", 
      },
    ];
    mockedSearches.list.mockResolvedValue(searches);

    const { result } = renderHook(() => useSearches(), {
      wrapper: withProviders(queryClient),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual(searches);
    expect(mockedSearches.list).toHaveBeenCalledTimes(1);
  });

  it("useCreateSearch calls API and invalidates queries", async () => {
    const created: Search = {
      id: 42,
      name: "New",
      source: "yad2",
      manufacturer_id: 8,
      manufacturer_name: "Honda",
      model_id: 10061,
      model_name: "Civic",
      year_min: 2020,
      year_max: 2024,
      price_max: 120000,
      engine_min_cc: 1600,
      max_km: 100000,
      max_hand: 3,
      keywords: "sunroof",
      exclude_keys: "",
      active: true,
      created_at: "2025-03-01T00:00:00Z",
    };
    const payload: CreateSearchRequest = {
      source: "yad2",
      manufacturer: 8,
      model: 10061,
      year_min: 2020,
      year_max: 2024,
      price_max: 120000,
      engine_min_cc: 1600,
      max_km: 100000,
      max_hand: 3,
      keywords: "sunroof",
    };
    mockedSearches.create.mockResolvedValue(created);
    const spy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useCreateSearch(), {
      wrapper: withProviders(queryClient),
    });

    await act(async () => {
      await result.current.mutateAsync(payload);
    });

    expect(mockedSearches.create).toHaveBeenCalledWith(payload);
    expect(spy).toHaveBeenCalledWith({ queryKey: ["searches"] });
  });

  it("useDeleteSearch calls API and invalidates queries", async () => {
    mockedSearches.delete.mockResolvedValue(undefined);
    const spy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useDeleteSearch(), {
      wrapper: withProviders(queryClient),
    });

    await act(async () => {
      await result.current.mutateAsync(99);
    });

    expect(mockedSearches.delete).toHaveBeenCalledWith(99);
    expect(spy).toHaveBeenCalledWith({ queryKey: ["searches"] });
  });
});
