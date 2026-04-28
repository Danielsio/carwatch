import { Link } from "react-router";
import { Plus, Play, Pause, Trash2, List, Search as SearchIcon, Activity } from "lucide-react";
import {
  useSearches,
  useDeleteSearch,
  usePauseSearch,
  useResumeSearch,
} from "@/hooks/useSearches";
import { formatPrice, formatKm } from "@/lib/utils";
import { useState } from "react";
import type { Search } from "@/lib/api";

export function SearchesPage() {
  const { data: searches, isLoading, isError } = useSearches();
  const deleteSearch = useDeleteSearch();
  const pauseSearch = usePauseSearch();
  const resumeSearch = useResumeSearch();
  const isMutating = deleteSearch.isPending || pauseSearch.isPending || resumeSearch.isPending;
  const [confirmDelete, setConfirmDelete] = useState<number | null>(null);

  if (isLoading) {
    return (
      <div className="space-y-4">
        <h1 className="text-2xl font-bold">החיפושים שלי</h1>
        <div className="grid gap-4 sm:grid-cols-2">
          {[1, 2].map((i) => (
            <div
              key={i}
              className="h-48 animate-pulse rounded-xl bg-muted"
            />
          ))}
        </div>
      </div>
    );
  }

  if (isError) {
    return (
      <div className="space-y-4">
        <h1 className="text-2xl font-bold">החיפושים שלי</h1>
        <div className="rounded-xl border border-destructive/50 bg-destructive/10 p-6 text-center">
          <p className="text-destructive font-medium">שגיאה בטעינת החיפושים</p>
          <p className="text-sm text-muted-foreground mt-1">נסה לרענן את הדף</p>
        </div>
      </div>
    );
  }

  const activeCount = searches?.filter((s) => s.active).length ?? 0;
  const pausedCount = (searches?.length ?? 0) - activeCount;

  return (
    <div className="space-y-4">
      {/* Stats summary */}
      {searches && searches.length > 0 && (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
          <div className="rounded-xl border border-border bg-card p-4 text-center">
            <SearchIcon className="mx-auto h-5 w-5 text-primary mb-1" />
            <p className="text-2xl font-bold">{searches.length}</p>
            <p className="text-xs text-muted-foreground">סה״כ חיפושים</p>
          </div>
          <div className="rounded-xl border border-border bg-card p-4 text-center">
            <Activity className="mx-auto h-5 w-5 text-green-600 mb-1" />
            <p className="text-2xl font-bold">{activeCount}</p>
            <p className="text-xs text-muted-foreground">פעילים</p>
          </div>
          <div className="rounded-xl border border-border bg-card p-4 text-center">
            <Pause className="mx-auto h-5 w-5 text-yellow-600 mb-1" />
            <p className="text-2xl font-bold">{pausedCount}</p>
            <p className="text-xs text-muted-foreground">מושהים</p>
          </div>
        </div>
      )}

      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">החיפושים שלי</h1>
        <Link
          to="/searches/new"
          className="inline-flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 transition-colors"
        >
          <Plus className="h-4 w-4" />
          חיפוש חדש
        </Link>
      </div>

      {!searches || searches.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border py-16">
          <p className="text-muted-foreground mb-4">
            אין חיפושים פעילים עדיין
          </p>
          <Link
            to="/searches/new"
            className="inline-flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
          >
            <Plus className="h-4 w-4" />
            צור חיפוש ראשון
          </Link>
        </div>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2">
          {searches.map((search) => (
            <SearchCard
              key={search.id}
              search={search}
              disabled={isMutating}
              onPause={() => pauseSearch.mutate(search.id)}
              onResume={() => resumeSearch.mutate(search.id)}
              onDelete={() => {
                if (confirmDelete === search.id) {
                  deleteSearch.mutate(search.id);
                  setConfirmDelete(null);
                } else {
                  setConfirmDelete(search.id);
                }
              }}
              isConfirmingDelete={confirmDelete === search.id}
              onCancelDelete={() => setConfirmDelete(null)}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function SearchCard({
  search,
  disabled,
  onPause,
  onResume,
  onDelete,
  isConfirmingDelete,
  onCancelDelete,
}: {
  search: Search;
  disabled: boolean;
  onPause: () => void;
  onResume: () => void;
  onDelete: () => void;
  isConfirmingDelete: boolean;
  onCancelDelete: () => void;
}) {
  return (
    <div className="rounded-xl border border-border bg-card p-5 shadow-sm">
      <div className="flex items-start justify-between mb-3">
        <div>
          <h3 className="text-lg font-semibold">
            {search.manufacturer_name} {search.model_name}
          </h3>
          <span className="text-xs text-muted-foreground">{search.source}</span>
        </div>
        <span
          className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${
            search.active
              ? "bg-green-100 text-green-800"
              : "bg-yellow-100 text-yellow-800"
          }`}
        >
          {search.active ? "פעיל" : "מושהה"}
        </span>
      </div>

      <div className="grid grid-cols-2 gap-2 text-sm text-muted-foreground mb-4">
        <div>
          שנים: {search.year_min}–{search.year_max}
        </div>
        {search.price_max > 0 && (
          <div>עד {formatPrice(search.price_max)}</div>
        )}
        {search.max_km > 0 && <div>עד {formatKm(search.max_km)}</div>}
        {search.max_hand > 0 && <div>עד יד {search.max_hand}</div>}
      </div>

      <div className="flex items-center gap-2 border-t border-border pt-3">
        <Link
          to={`/searches/${search.id}/listings`}
          className="inline-flex items-center gap-1.5 rounded-lg bg-secondary px-3 py-1.5 text-xs font-medium text-secondary-foreground hover:bg-secondary/80 transition-colors"
        >
          <List className="h-3.5 w-3.5" />
          תוצאות
        </Link>

        {search.active ? (
          <button
            onClick={onPause}
            disabled={disabled}
            className="inline-flex items-center gap-1.5 rounded-lg bg-secondary px-3 py-1.5 text-xs font-medium text-secondary-foreground hover:bg-secondary/80 transition-colors disabled:opacity-50"
          >
            <Pause className="h-3.5 w-3.5" />
            השהה
          </button>
        ) : (
          <button
            onClick={onResume}
            disabled={disabled}
            className="inline-flex items-center gap-1.5 rounded-lg bg-secondary px-3 py-1.5 text-xs font-medium text-secondary-foreground hover:bg-secondary/80 transition-colors disabled:opacity-50"
          >
            <Play className="h-3.5 w-3.5" />
            חדש
          </button>
        )}

        <div className="mr-auto">
          {isConfirmingDelete ? (
            <div className="flex items-center gap-1">
              <button
                onClick={onDelete}
                className="rounded-lg bg-destructive px-3 py-1.5 text-xs font-medium text-destructive-foreground hover:bg-destructive/90 transition-colors"
              >
                אישור
              </button>
              <button
                onClick={onCancelDelete}
                className="rounded-lg bg-secondary px-3 py-1.5 text-xs font-medium text-secondary-foreground hover:bg-secondary/80 transition-colors"
              >
                ביטול
              </button>
            </div>
          ) : (
            <button
              onClick={onDelete}
              className="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium text-destructive hover:bg-destructive/10 transition-colors"
            >
              <Trash2 className="h-3.5 w-3.5" />
              מחק
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
