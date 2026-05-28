import { renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { useLogStream } from "./useLogStream";
import { adminApi } from "@/lib/api";

vi.mock("@/lib/api", () => ({
  BASE_URL: "/api/v1",
  adminApi: {
    logs: vi.fn(async () => ({ items: [] })),
  },
}));

vi.mock("@/lib/auth-token", () => ({
  getAuthToken: vi.fn(async () => null),
}));

describe("useLogStream", () => {
  it("bootstraps with the maximum log limit", async () => {
    const { unmount } = renderHook(() => useLogStream(true));

    await waitFor(() => {
      expect(adminApi.logs).toHaveBeenCalledWith(10000);
    });

    unmount();
  });
});
