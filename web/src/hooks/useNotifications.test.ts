import React, { type ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor, act } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { notificationsApi } from "@/lib/api";
import {
  useNotificationCount,
  useNotifications,
  useMarkNotificationsSeen,
} from "./useNotifications";

vi.mock("@/lib/api", async (importOriginal) => {
  const mod = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...mod,
    notificationsApi: {
      count: vi.fn(),
      list: vi.fn(),
      markSeen: vi.fn(),
    },
  };
});

function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, refetchInterval: false },
      mutations: { retry: false },
    },
  });
}

function withProviders(client: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return React.createElement(QueryClientProvider, { client }, children);
  };
}

describe("useNotifications hooks", () => {
  const mockedNotifs = vi.mocked(notificationsApi);
  let queryClient: QueryClient;

  beforeEach(() => {
    queryClient = createTestQueryClient();
    vi.clearAllMocks();
  });

  afterEach(() => {
    queryClient.clear();
  });

  it("useNotificationCount returns count", async () => {
    mockedNotifs.count.mockResolvedValue({ count: 5 });

    const { result } = renderHook(() => useNotificationCount(), {
      wrapper: withProviders(queryClient),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual({ count: 5 });
  });

  it("useNotifications returns listings page", async () => {
    const response = { items: [], total: 0, limit: 20, offset: 0 };
    mockedNotifs.list.mockResolvedValue(response);

    const { result } = renderHook(() => useNotifications(20, 0), {
      wrapper: withProviders(queryClient),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual(response);
  });

  it("useMarkNotificationsSeen resets count to 0", async () => {
    mockedNotifs.markSeen.mockResolvedValue(undefined);
    queryClient.setQueryData(["notification-count"], { count: 5 });

    const { result } = renderHook(() => useMarkNotificationsSeen(), {
      wrapper: withProviders(queryClient),
    });

    await act(async () => {
      await result.current.mutateAsync();
    });

    expect(mockedNotifs.markSeen).toHaveBeenCalledTimes(1);
    expect(queryClient.getQueryData(["notification-count"])).toEqual({
      count: 0,
    });
  });
});
