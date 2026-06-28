import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ScoreBreakdownPopover } from "./ScoreBreakdownPopover";
import type { Listing, ScoreBreakdown } from "@/lib/api";

const baseListing = (): Listing => ({
  token: "t1",
  manufacturer: "Toyota",
  model: "Corolla",
  year: 2019,
  price: 85000,
  km: 60000,
  hand: 1,
  city: "Haifa",
  page_link: "https://example.com/a",
  first_seen_at: "2024-01-15T12:00:00Z",
});

const breakdown: ScoreBreakdown = {
  condition: 0.72,
  value: 0.51,
  engine: 1.0,
};

async function openPopover() {
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "הצג פירוט ציון" }));
}

describe("ScoreBreakdownPopover", () => {
  it("falls back to a plain score box when no breakdown is provided", () => {
    render(<ScoreBreakdownPopover score={8.5} listing={baseListing()} />);
    // No trigger to open a breakdown — just the score.
    expect(
      screen.queryByRole("button", { name: "הצג פירוט ציון" }),
    ).not.toBeInTheDocument();
  });

  it("marks the value dimension as excluded when there is no market data", async () => {
    render(
      <ScoreBreakdownPopover
        score={8.5}
        breakdown={breakdown}
        listing={baseListing()} // no median_price
      />,
    );
    await openPopover();

    // The value row is flagged as not counting toward the score...
    expect(await screen.findByText("לא נכלל בציון")).toBeInTheDocument();
    expect(
      screen.getByText("אין מספיק נתוני שוק להשוואה"),
    ).toBeInTheDocument();

    // ...so its weight chip and a misleading filled bar are suppressed.
    expect(screen.queryByText("35%")).not.toBeInTheDocument();
    expect(screen.queryByText("51%")).not.toBeInTheDocument();

    // Other dimensions are unaffected.
    expect(screen.getByText("60%")).toBeInTheDocument();
  });

  it("shows the value dimension as a weighted contributor when market data exists", async () => {
    const listing: Listing = { ...baseListing(), median_price: 100000 };
    render(
      <ScoreBreakdownPopover
        score={8.5}
        breakdown={breakdown}
        listing={listing}
      />,
    );
    await openPopover();

    // Weight chip and percentage are shown, and the market comparison renders.
    expect(await screen.findByText("35%")).toBeInTheDocument();
    expect(screen.getByText("51%")).toBeInTheDocument();
    expect(screen.getByText(/מתחת לשוק/)).toBeInTheDocument();

    expect(screen.queryByText("לא נכלל בציון")).not.toBeInTheDocument();
  });
});
