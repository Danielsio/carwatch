import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import TrySearchPage from "./TrySearchPage";
import { guestApi } from "@/lib/api";

vi.mock("@/lib/api", () => ({
  guestApi: {
    catalog: {
      manufacturers: vi.fn().mockResolvedValue([
        { id: 19, name: "Toyota", name_he: "טויוטה" },
        { id: 27, name: "Mazda", name_he: "מאזדה" },
      ]),
      models: vi.fn().mockResolvedValue([
        { id: 10226, name: "Corolla", name_he: "קורולה" },
      ]),
    },
    instantSearch: vi.fn().mockResolvedValue({ items: [], total: 0 }),
    capabilities: vi.fn().mockResolvedValue({ live_search: true }),
  },
  ApiError: class ApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));

function renderPage() {
  // Clear sessionStorage to get clean initial state
  sessionStorage.removeItem("carwatch_try_search_form");
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <TrySearchPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("TrySearchPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    sessionStorage.clear();
  });

  it("renders the form with header and submit button", async () => {
    renderPage();
    expect(screen.getByText("נסה חיפוש חינם")).toBeInTheDocument();
    expect(await screen.findByText("חפש עכשיו")).toBeInTheDocument();
  });

  // Instant search needs a live server-side fetch of Yad2, which is blocked in
  // production. Offering the form there just produces a 503, so the page must
  // explain and route the user to the flow that works (sign up + extension).
  it("hides the search form and explains when live search is unavailable", async () => {
    vi.mocked(guestApi.capabilities).mockResolvedValueOnce({
      live_search: false,
    });
    renderPage();

    await waitFor(() => {
      expect(screen.getByText("חיפוש מיידי אינו זמין כרגע")).toBeInTheDocument();
    });
    expect(screen.queryByText("חפש עכשיו")).not.toBeInTheDocument();
    expect(screen.getByText("הירשם והתקן את התוסף")).toBeInTheDocument();
    expect(guestApi.instantSearch).not.toHaveBeenCalled();
  });

  it("renders manufacturer and model selects", async () => {
    renderPage();
    expect(await screen.findByLabelText("יצרן")).toBeInTheDocument();
    expect(screen.getByLabelText("דגם")).toBeInTheDocument();
  });

  it("has submit button disabled when manufacturer is not selected", async () => {
    renderPage();
    const submitBtn = (await screen.findByText("חפש עכשיו")).closest("button")!;
    expect(submitBtn).toBeDisabled();
  });

  it("model select is disabled until manufacturer is chosen", async () => {
    renderPage();
    const modelSelect = (await screen.findByLabelText(
      "דגם",
    )) as HTMLSelectElement;
    expect(modelSelect).toBeDisabled();
  });

  it("enables model select after manufacturer is chosen", async () => {
    const user = userEvent.setup();
    renderPage();

    // Wait for manufacturers to load from the async query
    await waitFor(() => {
      const options = screen.getByLabelText("יצרן").querySelectorAll("option");
      expect(options.length).toBeGreaterThan(1);
    });

    const mfrSelect = screen.getByLabelText("יצרן") as HTMLSelectElement;
    await user.selectOptions(mfrSelect, "19");

    const modelSelect = screen.getByLabelText("דגם") as HTMLSelectElement;
    expect(modelSelect).not.toBeDisabled();
  });

  it("enables submit button after manufacturer is selected", async () => {
    const user = userEvent.setup();
    renderPage();

    // Wait for manufacturers to load from the async query
    await waitFor(() => {
      const options = screen.getByLabelText("יצרן").querySelectorAll("option");
      expect(options.length).toBeGreaterThan(1);
    });

    const mfrSelect = screen.getByLabelText("יצרן") as HTMLSelectElement;
    await user.selectOptions(mfrSelect, "19");

    const submitBtn = screen.getByText("חפש עכשיו").closest("button")!;
    expect(submitBtn).not.toBeDisabled();
  });
});
