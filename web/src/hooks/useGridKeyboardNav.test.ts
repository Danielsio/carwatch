import { describe, it, expect, vi, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useGridKeyboardNav } from "./useGridKeyboardNav";

function press(key: string) {
  act(() => {
    window.dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true }));
  });
}

afterEach(() => {
  document.body.innerHTML = "";
});

describe("useGridKeyboardNav", () => {
  it("moves the highlight with j / k", () => {
    const { result } = renderHook(() =>
      useGridKeyboardNav({ count: 3, onOpen: vi.fn() }),
    );
    expect(result.current.activeIndex).toBe(-1);
    press("j");
    expect(result.current.activeIndex).toBe(0);
    press("j");
    expect(result.current.activeIndex).toBe(1);
    press("k");
    expect(result.current.activeIndex).toBe(0);
  });

  it("clamps at the ends", () => {
    const { result } = renderHook(() =>
      useGridKeyboardNav({ count: 2, onOpen: vi.fn() }),
    );
    press("k"); // from -1 -> 0
    press("k"); // stays 0
    expect(result.current.activeIndex).toBe(0);
    press("j");
    press("j");
    press("j"); // clamp at 1
    expect(result.current.activeIndex).toBe(1);
  });

  it("fires onOpen / onSave / onSeen for the active index", () => {
    const onOpen = vi.fn();
    const onSave = vi.fn();
    const onSeen = vi.fn();
    renderHook(() =>
      useGridKeyboardNav({ count: 3, onOpen, onSave, onSeen }),
    );
    press("j"); // active = 0
    press("o");
    press("s");
    press("e");
    expect(onOpen).toHaveBeenCalledWith(0);
    expect(onSave).toHaveBeenCalledWith(0);
    expect(onSeen).toHaveBeenCalledWith(0);
  });

  it("does nothing when typing in an input", () => {
    const onOpen = vi.fn();
    const { result } = renderHook(() =>
      useGridKeyboardNav({ count: 3, onOpen }),
    );
    const input = document.createElement("input");
    document.body.appendChild(input);
    act(() => {
      input.dispatchEvent(new KeyboardEvent("keydown", { key: "j", bubbles: true }));
    });
    expect(result.current.activeIndex).toBe(-1);
  });

  it("ignores keys while a dialog is open", () => {
    const { result } = renderHook(() =>
      useGridKeyboardNav({ count: 3, onOpen: vi.fn() }),
    );
    const dialog = document.createElement("div");
    dialog.setAttribute("role", "dialog");
    document.body.appendChild(dialog);
    press("j");
    expect(result.current.activeIndex).toBe(-1);
  });
});
