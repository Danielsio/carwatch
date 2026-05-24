import { describe, it, expect, vi, beforeEach } from "vitest";
import { render } from "@testing-library/react";

vi.mock("@/lib/api", () => ({
  adminApi: {
    activity: vi.fn(),
  },
}));

import { adminApi } from "@/lib/api";
import { ActivityChart } from "./ActivityChart";

describe("ActivityChart", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("does not update state after unmount", async () => {
    let resolve: (v: { items: { date: string; new_listings: number; price_drops: number; new_users: number }[] }) => void;
    vi.mocked(adminApi.activity).mockReturnValue(
      new Promise((r) => { resolve = r; }) as ReturnType<typeof adminApi.activity>,
    );

    const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    try {
      const { unmount } = render(<ActivityChart />);
      unmount();

      resolve!({ items: [{ date: "2024-01-01", new_listings: 5, price_drops: 1, new_users: 0 }] });
      await new Promise((r) => setTimeout(r, 10));

      const reactWarnings = consoleSpy.mock.calls.filter(
        (call) => String(call[0]).includes("unmounted") || String(call[0]).includes("Can't perform"),
      );
      expect(reactWarnings).toHaveLength(0);
    } finally {
      consoleSpy.mockRestore();
    }
  });

  it("renders loading state initially", () => {
    vi.mocked(adminApi.activity).mockReturnValue(
      new Promise(() => {}) as ReturnType<typeof adminApi.activity>,
    );
    const { getByText } = render(<ActivityChart />);
    expect(getByText("טוען נתוני פעילות...")).toBeInTheDocument();
  });

  it("renders error state on API failure", async () => {
    vi.mocked(adminApi.activity).mockRejectedValue(new Error("fail"));

    const { findByText } = render(<ActivityChart />);
    expect(await findByText("שגיאה בטעינת נתוני פעילות")).toBeInTheDocument();
  });

  it("renders empty state when no data", async () => {
    vi.mocked(adminApi.activity).mockResolvedValue({ items: [] });

    const { findByText } = render(<ActivityChart />);
    expect(await findByText("אין נתוני פעילות")).toBeInTheDocument();
  });
});
