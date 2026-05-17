import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { NewSearchPage } from "./NewSearchPage";
import { ToastProvider } from "@/components/ui/Toast";

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

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <ToastProvider>
          <NewSearchPage />
        </ToastProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("NewSearchPage presets", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
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
