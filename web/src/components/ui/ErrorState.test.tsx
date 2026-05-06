import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ErrorState } from "./ErrorState";

describe("ErrorState", () => {
  it("renders title", () => {
    render(<ErrorState title="Something went wrong" />);
    expect(screen.getByText("Something went wrong")).toBeInTheDocument();
  });

  it("renders description when provided", () => {
    render(
      <ErrorState title="Error" description="Please try again later" />,
    );
    expect(screen.getByText("Please try again later")).toBeInTheDocument();
  });

  it("does not render description when omitted", () => {
    render(<ErrorState title="Error" />);
    expect(screen.queryByText("Please try again later")).not.toBeInTheDocument();
  });

  it("renders retry button when onRetry is provided", () => {
    const onRetry = vi.fn();
    render(<ErrorState title="Error" onRetry={onRetry} />);
    expect(screen.getByRole("button", { name: "נסה שוב" })).toBeInTheDocument();
  });

  it("calls onRetry when retry button is clicked", async () => {
    const user = userEvent.setup();
    const onRetry = vi.fn();
    render(<ErrorState title="Error" onRetry={onRetry} />);

    await user.click(screen.getByRole("button", { name: "נסה שוב" }));
    expect(onRetry).toHaveBeenCalledOnce();
  });

  it("does not render retry button when onRetry is omitted", () => {
    render(<ErrorState title="Error" />);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("uses custom retry label", () => {
    render(
      <ErrorState title="Error" onRetry={() => {}} retryLabel="Reload" />,
    );
    expect(screen.getByRole("button", { name: "Reload" })).toBeInTheDocument();
  });

  it("renders alert icon", () => {
    const { container } = render(<ErrorState title="Error" />);
    const icon = container.querySelector('[aria-hidden="true"]');
    expect(icon).toBeInTheDocument();
  });

  it("applies custom className", () => {
    const { container } = render(
      <ErrorState title="Error" className="custom-class" />,
    );
    expect(container.firstChild).toHaveClass("custom-class");
  });
});
