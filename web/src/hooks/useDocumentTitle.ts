import { useEffect } from "react";

export const DEFAULT_DOCUMENT_TITLE = "CarWatch — מעקב חכם אחרי רכבים";

/**
 * Sets `document.title` for SPA routes. Every route shares the single static
 * `<title>` from index.html, so public pages set their own for clearer browser
 * tabs, history entries, and shared-link previews. Pass no argument to apply the
 * default brand title (used by the landing/home route).
 */
export function useDocumentTitle(title?: string): void {
  useEffect(() => {
    document.title = title ? `${title} · CarWatch` : DEFAULT_DOCUMENT_TITLE;
  }, [title]);
}
