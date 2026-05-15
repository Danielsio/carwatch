import React, { type ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { api } from "@/lib/api";
import { useSearchStats } from "./useSearchStats";

vi.mock("@/lib/api", async (importOriginal) => {
  const mod = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...mod,
    api: {
      ...mod.api,
      searches: {
        ...mod.api.searches,
        stats: vi.fn(),
      },
    },
  };
});

function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });
}

function withProviders(client: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return React.createElement(QueryClientProvider, { client }, children);
  };
}

describe("useSearchStats", () => {
  const mockedSearches = vi.mocked(api.searches);
  let queryClient: QueryClient;

  beforeEach(() => {
    queryClient = createTestQueryClient();
    vi.clearAllMocks();
  });

  afterEach(() => {
    queryClient.clear();
  });

  it("fetches stats for a valid search ID", async () => {
    const mockStats = {
      total: 42,
      new_24h: 5,
      avg_price: 95000,
      min_price: 70000,
      max_price: 130000,
    };
    mockedSearches.stats.mockResolvedValue(mockStats);

    const { result } = renderHook(() => useSearchStats(1), {
      wrapper: withProviders(queryClient),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual(mockStats);
    expect(mockedSearches.stats).toHaveBeenCalledWith(1);
  });

  it("does not fetch for invalid search ID", () => {
    const { result } = renderHook(() => useSearchStats(0), {
      wrapper: withProviders(queryClient),
    });

    expect(result.current.isFetching).toBe(false);
    expect(mockedSearches.stats).not.toHaveBeenCalled();
  });
});
