import { Routes, Route } from "react-router";
import { Shell } from "./components/layout/Shell";
import { SearchesPage } from "./pages/SearchesPage";
import { NewSearchPage } from "./pages/NewSearchPage";
import { ListingsPage } from "./pages/ListingsPage";

export default function App() {
  return (
    <Routes>
      <Route element={<Shell />}>
        <Route path="/" element={<SearchesPage />} />
        <Route path="/searches/new" element={<NewSearchPage />} />
        <Route path="/searches/:id/listings" element={<ListingsPage />} />
      </Route>
    </Routes>
  );
}
