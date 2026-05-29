import type { ReactNode } from "react";
import { useState, useEffect } from "react";
import { Bookmark, AlertTriangle, Clock, Flame, TrendingDown, TrendingUp, Minus } from "lucide-react";
import { formatPrice, formatKm, relativeTime, cn, marketComparison, listingSource } from "@/lib/utils";
import type { Listing } from "@/lib/api";
import { MatchScoreBox } from "@/components/ui/MatchScoreBox";
import { scoreHsl, scoreLabel } from "@/lib/scoringAlgorithm";
import { manufacturerLogoSrc } from "@/lib/manufacturerLogo";

function dealInfo(listing: Listing): {
  label: string;
  icon: typeof TrendingDown;
  badgeClass: string;
  priceClass: string;
} | null {
  const mc = marketComparison(listing.price, listing.median_price, listing.base_price);
  if (!mc) return null;
  if (mc.diffPercent > 10) {
    return {
      label: `${mc.diffPercent}%−`,
      icon: TrendingDown,
      badgeClass: "bg-emerald-500 text-white",
      priceClass: "text-emerald-500",
    };
  }
  if (mc.diffPercent > 5) {
    return {
      label: `${mc.diffPercent}%−`,
      icon: TrendingDown,
      badgeClass: "bg-emerald-500/80 text-white",
      priceClass: "text-emerald-500",
    };
  }
  if (mc.diffPercent >= -5) {
    return {
      label: "שוק",
      icon: Minus,
      badgeClass: "bg-muted text-muted-foreground",
      priceClass: "text-muted-foreground",
    };
  }
  // Remaining range: diffPercent < -5 (slightly to heavily overpriced)
  return {
    label: `+${-mc.diffPercent}%`,
    icon: TrendingUp,
    badgeClass: "bg-red-500/90 text-white",
    priceClass: "text-red-400",
  };
}

type Freshness = "hot" | "today" | "week" | "older";

function listingFreshness(firstSeenAt: string): Freshness {
  if (!firstSeenAt) return "older";
  const posted = new Date(firstSeenAt).getTime();
  const hoursAgo = Math.max(0, Date.now() - posted) / (1000 * 60 * 60);
  if (hoursAgo < 1) return "hot";
  if (hoursAgo < 24) return "today";
  if (hoursAgo < 168) return "week";
  return "older";
}

