import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { PageHeader } from "./PageHeader";

function renderWithRouter(ui: React.ReactElement) {
  return render(<MemoryRouter>{ui}</MemoryRouter>);
}

describe("PageHeader", () => {
  it("renders title", () => {
    renderWithRouter(<PageHeader title="My Page" />);
    expect(
      screen.getByRole("heading", { level: 1, name: "My Page" }),
    ).toBeInTheDocument();
  });

  it("renders subtitle when provided", () => {
    renderWithRouter(<PageHeader title="Page" subtitle="A description" />);
    expect(screen.getByText("A description")).toBeInTheDocument();
  });

  it("does not render subtitle when omitted", () => {
    renderWithRouter(<PageHeader title="Page" />);
    expect(screen.queryByText("A description")).not.toBeInTheDocument();
  });

  it("renders back link when backTo is provided", () => {
    renderWithRouter(<PageHeader title="Page" backTo="/dashboard" />);
    const link = screen.getByRole("link", { name: /חזרה/ });
    expect(link).toBeInTheDocument();
    expect(link).toHaveAttribute("href", "/dashboard");
  });

  it("uses custom back label", () => {
    renderWithRouter(
      <PageHeader title="Page" backTo="/home" backLabel="Go back" />,
    );
    expect(screen.getByRole("link", { name: /Go back/ })).toBeInTheDocument();
  });

  it("does not render back link when backTo is omitted", () => {
    renderWithRouter(<PageHeader title="Page" />);
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });

  it("renders action slot", () => {
    renderWithRouter(
      <PageHeader title="Page" action={<button>Action</button>} />,
    );
    expect(screen.getByRole("button", { name: "Action" })).toBeInTheDocument();
  });

  it("renders as header element", () => {
    renderWithRouter(<PageHeader title="Page" />);
    expect(screen.getByRole("banner")).toBeInTheDocument();
  });

  it("applies custom className", () => {
    const { container } = renderWithRouter(
      <PageHeader title="Page" className="my-custom" />,
    );
    expect(container.querySelector("header")).toHaveClass("my-custom");
  });
});
