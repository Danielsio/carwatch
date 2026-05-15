import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Component, type ReactNode } from "react";
import { ErrorBoundary } from "./ErrorBoundary";

// Suppress React error boundary console.error noise in test output
beforeEach(() => {
  vi.spyOn(console, "error").mockImplementation(() => {});
});

/**
 * Class component that always throws. Using a class avoids React 19's
 * dev-mode concurrent recovery behavior that replays function components.
 */
class AlwaysThrows extends Component {
  render(): ReactNode {
    throw new Error("class boom");
  }
}

/**
 * Class component that throws on the first render only.
 * After getDerivedStateFromError resets, the boundary unmounts/remounts children,
 * creating a new instance that starts with renderCount=0. We use a static flag
 * that is flipped externally between renders.
 */
let shouldThrowFlag = true;

class ThrowOnceClass extends Component {
  render(): ReactNode {
    if (shouldThrowFlag) {
      throw new Error("first render boom");
    }
    return <div>recovered content</div>;
  }
}

describe("ErrorBoundary", () => {
  it("renders children when no error occurs", () => {
    render(
      <ErrorBoundary>
        <div>child content</div>
      </ErrorBoundary>,
    );

    expect(screen.getByText("child content")).toBeInTheDocument();
  });

  it("shows error UI when child throws", () => {
    render(
      <ErrorBoundary>
        <AlwaysThrows />
      </ErrorBoundary>,
    );

    // Hebrew error heading
    expect(screen.getByText("משהו השתבש")).toBeInTheDocument();
    // Retry button
    expect(screen.getByRole("button", { name: /נסה שוב/ })).toBeInTheDocument();
    // Home link
    expect(screen.getByRole("link", { name: /דף הבית/ })).toBeInTheDocument();
  });

  it("retries and re-renders children after clicking retry", async () => {
    shouldThrowFlag = true;
    const user = userEvent.setup();

    render(
      <ErrorBoundary>
        <ThrowOnceClass />
      </ErrorBoundary>,
    );

    // Error boundary catches the throw
    expect(screen.getByText("משהו השתבש")).toBeInTheDocument();

    // Flip flag so next render succeeds
    shouldThrowFlag = false;

    // Click retry resets the boundary, re-rendering children
    await user.click(screen.getByRole("button", { name: /נסה שוב/ }));

    expect(screen.getByText("recovered content")).toBeInTheDocument();
    expect(screen.queryByText("משהו השתבש")).not.toBeInTheDocument();
  });
});