export function ListingCardBody({
  listing,
  actions,
  hoverScale,
  showBookmarkOverlay,
}: {
  listing: Listing;
  actions?: ReactNode;
  hoverScale?: boolean;
  showBookmarkOverlay?: boolean;
}) {
  const [descExpanded, setDescExpanded] = useState(false);
  const rawDesc = listing.description?.trim() ?? "";
  const descLines = rawDesc.split(/\r\n|\r|\n/).length;
  const descLong = rawDesc.length > 160 || descLines > 3;

  useEffect(() => {
    setDescExpanded(false);
  }, [listing.token]);

  const logoSrc = manufacturerLogoSrc(listing.manufacturer);
  const source = listingSource(listing.page_link);
  const deal = dealInfo(listing);
  const isNew = listing.seen === false;
  const freshness = listingFreshness(listing.first_seen_at);

  return (
    <>
      {/* Image with gradient overlay */}
      <div className="relative aspect-[16/10] w-full overflow-hidden bg-secondary">
        {listing.image_url ? (
          <>
            <img
              src={listing.image_url}
              alt={`${listing.manufacturer} ${listing.model}`}
              referrerPolicy="no-referrer"
              width={640}
              height={400}
              className={cn(
                "h-full w-full object-cover transition-transform duration-700 ease-out",
                hoverScale && "group-hover:scale-[1.06]",
              )}
              loading="lazy"
            />
            <div className="pointer-events-none absolute inset-0 bg-gradient-to-t from-black/60 via-transparent to-transparent" />
          </>
        ) : (
          <div className="flex h-full w-full items-center justify-center bg-gradient-to-br from-secondary to-muted">
            <span className="text-5xl opacity-10">🚗</span>
          </div>
        )}

        {/* Top badges */}
        <div className="absolute inset-x-0 top-0 flex items-start justify-between p-3">
          <div className="flex flex-wrap items-center gap-1.5">
            {deal && deal.badgeClass !== "bg-muted text-muted-foreground" ? (
              <span className={cn("flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-bold shadow-sm", deal.badgeClass)}>
                <deal.icon className="h-3 w-3" />
                {deal.label}
              </span>
            ) : null}
            {listing.suspicious_reasons && listing.suspicious_reasons.length > 0 ? (
              <span className="flex items-center gap-1 rounded-full bg-amber-500/90 px-2 py-0.5 text-[11px] font-bold text-white shadow-sm">
                <AlertTriangle className="h-3 w-3" />
                חשוד
              </span>
            ) : null}
            {listing.removed_at ? (
              <span className="rounded-full bg-black/60 px-2 py-0.5 text-[11px] font-medium text-white/80 backdrop-blur-sm">
                נמכר
              </span>
            ) : null}
          </div>
          <div className="flex items-center gap-1.5">
            {showBookmarkOverlay ? (
              <div className="flex h-7 w-7 items-center justify-center rounded-full bg-black/40 backdrop-blur-sm" aria-hidden>
                <Bookmark className="h-3.5 w-3.5 fill-amber-400 text-amber-400" />
              </div>
            ) : null}
            {source ? (
              <span className="rounded-full bg-black/40 px-2 py-0.5 text-[10px] font-semibold text-white/90 backdrop-blur-sm">
                {source}
              </span>
            ) : null}
          </div>
        </div>

        {/* Bottom overlay — price + title on image */}
        <div className="absolute inset-x-0 bottom-0 p-3">
          <div className="flex items-end justify-between gap-2">
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-1.5">
                <h3 className="text-sm font-bold text-white drop-shadow-sm truncate">
                  {listing.manufacturer} {listing.model}
                </h3>
                {freshness === "hot" ? (
                  <span className="shrink-0 flex items-center gap-0.5 rounded-full bg-gradient-to-r from-orange-500 to-red-500 px-2 py-0.5 text-[9px] font-bold text-white shadow-sm animate-pulse">
                    <Flame className="h-2.5 w-2.5" />
                    חם!
                  </span>
                ) : freshness === "today" ? (
                  <span className="shrink-0 flex items-center gap-0.5 rounded-full bg-emerald-500 px-2 py-0.5 text-[9px] font-bold text-white shadow-sm">
                    <Clock className="h-2.5 w-2.5" />
                    חדש היום
                  </span>
                ) : isNew ? (
                  <span className="shrink-0 rounded-full bg-primary px-1.5 py-0.5 text-[9px] font-bold text-white shadow-sm">
                    NEW
                  </span>
                ) : null}
              </div>
              <p className="text-xs text-white/70 mt-0.5">
                {listing.year} · {listing.city || "—"}
              </p>
            </div>
            <span className="shrink-0 text-lg font-extrabold tabular-nums text-white drop-shadow-md">
              {formatPrice(listing.price)}
            </span>
          </div>
        </div>

        {/* Freshness glow ring */}
        {freshness === "hot" ? (
          <div className="pointer-events-none absolute inset-0 rounded-t-[inherit] ring-2 ring-inset ring-orange-500/60 shadow-[inset_0_0_24px_rgba(249,115,22,0.25)] animate-pulse" aria-hidden />
        ) : freshness === "today" ? (
          <div className="pointer-events-none absolute inset-0 rounded-t-[inherit] ring-2 ring-inset ring-emerald-500/50 shadow-[inset_0_0_20px_rgba(16,185,129,0.15)]" aria-hidden />
        ) : isNew ? (
          <div className="pointer-events-none absolute inset-0 rounded-t-[inherit] ring-2 ring-inset ring-primary/50 shadow-[inset_0_0_20px_rgba(59,130,246,0.15)]" aria-hidden />
        ) : null}
      </div>

      {/* Content */}
      <div className={cn("px-4 py-3 space-y-3", listing.removed_at && "opacity-50")}>
        {/* Score row */}
        <div className="flex items-center gap-2.5">
          {logoSrc ? (
            <img src={logoSrc} alt="" className="h-7 w-7 shrink-0 object-contain dark:invert dark:opacity-80" loading="lazy" decoding="async" />
          ) : null}
          {listing.fitness_score != null ? (
            <>
              <MatchScoreBox score={listing.fitness_score} size="sm" />
              <span className="text-xs font-semibold" style={{ color: scoreHsl(listing.fitness_score) }}>
                {listing.fitness_score.toFixed(1)} · {scoreLabel(listing.fitness_score)}
              </span>
            </>
          ) : null}
          <span className={cn(
            "ms-auto flex items-center gap-1 text-xs font-medium tabular-nums",
            freshness === "hot" ? "text-orange-500" :
            freshness === "today" ? "text-emerald-500" :
            "text-muted-foreground",
          )}>
            {freshness === "hot" ? <Flame className="h-3 w-3" /> :
             freshness === "today" ? <Clock className="h-3 w-3" /> :
             <Clock className="h-3 w-3 opacity-50" />}
            {relativeTime(listing.first_seen_at)}
          </span>
        </div>

        {/* Specs grid */}
        <div className="flex flex-wrap gap-1.5">
          <SpecChip label="ק״מ" value={listing.km > 0 ? formatKm(listing.km) : undefined} enriching={listing.km <= 0} />
          <SpecChip label="יד" value={listing.hand > 0 ? String(listing.hand) : undefined} />
          {listing.gear_box ? <SpecChip label="הילוכים" value={listing.gear_box} /> : null}
          {listing.engine_type ? <SpecChip label="דלק" value={listing.engine_type} /> : null}
          {listing.engine_volume ? <SpecChip label="נפח" value={`${(listing.engine_volume / 1000).toFixed(1)}L`} /> : null}
          {listing.horse_power ? <SpecChip label="כ״ס" value={String(listing.horse_power)} /> : null}
        </div>

        {/* Description */}
        {rawDesc ? (
          <div className="min-w-0">
            <p
              className={cn(
                "text-xs text-muted-foreground leading-relaxed break-words whitespace-pre-line",
                !descExpanded && descLong && "line-clamp-2",
              )}
            >
              {rawDesc}
            </p>
            {descLong ? (
              <button
                type="button"
                onClick={(e) => {
                  e.stopPropagation();
                  setDescExpanded((v) => !v);
                }}
                className="mt-0.5 text-[11px] font-semibold text-primary hover:underline"
              >
                {descExpanded ? "הצג פחות" : "עוד"}
              </button>
            ) : null}
          </div>
        ) : null}

        {/* Footer */}
        <div className="flex items-center gap-1.5 pt-2 border-t border-border/30">
          {actions}
        </div>
      </div>
    </>
  );
}

function SpecChip({ label, value, enriching }: { label: string; value?: string; enriching?: boolean }) {
  return (
    <div className="rounded-lg bg-secondary/70 px-2.5 py-1.5 text-center min-w-[3.5rem]">
      <p className="text-[10px] text-muted-foreground/70">{label}</p>
      <p className="text-xs font-semibold text-foreground truncate tabular-nums">
        {value ?? (enriching ? (
          <span className="inline-flex items-center gap-0.5 text-muted-foreground/50">
            <span className="h-1.5 w-1.5 rounded-full bg-amber-400 animate-pulse" />
            <span className="text-[9px]">מעשיר</span>
          </span>
        ) : "—")}
      </p>
    </div>
  );
}
