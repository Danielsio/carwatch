import { memo } from "react";
import { Link } from "react-router";
import { Eye, EyeOff, Bookmark } from "lucide-react";
import {
  formatPrice,
  formatKm,
  marketComparison,
  cn,
} from "@/lib/utils";
import type { Listing } from "@/lib/api";
import { scoreHsl } from "@/lib/scoringAlgorithm";
import { useListingActions } from "@/hooks/useListingActions";
import { useSpotlight } from "@/hooks/useSpotlight";
import { useToast } from "@/components/ui/Toast";

/** Dense, scannable single-row variant of a listing — used in compact mode. */
export const CompactListingCard = memo(function CompactListingCard({
  listing,
}: {
  listing: Listing;
}) {
  const { saved, seen, toggleSaved, toggleSeen } = useListingActions(listing);
  const { toast } = useToast();
  const spotlight = useSpotlight();
  const mc = marketComparison(
    listing.price,
    listing.median_price,
    listing.base_price,
  );

  const meta = [
    String(listing.year),
    listing.km > 0 ? formatKm(listing.km) : null,
    listing.hand > 0 ? `יד ${listing.hand}` : null,
    listing.city || null,
  ]
    .filter(Boolean)
    .join(" · ");

  return (
    <Link
      to={`/listings/${listing.token}`}
      state={{ listing }}
      {...spotlight}
      aria-label={`${listing.manufacturer} ${listing.model} ${listing.year} - ${formatPrice(listing.price)}`}
      className={cn(
        "spotlight group flex items-center gap-3 rounded-xl border border-border/40 bg-card p-2.5 transition-all duration-200 hover:border-primary/30 hover:shadow-[0_8px_28px_-12px_var(--color-glow-primary)] dir-rtl",
        listing.removed_at && "opacity-60",
      )}
    >
      <div className="relative h-16 w-24 shrink-0 overflow-hidden rounded-lg bg-secondary">
        {listing.image_url ? (
          <img
            src={listing.image_url}
            alt=""
            referrerPolicy="no-referrer"
            loading="lazy"
            className="h-full w-full object-cover"
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center text-lg opacity-20">
            🚗
          </div>
        )}
        {listing.seen === false ? (
          <span
            className="absolute end-1 top-1 h-2 w-2 rounded-full bg-primary ring-2 ring-card"
            aria-hidden
          />
        ) : null}
      </div>

      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-1.5">
          <p className="truncate text-sm font-semibold text-foreground">
            {listing.manufacturer} {listing.model}
          </p>
          {listing.fitness_score != null ? (
            <span
              className="shrink-0 text-[11px] font-bold tabular-nums"
              style={{ color: scoreHsl(listing.fitness_score) }}
            >
              {listing.fitness_score.toFixed(1)}
            </span>
          ) : null}
        </div>
        <p className="truncate text-xs text-muted-foreground">{meta}</p>
      </div>

      <div className="flex shrink-0 flex-col items-end">
        <span className="text-sm font-bold tabular-nums text-foreground">
          {formatPrice(listing.price)}
        </span>
        {mc ? (
          <span className={cn("text-[11px] font-medium", mc.color)}>
            {mc.diffPercent > 5
              ? `${mc.diffPercent}%−`
              : mc.diffPercent >= -5
                ? "שוק"
                : `${-mc.diffPercent}%+`}
          </span>
        ) : null}
      </div>

      <div className="flex shrink-0 items-center gap-0.5">
        <button
          type="button"
          aria-label={seen ? "החזר לרשימת החדשות" : "סמן כנצפה"}
          aria-pressed={seen}
          onClick={(e) => {
            e.preventDefault();
            e.stopPropagation();
            toggleSeen();
          }}
          className={cn(
            "flex h-11 w-11 items-center justify-center rounded-lg transition-colors",
            seen
              ? "bg-primary/10 text-primary"
              : "text-muted-foreground hover:bg-primary/5 hover:text-primary",
          )}
        >
          {seen ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
        </button>
        <button
          type="button"
          aria-label={saved ? "הסר משמורים" : "שמור מודעה"}
          aria-pressed={saved}
          onClick={(e) => {
            e.preventDefault();
            e.stopPropagation();
            toggleSaved({
              onSuccess: (next) =>
                toast(next ? "נשמר בהצלחה" : "הוסר מהשמורים", next ? "success" : "info"),
            });
          }}
          className={cn(
            "flex h-11 w-11 items-center justify-center rounded-lg transition-colors",
            saved
              ? "bg-primary/10 text-primary"
              : "text-muted-foreground hover:bg-primary/5 hover:text-primary",
          )}
        >
          <Bookmark className={cn("h-4 w-4", saved && "fill-current")} />
        </button>
      </div>
    </Link>
  );
});
