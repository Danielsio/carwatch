import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { NotFoundPage } from "./NotFoundPage";

const mockUseAuth = vi.fn();

vi.mock("@/contexts/AuthContext", () => ({
  useAuth: () => mockUseAuth(),
}));

function renderPage() {
  return render(
    <MemoryRouter>
      <NotFoundPage />
    </MemoryRouter>,
  );
}

describe("NotFoundPage", () => {
  it("renders 404 text", () => {
    mockUseAuth.mockReturnValue({ user: null });
    renderPage();
    expect(screen.getByText("404")).toBeInTheDocument();
  });

  it("renders Hebrew not-found message", () => {
    mockUseAuth.mockReturnValue({ user: null });
    renderPage();
    expect(screen.getByText("הדף לא נמצא")).toBeInTheDocument();
    expect(screen.getByText("הדף שחיפשת לא קיים או הוסר")).toBeInTheDocument();
  });

  it("links to home page for unauthenticated users", () => {
    mockUseAuth.mockReturnValue({ user: null });
    renderPage();
    const link = screen.getByRole("link", { name: "חזרה לדף הבית" });
    expect(link).toHaveAttribute("href", "/");
  });

  it("links to dashboard for authenticated users", () => {
    mockUseAuth.mockReturnValue({ user: { email: "test@example.com" } });
    renderPage();
    const link = screen.getByRole("link", { name: "חזרה לדף הבית" });
    expect(link).toHaveAttribute("href", "/dashboard");
  });
});
