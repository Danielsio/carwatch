import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { PageShell } from "./PageShell";

describe("PageShell", () => {
  it("renders children", () => {
    render(
      <PageShell>
        <p>Content</p>
      </PageShell>,
    );
    expect(screen.getByText("Content")).toBeInTheDocument();
  });

  it("applies default medium gap", () => {
    const { container } = render(
      <PageShell>
        <p>Content</p>
      </PageShell>,
    );
    expect(container.firstChild).toHaveClass("space-y-6");
  });

  it("applies small gap variant", () => {
    const { container } = render(
      <PageShell gap="sm">
        <p>Content</p>
      </PageShell>,
    );
    expect(container.firstChild).toHaveClass("space-y-4");
  });

  it("applies large gap variant", () => {
    const { container } = render(
      <PageShell gap="lg">
        <p>Content</p>
      </PageShell>,
    );
    expect(container.firstChild).toHaveClass("space-y-8");
  });

  it("merges custom className", () => {
    const { container } = render(
      <PageShell className="custom-class">
        <p>Content</p>
      </PageShell>,
    );
    expect(container.firstChild).toHaveClass("custom-class");
    expect(container.firstChild).toHaveClass("space-y-6");
  });

  it("renders multiple children", () => {
    render(
      <PageShell>
        <p>First</p>
        <p>Second</p>
        <p>Third</p>
      </PageShell>,
    );
    expect(screen.getByText("First")).toBeInTheDocument();
    expect(screen.getByText("Second")).toBeInTheDocument();
    expect(screen.getByText("Third")).toBeInTheDocument();
  });
});
