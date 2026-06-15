import { useEffect, useRef, useState } from "react";
import { useNavigate, Link } from "react-router";
import {
  Play,
  Pause,
  Edit2,
  Trash2,
  Car,
  ChevronLeft,
  ChevronDown,
} from "lucide-react";
import { cn, formatKm, formatPrice, relativeTime } from "@/lib/utils";
import type { Search, SearchCycleStatsItem } from "@/lib/api";
import { Button } from "@/components/ui/Button";
import { Sparkline } from "@/components/ui/Sparkline";
import { useSearchActivity } from "@/hooks/useSearchActivity";
import {
  manufacturerLogoSrc,
  manufacturerLogoSrcFromCatalogId,
} from "@/lib/manufacturerLogo";

export type SearchCardProps = {
  search: Search;
  disabled?: boolean;
  onPause: () => void;
  onResume: () => void;
  onDelete: () => void;
  isConfirmingDelete: boolean;
  onCancelDelete: () => void;
  cycleStats?: SearchCycleStatsItem;
};

export function SearchCard({
  search,
  disabled,
  onPause,
  onResume,
  onDelete,
  isConfirmingDelete,
  onCancelDelete,
  cycleStats,
}: SearchCardProps) {
  const navigate = useNavigate();
  const confirmRef = useRef<HTMLButtonElement>(null);
  const [statsOpen, setStatsOpen] = useState(false);
  const isActive = search.active;
  const listingsPath = `/searches/${search.id}/listings`;
  const stats = search.stats;
  const { data: activity } = useSearchActivity(search.id);
  const activitySeries = activity?.map((d) => d.count);

  useEffect(() => {
    if (isConfirmingDelete) confirmRef.current?.focus();
  }, [isConfirmingDelete]);

  const mfrLogo =
    manufacturerLogoSrcFromCatalogId(search.manufacturer_id) ??
    manufacturerLogoSrc(search.manufacturer_name);

  const filterTags = [
    `${search.manufacturer_name} ${search.model_name}`.trim(),
    search.year_min !== 0 || search.year_max !== 0
      ? `${search.year_min}–${search.year_max}`
      : null,
    search.price_min > 0 ? `מ-${formatPrice(search.price_min)}` : null,
    search.price_max > 0 ? `עד ${formatPrice(search.price_max)}` : null,
    search.max_km > 0 ? `עד ${formatKm(search.max_km)}` : null,
    search.max_hand > 0 ? `עד יד ${search.max_hand}` : null,
    search.gear_box ? search.gear_box : null,
    search.seller_filter === "private"
      ? "מוכר פרטי"
      : search.seller_filter === "commercial"
        ? "מוסך / סוכנות"
        : null,
    search.price_only ? "עם מחיר" : null,
    search.photo_only ? "עם תמונה" : null,
  ].filter(Boolean) as string[];

  const openCardClassName =
    "rounded-lg outline-none transition-colors focus-visible:ring-2 focus-visible:ring-primary";

  return (
    <article
      className={cn(
        "group card-hover rounded-2xl border bg-card p-5 transition-all duration-200",
        isActive ? "border-border" : "border-border/50 opacity-75",
      )}
    >
      <div className="mb-3 flex items-start justify-between gap-2">
        <Link
          to={listingsPath}
          className={cn(
            "flex min-w-0 flex-1 items-center gap-3",
            openCardClassName,
          )}
        >
          <div
            className={cn(
              "flex h-9 w-9 flex-shrink-0 items-center justify-center overflow-hidden rounded-xl",
              isActive ? "bg-primary/15" : "bg-muted",
            )}
          >
            {mfrLogo ? (
              <img
                src={mfrLogo}
                alt=""
                className="h-7 w-7 object-contain"
                loading="lazy"
                decoding="async"
              />
            ) : (
              <Car
                size={16}
                className={
                  isActive ? "text-primary" : "text-muted-foreground"
                }
              />
            )}
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
        </Link>

        <div className="flex shrink-0 flex-wrap items-center gap-1">
          {isActive ? (
            <button
              type="button"
              onClick={onPause}
              disabled={disabled}
              className={cn(
                "flex h-10 w-10 min-h-[44px] min-w-[44px] items-center justify-center rounded-lg transition-colors",
                "bg-warning/15 text-warning hover:bg-warning/25",
              )}
              aria-label="השהה חיפוש"
            >
              <Pause size={16} />
            </button>
          ) : (
            <button
              type="button"
              onClick={onResume}
              disabled={disabled}
              className={cn(
                "flex h-10 w-10 min-h-[44px] min-w-[44px] items-center justify-center rounded-lg transition-colors",
                "bg-success/15 text-success hover:bg-success/25",
              )}
              aria-label="חדש חיפוש"
            >
              <Play size={16} />
            </button>
          )}
          <button
            type="button"
            onClick={() => navigate(`/searches/${search.id}/edit`)}
            className="flex h-10 w-10 min-h-[44px] min-w-[44px] items-center justify-center rounded-lg bg-secondary text-muted-foreground transition-colors hover:bg-secondary/80 hover:text-foreground"
            aria-label="ערוך חיפוש"
          >
            <Edit2 size={16} />
          </button>
          {!isConfirmingDelete ? (
            <button
              type="button"
              onClick={onDelete}
              disabled={disabled}
              className="flex h-10 w-10 min-h-[44px] min-w-[44px] items-center justify-center rounded-lg bg-destructive/10 text-destructive transition-colors hover:bg-destructive/20"
              aria-label="מחק חיפוש"
            >
              <Trash2 size={16} />
            </button>
          ) : null}
        </div>
      </div>

      {filterTags.length > 0 ? (
        <Link
          to={listingsPath}
          className={cn("mb-3 block", openCardClassName)}
        >
          <div className="flex flex-wrap gap-1.5">
            {filterTags.map((tag, i) => (
              <span
                key={i}
                className="text-secondary-foreground rounded-full bg-secondary px-2.5 py-0.5 text-xs font-medium"
              >
                {tag}
              </span>
            ))}
          </div>
        </Link>
      ) : null}

      {stats && stats.total > 0 && (
        <div className="mb-3 text-xs text-muted-foreground">
          <span>{stats.total} {"מודעות"}</span>
          {stats.new_24h > 0 && (
            <>
              <span className="mx-1.5">&middot;</span>
              <span>{stats.new_24h} {"חדשות היום"}</span>
            </>
          )}
          {stats.avg_price > 0 && (
            <>
              <span className="mx-1.5">&middot;</span>
              <span>{"ממוצע"} {formatPrice(Math.round(stats.avg_price))}</span>
            </>
          )}
        </div>
      )}

      <Link
        to={listingsPath}
        className={cn(
          "border-border/60 flex items-center justify-between border-t pt-3",
          openCardClassName,
        )}
      >
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
      </Link>

      {activitySeries && activitySeries.some((v) => v > 0) ? (
        <div className="mt-3 flex items-center gap-2 border-t border-border/30 pt-3">
          <span className="text-[11px] text-muted-foreground">פעילות (14 ימים)</span>
          <Sparkline
            data={activitySeries}
            className="ms-auto"
            ariaLabel="מגמת מודעות חדשות ב-14 הימים האחרונים"
          />
        </div>
      ) : null}

      {cycleStats && (
        <div className="border-t border-border/30 pt-3 mt-3">
          <button
            type="button"
            onClick={() => setStatsOpen(prev => !prev)}
            className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors w-full"
            aria-expanded={statsOpen}
          >
            <ChevronDown className={cn("h-3.5 w-3.5 transition-transform", statsOpen && "rotate-180")} />
            <span>סריקה אחרונה</span>
            <span className="ms-auto text-[11px] tabular-nums opacity-60">{relativeTime(cycleStats.cycle_at)}</span>
          </button>
          {statsOpen && (
            <div className="mt-2.5 space-y-2 animate-fade-in">
              <div className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
                <span className="tabular-nums font-medium text-foreground">{cycleStats.feed_size.toLocaleString()}</span>
                <span>מודעות מתאימות</span>
                <span className="text-border">&#8594;</span>
                <span className="tabular-nums font-medium text-foreground">{cycleStats.matched.toLocaleString()}</span>
                <span>עברו סינון</span>
                <span className="text-border">&#8594;</span>
                <span className="tabular-nums font-medium text-primary">{cycleStats.delivered.toLocaleString()}</span>
                <span>חדשות</span>
              </div>
              {/* Match rate progress bar */}
              <div className="h-1.5 rounded-full bg-border/30 overflow-hidden">
                <div
                  className="h-full rounded-full bg-gradient-to-r from-primary/60 to-primary transition-all duration-500"
                  style={{ width: `${cycleStats.feed_size > 0 ? Math.min(100, (cycleStats.matched / cycleStats.feed_size) * 100) : 0}%` }}
                />
              </div>
              <div className="flex gap-3 text-[11px] text-muted-foreground">
                {cycleStats.km_filtered > 0 && (
                  <span>{cycleStats.km_filtered} ממתינות לנתוני ק״מ</span>
                )}
                {cycleStats.price_drops > 0 && (
                  <span className="text-score-good">{cycleStats.price_drops} ירידות מחיר</span>
                )}
                {cycleStats.dropped > 0 && (
                  <span className="text-muted-foreground/60">{cycleStats.dropped} הוסרו</span>
                )}
              </div>

              {/* Filter rejection breakdown */}
              {(() => {
                const rejections = [
                  { label: "יד גבוהה מדי", count: cycleStats.hand_over },
                  { label: "נפח מנוע", count: cycleStats.engine_cc },
                  { label: "מוכר מסחרי", count: cycleStats.seller },
                  { label: "אחר", count: cycleStats.other_filter },
                  { label: "ק״מ גבוה", count: cycleStats.km_over },
                  { label: "מחיר מחוץ לטווח", count: cycleStats.price_out },
                  { label: "שנה מחוץ לטווח", count: cycleStats.year_out },
                ].filter(r => r.count > 0).sort((a, b) => b.count - a.count);
                if (rejections.length === 0) return null;
                const totalFiltered = rejections.reduce((s, r) => s + r.count, 0);
                return (
                  <div className="flex flex-wrap items-center gap-1 text-[11px] text-muted-foreground/70">
                    <span>{totalFiltered} סוננו:</span>
                    {rejections.map((r, i) => (
                      <span key={r.label}>
                        {i > 0 && <span className="mx-0.5 opacity-40">·</span>}
                        <span className="tabular-nums">{r.count}</span>{" "}{r.label}
                      </span>
                    ))}
                  </div>
                );
              })()}

              {/* Score & price range */}
              {cycleStats.score_min != null && cycleStats.score_max != null && cycleStats.score_avg != null && (
                <div className="text-[11px] text-muted-foreground/70">
                  <span>ציון: </span>
                  <span className="tabular-nums font-medium text-foreground">
                    {cycleStats.score_min.toFixed(1)}–{cycleStats.score_max.toFixed(1)}
                  </span>
                  <span> (ממוצע {cycleStats.score_avg.toFixed(1)})</span>
                  {cycleStats.price_min != null && cycleStats.price_max != null && (
                    <>
                      <span className="mx-1 opacity-40">·</span>
                      <span className="tabular-nums">{formatPrice(cycleStats.price_min)}–{formatPrice(cycleStats.price_max)}</span>
                    </>
                  )}
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {isConfirmingDelete ? (
        <div className="border-border/60 mt-3 flex flex-wrap items-center justify-end gap-2 border-t pt-3" role="alertdialog" aria-label="אישור מחיקה" aria-describedby={`delete-desc-${search.id}`}>
          <p id={`delete-desc-${search.id}`} className="sr-only">
            האם אתה בטוח שברצונך למחוק את החיפוש &quot;{search.name}&quot;? פעולה זו לא ניתנת לביטול.
          </p>
          <Button
            ref={confirmRef}
            type="button"
            variant="destructive"
            size="md"
            onClick={onDelete}
            disabled={disabled}
            aria-label="אישור מחיקת חיפוש"
          >
            אישור מחיקה
          </Button>
          <Button
            type="button"
            variant="secondary"
            size="md"
            onClick={onCancelDelete}
            aria-label="ביטול מחיקה"
          >
            ביטול
          </Button>
        </div>
      ) : null}
    </article>
  );
}
