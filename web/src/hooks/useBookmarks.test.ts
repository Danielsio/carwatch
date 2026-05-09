import React, { type ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor, act } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Listing, ListingsResponse } from "@/lib/api";
import { api } from "@/lib/api";
import {
  useHistory,
  useRemoveBookmark,
  useSavedListings,
  useSaveBookmark,
} from "./useBookmarks";

vi.mock("@/lib/api", async (importOriginal) => {
  const mod = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...mod,
    api: {
      ...mod.api,
      saved: {
        list: vi.fn(),
        save: vi.fn(),
        remove: vi.fn(),
      },
      history: vi.fn(),
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

describe("useBookmarks hooks", () => {
  const mockedSaved = vi.mocked(api.saved);
  const mockedHistory = vi.mocked(api.history);

  let queryClient: QueryClient;

  beforeEach(() => {
    queryClient = createTestQueryClient();
    vi.clearAllMocks();
  });

  afterEach(() => {
    queryClient.clear();
  });

  it("useSavedListings returns data on success", async () => {
    const listing: Listing = {
      token: "tok-1",
      manufacturer: "Toyota",
      model: "Corolla",
      year: 2020,
      price: 100000,
      km: 50000,
      hand: 1,
      city: "Tel Aviv",
      page_link: "https://example.com/l/1",
      first_seen_at: "2025-01-01T00:00:00Z",
    };
    const body: ListingsResponse = {
      items: [listing],
      total: 1,
      limit: 20,
      offset: 0,
    };
    mockedSaved.list.mockResolvedValue(body);

    const { result } = renderHook(() => useSavedListings(20, 0), {
      wrapper: withProviders(queryClient),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual(body);
    expect(mockedSaved.list).toHaveBeenCalledWith({ limit: 20, offset: 0 });
  });

  it("useSaveBookmark calls the correct API and invalidates queries", async () => {
    mockedSaved.save.mockResolvedValue(undefined);
    const spy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useSaveBookmark(), {
      wrapper: withProviders(queryClient),
    });

    await act(async () => {
      await result.current.mutateAsync("my-token");
    });

    expect(mockedSaved.save).toHaveBeenCalledWith("my-token");
    expect(spy.mock.calls.map((c) => c[0])).toEqual(
      expect.arrayContaining([
        { queryKey: ["saved"] },
        { queryKey: ["listings"] },
        { queryKey: ["history"] },
        { queryKey: ["notifications"] },
      ]),
    );
    expect(spy).toHaveBeenCalledTimes(4);
  });

  it("useRemoveBookmark calls the correct API and invalidates queries", async () => {
    mockedSaved.remove.mockResolvedValue(undefined);
    const spy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useRemoveBookmark(), {
      wrapper: withProviders(queryClient),
    });

    await act(async () => {
      await result.current.mutateAsync("rm-token");
    });

    expect(mockedSaved.remove).toHaveBeenCalledWith("rm-token");
    expect(spy.mock.calls.map((c) => c[0])).toEqual(
      expect.arrayContaining([
        { queryKey: ["saved"] },
        { queryKey: ["listings"] },
        { queryKey: ["history"] },
        { queryKey: ["notifications"] },
      ]),
    );
    expect(spy).toHaveBeenCalledTimes(4);
  });

  it("useHistory returns data on success", async () => {
    const listing: Listing = {
      token: "hist-1",
      manufacturer: "Honda",
      model: "Civic",
      year: 2019,
      price: 90000,
      km: 40000,
      hand: 2,
      city: "Haifa",
      page_link: "https://example.com/l/hist-1",
      first_seen_at: "2025-02-01T00:00:00Z",
    };
    const body: ListingsResponse = {
      items: [listing],
      total: 1,
      limit: 10,
      offset: 5,
    };
    mockedHistory.mockResolvedValue(body);

    const { result } = renderHook(() => useHistory(10, 5), {
      wrapper: withProviders(queryClient),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual(body);
    expect(mockedHistory).toHaveBeenCalledWith({ limit: 10, offset: 5 });
  });
});
