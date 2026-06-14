import { describe, it, vi, beforeAll, afterAll, afterEach } from "vitest";
import { render } from "@testing-library/react";
import { BrowserRouter } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider } from "@/contexts/ThemeContext";

vi.mock("@/lib/firebase", () => ({
  auth: {},
  googleProvider: {},
}));
vi.mock("firebase/auth", () => ({
  onAuthStateChanged: (_a: unknown, cb: (u: null) => void) => { cb(null); return () => {}; },
  signOut: () => Promise.resolve(),
  getAuth: () => ({}),
  GoogleAuthProvider: class {},
}));

import { AuthProvider } from "@/contexts/AuthContext";
import { ToastProvider } from "@/components/ui/Toast";
import { server } from "./mocks/server";
import axe from "axe-core";

beforeAll(() => server.listen({ onUnhandledRequest: "bypass" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

function renderWithProviders(ui: React.ReactElement) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity } },
  });
  const { container } = render(
    <QueryClientProvider client={qc}>
      <BrowserRouter>
        <ThemeProvider>
          <AuthProvider>
            <ToastProvider>{ui}</ToastProvider>
          </AuthProvider>
        </ThemeProvider>
      </BrowserRouter>
    </QueryClientProvider>,
  );
  return container;
}

async function checkA11y(container: HTMLElement) {
  const results = await axe.run(container, {
    rules: {
      region: { enabled: false },
      "color-contrast": { enabled: false },
    },
  });
  const violations = results.violations.filter(
    (v) => v.impact === "critical" || v.impact === "serious",
  );
  if (violations.length > 0) {
    const details = violations
      .map((v) => `[${v.impact}] ${v.id}: ${v.description} (${v.nodes.length} nodes)`)
      .join("\n");
    throw new Error(`a11y violations:\n${details}`);
  }
}

describe("a11y smoke tests", () => {
  it("SettingsPage has no critical a11y violations", async () => {
    const { SettingsPage } = await import("@/pages/SettingsPage");
    const container = renderWithProviders(<SettingsPage />);
    await checkA11y(container);
  });

  it("SearchesPage has no critical a11y violations", async () => {
    const { SearchesPage } = await import("@/pages/SearchesPage");
    const container = renderWithProviders(<SearchesPage />);
    await checkA11y(container);
  });
});
