import { Routes, Route } from "react-router";
import { Shell } from "./components/layout/Shell";
import { ProtectedRoute } from "./components/ProtectedRoute";
import { SearchesPage } from "./pages/SearchesPage";
import { NewSearchPage } from "./pages/NewSearchPage";
import { ListingsPage } from "./pages/ListingsPage";
import { ListingDetailPage } from "./pages/ListingDetailPage";
import { AdminPage } from "./pages/AdminPage";
import { SavedPage } from "./pages/SavedPage";
import { HistoryPage } from "./pages/HistoryPage";
import { NotificationsPage } from "./pages/NotificationsPage";
import { LoginPage } from "./pages/LoginPage";
import { SignupPage } from "./pages/SignupPage";
import { EditSearchPage } from "./pages/EditSearchPage";
import { NotFoundPage } from "./pages/NotFoundPage";
import { LandingPage } from "./pages/LandingPage";
import { SettingsPage } from "./pages/SettingsPage";

export default function App() {
  return (
    <Routes>
      <Route path="/welcome" element={<LandingPage />} />
      <Route path="/login" element={<LoginPage />} />
      <Route path="/signup" element={<SignupPage />} />
      <Route element={<ProtectedRoute />}>
        <Route element={<Shell />}>
          <Route path="/" element={<SearchesPage />} />
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
  );
}
