import { useCallback, useMemo } from "react";

export type SpotlightHandlers = {
  onPointerMove: (e: React.PointerEvent<HTMLElement>) => void;
};

const NOOP = () => {};

/**
 * True on touch-primary devices. There the cursor spotlight never shows (it
 * only reveals on `:hover`/`:focus-within`), so the per-pointermove
 * `getBoundingClientRect()` would be pure overhead — and worse, it fires during
 * touch scrolling and forces a synchronous layout each frame (scroll jank).
 */
function isCoarsePointer(): boolean {
  return (
    typeof window !== "undefined" &&
    typeof window.matchMedia === "function" &&
    window.matchMedia("(pointer: coarse)").matches
  );
}

/**
 * Cursor-following spotlight. Pair with the `spotlight` utility class.
 * Writes `--spot-x` / `--spot-y` straight to the element style on pointer
 * move, so it never triggers a React re-render. Position is element-relative.
 * On touch devices it returns a no-op handler (see {@link isCoarsePointer}).
 */
export function useSpotlight(): SpotlightHandlers {
  const onPointerMove = useCallback((e: React.PointerEvent<HTMLElement>) => {
    const el = e.currentTarget;
    const rect = el.getBoundingClientRect();
    el.style.setProperty("--spot-x", `${e.clientX - rect.left}px`);
    el.style.setProperty("--spot-y", `${e.clientY - rect.top}px`);
  }, []);

  return useMemo(
    () => (isCoarsePointer() ? { onPointerMove: NOOP } : { onPointerMove }),
    [onPointerMove],
  );
}
