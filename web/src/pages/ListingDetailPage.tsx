import { useLocation, Link, useNavigate, useParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import type { ReactNode } from "react";
import {
  ArrowRight,
  ExternalLink,
  Calendar,
  Gauge,
  Hand,
  MapPin,
  Clock,
  Car,
  Fuel,
  Cog,
  Zap,
  Eye,
  EyeOff,
  AlertTriangle,
  Bookmark,
  BookmarkCheck,
  Shield,
  Store,
  User,
} from "lucide-react";
import { formatPrice, formatKm, relativeTime, safeHref, marketComparison, cn, listingSource } from "@/lib/utils";
import { api, ApiError } from "@/lib/api";
import type { Listing } from "@/lib/api";
import { Button } from "@/components/ui/Button";
import { MatchScoreBox } from "@/components/ui/MatchScoreBox";
import { scoreHsl, scoreLabel } from "@/lib/scoringAlgorithm";
import { useListingActions } from "@/hooks/useListingActions";
import { manufacturerLogoSrc } from "@/lib/manufacturerLogo";
import { Skeleton } from "@/components/ui/Skeleton";
import { EmptyState } from "@/components/ui/EmptyState";
import { PriceHistoryChart } from "@/components/PriceHistoryChart";

export function ListingDetailPage() {
  const location = useLocation();
  const navigate = useNavigate();
  const { token } = useParams();
  const stateListingRaw = location.state?.listing as Listing | undefined;
  const stateListingForToken =
    stateListingRaw?.token === token ? stateListingRaw : undefined;

  const {
    data: listing,
    error,
    isPending,
  } = useQuery({
    queryKey: ["listing", token ?? ""],
    queryFn: async () => {
      const t = token;
      if (!t) {
        throw new Error("missing token");
      }
      return api.listing(t);
    },
    enabled: Boolean(token),
    initialData: stateListingForToken,
    staleTime: 0,
  });

  if (!token) {
    return (
      <EmptyState
        icon={Car}
        title="מודעה לא נמצאה"
        description="קישור לא תקין"
        action={
          <Button asChild>
            <Link to="/dashboard">
              <ArrowRight className="h-4 w-4" />
              חזרה לחיפושים
            </Link>
          </Button>
        }
      />
    );
  }

  if (isPending && !listing) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-40 rounded-lg" />
        <Skeleton className="aspect-video w-full rounded-2xl" />
        <Skeleton className="h-12 w-60 rounded-lg" />
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          {[1, 2, 3, 4].map((i) => (
            <Skeleton key={i} className="h-20 rounded-2xl" />
          ))}
        </div>
      </div>
    );
  }

  if (error instanceof ApiError && error.status === 404) {
    return (
      <EmptyState
        icon={Car}
        title="מודעה לא נמצאה"
        description="ניתן לגשת למודעה דרך רשימת התוצאות"
        action={
          <Button asChild>
            <Link to="/dashboard">
              <ArrowRight className="h-4 w-4" />
              חזרה לחיפושים
            </Link>
          </Button>
        }
      />
    );
  }

  if (error) {
    return (
      <EmptyState
        icon={Car}
        title="משהו השתבש"
        description="נסה שוב מאוחר יותר או חזור לרשימת התוצאות"
        action={
          <Button asChild>
            <Link to="/dashboard">
              <ArrowRight className="h-4 w-4" />
              חזרה לחיפושים
            </Link>
          </Button>
        }
      />
    );
  }

  if (!listing) {
    return null;
  }

  const canGoBack = location.key !== "default";

  return (
    <ListingDetailContent
      listing={listing}
      backButton={
        <Button variant="ghost" size="sm" onClick={() => canGoBack ? navigate(-1) : navigate('/dashboard')} className="text-muted-foreground hover:text-foreground -mr-2">
          <ArrowRight className="h-4 w-4" />
          חזרה לתוצאות
        </Button>
      }
    />
  );
}

