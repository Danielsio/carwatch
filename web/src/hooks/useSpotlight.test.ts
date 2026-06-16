import { describe, it, expect, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { useSpotlight } from "./useSpotlight";

describe("useSpotlight", () => {
  const realMatchMedia = window.matchMedia;
  afterEach(() => {
    window.matchMedia = realMatchMedia;
  });

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

  it("no-ops on coarse-pointer (touch) devices — no layout reads on scroll", () => {
    window.matchMedia = ((query: string) => ({
      matches: query.includes("coarse"),
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    })) as typeof window.matchMedia;

    const { result } = renderHook(() => useSpotlight());

    let rectRead = false;
    const el = document.createElement("div");
    el.getBoundingClientRect = () => {
      rectRead = true;
      return { left: 0, top: 0, width: 10, height: 10 } as DOMRect;
    };

    result.current.onPointerMove({
      currentTarget: el,
      clientX: 5,
      clientY: 5,
    } as unknown as React.PointerEvent<HTMLElement>);

    expect(rectRead).toBe(false);
    expect(el.style.getPropertyValue("--spot-x")).toBe("");
  });
});
