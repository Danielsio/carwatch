import { describe, it, expect, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useListingDensity } from "./useListingDensity";

describe("useListingDensity", () => {
  beforeEach(() => localStorage.clear());

  it("defaults to comfortable", () => {
    const { result } = renderHook(() => useListingDensity());
    expect(result.current[0]).toBe("comfortable");
  });

  it("reads a persisted compact value", () => {
    localStorage.setItem("listings-density", "compact");
    const { result } = renderHook(() => useListingDensity());
    expect(result.current[0]).toBe("compact");
  });

  it("updates and persists the density", () => {
    const { result } = renderHook(() => useListingDensity());
    act(() => result.current[1]("compact"));
    expect(result.current[0]).toBe("compact");
    expect(localStorage.getItem("listings-density")).toBe("compact");
  });
});
