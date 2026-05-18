import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ListingsPage } from "./ListingsPage";
import { ToastProvider } from "@/components/ui/Toast";

// Polyfill IntersectionObserver for jsdom
class MockIntersectionObserver {
  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();
  constructor() {}
}
vi.stubGlobal("IntersectionObserver", MockIntersectionObserver);

const mockListings = [
  {
    token: "tok1",
    manufacturer: "Toyota",
    model: "Corolla",
    year: 2021,
    price: 120000,
    km: 50000,
    hand: 2,
    city: "Tel Aviv",
    page_link: "https://yad2.co.il/item/tok1",
    image_url: "https://img.yad2.co.il/tok1.jpg",
    first_seen_at: "2024-01-01T00:00:00Z",
    saved: false,
    seen: false,
  },
];

let infiniteReturn: {
  data: { pages: Array<{ items: typeof mockListings; total: number; offset: number; limit: number }> } | undefined;
  isLoading: boolean;
  isError: boolean;
  fetchNextPage: () => void;
  hasNextPage: boolean;
  isFetchingNextPage: boolean;
} = {
  data: { pages: [{ items: mockListings, total: 1, offset: 0, limit: 20 }] },
  isLoading: false,
  isError: false,
  fetchNextPage: vi.fn(),
  hasNextPage: false,
  isFetchingNextPage: false,
};

vi.mock("@/hooks/useInfiniteListings", () => ({
  useInfiniteListings: () => infiniteReturn,
}));

vi.mock("@/hooks/useBookmarks", () => ({
  useSaveBookmark: () => ({ mutate: vi.fn() }),
  useRemoveBookmark: () => ({ mutate: vi.fn() }),
}));

vi.mock("@/hooks/useListingSeen", () => ({
  useMarkListingSeen: () => ({ mutate: vi.fn() }),
  useUnmarkListingSeen: () => ({ mutate: vi.fn() }),
}));

vi.mock("@/lib/api", () => ({
  api: {
    refreshListings: vi.fn().mockResolvedValue({ items: [] }),
  },
}));

function renderPage(searchId: string = "1") {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[`/listings/${searchId}`]}>
        <ToastProvider>
          <Routes>
            <Route path="/listings/:id" element={<ListingsPage />} />
          </Routes>
        </ToastProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("ListingsPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    infiniteReturn = {
      data: { pages: [{ items: mockListings, total: 1, offset: 0, limit: 20 }] },
      isLoading: false,
      isError: false,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
    };
  });

  it("renders listings with page header", () => {
    renderPage();
    expect(screen.getByText("תוצאות")).toBeInTheDocument();
  });

  it("renders sort buttons", () => {
    renderPage();
    expect(screen.getByText("חדשים")).toBeInTheDocument();
    expect(screen.getByText("מחיר ↑")).toBeInTheDocument();
    expect(screen.getByText("ציון")).toBeInTheDocument();
  });

  it("shows listing count in subtitle", () => {
    renderPage();
    expect(screen.getByText("1 מודעות")).toBeInTheDocument();
  });

  it("shows empty state when no listings", () => {
    infiniteReturn = {
      ...infiniteReturn,
      data: { pages: [{ items: [], total: 0, offset: 0, limit: 20 }] },
    };
    renderPage();
    expect(screen.getByText("אין תוצאות עדיין")).toBeInTheDocument();
  });

  it("shows error state on fetch failure", () => {
    infiniteReturn = {
      ...infiniteReturn,
      data: undefined,
      isError: true,
    };
    renderPage();
    expect(screen.getByText("שגיאה בטעינת התוצאות")).toBeInTheDocument();
  });
});
