import { Link } from "react-router";
import {
  Plus,
  Play,
  Pause,
  Trash2,
  List,
  Search as SearchIcon,
  Activity,
} from "lucide-react";
import {
  useSearches,
  useDeleteSearch,
  usePauseSearch,
  useResumeSearch,
} from "@/hooks/useSearches";
import { formatPrice, formatKm, cn } from "@/lib/utils";
import { useState } from "react";
import type { Search } from "@/lib/api";

export function SearchesPage() {
  const { data: searches, isLoading, isError } = useSearches();
  const deleteSearch = useDeleteSearch();
  const pauseSearch = usePauseSearch();
  const resumeSearch = useResumeSearch();
  const isMutating =
    deleteSearch.isPending || pauseSearch.isPending || resumeSearch.isPending;
  const [confirmDelete, setConfirmDelete] = useState<number | null>(null);

  if (isLoading) {
    return (
      <div className="space-y-6">
        <div className="h-8 w-48 shimmer-skeleton rounded-lg" />
        <div className="grid gap-3 sm:grid-cols-3">
          {[1, 2, 3].map((i) => (
            <div key={i} className="h-24 shimmer-skeleton rounded-2xl" />
          ))}
        </div>
        <div className="grid gap-4 sm:grid-cols-2">
          {[1, 2].map((i) => (
            <div key={i} className="h-52 shimmer-skeleton rounded-2xl" />
          ))}
        </div>
      </div>
    );
  }

  if (isError) {
    return (
      <div className="space-y-6">
        <h1 className="text-2xl font-semibold tracking-tight">
          החיפושים שלי
        </h1>
        <div className="rounded-2xl border border-destructive/20 bg-destructive/5 p-8 text-center">
          <p className="text-destructive font-medium">
            שגיאה בטעינת החיפושים
          </p>
          <p className="text-sm text-muted-foreground mt-1">נסה לרענן את הדף</p>
        </div>
      </div>
    );
  }

  const activeCount = searches?.filter((s) => s.active).length ?? 0;
  const pausedCount = (searches?.length ?? 0) - activeCount;

  return (
    <div className="space-y-6">
      {/* Stats row */}
      {searches && searches.length > 0 && (
        <div className="grid grid-cols-3 gap-3">
          <StatCard
            icon={SearchIcon}
            value={searches.length}
            label="סה״כ חיפושים"
            color="text-primary"
          />
          <StatCard
            icon={Activity}
            value={activeCount}
            label="פעילים"
            color="text-score-great"
          />
          <StatCard
            icon={Pause}
            value={pausedCount}
            label="מושהים"
            color="text-score-good"
          />
        </div>
      )}

      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold tracking-tight">
          החיפושים שלי
        </h1>
        <Link
          to="/searches/new"
          className="inline-flex items-center gap-2 rounded-xl bg-primary px-5 py-2.5 text-sm font-medium text-primary-foreground shadow-[0_0_20px_rgba(59,130,246,0.25)] transition-all duration-200 hover:bg-primary/90 hover:shadow-[0_0_30px_rgba(59,130,246,0.35)] active:scale-[0.98]"
        >
          <Plus className="h-4 w-4" />
          חיפוש חדש
        </Link>
      </div>

      {/* Search cards */}
      {!searches || searches.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-2xl border border-border/50 bg-card/50 py-20">
          <SearchIcon className="h-10 w-10 text-muted-foreground/20 mb-4" />
          <p className="text-muted-foreground mb-5">
            אין חיפושים פעילים עדיין
          </p>
          <Link
            to="/searches/new"
            className="inline-flex items-center gap-2 rounded-xl bg-primary px-5 py-2.5 text-sm font-medium text-primary-foreground shadow-[0_0_20px_rgba(59,130,246,0.25)] transition-all duration-200 hover:bg-primary/90 hover:shadow-[0_0_30px_rgba(59,130,246,0.35)] active:scale-[0.98]"
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

function StatCard({
  icon: Icon,
  value,
  label,
  color,
}: {
  icon: React.ComponentType<{ className?: string }>;
  value: number;
  label: string;
  color: string;
}) {
  return (
    <div className="rounded-2xl border border-border/50 bg-card p-4 text-center transition-colors duration-200 hover:border-border">
      <Icon className={cn("mx-auto h-5 w-5 mb-1.5", color)} />
      <p className="text-2xl font-bold tabular-nums">{value}</p>
      <p className="text-xs text-muted-foreground mt-0.5">{label}</p>
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
    <div className="group rounded-2xl border border-border/50 bg-card p-5 transition-all duration-200 hover:border-border hover:shadow-[0_4px_24px_rgba(0,0,0,0.3)]">
      <div className="flex items-start justify-between mb-3">
        <div>
          <h3 className="text-lg font-semibold">
            {search.manufacturer_name} {search.model_name}
          </h3>
          <span className="text-xs text-muted-foreground">{search.source}</span>
        </div>
        <span
          className={cn(
            "inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-semibold",
            search.active
              ? "bg-score-great/15 text-score-great"
              : "bg-score-good/15 text-score-good",
          )}
        >
          {search.active ? "פעיל" : "מושהה"}
        </span>
      </div>

      <div className="grid grid-cols-2 gap-2 text-sm text-muted-foreground mb-4">
        <div>
          שנים: {search.year_min}–{search.year_max}
        </div>
        {search.price_max > 0 && (
          <div className="tabular-nums">עד {formatPrice(search.price_max)}</div>
        )}
        {search.max_km > 0 && (
          <div className="tabular-nums">עד {formatKm(search.max_km)}</div>
        )}
        {search.max_hand > 0 && <div>עד יד {search.max_hand}</div>}
      </div>

      <div className="flex items-center gap-2 border-t border-border/50 pt-3">
        <Link
          to={`/searches/${search.id}/listings`}
          className="inline-flex items-center gap-1.5 rounded-lg bg-secondary px-3 py-1.5 text-xs font-medium text-secondary-foreground transition-all duration-200 hover:bg-accent active:scale-[0.97]"
        >
          <List className="h-3.5 w-3.5" />
          תוצאות
        </Link>

        {search.active ? (
          <button
            onClick={onPause}
            disabled={disabled}
            className="inline-flex items-center gap-1.5 rounded-lg bg-secondary px-3 py-1.5 text-xs font-medium text-secondary-foreground transition-all duration-200 hover:bg-accent disabled:opacity-50 active:scale-[0.97]"
          >
            <Pause className="h-3.5 w-3.5" />
            השהה
          </button>
        ) : (
          <button
            onClick={onResume}
            disabled={disabled}
            className="inline-flex items-center gap-1.5 rounded-lg bg-secondary px-3 py-1.5 text-xs font-medium text-secondary-foreground transition-all duration-200 hover:bg-accent disabled:opacity-50 active:scale-[0.97]"
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
                className="rounded-lg bg-destructive px-3 py-1.5 text-xs font-medium text-destructive-foreground transition-all duration-200 hover:bg-destructive/90 active:scale-[0.97]"
              >
                אישור
              </button>
              <button
                onClick={onCancelDelete}
                className="rounded-lg bg-secondary px-3 py-1.5 text-xs font-medium text-secondary-foreground transition-all duration-200 hover:bg-accent"
              >
                ביטול
              </button>
            </div>
          ) : (
            <button
              onClick={onDelete}
              className="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium text-destructive transition-colors duration-200 hover:bg-destructive/10"
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
