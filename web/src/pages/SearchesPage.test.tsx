import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { SearchesPage } from "./SearchesPage";
import { ToastProvider } from "@/components/ui/Toast";

const mockSearches = [
  {
    id: 1,
    name: "Toyota Corolla",
    manufacturer: 19,
    manufacturer_name: "Toyota",
    model: 10226,
    model_name: "Corolla",
    source: "yad2",
    year_min: 2020,
    year_max: 2024,
    price_max: 200000,
    active: true,
    listings_count: 5,
    created_at: "2024-01-01T00:00:00Z",
  },
];

let searchesReturn: {
  data: typeof mockSearches | undefined;
  isLoading: boolean;
  isError: boolean;
} = { data: mockSearches, isLoading: false, isError: false };

vi.mock("@/hooks/useSearches", () => ({
  useSearches: () => searchesReturn,
  useDeleteSearch: () => ({ mutate: vi.fn(), isPending: false }),
  usePauseSearch: () => ({ mutate: vi.fn(), isPending: false }),
  useResumeSearch: () => ({ mutate: vi.fn(), isPending: false }),
}));

vi.mock("@/hooks/useNotifications", () => ({
  useNotificationCount: () => ({ data: { count: 3 } }),
  useNotifications: () => ({
    data: { items: [], total: 0 },
  }),
}));

vi.mock("@/hooks/usePageTitle", () => ({
  usePageTitle: vi.fn(),
}));

vi.mock("@/hooks/useSearchStats", () => ({
  useSearchStats: () => ({ data: null }),
}));

vi.mock("@/contexts/AuthContext", () => ({
  useAuth: () => ({
    user: { email: "test@example.com" },
    loading: false,
    signOut: vi.fn(),
  }),
}));

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <ToastProvider>
          <SearchesPage />
        </ToastProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("SearchesPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    searchesReturn = { data: mockSearches, isLoading: false, isError: false };
  });

  it("renders dashboard header and stats with search data", () => {
    renderPage();
    expect(screen.getByText("לוח בקרה")).toBeInTheDocument();
    expect(screen.getByText("חיפושים שמורים")).toBeInTheDocument();
  });

  it("renders the stat cards", () => {
    renderPage();
    // The daily digest section also contains these labels, so use getAllByText.
    expect(screen.getAllByText("חיפושים פעילים").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("מודעות שנמצאו").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("מודעות חדשות")).toBeInTheDocument();
    expect(screen.getByText("סה״כ חיפושים")).toBeInTheDocument();
  });

  it("shows empty state when no searches exist", () => {
    searchesReturn = { data: [], isLoading: false, isError: false };
    renderPage();
    expect(screen.getByText("ברוך הבא ל-CarWatch!")).toBeInTheDocument();
    expect(screen.getByText("צור חיפוש ראשון")).toBeInTheDocument();
  });

  it("renders loading skeletons while loading", () => {
    searchesReturn = { data: undefined, isLoading: true, isError: false };
    const { container } = renderPage();
    const skeletons = container.querySelectorAll('[data-slot="skeleton"], [class*="shimmer"], [class*="skeleton"]');
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it("shows error state on fetch failure", () => {
    searchesReturn = { data: undefined, isLoading: false, isError: true };
    renderPage();
    expect(screen.getByText("שגיאה בטעינת החיפושים")).toBeInTheDocument();
  });
});