function ListingDetailContent({
  listing,
  backButton,
}: {
  listing: Listing;
  backButton: ReactNode;
}) {
  const { saved, seen, toggleSaved, toggleSeen, isSaving, isTogglingSeen } = useListingActions(listing);

  const detailLogoSrc = manufacturerLogoSrc(listing.manufacturer);
  const source = listingSource(listing.page_link);
  const mc = marketComparison(listing.price, listing.median_price, listing.base_price);

  return (
    <div className="space-y-5 pb-24 md:pb-8">
      {backButton}

      {/* Removed / Suspicious banners */}
      {listing.removed_at ? (
        <div className="rounded-xl border border-border/50 bg-muted/50 px-4 py-3 text-sm text-muted-foreground">
          {"המודעה הוסרה מהאתר — ככל הנראה הרכב נמכר."}
        </div>
      ) : null}

      {listing.suspicious_reasons && listing.suspicious_reasons.length > 0 ? (
        <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 px-4 py-3 space-y-1.5">
          <div className="flex items-center gap-2 text-sm font-semibold text-amber-700 dark:text-amber-400">
            <AlertTriangle className="h-4 w-4" />
            {"מודעה חשודה"}
          </div>
          <ul className="list-disc list-inside text-xs text-amber-700/80 dark:text-amber-400/80 space-y-0.5">
            {listing.suspicious_reasons.map((reason) => (
              <li key={reason}>
                {reason === "price_below_market"
                  ? "המחיר נמוך משמעותית ממחיר השוק"
                  : reason === "no_photo_low_price"
                    ? "אין תמונה והמחיר נמוך"
                    : reason}
              </li>
            ))}
          </ul>
        </div>
      ) : null}

      {/* Hero image */}
      {listing.image_url ? (
        <div className="aspect-video w-full overflow-hidden rounded-2xl bg-secondary">
          <img
            src={listing.image_url}
            alt={`${listing.manufacturer} ${listing.model}`}
            referrerPolicy="no-referrer"
            className="h-full w-full object-cover"
          />
        </div>
      ) : (
        <div className="aspect-video w-full rounded-2xl bg-secondary flex items-center justify-center">
          <span className="text-6xl opacity-15">🚗</span>
        </div>
      )}

      {/* Title + Score + Price */}
      <div className="flex items-start gap-4">
        {detailLogoSrc ? (
          <img
            src={detailLogoSrc}
            alt=""
            className="mt-1 h-11 w-11 shrink-0 object-contain"
            loading="lazy"
            decoding="async"
          />
        ) : null}
        {listing.fitness_score != null ? (
          <MatchScoreBox score={listing.fitness_score} size="lg" />
        ) : null}
        <div className="min-w-0 flex-1">
          <h1 className="text-xl font-bold tracking-tight sm:text-2xl">
            {listing.manufacturer} {listing.model}
          </h1>
          {listing.sub_model && (
            <p className="mt-0.5 text-sm text-muted-foreground font-medium">
              {listing.sub_model}
              {listing.engine_volume ? ` ${listing.engine_volume}` : ""}
              {listing.horse_power ? ` (${listing.horse_power} כ"ס)` : ""}
            </p>
          )}
          {listing.fitness_score != null ? (
            <p
              className="mt-1 text-sm font-medium"
              style={{ color: scoreHsl(listing.fitness_score) }}
            >
              {scoreLabel(listing.fitness_score)}
            </p>
          ) : null}
        </div>
        <div className="shrink-0 text-end">
          <span className="text-xl font-bold tabular-nums text-primary sm:text-2xl">
            {formatPrice(listing.price)}
          </span>
          {mc ? (
            <p className={cn("text-xs font-medium mt-0.5", mc.color)}>
              {mc.label}
            </p>
          ) : null}
        </div>
      </div>

      {/* Spec pills */}
      <div className="flex flex-wrap gap-2">
        <SpecPill icon={Calendar} value={String(listing.year)} />
        <SpecPill icon={Gauge} value={formatKm(listing.km)} />
        <SpecPill icon={Hand} value={`יד ${listing.hand > 0 ? listing.hand : "—"}`} />
        <SpecPill icon={MapPin} value={listing.city || "—"} />
        {listing.gear_box ? <SpecPill icon={Cog} value={listing.gear_box} /> : null}
        {listing.engine_type ? <SpecPill icon={Fuel} value={listing.engine_type} /> : null}
        {listing.engine_volume ? <SpecPill icon={Fuel} value={`${listing.engine_volume} סמ"ק`} /> : null}
        {listing.horse_power ? <SpecPill icon={Zap} value={`${listing.horse_power} כ"ס`} /> : null}
      </div>

      {/* Market value comparison */}
      <MarketValueCard
        price={listing.price}
        medianPrice={listing.median_price}
        basePrice={listing.base_price}
        cohortSize={listing.cohort_size}
      />

      {/* Price history chart */}
      <PriceHistoryChart token={listing.token} currentPrice={listing.price} />

      {/* Description */}
      {listing.description && (
        <div className="rounded-2xl border border-border/50 bg-card p-5">
          <h2 className="text-sm font-semibold text-foreground mb-2">תיאור</h2>
          <p className="text-sm text-muted-foreground leading-relaxed whitespace-pre-line break-words">
            {listing.description}
          </p>
        </div>
      )}

      {/* Trust indicators */}
      <div className="rounded-2xl border border-border/50 bg-card p-5 space-y-3">
        <h2 className="text-sm font-semibold text-foreground">פרטי מודעה</h2>
        <div className="flex flex-wrap gap-x-6 gap-y-2 text-sm text-muted-foreground">
          <span className="flex items-center gap-1.5">
            <Clock className="h-3.5 w-3.5" />
            {relativeTime(listing.first_seen_at)}
          </span>
          {source ? (
            <span className="flex items-center gap-1.5">
              <Shield className="h-3.5 w-3.5" />
              מקור: {source}
            </span>
          ) : null}
          {listing.is_commercial != null ? (
            <span className="flex items-center gap-1.5">
              {listing.is_commercial ? (
                <><Store className="h-3.5 w-3.5" />מסחרי</>
              ) : (
                <><User className="h-3.5 w-3.5" />פרטי</>
              )}
            </span>
          ) : null}
          {listing.image_url ? (
            <span className="flex items-center gap-1.5">
              <Eye className="h-3.5 w-3.5" />
              כולל תמונה
            </span>
          ) : (
            <span className="flex items-center gap-1.5 text-amber-600 dark:text-amber-400">
              <EyeOff className="h-3.5 w-3.5" />
              ללא תמונה
            </span>
          )}
        </div>
      </div>

      {/* Desktop actions */}
      <div className="hidden md:flex flex-wrap gap-3">
        <Button
          type="button"
          variant={saved ? "secondary" : "primary"}
          size="lg"
          disabled={isSaving}
          onClick={() => toggleSaved()}
        >
          {saved ? (
            <><BookmarkCheck className="h-4 w-4" />שמור</>
          ) : (
            <><Bookmark className="h-4 w-4" />שמור למועדפים</>
          )}
        </Button>
        <Button
          type="button"
          variant="secondary"
          size="lg"
          disabled={isTogglingSeen}
          onClick={() => toggleSeen()}
        >
          {seen ? (
            <><EyeOff className="h-4 w-4" />החזר לחדשות</>
          ) : (
            <><Eye className="h-4 w-4" />סמן כנצפה</>
          )}
        </Button>
        {safeHref(listing.page_link) ? (
          <Button
            as="a"
            href={safeHref(listing.page_link)!}
            target="_blank"
            rel="noopener noreferrer"
            variant="secondary"
            size="lg"
          >
            <ExternalLink className="h-4 w-4" />
            צפה במודעה
          </Button>
        ) : null}
      </div>

      {/* Mobile sticky action bar */}
      <div className="fixed inset-x-0 bottom-[var(--bottom-nav-h)] z-40 border-t border-border/50 bg-card/95 px-4 py-3 backdrop-blur-xl md:hidden">
        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant={saved ? "secondary" : "primary"}
            size="md"
            className="flex-1"
            disabled={isSaving}
            onClick={() => toggleSaved()}
          >
            {saved ? (
              <><BookmarkCheck className="h-4 w-4" />שמור</>
            ) : (
              <><Bookmark className="h-4 w-4" />שמור</>
            )}
          </Button>
          <Button
            type="button"
            variant="secondary"
            size="md"
            disabled={isTogglingSeen}
            onClick={() => toggleSeen()}
          >
            {seen ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
          </Button>
          {safeHref(listing.page_link) ? (
            <Button
              as="a"
              href={safeHref(listing.page_link)!}
              target="_blank"
              rel="noopener noreferrer"
              variant="secondary"
              size="md"
            >
              <ExternalLink className="h-4 w-4" />
            </Button>
          ) : null}
        </div>
      </div>
    </div>
  );
}

function MarketValueCard({
  price,
  medianPrice,
  basePrice,
  cohortSize,
}: {
  price: number;
  medianPrice?: number;
  basePrice?: number;
  cohortSize?: number;
}) {
  const hasBase = basePrice != null && basePrice > 0;
  const hasMedian = medianPrice != null && medianPrice > 0;

  if (!hasBase && !hasMedian) {
    return (
      <div className="rounded-2xl border border-border/50 bg-card p-5">
        <h2 className="text-sm font-semibold text-foreground mb-1">שווי שוק</h2>
        <p className="text-sm text-muted-foreground">אין מידע על מחיר שוק לרכב זה</p>
      </div>
    );
  }

  const referenceForBar = hasBase ? basePrice! : medianPrice!;
  const mc = marketComparison(price, medianPrice, basePrice);
  if (!mc) return null;

  const ratio = Math.min(Math.max(price / referenceForBar, 0.5), 1.5);
  const pct = ((ratio - 0.5) / 1.0) * 100;

  const barTint =
    mc.diffPercent > 5
      ? "bg-emerald-500/30"
      : mc.diffPercent >= -5
        ? "bg-muted-foreground/20"
        : "bg-amber-500/30";
  const markerTint =
    mc.diffPercent > 5
      ? "bg-emerald-500"
      : mc.diffPercent >= -5
        ? "bg-muted-foreground"
        : "bg-amber-500";

  return (
    <div className="rounded-2xl border border-border/50 bg-card p-5 space-y-3">
      <div className="flex items-baseline justify-between gap-2">
        <h2 className="text-sm font-semibold text-foreground">
          {hasBase ? "מחירון Yad2" : "שווי שוק"}
        </h2>
        <span className="text-xl font-bold tabular-nums text-foreground">
          {formatPrice(referenceForBar)}
        </span>
      </div>

      <div className="flex items-baseline justify-between gap-2">
        <span className={cn("text-sm font-medium", mc.color)}>{mc.label}</span>
        <span className={cn("text-sm font-semibold tabular-nums", mc.color)}>
          {mc.diffPercent > 5
            ? `${mc.diffPercent}%−`
            : mc.diffPercent >= -5
              ? "≈"
              : `${-mc.diffPercent}%+`}
        </span>
      </div>

      <div className="relative h-2 rounded-full bg-secondary overflow-hidden">
        <div
          className={cn("absolute inset-y-0 start-0 rounded-full transition-all", barTint)}
          style={{ width: `${pct}%` }}
        />
        <div
          className={cn(
            "absolute top-1/2 -translate-y-1/2 h-4 w-1 rounded-full",
            markerTint,
          )}
          style={{ insetInlineStart: `${pct}%` }}
        />
        <div
          className="absolute top-1/2 -translate-y-1/2 h-4 w-0.5 rounded-full bg-foreground/40"
          style={{ insetInlineStart: "50%" }}
        />
      </div>
      <div className="flex justify-between text-[10px] text-muted-foreground tabular-nums">
        <span>{formatPrice(price)}</span>
        <span>{hasBase ? "מחירון" : "חציון"}</span>
      </div>

      {hasBase && hasMedian ? (
        <p className="text-xs text-muted-foreground text-center">
          מחיר ממוצע במודעות:{" "}
          <span className="tabular-nums font-medium text-foreground/90">
            {formatPrice(medianPrice!)}
          </span>
        </p>
      ) : null}

      {cohortSize != null && cohortSize > 0 && (
        <p className="text-[11px] text-muted-foreground text-center">
          על בסיס {cohortSize} מודעות דומות
        </p>
      )}
    </div>
  );
}

function SpecPill({
  icon: Icon,
  value,
}: {
  icon: React.ComponentType<{ className?: string }>;
  value: string;
}) {
  return (
    <span className="inline-flex items-center gap-1.5 rounded-lg bg-secondary px-3 py-1.5 text-sm text-muted-foreground">
      <Icon className="h-3.5 w-3.5 shrink-0" />
      <span className="tabular-nums">{value}</span>
    </span>
  );
}
