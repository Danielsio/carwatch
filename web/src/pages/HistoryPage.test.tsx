import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { HistoryPage } from "./HistoryPage";

const mockListing = {
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
  seen: true,
};

let historyReturn: {
  data: { items: typeof mockListing[]; total: number } | undefined;
  isLoading: boolean;
  isError: boolean;
} = {
  data: { items: [mockListing], total: 1 },
  isLoading: false,
  isError: false,
};

vi.mock("@/hooks/useBookmarks", () => ({
  useHistory: () => historyReturn,
}));

vi.mock("@/hooks/usePageTitle", () => ({
  usePageTitle: vi.fn(),
}));

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <HistoryPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("HistoryPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    historyReturn = {
      data: { items: [mockListing], total: 1 },
      isLoading: false,
      isError: false,
    };
  });

  it("renders page header", () => {
    renderPage();
    expect(screen.getByText("היסטוריה")).toBeInTheDocument();
  });

  it("renders empty state when no history", () => {
    historyReturn = {
      data: { items: [], total: 0 },
      isLoading: false,
      isError: false,
    };
    renderPage();
    expect(screen.getByText("אין מודעות בהיסטוריה")).toBeInTheDocument();
    expect(screen.getByText("חזרה ללוח הבקרה")).toBeInTheDocument();
  });

  it("renders history listings with total count", () => {
    renderPage();
    expect(screen.getByText("(1 מודעות)")).toBeInTheDocument();
  });

  it("shows loading skeletons while loading", () => {
    historyReturn = { data: undefined, isLoading: true, isError: false };
    const { container } = renderPage();
    const skeletons = container.querySelectorAll(
      '[class*="shimmer"], [class*="skeleton"], [class*="Skeleton"]',
    );
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it("shows error state on fetch failure", () => {
    historyReturn = { data: undefined, isLoading: false, isError: true };
    renderPage();
    expect(screen.getByText("שגיאה בטעינת ההיסטוריה")).toBeInTheDocument();
  });
});
