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

  // Firebase Auth (~160KB) is imported lazily so it never blocks first paint on
  // public routes (landing, /try, auth-less visits). The auth-state subscription
  // is established as soon as the chunk resolves.
  useEffect(() => {
    let cancelled = false;
    let unsubscribe = () => {};

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
      });
    })();

    return () => {
      cancelled = true;
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
