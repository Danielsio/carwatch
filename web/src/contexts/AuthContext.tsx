import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import type { Auth, User } from "firebase/auth";
import { useQueryClient } from "@tanstack/react-query";
import { setAuthTokenGetter } from "@/lib/auth-token";
import { reportWebVitals } from "@/lib/webVitals";

type AuthContextValue = {
  user: User | null;
  loading: boolean;
  signOut: () => Promise<void>;
  getIdToken: (forceRefresh?: boolean) => Promise<string | null>;
};

const AuthContext = createContext<AuthContextValue | null>(null);

const AUTH_HINT_KEY = "cw:has-session";

// Best-effort flag (NOT a security boundary) recording whether the last known
// auth state had a signed-in user. It lets a fresh page load decide whether to
// fetch the Firebase SDK eagerly (returning user → restore session fast) or
// defer it to idle (anonymous visitor → don't compete with landing hydration).
// Wrapped in try/catch since localStorage throws in private mode / when storage
// is disabled.
function hasAuthHint(): boolean {
  try {
    return localStorage.getItem(AUTH_HINT_KEY) === "1";
  } catch {
    return false;
  }
}

function rememberAuthHint(hasSession: boolean): void {
  try {
    if (hasSession) localStorage.setItem(AUTH_HINT_KEY, "1");
    else localStorage.removeItem(AUTH_HINT_KEY);
  } catch {
    /* ignore storage failures */
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const queryClient = useQueryClient();
  const prevUidRef = useRef<string | null>(null);
  const authRef = useRef<Auth | null>(null);
  const vitalsStartedRef = useRef(false);

  // Token getter reads the live Firebase instance once it has loaded. Registered
  // synchronously so any API call made before Firebase finishes loading simply
  // gets a null token (same as signed-out) rather than throwing.
  useEffect(() => {
    setAuthTokenGetter(async (forceRefresh?: boolean) => {
      const u = authRef.current?.currentUser;
      if (!u) return null;
      return u.getIdToken(forceRefresh);
    });
    if (!vitalsStartedRef.current) {
      vitalsStartedRef.current = true;
      void reportWebVitals();
    }
    return () => {
      setAuthTokenGetter(async () => null);
    };
  }, []);

  // Firebase Auth (~233KB) is imported lazily so it never blocks first paint on
  // public routes (landing, /try, auth-less visits). The auth-state subscription
  // is established as soon as the chunk resolves.
  useEffect(() => {
    let cancelled = false;
    let idleHandle: number | null = null;
    let unsubscribe = () => {};

    const start = () => {
      void (async () => {
        const [{ auth }, { onAuthStateChanged }] = await Promise.all([
          import("@/lib/firebase"),
          import("firebase/auth"),
        ]);
        if (cancelled) return;
        authRef.current = auth;
        unsubscribe = onAuthStateChanged(auth, (u) => {
          const newUid = u?.uid ?? null;
          if (prevUidRef.current !== null && prevUidRef.current !== newUid) {
            queryClient.clear();
          }
          prevUidRef.current = newUid;
          setUser(u);
          setLoading(false);
          rememberAuthHint(newUid !== null);
        });
      })();
    };

    // Returning users (and browsers without requestIdleCallback, e.g. older
    // Safari) load Firebase right away so a persisted session restores without
    // delay. First-time / signed-out visitors defer the SDK to idle so its
    // download + parse never competes with landing hydration on slow mobiles.
    if (hasAuthHint() || !("requestIdleCallback" in window)) {
      start();
    } else {
      idleHandle = requestIdleCallback(start, { timeout: 3000 });
    }

    return () => {
      cancelled = true;
      if (idleHandle !== null && "cancelIdleCallback" in window) {
        cancelIdleCallback(idleHandle);
      }
      unsubscribe();
    };
  }, [queryClient]);

  const signOut = useCallback(async () => {
    const auth = authRef.current;
    if (!auth) return;
    const { signOut: firebaseSignOut } = await import("firebase/auth");
    await firebaseSignOut(auth);
  }, []);

  const getIdToken = useCallback(async (forceRefresh?: boolean) => {
    const u = authRef.current?.currentUser;
    if (!u) return null;
    return u.getIdToken(forceRefresh);
  }, []);

  const value = useMemo(
    () => ({ user, loading, signOut, getIdToken }),
    [user, loading, signOut, getIdToken],
  );

  // Always render children — public pages (landing, /try, /login, /signup)
  // should never be blocked by the auth loading spinner. The loading gate lives
  // in ProtectedRoute, which shows a spinner only for routes that require auth.
  return (
    <AuthContext.Provider value={value}>
      {children}
    </AuthContext.Provider>
  );
}

// eslint-disable-next-line react-refresh/only-export-components -- context hook
export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth must be used within AuthProvider");
  }
  return ctx;
}
