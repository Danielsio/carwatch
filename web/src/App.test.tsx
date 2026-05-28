import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes, Outlet } from "react-router";
import { ProtectedRoute } from "@/components/ProtectedRoute";

let authState = { user: null as unknown, loading: false };

vi.mock("@/contexts/AuthContext", () => ({
  useAuth: () => authState,
}));

function TestApp() {
  return (
    <Routes>
      <Route path="/" element={<div>Landing</div>} />
      <Route path="/login" element={<div>Login</div>} />
      <Route path="/signup" element={<div>Signup</div>} />
      <Route path="/try" element={<div>Try Search</div>} />
      <Route element={<ProtectedRoute />}>
        <Route element={<Outlet />}>
          <Route path="/dashboard" element={<div>Dashboard</div>} />
          <Route path="/saved" element={<div>Saved</div>} />
          <Route path="/history" element={<div>History</div>} />
          <Route path="/settings" element={<div>Settings</div>} />
          <Route path="/notifications" element={<div>Notifications</div>} />
          <Route path="/searches/new" element={<div>New Search</div>} />
          <Route path="/listings/tok-1" element={<div>Listing Detail</div>} />
        </Route>
      </Route>
    </Routes>
  );
}

describe("Route access matrix", () => {
  beforeEach(() => {
    authState = { user: null, loading: false };
  });

  describe("guest (unauthenticated)", () => {
    const allowed = [
      { path: "/", text: "Landing" },
      { path: "/login", text: "Login" },
      { path: "/signup", text: "Signup" },
      { path: "/try", text: "Try Search" },
    ];

    allowed.forEach(({ path, text }) => {
      it(`can access ${path}`, () => {
        authState = { user: null, loading: false };
        render(<MemoryRouter initialEntries={[path]}><TestApp /></MemoryRouter>);
        expect(screen.getByText(text)).toBeInTheDocument();
      });
    });

    const blocked = ["/dashboard", "/saved", "/history", "/settings", "/notifications", "/searches/new", "/listings/tok-1"];
    blocked.forEach((path) => {
      it(`is redirected from ${path} to /login`, () => {
        authState = { user: null, loading: false };
        render(<MemoryRouter initialEntries={[path]}><TestApp /></MemoryRouter>);
        expect(screen.getByText("Login")).toBeInTheDocument();
      });
    });
  });

  describe("authenticated user", () => {
    const routes = [
      { path: "/dashboard", text: "Dashboard" },
      { path: "/saved", text: "Saved" },
      { path: "/history", text: "History" },
      { path: "/settings", text: "Settings" },
      { path: "/notifications", text: "Notifications" },
      { path: "/searches/new", text: "New Search" },
      { path: "/listings/tok-1", text: "Listing Detail" },
    ];

    routes.forEach(({ path, text }) => {
      it(`can access ${path}`, () => {
        authState = { user: { email: "test@example.com" }, loading: false };
        render(<MemoryRouter initialEntries={[path]}><TestApp /></MemoryRouter>);
        expect(screen.getByText(text)).toBeInTheDocument();
      });
    });
  });

  it("loading state shows spinner, not content", () => {
    authState = { user: null, loading: true };
    render(<MemoryRouter initialEntries={["/dashboard"]}><TestApp /></MemoryRouter>);
    expect(screen.getByRole("status")).toBeInTheDocument();
    expect(screen.queryByText("Dashboard")).not.toBeInTheDocument();
  });
});
