import { describe, it, expect } from "vitest";
import { render } from "@testing-library/react";
import { AuroraBackground } from "./AuroraBackground";

describe("AuroraBackground", () => {
  it("is hidden from assistive tech and non-interactive", () => {
    const { container } = render(<AuroraBackground />);
    const root = container.firstChild as HTMLElement;
    expect(root).toHaveAttribute("aria-hidden");
    expect(root.className).toContain("pointer-events-none");
    expect(root.className).toContain("-z-10");
  });

  it("defaults to the app variant", () => {
    const { container } = render(<AuroraBackground />);
    const root = container.firstChild as HTMLElement;
    expect(root).toHaveAttribute("data-variant", "app");
  });

  it("renders the bolder hero variant when requested", () => {
    const { container } = render(<AuroraBackground variant="hero" />);
    const root = container.firstChild as HTMLElement;
    expect(root).toHaveAttribute("data-variant", "hero");
  });

  it("renders three animated aurora blobs", () => {
    const { container } = render(<AuroraBackground />);
    expect(container.querySelectorAll(".aurora-blob")).toHaveLength(3);
  });

  it("merges a custom className", () => {
    const { container } = render(<AuroraBackground className="opacity-0" />);
    expect(container.firstChild).toHaveClass("opacity-0");
  });
});
