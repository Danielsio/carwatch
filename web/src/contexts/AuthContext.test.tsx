import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AuthProvider, useAuth } from "./AuthContext";

// Capture the onAuthStateChanged callback so we can trigger it manually
let authStateCallback: ((user: unknown) => void) | null = null;
const mockSignOut = vi.fn().mockResolvedValue(undefined);

vi.mock("firebase/auth", () => ({
  onAuthStateChanged: (_auth: unknown, cb: (user: unknown) => void) => {
    authStateCallback = cb;
    return () => {
      authStateCallback = null;
    };
  },
  signOut: (...args: unknown[]) => mockSignOut(...args),
  getAuth: () => ({}),
}));

vi.mock("@/lib/firebase", () => ({
  auth: { currentUser: null },
}));

vi.mock("@/lib/auth-token", () => ({
  setAuthTokenGetter: vi.fn(),
}));

function TestConsumer() {
  const { user, loading, signOut } = useAuth();
  return (
    <div>
      <span data-testid="loading">{String(loading)}</span>
      <span data-testid="user">{user ? (user as { email: string }).email : "null"}</span>
      <button onClick={signOut}>logout</button>
    </div>
  );
}

function renderWithProviders() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>
    </QueryClientProvider>,
  );
}

describe("AuthContext", () => {
  beforeEach(() => {
    authStateCallback = null;
    mockSignOut.mockClear();
  });

  it("shows loading screen initially", () => {
    renderWithProviders();
    // AuthProvider renders a loading spinner while loading=true
    // The TestConsumer is NOT rendered because AuthProvider shows AuthLoadingScreen instead of children
    expect(screen.queryByTestId("loading")).not.toBeInTheDocument();
  });

  it("sets user after auth resolves with a user", async () => {
    renderWithProviders();

    await act(async () => {
      authStateCallback?.({ uid: "u1", email: "test@example.com" });
    });

    expect(screen.getByTestId("loading").textContent).toBe("false");
    expect(screen.getByTestId("user").textContent).toBe("test@example.com");
  });

  it("sets user to null after auth resolves with null", async () => {
    renderWithProviders();

    await act(async () => {
      authStateCallback?.(null);
    });

    expect(screen.getByTestId("loading").textContent).toBe("false");
    expect(screen.getByTestId("user").textContent).toBe("null");
  });

  it("calls firebase signOut on logout", async () => {
    const user = userEvent.setup();
    renderWithProviders();

    await act(async () => {
      authStateCallback?.({ uid: "u1", email: "test@example.com" });
    });

    await user.click(screen.getByRole("button", { name: "logout" }));
    expect(mockSignOut).toHaveBeenCalledTimes(1);
  });
});
