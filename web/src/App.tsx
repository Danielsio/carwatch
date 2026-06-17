import { lazy, Suspense } from "react";
import type { ReactNode } from "react";
import { Routes, Route, Navigate } from "react-router";
import { Loader2 } from "lucide-react";
import { ProtectedRoute } from "./components/ProtectedRoute";
import { ErrorBoundary } from "./components/ErrorBoundary";
import { useMe } from "./hooks/useMe";

const LandingPage = lazy(() =>
  import("./pages/LandingPage").then((m) => ({ default: m.LandingPage })),
);
const LoginPage = lazy(() =>
  import("./pages/LoginPage").then((m) => ({ default: m.LoginPage })),
);
const SignupPage = lazy(() =>
  import("./pages/SignupPage").then((m) => ({ default: m.SignupPage })),
);
const SearchesPage = lazy(() =>
  import("./pages/SearchesPage").then((m) => ({ default: m.SearchesPage })),
);
const NewSearchPage = lazy(() =>
  import("./pages/NewSearchPage").then((m) => ({ default: m.NewSearchPage })),
);
const ListingsPage = lazy(() =>
  import("./pages/ListingsPage").then((m) => ({ default: m.ListingsPage })),
);
const ListingDetailPage = lazy(() =>
  import("./pages/ListingDetailPage").then((m) => ({
    default: m.ListingDetailPage,
  })),
);
const AdminPage = lazy(() =>
  import("./pages/AdminPage").then((m) => ({ default: m.AdminPage })),
);
const SavedPage = lazy(() =>
  import("./pages/SavedPage").then((m) => ({ default: m.SavedPage })),
);
const HistoryPage = lazy(() =>
  import("./pages/HistoryPage").then((m) => ({ default: m.HistoryPage })),
);
const NotificationsPage = lazy(() =>
  import("./pages/NotificationsPage").then((m) => ({
    default: m.NotificationsPage,
  })),
);
const EditSearchPage = lazy(() =>
  import("./pages/EditSearchPage").then((m) => ({
    default: m.EditSearchPage,
  })),
);
const TrySearchPage = lazy(() => import("./pages/TrySearchPage"));
const NotFoundPage = lazy(() =>
  import("./pages/NotFoundPage").then((m) => ({ default: m.NotFoundPage })),
);
const SettingsPage = lazy(() =>
  import("./pages/SettingsPage").then((m) => ({ default: m.SettingsPage })),
);
// Shell is the authenticated app chrome (nav, command palette). Lazy-loading it
// keeps its base-ui/cmdk deps off the public landing's first-paint path.
const Shell = lazy(() =>
  import("./components/layout/Shell").then((m) => ({ default: m.Shell })),
);

function AdminGuard({ children }: { children: React.ReactNode }) {
  const { data: me, isLoading, isError } = useMe();
  if (isLoading) return <PageFallback />;
  if (isError) {
    return (
      <div className="flex min-h-[40vh] flex-col items-center justify-center gap-2 text-muted-foreground">
        <span>שגיאה בבדיקת הרשאות</span>
      </div>
    );
  }
  if (!me?.is_admin) return <Navigate to="/dashboard" replace />;
  return <>{children}</>;
}

function PageFallback() {
  return (
    <div className="flex min-h-[40vh] items-center justify-center">
      <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
    </div>
  );
}

function RouteGuard({ children }: { children: ReactNode }) {
  return <ErrorBoundary routeLevel>{children}</ErrorBoundary>;
}

export default function App() {
  return (
    <Suspense fallback={<PageFallback />}>
      <Routes>
        <Route path="/" element={<RouteGuard><LandingPage /></RouteGuard>} />
        <Route path="/login" element={<RouteGuard><LoginPage /></RouteGuard>} />
        <Route path="/signup" element={<RouteGuard><SignupPage /></RouteGuard>} />
        <Route path="/try" element={<RouteGuard><Suspense fallback={<PageFallback />}><TrySearchPage /></Suspense></RouteGuard>} />
        <Route element={<ProtectedRoute />}>
          <Route element={<Shell />}>
            <Route path="/dashboard" element={<RouteGuard><SearchesPage /></RouteGuard>} />
            <Route path="/searches/new" element={<RouteGuard><NewSearchPage /></RouteGuard>} />
            <Route path="/searches/:id/edit" element={<RouteGuard><EditSearchPage /></RouteGuard>} />
            <Route path="/searches/:id/listings" element={<RouteGuard><ListingsPage /></RouteGuard>} />
            <Route path="/listings/:token" element={<RouteGuard><ListingDetailPage /></RouteGuard>} />
            <Route path="/settings" element={<RouteGuard><SettingsPage /></RouteGuard>} />
            <Route path="/admin" element={<AdminGuard><RouteGuard><AdminPage /></RouteGuard></AdminGuard>} />
            <Route path="/saved" element={<RouteGuard><SavedPage /></RouteGuard>} />
            <Route path="/history" element={<RouteGuard><HistoryPage /></RouteGuard>} />
            <Route path="/notifications" element={<RouteGuard><NotificationsPage /></RouteGuard>} />
          </Route>
        </Route>
        <Route path="*" element={<RouteGuard><NotFoundPage /></RouteGuard>} />
      </Routes>
    </Suspense>
  );
}
