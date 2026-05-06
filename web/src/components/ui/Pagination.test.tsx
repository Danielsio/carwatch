import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Pagination } from "./Pagination";

describe("Pagination", () => {
  const defaults = {
    offset: 0,
    total: 50,
    pageSize: 20,
    onPrev: vi.fn(),
    onNext: vi.fn(),
  };

  it("renders page range text", () => {
    render(<Pagination {...defaults} />);
    expect(screen.getByText(/1–20 מתוך 50/)).toBeInTheDocument();
  });

  it("shows next button on first page", () => {
    render(<Pagination {...defaults} />);
    expect(screen.getByRole("button", { name: "הבא" })).toBeInTheDocument();
  });

  it("does not show prev button on first page", () => {
    render(<Pagination {...defaults} />);
    expect(screen.queryByRole("button", { name: "הקודם" })).not.toBeInTheDocument();
  });

  it("shows both buttons on middle page", () => {
    render(<Pagination {...defaults} offset={20} />);
    expect(screen.getByRole("button", { name: "הבא" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "הקודם" })).toBeInTheDocument();
  });

  it("does not show next button on last page", () => {
    render(<Pagination {...defaults} offset={40} />);
    expect(screen.queryByRole("button", { name: "הבא" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "הקודם" })).toBeInTheDocument();
  });

  it("calls onNext when next button is clicked", async () => {
    const user = userEvent.setup();
    const onNext = vi.fn();
    render(<Pagination {...defaults} onNext={onNext} />);

    await user.click(screen.getByRole("button", { name: "הבא" }));
    expect(onNext).toHaveBeenCalledOnce();
  });

  it("calls onPrev when prev button is clicked", async () => {
    const user = userEvent.setup();
    const onPrev = vi.fn();
    render(<Pagination {...defaults} offset={20} onPrev={onPrev} />);

    await user.click(screen.getByRole("button", { name: "הקודם" }));
    expect(onPrev).toHaveBeenCalledOnce();
  });

  it("returns null when total fits in one page and offset is 0", () => {
    const { container } = render(
      <Pagination {...defaults} total={15} />,
    );
    expect(container.firstChild).toBeNull();
  });

  it("shows correct range on last partial page", () => {
    render(<Pagination {...defaults} offset={40} />);
    expect(screen.getByText(/41–50 מתוך 50/)).toBeInTheDocument();
  });

  it("handles exact page boundary", () => {
    render(<Pagination {...defaults} total={40} offset={20} />);
    expect(screen.getByText(/21–40 מתוך 40/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "הבא" })).not.toBeInTheDocument();
  });
});
