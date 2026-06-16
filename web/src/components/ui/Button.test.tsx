import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { Button } from "./button";

describe("Button", () => {
  it("renders a button with its label", () => {
    render(<Button>שמור</Button>);
    expect(screen.getByRole("button", { name: "שמור" })).toBeInTheDocument();
  });

  it("defaults to type=button", () => {
    render(<Button>שמור</Button>);
    expect(screen.getByRole("button")).toHaveAttribute("type", "button");
  });

  describe("loading", () => {
    it("disables the button and marks it busy", () => {
      render(<Button loading>שמור</Button>);
      const btn = screen.getByRole("button");
      expect(btn).toBeDisabled();
      expect(btn).toHaveAttribute("aria-busy", "true");
    });

    it("renders a spinner while loading", () => {
      const { container } = render(<Button loading>שמור</Button>);
      expect(container.querySelector(".animate-spin")).toBeInTheDocument();
    });

    it("does not fire onClick while loading", () => {
      const onClick = vi.fn();
      render(
        <Button loading onClick={onClick}>
          שמור
        </Button>,
      );
      fireEvent.click(screen.getByRole("button"));
      expect(onClick).not.toHaveBeenCalled();
    });

    it("is not busy when not loading", () => {
      render(<Button>שמור</Button>);
      expect(screen.getByRole("button")).not.toHaveAttribute("aria-busy");
    });
  });

  it("respects an explicit disabled prop", () => {
    render(<Button disabled>שמור</Button>);
    expect(screen.getByRole("button")).toBeDisabled();
  });

  it("supports asChild composition", () => {
    render(
      <Button asChild>
        <a href="/x">קישור</a>
      </Button>,
    );
    const link = screen.getByRole("link", { name: "קישור" });
    expect(link).toHaveAttribute("href", "/x");
  });
});
