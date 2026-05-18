import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import TrySearchPage from "./TrySearchPage";

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

  it("renders the form with header and submit button", () => {
    renderPage();
    expect(screen.getByText("נסה חיפוש חינם")).toBeInTheDocument();
    expect(screen.getByText("חפש עכשיו")).toBeInTheDocument();
  });

  it("renders manufacturer and model selects", () => {
    renderPage();
    expect(screen.getByLabelText("יצרן")).toBeInTheDocument();
    expect(screen.getByLabelText("דגם")).toBeInTheDocument();
  });

  it("has submit button disabled when manufacturer is not selected", () => {
    renderPage();
    const submitBtn = screen.getByText("חפש עכשיו").closest("button")!;
    expect(submitBtn).toBeDisabled();
  });

  it("model select is disabled until manufacturer is chosen", () => {
    renderPage();
    const modelSelect = screen.getByLabelText("דגם") as HTMLSelectElement;
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
