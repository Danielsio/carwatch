import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { SettingsPage } from "./SettingsPage";
import { ToastProvider } from "@/components/ui/Toast";

const mockToggle = vi.fn();
let mockTheme = "dark";

vi.mock("@/contexts/AuthContext", () => ({
  useAuth: () => ({
    user: { email: "test@example.com" },
    signOut: vi.fn().mockResolvedValue(undefined),
  }),
}));

vi.mock("@/contexts/ThemeContext", () => ({
  useTheme: () => ({
    theme: mockTheme,
    toggle: mockToggle,
  }),
}));

vi.mock("@/hooks/usePageTitle", () => ({
  usePageTitle: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  telegramApi: {
    status: vi.fn().mockResolvedValue({ connected: false }),
    createLink: vi.fn().mockResolvedValue({ link: "https://t.me/bot?start=link_abc" }),
  },
}));

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <ToastProvider>
          <SettingsPage />
        </ToastProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("SettingsPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    mockTheme = "dark";
  });

  it("renders settings page with header", () => {
    renderPage();
    expect(screen.getByText("הגדרות")).toBeInTheDocument();
    expect(screen.getByText("ניהול חשבון וחיבורים")).toBeInTheDocument();
  });

  it("displays user email", () => {
    renderPage();
    expect(screen.getByText("test@example.com")).toBeInTheDocument();
  });

  it("shows theme toggle switch", () => {
    renderPage();
    const toggle = screen.getByRole("switch", { name: "מצב כהה" });
    expect(toggle).toBeInTheDocument();
    expect(toggle).toHaveAttribute("aria-checked", "true");
  });

  it("calls toggle when theme switch is clicked", async () => {
    const user = userEvent.setup();
    renderPage();
    const toggle = screen.getByRole("switch", { name: "מצב כהה" });
    await user.click(toggle);
    expect(mockToggle).toHaveBeenCalledTimes(1);
  });

  it("shows Telegram connection section", () => {
    renderPage();
    expect(screen.getByText("התראות Telegram")).toBeInTheDocument();
  });

  it("shows sign out button", () => {
    renderPage();
    expect(screen.getByText("התנתק מהחשבון")).toBeInTheDocument();
  });
});
