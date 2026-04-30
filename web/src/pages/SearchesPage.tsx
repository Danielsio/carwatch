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
import { PageHeader } from "@/components/ui/PageHeader";
import { Button } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";
import { EmptyState } from "@/components/ui/EmptyState";
import { Skeleton } from "@/components/ui/Skeleton";
import { useToast } from "@/components/ui/Toast";

export function SearchesPage() {
  const { toast } = useToast();
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
        <div className="flex items-start justify-between gap-4 border-b border-border/50 pb-6">
          <Skeleton className="h-8 w-48 rounded-lg" />
          <Skeleton className="h-10 w-36 rounded-md" />
        </div>
        <div className="grid gap-3 sm:grid-cols-3">
          {[1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-24 rounded-2xl" />
          ))}
        </div>
        <div className="grid gap-4 sm:grid-cols-2">
          {[1, 2].map((i) => (
            <Skeleton key={i} className="h-52 rounded-2xl" />
          ))}
        </div>
      </div>
    );
  }

  if (isError) {
    return (
      <div className="space-y-6">
        <PageHeader title="החיפושים שלי" />
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

      <PageHeader
        title="החיפושים שלי"
        action={
          <Button asChild>
            <Link to="/searches/new">
              <Plus className="h-4 w-4" />
              חיפוש חדש
            </Link>
          </Button>
        }
      />

      {/* Search cards */}
      {!searches || searches.length === 0 ? (
        <EmptyState
          icon={SearchIcon}
          title="אין חיפושים פעילים עדיין"
          action={
            <Button asChild>
              <Link to="/searches/new">
                <Plus className="h-4 w-4" />
                צור חיפוש ראשון
              </Link>
            </Button>
          }
        />
      ) : (
        <div className="grid gap-4 sm:grid-cols-2">
          {searches.map((search) => (
            <SearchCard
              key={search.id}
              search={search}
              disabled={isMutating}
              onPause={() =>
                pauseSearch.mutate(search.id, {
                  onSuccess: () => toast("החיפוש הושהה", "info"),
                })
              }
              onResume={() =>
                resumeSearch.mutate(search.id, {
                  onSuccess: () => toast("החיפוש חודש", "success"),
                })
              }
              onDelete={() => {
                if (confirmDelete === search.id) {
                  deleteSearch.mutate(search.id, {
                    onSuccess: () => toast("החיפוש נמחק", "success"),
                  });
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
        <Badge variant={search.active ? "success" : "warning"}>
          {search.active ? "פעיל" : "מושהה"}
        </Badge>
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
        <Button
          as={Link}
          to={`/searches/${search.id}/listings`}
          variant="secondary"
          size="sm"
        >
          <List className="h-3.5 w-3.5" />
          תוצאות
        </Button>

        {search.active ? (
          <Button
            type="button"
            variant="secondary"
            size="sm"
            onClick={onPause}
            disabled={disabled}
          >
            <Pause className="h-3.5 w-3.5" />
            השהה
          </Button>
        ) : (
          <Button
            type="button"
            variant="secondary"
            size="sm"
            onClick={onResume}
            disabled={disabled}
          >
            <Play className="h-3.5 w-3.5" />
            חדש
          </Button>
        )}

        <div className="mr-auto">
          {isConfirmingDelete ? (
            <div className="flex items-center gap-1">
              <Button
                type="button"
                variant="destructive"
                size="sm"
                onClick={onDelete}
              >
                אישור
              </Button>
              <Button
                type="button"
                variant="secondary"
                size="sm"
                onClick={onCancelDelete}
              >
                ביטול
              </Button>
            </div>
          ) : (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={onDelete}
              className="text-destructive hover:bg-destructive/10"
            >
              <Trash2 className="h-3.5 w-3.5" />
              מחק
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}
