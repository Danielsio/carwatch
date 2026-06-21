import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ListingCardBody } from "./ListingCardBody";
import type { Listing } from "@/lib/api";

const baseListing = (): Listing => ({
  token: "t1",
  manufacturer: "Mazda",
  model: "3",
  year: 2021,
  price: 90000,
  km: 40000,
  hand: 2,
  city: "Tel Aviv",
  page_link: "https://example.com/a",
  first_seen_at: "2024-01-15T12:00:00Z",
});

describe("ListingCardBody", () => {
  it("renders description snippet when present", () => {
    const listing = {
      ...baseListing(),
      description: "מכונית במצב מצוין, טסט עד 2026.",
    };
    render(<ListingCardBody listing={listing} />);
    expect(
      screen.getByText(/מכונית במצב מצוין/),
    ).toBeInTheDocument();
  });

  it("toggles long description with expand control", async () => {
    const user = userEvent.setup();
    const longText = "א".repeat(200);
    const listing = { ...baseListing(), description: longText };
    render(<ListingCardBody listing={listing} />);
    const toggle = screen.getByRole("button", { name: "עוד" });
    await user.click(toggle);
    expect(screen.getByRole("button", { name: "הצג פחות" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "הצג פחות" }));
    expect(screen.getByRole("button", { name: "עוד" })).toBeInTheDocument();
  });

  it("omits description block when empty", () => {
    render(<ListingCardBody listing={baseListing()} />);
    expect(screen.queryByRole("button", { name: "עוד" })).not.toBeInTheDocument();
  });

  it("shows the source post date, not the fetch date, when posted_at is present", () => {
    const day = 24 * 60 * 60 * 1000;
    const now = Date.now();
    const listing: Listing = {
      ...baseListing(),
      // Fetched just now, but posted ~35 days ago on the source.
      first_seen_at: new Date(now).toISOString(),
      posted_at: new Date(now - 35 * day).toISOString(),
    };
    render(<ListingCardBody listing={listing} />);
    expect(screen.getByText("לפני חודש")).toBeInTheDocument();
    expect(screen.queryByText("היום")).not.toBeInTheDocument();
  });

  it("falls back to first_seen_at when posted_at is absent", () => {
    const day = 24 * 60 * 60 * 1000;
    const now = Date.now();
    const listing: Listing = {
      ...baseListing(),
      first_seen_at: new Date(now - 35 * day).toISOString(),
      posted_at: undefined,
    };
    render(<ListingCardBody listing={listing} />);
    expect(screen.getByText("לפני חודש")).toBeInTheDocument();
  });
});
