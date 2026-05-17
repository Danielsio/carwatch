import type { ReactNode } from "react";
import { useState, useEffect } from "react";
import { Bookmark, AlertTriangle, ExternalLink } from "lucide-react";
import { formatPrice, formatKm, relativeTime, cn, marketComparison } from "@/lib/utils";
import type { Listing } from "@/lib/api";
import { MatchScoreBox } from "@/components/ui/MatchScoreBox";
import { scoreHsl, scoreLabel } from "@/lib/scoringAlgorithm";
import { manufacturerLogoSrc } from "@/lib/manufacturerLogo";

function listingSource(pageLink: string): "Yad2" | "WinWin" | null {
  if (!pageLink) return null;
  if (pageLink.includes("yad2")) return "Yad2";
  if (pageLink.includes("winwin")) return "WinWin";
  return null;
}

function dealBadge(listing: Listing): {
  label: string;
  className: string;
} | null {
  const mc = marketComparison(listing.price, listing.median_price, listing.base_price);
  if (!mc) return null;
  if (mc.diffPercent > 10) {
    return { label: "עסקה מעולה", className: "bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 ring-emerald-500/20" };
  }
  if (mc.diffPercent > 5) {
    return { label: "מחיר טוב", className: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-emerald-500/15" };
  }
  if (mc.diffPercent < -10) {
    return { label: "מעל השוק", className: "bg-red-500/10 text-red-600 dark:text-red-400 ring-red-500/15" };
  }
  return null;
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
  const deal = dealBadge(listing);
  const isNew = listing.seen === false;

  return (
    <>
      {/* Image */}
      <div className="relative aspect-video w-full overflow-hidden bg-secondary">
        {listing.image_url ? (
          <img
            src={listing.image_url}
            alt={`${listing.manufacturer} ${listing.model}`}
            referrerPolicy="no-referrer"
            className={cn(
              "h-full w-full object-cover transition-transform duration-500 ease-out",
              hoverScale && "group-hover:scale-105",
            )}
            loading="lazy"
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center">
            <span className="text-4xl opacity-15">🚗</span>
          </div>
        )}

        {/* Image overlays */}
        <div className="absolute inset-x-0 top-0 flex items-start justify-between p-2.5">
          <div className="flex flex-wrap items-center gap-1.5">
            {deal ? (
              <span className={cn("rounded-md px-2 py-0.5 text-[11px] font-semibold ring-1 ring-inset backdrop-blur-sm", deal.className)}>
                {deal.label}
              </span>
            ) : null}
            {listing.suspicious_reasons && listing.suspicious_reasons.length > 0 ? (
              <span className="flex items-center gap-1 rounded-md bg-amber-500/15 px-2 py-0.5 text-[11px] font-semibold text-amber-600 ring-1 ring-inset ring-amber-500/20 backdrop-blur-sm dark:text-amber-400">
                <AlertTriangle className="h-3 w-3" />
                זהירות
              </span>
            ) : null}
            {listing.removed_at ? (
              <span className="rounded-md bg-muted/80 px-2 py-0.5 text-[11px] font-medium text-muted-foreground ring-1 ring-inset ring-border/50 backdrop-blur-sm">
                נמכר כנראה
              </span>
            ) : null}
          </div>
          <div className="flex items-center gap-1.5">
            {showBookmarkOverlay ? (
              <div
                className="flex h-7 w-7 items-center justify-center rounded-full bg-background/80 shadow-sm ring-1 ring-border/40 backdrop-blur-sm"
                aria-hidden
              >
                <Bookmark className="h-3.5 w-3.5 fill-amber-500 text-amber-600 dark:text-amber-400 dark:fill-amber-400" />
              </div>
            ) : null}
            {source ? (
              <span className="rounded-md bg-background/80 px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground ring-1 ring-border/40 backdrop-blur-sm">
                {source}
              </span>
            ) : null}
          </div>
        </div>

        {/* New indicator glow */}
        {isNew ? (
          <div className="pointer-events-none absolute inset-0 ring-2 ring-inset ring-primary/40" aria-hidden />
        ) : null}
      </div>

      {/* Content */}
      <div className={cn("p-4", listing.removed_at && "opacity-60")}>
        {/* Title row: logo + name + score + price */}
        <div className="mb-2.5 flex items-start gap-3">
          {logoSrc ? (
            <img
              src={logoSrc}
              alt=""
              className="h-9 w-9 shrink-0 object-contain"
              loading="lazy"
              decoding="async"
            />
          ) : null}
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <h3 className="font-semibold leading-tight text-card-foreground">
                {listing.manufacturer} {listing.model}
              </h3>
              {isNew ? (
                <span className="rounded-full bg-primary/10 px-1.5 py-0.5 text-[10px] font-bold text-primary">
                  חדש
                </span>
              ) : null}
            </div>
            <p className="mt-0.5 text-xs text-muted-foreground">
              {listing.year}
              {listing.fitness_score != null ? (
                <span className="mx-1.5 text-border">·</span>
              ) : null}
              {listing.fitness_score != null ? (
                <span className="font-medium" style={{ color: scoreHsl(listing.fitness_score) }}>
                  {scoreLabel(listing.fitness_score)}
                </span>
              ) : null}
            </p>
          </div>
          <div className="flex shrink-0 items-start gap-2">
            {listing.fitness_score != null ? (
              <MatchScoreBox score={listing.fitness_score} size="sm" />
            ) : null}
            <div className="text-end">
              <span className="text-lg font-bold tabular-nums text-primary leading-tight">
                {formatPrice(listing.price)}
              </span>
              {(() => {
                const mc = marketComparison(listing.price, listing.median_price, listing.base_price);
                if (mc) {
                  return (
                    <p className={cn("text-[11px] font-medium mt-0.5", mc.color)}>
                      {mc.label}
                    </p>
                  );
                }
                return null;
              })()}
            </div>
          </div>
        </div>

        {/* Meta pills */}
        <div className="mb-3 flex flex-wrap gap-1.5">
          {listing.km > 0 ? (
            <span className="rounded-md bg-secondary px-2 py-0.5 text-[11px] tabular-nums text-muted-foreground">
              {formatKm(listing.km)}
            </span>
          ) : null}
          <span className="rounded-md bg-secondary px-2 py-0.5 text-[11px] text-muted-foreground">
            יד {listing.hand > 0 ? listing.hand : "—"}
          </span>
          {listing.city ? (
            <span className="rounded-md bg-secondary px-2 py-0.5 text-[11px] text-muted-foreground">
              {listing.city}
            </span>
          ) : null}
          {listing.gear_box ? (
            <span className="rounded-md bg-secondary px-2 py-0.5 text-[11px] text-muted-foreground">
              {listing.gear_box}
            </span>
          ) : null}
        </div>

        {/* Description */}
        {rawDesc ? (
          <div className="mb-3 min-w-0">
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
                className="mt-1 text-xs font-medium text-primary hover:underline"
              >
                {descExpanded ? "הצג פחות" : "המשך קריאה"}
              </button>
            ) : null}
          </div>
        ) : null}

        {/* Footer: time + actions */}
        <div className="flex items-center gap-2">
          <span className="me-auto text-[11px] font-medium text-muted-foreground">
            {relativeTime(listing.first_seen_at)}
          </span>
          {actions}
          {listing.page_link ? (
            <a
              href={listing.page_link}
              target="_blank"
              rel="noopener noreferrer"
              onClick={(e) => e.stopPropagation()}
              className="flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
              aria-label="צפה במודעה המקורית"
            >
              <ExternalLink className="h-3.5 w-3.5" />
            </a>
          ) : null}
        </div>
      </div>
    </>
  );
}
