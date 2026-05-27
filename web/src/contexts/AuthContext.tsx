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
import type { User } from "firebase/auth";
import { onAuthStateChanged, signOut as firebaseSignOut } from "firebase/auth";
import { useQueryClient } from "@tanstack/react-query";
import { auth } from "@/lib/firebase";
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
  const vitalsStartedRef = useRef(false);

  useEffect(() => {
    setAuthTokenGetter(async (forceRefresh?: boolean) => {
      const u = auth.currentUser;
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

  useEffect(() => {
    return onAuthStateChanged(auth, (u) => {
      const newUid = u?.uid ?? null;
      if (prevUidRef.current !== null && prevUidRef.current !== newUid) {
        queryClient.clear();
      }
      prevUidRef.current = newUid;
      setUser(u);
      setLoading(false);
    });
  }, [queryClient]);

  const signOut = useCallback(() => firebaseSignOut(auth), []);

  const getIdToken = useCallback(async (forceRefresh?: boolean) => {
    const u = auth.currentUser;
    if (!u) return null;
    return u.getIdToken(forceRefresh);
  }, []);

  const value = useMemo(
    () => ({ user, loading, signOut, getIdToken }),
    [user, loading, signOut, getIdToken],
  );

  // Always render children â public pages (landing, /try, /login, /signup)
  // should never be blocked by the auth loading spinner. The loading gate
  // now lives in ProtectedRoute, which shows a spinner only for routes
  // that actually require authentication.
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
