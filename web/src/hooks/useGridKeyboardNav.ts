import { useCallback, useEffect, useRef, useState } from "react";

export type GridKeyboardNav = {
  /** Highlighted index, or -1 when nothing is selected yet. */
  activeIndex: number;
  setActiveIndex: (i: number) => void;
};

/**
 * Power-user keyboard navigation for a list/grid:
 *   j / ↓  next        k / ↑  previous
 *   o / ↵  open        s  save        e  toggle seen
 *
 * Ignores keystrokes while typing in a field or while an overlay (command
 * palette / dialog) is open. The window listener binds once; current callbacks
 * and count are read through a ref so it never needs to re-bind.
 */
export function useGridKeyboardNav(opts: {
  count: number;
  onOpen: (index: number) => void;
  onSave?: (index: number) => void;
  onSeen?: (index: number) => void;
  enabled?: boolean;
}): GridKeyboardNav {
  const { count, onOpen, onSave, onSeen, enabled = true } = opts;
  const [activeIndex, setActiveIndex] = useState(-1);

  const activeRef = useRef(activeIndex);
  activeRef.current = activeIndex;
  const cbRef = useRef({ count, onOpen, onSave, onSeen });
  cbRef.current = { count, onOpen, onSave, onSeen };

  // Keep the highlight in range as the list shrinks/grows.
  useEffect(() => {
    setActiveIndex((i) => (i >= count ? count - 1 : i));
  }, [count]);

  useEffect(() => {
    if (!enabled) return;
    function handler(e: KeyboardEvent) {
      const target = e.target as HTMLElement | null;
      const tag = target?.tagName;
      if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return;
      if (target?.isContentEditable) return;
      if (e.ctrlKey || e.metaKey || e.altKey) return;
      if (document.querySelector('[role="dialog"]')) return;

      const { count: c, onOpen, onSave, onSeen } = cbRef.current;
      if (c <= 0) return;

      switch (e.key) {
        case "j":
        case "ArrowDown":
          e.preventDefault();
          setActiveIndex((i) => Math.min(c - 1, i + 1));
          break;
        case "k":
        case "ArrowUp":
          e.preventDefault();
          setActiveIndex((i) => (i <= 0 ? 0 : i - 1));
          break;
        case "o":
        case "Enter":
          if (activeRef.current >= 0) {
            e.preventDefault();
            onOpen(activeRef.current);
          }
          break;
        case "s":
          if (activeRef.current >= 0 && onSave) {
            e.preventDefault();
            onSave(activeRef.current);
          }
          break;
        case "e":
          if (activeRef.current >= 0 && onSeen) {
            e.preventDefault();
            onSeen(activeRef.current);
          }
          break;
      }
    }
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [enabled]);

  const set = useCallback((i: number) => setActiveIndex(i), []);
  return { activeIndex, setActiveIndex: set };
}
