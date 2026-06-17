import { MemoryRouter } from "react-router";
import type { ReactElement } from "react";
import { ThemeProvider } from "@/contexts/ThemeContext";
import { LandingPage } from "@/pages/LandingPage";

/**
 * Build-time entry used by `scripts/prerender-landing.mjs` to render the public
 * landing page to static HTML (SEO + fast first paint). It deliberately wraps
 * the page in only the providers it actually consumes at render time:
 *
 *  - `ThemeProvider` — `LandingNav` reads it via `useTheme`. It is SSR-safe:
 *    `getInitialTheme()` guards on `typeof window === "undefined"`.
 *  - `MemoryRouter` — the landing renders `<Link>`s, which need a router context.
 *
 * No Auth/Query providers: the landing tree uses neither during render (Firebase
 * is loaded lazily inside an effect, and `useAppVersion` fetches in an effect).
 * The client still boots through the full provider tree in `main.tsx`; this
 * markup is only the initial paint that React replaces on mount.
 */
export function createLandingElement(): ReactElement {
  return (
    <ThemeProvider>
      <MemoryRouter initialEntries={["/"]}>
        <LandingPage />
      </MemoryRouter>
    </ThemeProvider>
  );
}
