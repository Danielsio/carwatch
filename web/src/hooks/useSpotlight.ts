import { useCallback, useMemo, useRef } from "react";

export type SpotlightHandlers = {
  onPointerEnter: (e: React.PointerEvent<HTMLElement>) => void;
  onPointerMove: (e: React.PointerEvent<HTMLElement>) => void;
};

const NOOP = () => {};

/**
 * True on touch-primary devices. There the cursor spotlight never shows (it
 * only reveals on `:hover`/`:focus-within`), so `getBoundingClientRect()` would
 * be pure overhead — and worse, it forces a synchronous layout during touch
 * scrolling (scroll jank).
 */
function isCoarsePointer(): boolean {
  return (
    typeof window !== "undefined" &&
    typeof window.matchMedia === "function" &&
    window.matchMedia("(pointer: coarse)").matches
  );
}

/**
 * Cursor-following spotlight. Pair with the `spotlight` utility class. Writes
 * `--spot-x` / `--spot-y` straight to the element style, so it never triggers a
 * React re-render. Position is element-relative.
 *
 * The element rect is read once on pointer ENTER and cached, then reused for
 * every move. `getBoundingClientRect()` forces a synchronous layout; reading it
 * on every pointermove made each scroll frame pay a reflow — pointermove keeps
 * firing as content slides under a stationary cursor — which was a real scroll
 * jank source. One read per hover instead; a sub-pixel drift if the element
 * scrolls mid-hover is imperceptible on a decorative glow. Touch devices no-op.
 */
export function useSpotlight(): SpotlightHandlers {
  const rectRef = useRef<{ left: number; top: number } | null>(null);

  const onPointerEnter = useCallback((e: React.PointerEvent<HTMLElement>) => {
    const r = e.currentTarget.getBoundingClientRect();
    rectRef.current = { left: r.left, top: r.top };
  }, []);

  const onPointerMove = useCallback((e: React.PointerEvent<HTMLElement>) => {
    const rect = rectRef.current;
    if (!rect) return;
    const el = e.currentTarget;
    el.style.setProperty("--spot-x", `${e.clientX - rect.left}px`);
    el.style.setProperty("--spot-y", `${e.clientY - rect.top}px`);
  }, []);

  return useMemo(
    () =>
      isCoarsePointer()
        ? { onPointerEnter: NOOP, onPointerMove: NOOP }
        : { onPointerEnter, onPointerMove },
    [onPointerEnter, onPointerMove],
  );
}
