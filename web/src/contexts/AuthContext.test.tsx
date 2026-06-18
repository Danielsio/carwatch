import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, act, waitFor } from "@testing-library/react";
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
  const result = render(
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>
    </QueryClientProvider>,
  );
  return { ...result, queryClient };
}

// Firebase is imported lazily, so the auth-state subscription (and thus
// authStateCallback) is registered a microtask after render. Wait for it before
// driving auth-state changes.
async function flushAuthInit() {
  await waitFor(() => expect(authStateCallback).not.toBeNull());
}

describe("AuthContext", () => {
  beforeEach(() => {
    authStateCallback = null;
    mockSignOut.mockClear();
    localStorage.clear();
  });

  it("stays loading=true until the lazy auth subscription emits", async () => {
    renderWithProviders();
    await flushAuthInit();
    // Subscribed, but no auth event yet → still loading.
    expect(screen.getByTestId("loading")).toHaveTextContent("true");
  });

  it("sets user after auth resolves with a user", async () => {
    renderWithProviders();
    await flushAuthInit();

    await act(async () => {
      authStateCallback?.({ uid: "u1", email: "test@example.com" });
    });

    expect(screen.getByTestId("loading").textContent).toBe("false");
    expect(screen.getByTestId("user").textContent).toBe("test@example.com");
  });

  it("sets user to null after auth resolves with null", async () => {
    renderWithProviders();
    await flushAuthInit();

    await act(async () => {
      authStateCallback?.(null);
    });

    expect(screen.getByTestId("loading").textContent).toBe("false");
    expect(screen.getByTestId("user").textContent).toBe("null");
  });

  it("calls firebase signOut on logout", async () => {
    const user = userEvent.setup();
    renderWithProviders();
    await flushAuthInit();

    await act(async () => {
      authStateCallback?.({ uid: "u1", email: "test@example.com" });
    });

    await user.click(screen.getByRole("button", { name: "logout" }));
    expect(mockSignOut).toHaveBeenCalledTimes(1);
  });

  it("clears query cache when user UID changes (user switch)", async () => {
    const { queryClient } = renderWithProviders();
    await flushAuthInit();
    const clearSpy = vi.spyOn(queryClient, "clear");

    // User A logs in — first auth event, prevUid is null so no clear
    await act(async () => {
      authStateCallback?.({ uid: "user-a", email: "a@example.com" });
    });
    expect(screen.getByTestId("user").textContent).toBe("a@example.com");
    expect(clearSpy).not.toHaveBeenCalled();

    // User B takes over — UID changed, cache must be cleared
    await act(async () => {
      authStateCallback?.({ uid: "user-b", email: "b@example.com" });
    });
    expect(screen.getByTestId("user").textContent).toBe("b@example.com");
    expect(clearSpy).toHaveBeenCalledTimes(1);
  });

  it("does not clear query cache on initial login", async () => {
    const { queryClient } = renderWithProviders();
    await flushAuthInit();
    const clearSpy = vi.spyOn(queryClient, "clear");

    // First auth event — prevUid starts as null, no clear expected
    await act(async () => {
      authStateCallback?.({ uid: "u1", email: "test@example.com" });
    });
    expect(clearSpy).not.toHaveBeenCalled();
  });

  it("records a session hint in localStorage when a user is present", async () => {
    renderWithProviders();
    await flushAuthInit();

    await act(async () => {
      authStateCallback?.({ uid: "u1", email: "test@example.com" });
    });

    expect(localStorage.getItem("cw:has-session")).toBe("1");
  });

  it("clears the session hint when auth resolves signed-out", async () => {
    localStorage.setItem("cw:has-session", "1");
    renderWithProviders();
    await flushAuthInit();

    await act(async () => {
      authStateCallback?.(null);
    });

    expect(localStorage.getItem("cw:has-session")).toBeNull();
  });

  it("clears query cache when switching from a user to signed-out", async () => {
    const { queryClient } = renderWithProviders();
    await flushAuthInit();
    const clearSpy = vi.spyOn(queryClient, "clear");

    // User logs in
    await act(async () => {
      authStateCallback?.({ uid: "u1", email: "test@example.com" });
    });
    expect(clearSpy).not.toHaveBeenCalled();

    // User signs out — UID changes from "u1" to null
    await act(async () => {
      authStateCallback?.(null);
    });
    expect(clearSpy).toHaveBeenCalledTimes(1);
  });
});
