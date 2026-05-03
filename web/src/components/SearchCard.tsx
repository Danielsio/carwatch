import { useNavigate } from "react-router";
import {
  Play,
  Pause,
  Edit2,
  Trash2,
  Car,
  ChevronLeft,
} from "lucide-react";
import { cn, formatKm, formatPrice, relativeTime } from "@/lib/utils";
import type { Search } from "@/lib/api";
import { Button } from "@/components/ui/Button";

export type SearchCardProps = {
  search: Search;
  disabled?: boolean;
  onPause: () => void;
  onResume: () => void;
  onDelete: () => void;
  isConfirmingDelete: boolean;
  onCancelDelete: () => void;
};

export function SearchCard({
  search,
  disabled,
  onPause,
  onResume,
  onDelete,
  isConfirmingDelete,
  onCancelDelete,
}: SearchCardProps) {
  const navigate = useNavigate();
  const isActive = search.active;
  const listingsPath = `/searches/${search.id}/listings`;

  const filterTags = [
    `${search.manufacturer_name} ${search.model_name}`.trim(),
    `${search.year_min}–${search.year_max}`,
    search.price_max > 0 ? `עד ${formatPrice(search.price_max)}` : null,
    search.max_km > 0 ? `עד ${formatKm(search.max_km)}` : null,
    search.max_hand > 0 ? `עד יד ${search.max_hand}` : null,
  ].filter(Boolean) as string[];

  return (
    <div
      role="button"
      tabIndex={0}
      className={cn(
        "group card-hover cursor-pointer rounded-2xl border bg-card p-5 transition-all duration-200 outline-none focus-visible:ring-2 focus-visible:ring-primary",
        isActive ? "border-border" : "border-border/50 opacity-75",
      )}
      onClick={() => navigate(listingsPath)}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          navigate(listingsPath);
        }
      }}
    >
      <div className="mb-3 flex items-start justify-between">
        <div className="flex min-w-0 flex-1 items-center gap-3">
          <div
            className={cn(
              "flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-xl",
              isActive ? "bg-primary/15" : "bg-muted",
            )}
          >
            <Car
              size={16}
              className={
                isActive ? "text-primary" : "text-muted-foreground"
              }
            />
          </div>
          <div className="min-w-0">
            <h3 className="text-sm leading-tight font-semibold text-foreground">
              {search.name}
            </h3>
            <div className="mt-0.5 flex items-center gap-1.5">
              <div
                className={cn(
                  "h-1.5 w-1.5 rounded-full",
                  isActive ? "animate-pulse bg-success" : "bg-muted-foreground",
                )}
              />
              <span className="text-xs text-muted-foreground">
                {isActive ? "פעיל" : "מושהה"}
              </span>
            </div>
          </div>
        </div>

        <div
          className="flex items-center gap-1"
          onClick={(e) => e.stopPropagation()}
        >
          {isActive ? (
            <button
              type="button"
              onClick={onPause}
              disabled={disabled}
              className={cn(
                "flex h-7 w-7 items-center justify-center rounded-lg transition-colors",
                "bg-warning/15 text-warning hover:bg-warning/25",
              )}
              aria-label="השהה חיפוש"
            >
              <Pause size={12} />
            </button>
          ) : (
            <button
              type="button"
              onClick={onResume}
              disabled={disabled}
              className={cn(
                "flex h-7 w-7 items-center justify-center rounded-lg transition-colors",
                "bg-success/15 text-success hover:bg-success/25",
              )}
              aria-label="חדש חיפוש"
            >
              <Play size={12} />
            </button>
          )}
          <button
            type="button"
            onClick={() => navigate(`/searches/${search.id}/edit`)}
            className="flex h-7 w-7 items-center justify-center rounded-lg bg-secondary text-muted-foreground transition-colors hover:bg-secondary/80 hover:text-foreground"
            aria-label="ערוך חיפוש"
          >
            <Edit2 size={12} />
          </button>
          {!isConfirmingDelete ? (
            <button
              type="button"
              onClick={onDelete}
              disabled={disabled}
              className="flex h-7 w-7 items-center justify-center rounded-lg bg-destructive/10 text-destructive transition-colors hover:bg-destructive/20"
              aria-label="מחק חיפוש"
            >
              <Trash2 size={12} />
            </button>
          ) : null}
        </div>
      </div>

      {filterTags.length > 0 ? (
        <div className="mb-3 flex flex-wrap gap-1.5">
          {filterTags.map((tag, i) => (
            <span
              key={i}
              className="text-secondary-foreground rounded-full bg-secondary px-2.5 py-0.5 text-xs font-medium"
            >
              {tag}
            </span>
          ))}
        </div>
      ) : null}

      <div className="border-border/60 flex items-center justify-between border-t pt-3">
        <div className="text-center">
          <div className="text-foreground text-base font-bold tabular-nums">
            {search.listings_count ?? 0}
          </div>
          <div className="text-[10px] text-muted-foreground">מודעות</div>
        </div>
        <div className="flex items-center gap-1 text-xs text-muted-foreground/70">
          <span>{relativeTime(search.created_at)}</span>
          <ChevronLeft
            size={13}
            className="text-muted-foreground/40 transition-colors group-hover:text-primary/60"
            aria-hidden
          />
        </div>
      </div>

      {isConfirmingDelete ? (
        <div
          className="border-border/60 mt-3 flex flex-wrap items-center justify-end gap-2 border-t pt-3"
          onClick={(e) => e.stopPropagation()}
        >
          <Button
            type="button"
            variant="destructive"
            size="sm"
            onClick={onDelete}
            disabled={disabled}
          >
            אישור מחיקה
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
      ) : null}
    </div>
  );
}
