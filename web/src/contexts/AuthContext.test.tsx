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

function renderWithProviders(queryClient?: QueryClient) {
  const qc =
    queryClient ??
    new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
  return render(
    <QueryClientProvider client={qc}>
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

  it("exposes loading=true initially while rendering children", () => {
    renderWithProviders();
    expect(screen.getByTestId("loading")).toHaveTextContent("true");
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

  it("clears query cache when user UID changes (prevents data leaking between users)", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const clearSpy = vi.spyOn(queryClient, "clear");

    renderWithProviders(queryClient);

    // User A logs in
    await act(async () => {
      authStateCallback?.({ uid: "user-a", email: "a@example.com" });
    });
    expect(clearSpy).not.toHaveBeenCalled();

    // User B replaces User A (e.g. sign-out then sign-in as different user)
    await act(async () => {
      authStateCallback?.({ uid: "user-b", email: "b@example.com" });
    });
    expect(clearSpy).toHaveBeenCalledTimes(1);

    clearSpy.mockRestore();
  });

  it("clears query cache when user signs out after being logged in", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const clearSpy = vi.spyOn(queryClient, "clear");

    renderWithProviders(queryClient);

    // User logs in
    await act(async () => {
      authStateCallback?.({ uid: "u1", email: "test@example.com" });
    });
    expect(clearSpy).not.toHaveBeenCalled();

    // User signs out (uid becomes null)
    await act(async () => {
      authStateCallback?.(null);
    });
    expect(clearSpy).toHaveBeenCalledTimes(1);

    clearSpy.mockRestore();
  });

  it("does not clear query cache on initial auth resolve", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const clearSpy = vi.spyOn(queryClient, "clear");

    renderWithProviders(queryClient);

    // First auth callback (initial load) should not clear cache
    await act(async () => {
      authStateCallback?.({ uid: "u1", email: "test@example.com" });
    });
    expect(clearSpy).not.toHaveBeenCalled();

    clearSpy.mockRestore();
  });
});
