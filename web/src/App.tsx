import { Routes, Route } from "react-router";
import { Shell } from "./components/layout/Shell";
import { SearchesPage } from "./pages/SearchesPage";
import { NewSearchPage } from "./pages/NewSearchPage";
import { ListingsPage } from "./pages/ListingsPage";
import { ListingDetailPage } from "./pages/ListingDetailPage";
import { AdminPage } from "./pages/AdminPage";

export default function App() {
  return (
    <Routes>
      <Route element={<Shell />}>
        <Route path="/" element={<SearchesPage />} />
        <Route path="/searches/new" element={<NewSearchPage />} />
        <Route path="/searches/:id/listings" element={<ListingsPage />} />
        <Route path="/listings/:token" element={<ListingDetailPage />} />
        <Route path="/admin" element={<AdminPage />} />
      </Route>
    </Routes>
  );
}
