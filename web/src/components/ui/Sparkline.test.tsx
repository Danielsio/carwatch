import { describe, it, expect } from "vitest";
import { render } from "@testing-library/react";
import { Sparkline } from "./Sparkline";

describe("Sparkline", () => {
  it("renders an svg with a polyline for the data", () => {
    const { container } = render(<Sparkline data={[0, 2, 1, 5, 3]} />);
    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
    expect(container.querySelector("polyline")).toBeInTheDocument();
    expect(container.querySelector("polygon")).toBeInTheDocument();
  });

  it("returns nothing for fewer than two points", () => {
    const { container } = render(<Sparkline data={[3]} />);
    expect(container.querySelector("svg")).not.toBeInTheDocument();
  });

  it("exposes an accessible label when provided", () => {
    const { getByRole } = render(
      <Sparkline data={[1, 2, 3]} ariaLabel="trend" />,
    );
    expect(getByRole("img", { name: "trend" })).toBeInTheDocument();
  });
});
