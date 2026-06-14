import { useCallback } from "react";

export type SpotlightHandlers = {
  onPointerMove: (e: React.PointerEvent<HTMLElement>) => void;
};

/**
 * Cursor-following spotlight. Pair with the `spotlight` utility class.
 * Writes `--spot-x` / `--spot-y` straight to the element style on pointer
 * move, so it never triggers a React re-render. Position is element-relative.
 */
export function useSpotlight(): SpotlightHandlers {
  const onPointerMove = useCallback((e: React.PointerEvent<HTMLElement>) => {
    const el = e.currentTarget;
    const rect = el.getBoundingClientRect();
    el.style.setProperty("--spot-x", `${e.clientX - rect.left}px`);
    el.style.setProperty("--spot-y", `${e.clientY - rect.top}px`);
  }, []);

  return { onPointerMove };
}
