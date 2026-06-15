import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { ToastProvider, useToast, type ToastOptions, type ToastType } from "./Toast";

function Harness({
  type = "info",
  options,
}: {
  type?: ToastType;
  options?: ToastOptions;
}) {
  const { toast } = useToast();
  return (
    <button onClick={() => toast("נשמר בהצלחה", type, options)}>fire</button>
  );
}

function renderHarness(props: Parameters<typeof Harness>[0] = {}) {
  return render(
    <ToastProvider>
      <Harness {...props} />
    </ToastProvider>,
  );
}

describe("Toast", () => {
  it("shows a toast with a polite status role for non-errors", () => {
    renderHarness({ type: "success" });
    fireEvent.click(screen.getByText("fire"));
    const toast = screen.getByRole("status");
    expect(toast).toHaveTextContent("נשמר בהצלחה");
    expect(toast).toHaveAttribute("aria-live", "polite");
  });

  it("uses an assertive alert role for errors", () => {
    renderHarness({ type: "error" });
    fireEvent.click(screen.getByText("fire"));
    expect(screen.getByRole("alert")).toHaveAttribute("aria-live", "assertive");
  });

  it("renders an action button and runs it", () => {
    const onUndo = vi.fn();
    renderHarness({ options: { action: { label: "בטל", onClick: onUndo } } });
    fireEvent.click(screen.getByText("fire"));
    fireEvent.click(screen.getByRole("button", { name: "בטל" }));
    expect(onUndo).toHaveBeenCalledTimes(1);
  });

  it("dismisses when the close button is clicked", async () => {
    vi.useFakeTimers();
    try {
      renderHarness();
      fireEvent.click(screen.getByText("fire"));
      expect(screen.getByRole("status")).toBeInTheDocument();
      fireEvent.click(screen.getByRole("button", { name: "סגור הודעה" }));
      // exit animation timeout
      act(() => {
        vi.advanceTimersByTime(300);
      });
      expect(screen.queryByRole("status")).not.toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it("auto-dismisses after its duration", () => {
    vi.useFakeTimers();
    try {
      renderHarness({ options: { duration: 1000 } });
      fireEvent.click(screen.getByText("fire"));
      expect(screen.getByRole("status")).toBeInTheDocument();
      // First advance fires the dismiss timer (begins exit + re-render),
      // second advance fires the exit-animation removal timer.
      act(() => {
        vi.advanceTimersByTime(1000);
      });
      act(() => {
        vi.advanceTimersByTime(300);
      });
      expect(screen.queryByRole("status")).not.toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });
});
