import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Routes, Route } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { NewSearchPage } from "./NewSearchPage";
import { ToastProvider } from "@/components/ui/Toast";

let authState: { user: { email: string } | null; loading: boolean };

vi.mock("@/contexts/AuthContext", () => ({
  useAuth: () => authState,
}));

vi.mock("@/hooks/useCatalog", () => ({
  useManufacturers: () => ({ data: [] }),
  useModels: () => ({ data: [] }),
}));

vi.mock("@/hooks/useSearches", () => ({
  useCreateSearch: () => ({
    mutate: vi.fn(),
    isPending: false,
  }),
}));

function renderPage(initialEntries = ["/searches/new"]) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={initialEntries}>
        <ToastProvider>
          <Routes>
            <Route path="/searches/new" element={<NewSearchPage />} />
            <Route path="/try" element={<div>Try Search Page</div>} />
          </Routes>
        </ToastProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("NewSearchPage presets", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    authState = { user: { email: "test@example.com" }, loading: false };
  });

  it("renders preset cards in initial state", () => {
    renderPage();
    expect(screen.getByText("רכב משפחתי")).toBeInTheDocument();
    expect(screen.getByText("רכב ראשון")).toBeInTheDocument();
    expect(screen.getByText("SUV")).toBeInTheDocument();
    expect(screen.getByText("היברידי / חשמלי")).toBeInTheDocument();
  });

  it("hides presets after clicking one", async () => {
    const user = userEvent.setup();
    renderPage();
    const presetBtn = screen.getByText("רכב ראשון").closest("button")!;
    await user.click(presetBtn);
    expect(screen.queryByText("התחל מתבנית")).not.toBeInTheDocument();
  });

  it("fills priceMax when 'רכב ראשון' preset is clicked", async () => {
    const user = userEvent.setup();
    renderPage();
    const presetBtn = screen.getByText("רכב ראשון").closest("button")!;
    await user.click(presetBtn);
    const nextBtn = screen.getByText("הבא");
    await user.click(nextBtn);
    const priceMaxInput = screen.getByLabelText(/מחיר מקסימום/) as HTMLInputElement;
    expect(priceMaxInput.value).toBe("80000");
  });

  it("fills yearMin when 'SUV' preset is clicked", async () => {
    const user = userEvent.setup();
    renderPage();
    const presetBtn = screen.getByText("SUV").closest("button")!;
    await user.click(presetBtn);
    const nextBtn = screen.getByText("הבא");
    await user.click(nextBtn);
    const yearMinInput = screen.getByLabelText(/שנה מ-/) as HTMLInputElement;
    const expectedYear = new Date().getFullYear() - 5;
    expect(yearMinInput.value).toBe(String(expectedYear));
  });
});

describe("NewSearchPage guest redirect", () => {
  it("redirects unauthenticated guests to /try", () => {
    authState = { user: null, loading: false };
    renderPage();
    expect(screen.getByText("Try Search Page")).toBeInTheDocument();
  });
});
