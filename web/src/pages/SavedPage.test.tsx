import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { SavedPage } from "./SavedPage";
import { ToastProvider } from "@/components/ui/Toast";

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
  saved: true,
  seen: false,
};

const mockRemoveMutate = vi.fn();

let savedReturn: {
  data: { items: typeof mockListing[]; total: number } | undefined;
  isLoading: boolean;
  isError: boolean;
} = {
  data: { items: [mockListing], total: 1 },
  isLoading: false,
  isError: false,
};

vi.mock("@/hooks/useBookmarks", () => ({
  useSavedListings: () => savedReturn,
  useRemoveBookmark: () => ({ mutate: mockRemoveMutate }),
  useSaveBookmark: () => ({ mutate: vi.fn() }),
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
        <ToastProvider>
          <SavedPage />
        </ToastProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("SavedPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    savedReturn = {
      data: { items: [mockListing], total: 1 },
      isLoading: false,
      isError: false,
    };
  });

  it("renders page header", () => {
    renderPage();
    expect(screen.getByText("שמורים")).toBeInTheDocument();
  });

  it("renders empty state when no saved listings", () => {
    savedReturn = {
      data: { items: [], total: 0 },
      isLoading: false,
      isError: false,
    };
    renderPage();
    expect(screen.getByText("אין מודעות שמורות")).toBeInTheDocument();
    expect(screen.getByText("חזרה ללוח הבקרה")).toBeInTheDocument();
  });

  it("renders saved listings with total count", () => {
    renderPage();
    expect(screen.getByText("(1)")).toBeInTheDocument();
  });

  it("shows loading skeletons while loading", () => {
    savedReturn = { data: undefined, isLoading: true, isError: false };
    const { container } = renderPage();
    const skeletons = container.querySelectorAll(
      '[class*="shimmer"], [class*="skeleton"], [class*="Skeleton"]',
    );
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it("shows error state on fetch failure", () => {
    savedReturn = { data: undefined, isLoading: false, isError: true };
    renderPage();
    expect(screen.getByText("שגיאה בטעינת המודעות")).toBeInTheDocument();
  });

  it("calls removeBookmark when remove button is clicked", async () => {
    const user = userEvent.setup();
    renderPage();
    const removeBtn = screen.getByText("הסר");
    await user.click(removeBtn);
    expect(mockRemoveMutate).toHaveBeenCalledWith(
      "tok1",
      expect.objectContaining({ onSuccess: expect.any(Function) }),
    );
  });
});
