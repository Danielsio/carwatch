import { lazy, Suspense } from "react";
import { Routes, Route } from "react-router";
import { Loader2 } from "lucide-react";
import { Shell } from "./components/layout/Shell";
import { ProtectedRoute } from "./components/ProtectedRoute";

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
const NotFoundPage = lazy(() =>
  import("./pages/NotFoundPage").then((m) => ({ default: m.NotFoundPage })),
);
const SettingsPage = lazy(() =>
  import("./pages/SettingsPage").then((m) => ({ default: m.SettingsPage })),
);

function PageFallback() {
  return (
    <div className="flex min-h-[40vh] items-center justify-center">
      <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
    </div>
  );
}

export default function App() {
  return (
    <Suspense fallback={<PageFallback />}>
      <Routes>
        <Route path="/" element={<LandingPage />} />
        <Route path="/login" element={<LoginPage />} />
        <Route path="/signup" element={<SignupPage />} />
        <Route element={<ProtectedRoute />}>
          <Route element={<Shell />}>
            <Route path="/dashboard" element={<SearchesPage />} />
            <Route path="/searches/new" element={<NewSearchPage />} />
            <Route path="/searches/:id/edit" element={<EditSearchPage />} />
            <Route path="/searches/:id/listings" element={<ListingsPage />} />
            <Route path="/listings/:token" element={<ListingDetailPage />} />
            <Route path="/settings" element={<SettingsPage />} />
            <Route path="/admin" element={<AdminPage />} />
            <Route path="/saved" element={<SavedPage />} />
            <Route path="/history" element={<HistoryPage />} />
            <Route path="/notifications" element={<NotificationsPage />} />
            <Route path="*" element={<NotFoundPage />} />
          </Route>
        </Route>
      </Routes>
    </Suspense>
  );
}
