import { describe, it, expect, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { useSpotlight } from "./useSpotlight";

describe("useSpotlight", () => {
  const realMatchMedia = window.matchMedia;
  afterEach(() => {
    window.matchMedia = realMatchMedia;
  });

  it("caches the rect on enter and writes element-relative coords on move", () => {
    const { result } = renderHook(() => useSpotlight());

    const el = document.createElement("div");
    el.getBoundingClientRect = () =>
      ({ left: 100, top: 50, width: 200, height: 120 }) as DOMRect;

    result.current.onPointerEnter({
      currentTarget: el,
    } as unknown as React.PointerEvent<HTMLElement>);
    result.current.onPointerMove({
      currentTarget: el,
      clientX: 150,
      clientY: 90,
    } as unknown as React.PointerEvent<HTMLElement>);

    expect(el.style.getPropertyValue("--spot-x")).toBe("50px");
    expect(el.style.getPropertyValue("--spot-y")).toBe("40px");
  });

  it("move before enter is a no-op (nothing cached yet)", () => {
    const { result } = renderHook(() => useSpotlight());

    const el = document.createElement("div");
    result.current.onPointerMove({
      currentTarget: el,
      clientX: 10,
      clientY: 10,
    } as unknown as React.PointerEvent<HTMLElement>);

    expect(el.style.getPropertyValue("--spot-x")).toBe("");
  });

  it("returns a stable handler across renders", () => {
    const { result, rerender } = renderHook(() => useSpotlight());
    const first = result.current.onPointerMove;
    rerender();
    expect(result.current.onPointerMove).toBe(first);
  });

  it("no-ops on coarse-pointer (touch) devices — no layout reads", () => {
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

    // Even the enter handler (the one that reads layout on desktop) must not
    // touch the rect on touch devices.
    result.current.onPointerEnter({
      currentTarget: el,
    } as unknown as React.PointerEvent<HTMLElement>);
    result.current.onPointerMove({
      currentTarget: el,
      clientX: 5,
      clientY: 5,
    } as unknown as React.PointerEvent<HTMLElement>);

    expect(rectRead).toBe(false);
    expect(el.style.getPropertyValue("--spot-x")).toBe("");
  });
});
