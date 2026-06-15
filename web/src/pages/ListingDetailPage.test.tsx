import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ListingDetailPage } from "./ListingDetailPage";

const { mockListing } = vi.hoisted(() => ({
  mockListing: {
    token: "tok-1",
    manufacturer: "Toyota",
    model: "Corolla",
    sub_model: "Hybrid",
    year: 2021,
    price: 120000,
    km: 50000,
    hand: 2,
    city: "תל אביב",
    page_link: "https://www.yad2.co.il/item/tok-1",
    image_url: "https://img/tok-1.jpg",
    engine_type: "היברידי",
    gear_box: "אוטומטית",
    horse_power: 122,
    fitness_score: 8.4,
    median_price: 130000,
    first_seen_at: "2026-06-10T00:00:00Z",
  },
}));

vi.mock("@/hooks/useListingActions", () => ({
  useListingActions: () => ({
    saved: false,
    seen: false,
    toggleSaved: vi.fn(),
    toggleSeen: vi.fn(),
    isSaving: false,
    isTogglingSeen: false,
  }),
}));

vi.mock("@/components/PriceHistoryChart", () => ({
  PriceHistoryChart: () => null,
}));

vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...actual,
    api: { ...actual.api, listing: vi.fn().mockResolvedValue(mockListing) },
  };
});

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter
        initialEntries={[
          { pathname: "/listings/tok-1", state: { listing: mockListing } },
        ]}
      >
        <Routes>
          <Route path="/listings/:token" element={<ListingDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("ListingDetailPage", () => {
  it("renders the hero heading and price", () => {
    renderPage();
    expect(
      screen.getByRole("heading", { name: /Toyota Corolla/ }),
    ).toBeInTheDocument();
    expect(screen.getAllByText(/120,000/).length).toBeGreaterThan(0);
  });

  it("renders spec tiles with labels", () => {
    renderPage();
    expect(screen.getByText("ק״מ")).toBeInTheDocument();
    expect(screen.getByText("הילוכים")).toBeInTheDocument();
    expect(screen.getByText("דלק")).toBeInTheDocument();
  });

  it("shows the match score summary", () => {
    renderPage();
    expect(screen.getByText("ציון התאמה")).toBeInTheDocument();
  });
});
