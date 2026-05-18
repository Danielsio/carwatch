import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { MatchScoreBox } from "./MatchScoreBox";

describe("MatchScoreBox", () => {
  it("renders formatted score", () => {
    render(<MatchScoreBox score={8.5} />);
    expect(screen.getByText("8.5")).toBeInTheDocument();
  });

  it("clamps score above 10 to 10.0", () => {
    render(<MatchScoreBox score={15} />);
    expect(screen.getByText("10.0")).toBeInTheDocument();
  });

  it("clamps negative score to 0.0", () => {
    render(<MatchScoreBox score={-3} />);
    expect(screen.getByText("0.0")).toBeInTheDocument();
  });

  it("handles NaN as 0.0", () => {
    render(<MatchScoreBox score={NaN} />);
    expect(screen.getByText("0.0")).toBeInTheDocument();
  });

  it("renders /10 denominator", () => {
    render(<MatchScoreBox score={7} />);
    expect(screen.getByText("/10")).toBeInTheDocument();
  });

  it("has correct aria-label", () => {
    render(<MatchScoreBox score={9.2} />);
    expect(
      screen.getByLabelText("ציון 9.2 מתוך 10"),
    ).toBeInTheDocument();
  });

  it("applies sm size class", () => {
    const { container } = render(<MatchScoreBox score={5} size="sm" />);
    expect(container.firstElementChild?.classList.contains("h-9")).toBe(true);
  });

  it("applies lg size class", () => {
    const { container } = render(<MatchScoreBox score={5} size="lg" />);
    expect(container.firstElementChild?.classList.contains("h-14")).toBe(true);
  });
});
