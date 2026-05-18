import React, { type ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, act } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { api } from "@/lib/api";
import { useMarkListingSeen, useUnmarkListingSeen } from "./useListingSeen";

vi.mock("@/lib/api", async (importOriginal) => {
  const mod = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...mod,
    api: {
      ...mod.api,
      markListingSeen: vi.fn(),
      unmarkListingSeen: vi.fn(),
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

describe("useListingSeen hooks", () => {
  const mockedApi = vi.mocked(api);
  let queryClient: QueryClient;

  beforeEach(() => {
    queryClient = createTestQueryClient();
    vi.clearAllMocks();
  });

  afterEach(() => {
    queryClient.clear();
  });

  it("useMarkListingSeen calls api and invalidates queries", async () => {
    mockedApi.markListingSeen.mockResolvedValue(undefined);
    const spy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useMarkListingSeen(), {
      wrapper: withProviders(queryClient),
    });

    await act(async () => {
      await result.current.mutateAsync("tok-123");
    });

    expect(mockedApi.markListingSeen).toHaveBeenCalledWith("tok-123");
    expect(spy).toHaveBeenCalled();
  });

  it("useUnmarkListingSeen calls api and invalidates queries", async () => {
    mockedApi.unmarkListingSeen.mockResolvedValue(undefined);
    const spy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useUnmarkListingSeen(), {
      wrapper: withProviders(queryClient),
    });

    await act(async () => {
      await result.current.mutateAsync("tok-456");
    });

    expect(mockedApi.unmarkListingSeen).toHaveBeenCalledWith("tok-456");
    expect(spy).toHaveBeenCalled();
  });
});
