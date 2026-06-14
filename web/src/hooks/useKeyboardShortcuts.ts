import { useEffect } from "react";
import { useNavigate, useLocation } from "react-router";

const SHORTCUT_ROUTES: Record<string, string> = {
  d: "/dashboard",
  h: "/history",
  s: "/settings",
  n: "/notifications",
};

export function useKeyboardShortcuts() {
  const navigate = useNavigate();
  const location = useLocation();

  useEffect(() => {
    function handler(e: KeyboardEvent) {
      const tag = (e.target as HTMLElement)?.tagName;
      if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return;
      if (e.ctrlKey || e.metaKey || e.altKey) return;

      // "/" and "⌘K" are owned by the command palette — don't compete.
      if (e.key === "Escape") {
        // Let an open overlay (command palette, dialog) handle its own Escape.
        if (document.querySelector('[role="dialog"]')) return;
        const main = document.querySelector("main");
        if (main) main.scrollTo({ top: 0, behavior: "smooth" });
        return;
      }

      const route = SHORTCUT_ROUTES[e.key];
      if (route && location.pathname !== route) {
        navigate(route);
      }
    }

    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [navigate, location.pathname]);
}
