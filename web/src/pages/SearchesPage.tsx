import { Link } from "react-router";
import {
  Plus,
  Play,
  Pause,
  Trash2,
  List,
  Search as SearchIcon,
  Activity,
  Zap,
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
      <div className="space-y-6 animate-fade-in">
        <div className="grid gap-4 sm:grid-cols-3">
          {[1, 2, 3].map((i) => (
            <div key={i} className="h-28 rounded-2xl skeleton" />
          ))}
        </div>
        <div className="h-8 w-48 rounded-lg skeleton" />
        <div className="grid gap-4 sm:grid-cols-2">
          {[1, 2].map((i) => (
            <div key={i} className="h-52 rounded-2xl skeleton" />
          ))}
        </div>
      </div>
    );
  }

  if (isError) {
    return (
      <div className="space-y-6 animate-fade-in">
        <h1 className="text-2xl font-bold">החיפושים שלי</h1>
        <div className="rounded-2xl border border-destructive/30 bg-destructive/5 p-8 text-center">
          <p className="text-destructive font-semibold text-lg">
            שגיאה בטעינת החיפושים
          </p>
          <p className="text-sm text-muted-foreground mt-2">נסה לרענן את הדף</p>
        </div>
      </div>
    );
  }

  const activeCount = searches?.filter((s) => s.active).length ?? 0;
  const pausedCount = (searches?.length ?? 0) - activeCount;

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Stats hero */}
      {searches && searches.length > 0 && (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
          <StatCard
            icon={SearchIcon}
            value={searches.length}
            label="סה״כ חיפושים"
            gradient="from-blue-500/10 to-indigo-500/10 dark:from-blue-500/20 dark:to-indigo-500/20"
            iconColor="text-blue-600 dark:text-blue-400"
            className="stagger-1 animate-slide-up"
          />
          <StatCard
            icon={Activity}
            value={activeCount}
            label="פעילים"
            gradient="from-emerald-500/10 to-green-500/10 dark:from-emerald-500/20 dark:to-green-500/20"
            iconColor="text-emerald-600 dark:text-emerald-400"
            className="stagger-2 animate-slide-up"
          />
          <StatCard
            icon={Pause}
            value={pausedCount}
            label="מושהים"
            gradient="from-amber-500/10 to-orange-500/10 dark:from-amber-500/20 dark:to-orange-500/20"
            iconColor="text-amber-600 dark:text-amber-400"
            className="col-span-2 sm:col-span-1 stagger-3 animate-slide-up"
          />
        </div>
      )}

      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <h1 className="text-2xl font-bold tracking-tight">החיפושים שלי</h1>
        </div>
        <Link
          to="/searches/new"
          className="inline-flex items-center gap-2 rounded-xl gradient-primary px-5 py-2.5 text-sm font-semibold text-white shadow-md hover:shadow-lg hover:brightness-110 transition-all duration-200"
        >
          <Plus className="h-4 w-4" />
          חיפוש חדש
        </Link>
      </div>

      {/* Empty state */}
      {!searches || searches.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-2xl border-2 border-dashed border-border py-20 animate-fade-in">
          <div className="flex h-16 w-16 items-center justify-center rounded-2xl bg-primary/10 mb-4">
            <Zap className="h-8 w-8 text-primary" />
          </div>
          <p className="text-lg font-semibold mb-1">אין חיפושים עדיין</p>
          <p className="text-muted-foreground mb-6">
            צור חיפוש ראשון ונתחיל למצוא לך רכב
          </p>
          <Link
            to="/searches/new"
            className="inline-flex items-center gap-2 rounded-xl gradient-primary px-6 py-3 text-sm font-semibold text-white shadow-md hover:shadow-lg hover:brightness-110 transition-all"
          >
            <Plus className="h-4 w-4" />
            צור חיפוש ראשון
          </Link>
        </div>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2">
          {searches.map((search, i) => (
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
              className={`stagger-${Math.min(i + 1, 6)} animate-slide-up`}
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
  gradient,
  iconColor,
  className,
}: {
  icon: React.ComponentType<{ className?: string }>;
  value: number;
  label: string;
  gradient: string;
  iconColor: string;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "rounded-2xl border border-border bg-gradient-to-br p-5 card-shine",
        gradient,
        className,
      )}
    >
      <Icon className={cn("h-5 w-5 mb-2", iconColor)} />
      <p className="text-3xl font-bold tracking-tight">{value}</p>
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
  className,
}: {
  search: Search;
  disabled: boolean;
  onPause: () => void;
  onResume: () => void;
  onDelete: () => void;
  isConfirmingDelete: boolean;
  onCancelDelete: () => void;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "rounded-2xl border border-border bg-card p-5 shadow-sm hover-lift gradient-card",
        className,
      )}
    >
      <div className="flex items-start justify-between mb-4">
        <div>
          <h3 className="text-lg font-bold tracking-tight">
            {search.manufacturer_name} {search.model_name}
          </h3>
          <span className="text-xs text-muted-foreground font-medium uppercase tracking-wide">
            {search.source}
          </span>
        </div>
        <span
          className={cn(
            "inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-semibold",
            search.active
              ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-400"
              : "bg-amber-100 text-amber-700 dark:bg-amber-500/20 dark:text-amber-400",
          )}
        >
          <span
            className={cn(
              "h-1.5 w-1.5 rounded-full",
              search.active ? "bg-emerald-500" : "bg-amber-500",
            )}
          />
          {search.active ? "פעיל" : "מושהה"}
        </span>
      </div>

      <div className="grid grid-cols-2 gap-2 text-sm text-muted-foreground mb-4">
        <div className="flex items-center gap-1.5">
          <span className="text-foreground font-medium">שנים:</span>
          {search.year_min}–{search.year_max}
        </div>
        {search.price_max > 0 && (
          <div>
            <span className="text-foreground font-medium">עד</span>{" "}
            {formatPrice(search.price_max)}
          </div>
        )}
        {search.max_km > 0 && (
          <div>
            <span className="text-foreground font-medium">עד</span>{" "}
            {formatKm(search.max_km)}
          </div>
        )}
        {search.max_hand > 0 && (
          <div>
            <span className="text-foreground font-medium">עד</span> יד{" "}
            {search.max_hand}
          </div>
        )}
      </div>

      <div className="flex items-center gap-2 border-t border-border pt-4">
        <Link
          to={`/searches/${search.id}/listings`}
          className="inline-flex items-center gap-1.5 rounded-xl bg-primary/10 px-3 py-1.5 text-xs font-semibold text-primary hover:bg-primary/20 transition-colors duration-200"
        >
          <List className="h-3.5 w-3.5" />
          תוצאות
        </Link>

        {search.active ? (
          <button
            onClick={onPause}
            disabled={disabled}
            className="inline-flex items-center gap-1.5 rounded-xl bg-secondary px-3 py-1.5 text-xs font-medium text-secondary-foreground hover:bg-secondary/80 transition-colors disabled:opacity-50"
          >
            <Pause className="h-3.5 w-3.5" />
            השהה
          </button>
        ) : (
          <button
            onClick={onResume}
            disabled={disabled}
            className="inline-flex items-center gap-1.5 rounded-xl bg-secondary px-3 py-1.5 text-xs font-medium text-secondary-foreground hover:bg-secondary/80 transition-colors disabled:opacity-50"
          >
            <Play className="h-3.5 w-3.5" />
            חדש
          </button>
        )}

        <div className="mr-auto">
          {isConfirmingDelete ? (
            <div className="flex items-center gap-1.5 animate-scale-in">
              <button
                onClick={onDelete}
                className="rounded-xl bg-destructive px-3 py-1.5 text-xs font-semibold text-destructive-foreground hover:bg-destructive/90 transition-colors"
              >
                אישור
              </button>
              <button
                onClick={onCancelDelete}
                className="rounded-xl bg-secondary px-3 py-1.5 text-xs font-medium text-secondary-foreground hover:bg-secondary/80 transition-colors"
              >
                ביטול
              </button>
            </div>
          ) : (
            <button
              onClick={onDelete}
              className="inline-flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-medium text-destructive hover:bg-destructive/10 transition-colors"
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
