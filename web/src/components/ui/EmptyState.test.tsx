import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Bell } from "lucide-react";
import { EmptyState } from "./EmptyState";

describe("EmptyState", () => {
  it("renders title", () => {
    render(<EmptyState icon={Bell} title="No items" />);
    expect(screen.getByText("No items")).toBeInTheDocument();
  });

  it("renders description when provided", () => {
    render(
      <EmptyState icon={Bell} title="No items" description="Check back later" />,
    );
    expect(screen.getByText("Check back later")).toBeInTheDocument();
  });

  it("does not render description when omitted", () => {
    render(<EmptyState icon={Bell} title="No items" />);
    const paragraphs = screen.getAllByText(/./);
    expect(paragraphs).toHaveLength(1);
  });

  it("renders action slot when provided", () => {
    render(
      <EmptyState
        icon={Bell}
        title="No items"
        action={<button>Add item</button>}
      />,
    );
    expect(screen.getByRole("button", { name: "Add item" })).toBeInTheDocument();
  });

  it("does not render action slot when omitted", () => {
    render(<EmptyState icon={Bell} title="No items" />);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("applies responsive padding classes", () => {
    const { container } = render(
      <EmptyState icon={Bell} title="No items" />,
    );
    const wrapper = container.firstChild as HTMLElement;
    expect(wrapper).toHaveClass("py-10");
    expect(wrapper.className).toContain("sm:py-14");
    expect(wrapper.className).toContain("md:py-16");
  });

  it("applies custom className", () => {
    const { container } = render(
      <EmptyState icon={Bell} title="No items" className="min-h-[50vh]" />,
    );
    expect(container.firstChild).toHaveClass("min-h-[50vh]");
  });

  it("renders the icon with aria-hidden", () => {
    const { container } = render(
      <EmptyState icon={Bell} title="No items" />,
    );
    const icon = container.querySelector('[aria-hidden="true"]');
    expect(icon).toBeInTheDocument();
  });
});
