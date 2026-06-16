import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { ConfirmDialog } from "./ConfirmDialog";

function setup(props = {}) {
  const onConfirm = vi.fn();
  const onCancel = vi.fn();
  render(
    <ConfirmDialog
      open
      title="האם לצאת מהחשבון?"
      description="תצטרך להתחבר מחדש."
      confirmLabel="התנתק"
      cancelLabel="ביטול"
      onConfirm={onConfirm}
      onCancel={onCancel}
      {...props}
    />,
  );
  return { onConfirm, onCancel };
}

describe("ConfirmDialog", () => {
  it("renders nothing when closed", () => {
    render(
      <ConfirmDialog open={false} title="x" onConfirm={() => {}} onCancel={() => {}} />,
    );
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("renders a dialog with title and description", () => {
    setup();
    expect(screen.getByText("האם לצאת מהחשבון?")).toBeInTheDocument();
    expect(screen.getByText("תצטרך להתחבר מחדש.")).toBeInTheDocument();
  });

  it("calls onConfirm when the confirm action is clicked", () => {
    const { onConfirm } = setup();
    fireEvent.click(screen.getByRole("button", { name: "התנתק" }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it("calls onCancel when the cancel action is clicked", () => {
    const { onCancel } = setup();
    fireEvent.click(screen.getByRole("button", { name: "ביטול" }));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it("uses a destructive confirm button when requested", () => {
    setup({ variant: "destructive" });
    expect(
      screen.getByRole("button", { name: "התנתק" }).className,
    ).toContain("destructive");
  });

  it("disables actions while loading", () => {
    setup({ loading: true });
    expect(screen.getByRole("button", { name: "התנתק" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "ביטול" })).toBeDisabled();
  });
});
