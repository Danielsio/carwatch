import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { ProtectedRoute } from "./ProtectedRoute";

let authState = { user: null as unknown, loading: false };

vi.mock("@/contexts/AuthContext", () => ({
  useAuth: () => authState,
}));

function renderWithRoute(initialPath: string = "/protected") {
  return render(
    <MemoryRouter initialEntries={[initialPath]}>
      <Routes>
        <Route element={<ProtectedRoute />}>
          <Route path="/protected" element={<div>Protected Content</div>} />
        </Route>
        <Route path="/login" element={<div>Login Page</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("ProtectedRoute", () => {
  beforeEach(() => {
    authState = { user: null, loading: false };
  });

  it("renders outlet for authenticated users", () => {
    authState = { user: { email: "test@example.com" }, loading: false };
    renderWithRoute();
    expect(screen.getByText("Protected Content")).toBeInTheDocument();
  });

  it("shows loading spinner when auth is loading", () => {
    authState = { user: null, loading: true };
    renderWithRoute();
    expect(screen.getByRole("status")).toBeInTheDocument();
    expect(screen.getByText("טוען...")).toBeInTheDocument();
  });

  it("renders content for unauthenticated guests (no redirect)", () => {
    authState = { user: null, loading: false };
    renderWithRoute();
    expect(screen.getByText("Protected Content")).toBeInTheDocument();
    expect(screen.queryByText("Login Page")).not.toBeInTheDocument();
  });
});
