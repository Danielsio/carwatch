import { render, screen, act } from "@testing-library/react";
import { describe, it, expect, beforeEach, vi } from "vitest";
import { ThemeProvider, useTheme } from "./ThemeContext";

const listeners: Array<(e: { matches: boolean }) => void> = [];
const mockMatchMedia = vi.fn().mockImplementation(() => ({
  matches: false,
  addEventListener: (_: string, cb: (e: { matches: boolean }) => void) => {
    listeners.push(cb);
  },
  removeEventListener: (_: string, cb: (e: { matches: boolean }) => void) => {
    const idx = listeners.indexOf(cb);
    if (idx >= 0) listeners.splice(idx, 1);
  },
}));

function ThemeDisplay() {
  const { theme, toggle } = useTheme();
  return (
    <div>
      <span data-testid="theme">{theme}</span>
      <button onClick={toggle}>Toggle</button>
    </div>
  );
}

describe("ThemeContext", () => {
  beforeEach(() => {
    localStorage.clear();
    document.documentElement.classList.remove("dark");
    listeners.length = 0;
    window.matchMedia = mockMatchMedia;
  });

  it("defaults to dark when no preference stored", () => {
    render(
      <ThemeProvider>
        <ThemeDisplay />
      </ThemeProvider>,
    );
    expect(screen.getByTestId("theme").textContent).toBe("dark");
  });

  it("respects stored light preference", () => {
    localStorage.setItem("theme", "light");
    render(
      <ThemeProvider>
        <ThemeDisplay />
      </ThemeProvider>,
    );
    expect(screen.getByTestId("theme").textContent).toBe("light");
  });

  it("toggles from dark to light", async () => {
    render(
      <ThemeProvider>
        <ThemeDisplay />
      </ThemeProvider>,
    );
    expect(screen.getByTestId("theme").textContent).toBe("dark");

    await act(async () => {
      screen.getByText("Toggle").click();
    });
    expect(screen.getByTestId("theme").textContent).toBe("light");
    expect(localStorage.getItem("theme")).toBe("light");
  });

  it("persists toggle to localStorage", async () => {
    render(
      <ThemeProvider>
        <ThemeDisplay />
      </ThemeProvider>,
    );

    await act(async () => {
      screen.getByText("Toggle").click();
    });
    expect(localStorage.getItem("theme")).toBe("light");

    await act(async () => {
      screen.getByText("Toggle").click();
    });
    expect(localStorage.getItem("theme")).toBe("dark");
  });

  it("throws when useTheme is used outside provider", () => {
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});
    expect(() => render(<ThemeDisplay />)).toThrow(
      "useTheme must be used within ThemeProvider",
    );
    spy.mockRestore();
  });
});
