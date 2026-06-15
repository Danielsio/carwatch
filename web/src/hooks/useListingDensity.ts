import { useCallback, useState } from "react";

export type Density = "comfortable" | "compact";

const STORAGE_KEY = "listings-density";

/** Listing-grid density, persisted to localStorage. */
export function useListingDensity(): readonly [Density, (d: Density) => void] {
  const [density, setDensityState] = useState<Density>(() => {
    try {
      return localStorage.getItem(STORAGE_KEY) === "compact"
        ? "compact"
        : "comfortable";
    } catch {
      return "comfortable";
    }
  });

  const setDensity = useCallback((d: Density) => {
    setDensityState(d);
    try {
      localStorage.setItem(STORAGE_KEY, d);
    } catch {
      // ignore storage failures (private mode, quota)
    }
  }, []);

  return [density, setDensity] as const;
}
