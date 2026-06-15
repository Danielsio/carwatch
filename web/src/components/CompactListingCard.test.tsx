import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { CompactListingCard } from "./CompactListingCard";
import { ToastProvider } from "@/components/ui/Toast";
import type { Listing } from "@/lib/api";

const { toggleSaved, toggleSeen } = vi.hoisted(() => ({
  toggleSaved: vi.fn(),
  toggleSeen: vi.fn(),
}));

vi.mock("@/hooks/useListingActions", () => ({
  useListingActions: () => ({
    saved: false,
    seen: false,
    toggleSaved,
    toggleSeen,
    isSaving: false,
    isTogglingSeen: false,
  }),
}));

const listing: Listing = {
  token: "tok-1",
  manufacturer: "Toyota",
  model: "Corolla",
  year: 2021,
  price: 120000,
  km: 50000,
  hand: 2,
  city: "חיפה",
  page_link: "https://www.yad2.co.il/item/tok-1",
  fitness_score: 8.4,
  first_seen_at: "2026-06-10T00:00:00Z",
};

function renderRow() {
  return render(
    <MemoryRouter>
      <ToastProvider>
        <CompactListingCard listing={listing} />
      </ToastProvider>
    </MemoryRouter>,
  );
}

describe("CompactListingCard", () => {
  it("links to the listing detail with an accessible label", () => {
    renderRow();
    const link = screen.getByRole("link");
    expect(link).toHaveAttribute("href", "/listings/tok-1");
    expect(link).toHaveAccessibleName(/Toyota Corolla 2021/);
  });

  it("shows price and compact meta", () => {
    renderRow();
    expect(screen.getByText(/120,000/)).toBeInTheDocument();
    expect(screen.getByText(/חיפה/)).toBeInTheDocument();
    expect(screen.getByText("8.4")).toBeInTheDocument();
  });

  it("toggles saved without navigating", () => {
    renderRow();
    fireEvent.click(screen.getByRole("button", { name: "שמור מודעה" }));
    expect(toggleSaved).toHaveBeenCalledTimes(1);
  });

  it("toggles seen", () => {
    renderRow();
    fireEvent.click(screen.getByRole("button", { name: "סמן כנצפה" }));
    expect(toggleSeen).toHaveBeenCalledTimes(1);
  });
});
