import { describe, it, expect } from "vitest";
import { renderHook } from "@testing-library/react";
import { useSpotlight } from "./useSpotlight";

describe("useSpotlight", () => {
  it("writes element-relative coordinates to CSS vars on pointer move", () => {
    const { result } = renderHook(() => useSpotlight());

    const el = document.createElement("div");
    el.getBoundingClientRect = () =>
      ({ left: 100, top: 50, width: 200, height: 120 }) as DOMRect;

    result.current.onPointerMove({
      currentTarget: el,
      clientX: 150,
      clientY: 90,
    } as unknown as React.PointerEvent<HTMLElement>);

    expect(el.style.getPropertyValue("--spot-x")).toBe("50px");
    expect(el.style.getPropertyValue("--spot-y")).toBe("40px");
  });

  it("returns a stable handler across renders", () => {
    const { result, rerender } = renderHook(() => useSpotlight());
    const first = result.current.onPointerMove;
    rerender();
    expect(result.current.onPointerMove).toBe(first);
  });
});
