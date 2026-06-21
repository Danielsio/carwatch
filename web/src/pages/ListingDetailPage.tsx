import { useLocation, Link, useNavigate, useParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { lazy, Suspense, type ReactNode } from "react";
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
  TrendingDown,
} from "lucide-react";
import { formatPrice, formatKm, relativeTime, safeHref, marketComparison, cn, listingSource } from "@/lib/utils";
import { api, ApiError } from "@/lib/api";
import type { Listing } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { MatchScoreBox } from "@/components/ui/MatchScoreBox";
import { scoreHsl, scoreLabel } from "@/lib/scoringAlgorithm";
import { useListingActions } from "@/hooks/useListingActions";
import { useSpotlight } from "@/hooks/useSpotlight";
import { manufacturerLogoSrc } from "@/lib/manufacturerLogo";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/EmptyState";
const PriceHistoryChart = lazy(() =>
  import("@/components/PriceHistoryChart").then((m) => ({ default: m.PriceHistoryChart })),
);

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
  const spotlight = useSpotlight();

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

      {/* Hero — magazine-style with overlaid title, price and badges */}
      <div
        {...spotlight}
        className="spotlight group relative aspect-video w-full overflow-hidden rounded-3xl border border-border/50 bg-secondary shadow-[0_24px_70px_-24px_var(--color-glow-primary)] animate-scale-in motion-reduce:animate-none"
      >
        {listing.image_url ? (
          <img
            src={listing.image_url}
            alt={`${listing.manufacturer} ${listing.model}`}
            referrerPolicy="no-referrer"
            width={1280}
            height={720}
            className="h-full w-full object-cover transition-transform duration-700 ease-out group-hover:scale-[1.03] motion-reduce:transition-none"
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center bg-gradient-to-br from-secondary to-muted">
            <span className="text-7xl opacity-15">🚗</span>
          </div>
        )}
        <div className="pointer-events-none absolute inset-0 bg-gradient-to-t from-black/75 via-black/15 to-transparent" />

        {/* Top badges */}
        <div className="absolute inset-x-0 top-0 z-[1] flex items-start justify-between gap-2 p-4">
          <div className="flex flex-wrap items-center gap-1.5">
            {mc && mc.diffPercent > 5 ? (
              <span className="flex items-center gap-1 rounded-full bg-deal-good px-2.5 py-1 text-xs font-bold text-white shadow-sm">
                <TrendingDown className="h-3.5 w-3.5" />
                {mc.diffPercent}%− מתחת לשוק
              </span>
            ) : null}
            {listing.removed_at ? (
              <span className="rounded-full bg-black/60 px-2.5 py-1 text-xs font-medium text-white/80 backdrop-blur-sm">
                נמכר
              </span>
            ) : null}
          </div>
          {source ? (
            <span className="rounded-full bg-black/40 px-2.5 py-1 text-[11px] font-semibold text-white/90 backdrop-blur-sm">
              {source}
            </span>
          ) : null}
        </div>

        {/* Bottom: logo + title + price */}
        <div className="absolute inset-x-0 bottom-0 z-[1] flex items-end justify-between gap-3 p-5">
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              {detailLogoSrc ? (
                <img
                  src={detailLogoSrc}
                  alt=""
                  className="h-8 w-8 shrink-0 object-contain drop-shadow"
                  loading="lazy"
                  decoding="async"
                />
              ) : null}
              <h1 className="truncate text-2xl font-extrabold tracking-tight text-white drop-shadow sm:text-3xl">
                {listing.manufacturer} {listing.model}
              </h1>
            </div>
            <p className="mt-1 text-sm text-white/75">
              {[listing.sub_model, listing.year, listing.city || null]
                .filter(Boolean)
                .join(" · ")}
            </p>
          </div>
          <span className="shrink-0 text-2xl font-extrabold tabular-nums text-white drop-shadow sm:text-3xl">
            {formatPrice(listing.price)}
          </span>
        </div>
      </div>

      {/* Score + market summary */}
      {(listing.fitness_score != null || mc) && (
        <div className="flex flex-wrap items-stretch gap-3">
          {listing.fitness_score != null ? (
            <div className="flex items-center gap-2.5 rounded-2xl border border-border/50 bg-card px-3.5 py-2.5">
              <MatchScoreBox score={listing.fitness_score} size="lg" />
              <div>
                <p className="text-xs text-muted-foreground">ציון התאמה</p>
                <p
                  className="text-sm font-semibold"
                  style={{ color: scoreHsl(listing.fitness_score) }}
                >
                  {listing.fitness_score.toFixed(1)} · {scoreLabel(listing.fitness_score)}
                </p>
              </div>
            </div>
          ) : null}
          {mc ? (
            <div className="flex flex-col justify-center rounded-2xl border border-border/50 bg-card px-3.5 py-2.5">
              <p className="text-xs text-muted-foreground">מול מחיר השוק</p>
              <p className={cn("text-sm font-semibold", mc.color)}>{mc.label}</p>
            </div>
          ) : null}
        </div>
      )}

      {/* Spec tiles */}
      <div className="grid grid-cols-3 gap-2 sm:grid-cols-4">
        <SpecPill icon={Calendar} label="שנה" value={String(listing.year)} />
        <SpecPill icon={Gauge} label="ק״מ" value={formatKm(listing.km)} />
        <SpecPill icon={Hand} label="יד" value={listing.hand > 0 ? String(listing.hand) : "—"} />
        <SpecPill icon={MapPin} label="עיר" value={listing.city || "—"} />
        {listing.gear_box ? <SpecPill icon={Cog} label="הילוכים" value={listing.gear_box} /> : null}
        {listing.engine_type ? <SpecPill icon={Fuel} label="דלק" value={listing.engine_type} /> : null}
        {listing.engine_volume ? <SpecPill icon={Fuel} label="נפח" value={`${listing.engine_volume}`} /> : null}
        {listing.horse_power ? <SpecPill icon={Zap} label="כ״ס" value={String(listing.horse_power)} /> : null}
      </div>

      {/* Market value comparison */}
      <MarketValueCard
        price={listing.price}
        medianPrice={listing.median_price}
        basePrice={listing.base_price}
        cohortSize={listing.cohort_size}
      />

      {/* Price history chart */}
      <Suspense fallback={<Skeleton className="h-52 rounded-2xl" />}>
        <PriceHistoryChart token={listing.token} currentPrice={listing.price} />
      </Suspense>

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
            {relativeTime(listing.posted_at || listing.first_seen_at)}
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
          <Button asChild variant="secondary" size="lg">
            <a
              href={safeHref(listing.page_link)!}
              target="_blank"
              rel="noopener noreferrer"
            >
              <ExternalLink className="h-4 w-4" />
              צפה במודעה
            </a>
          </Button>
        ) : null}
      </div>

      {/* Mobile sticky action bar */}
      <div className="fixed inset-x-0 bottom-[var(--bottom-nav-h)] z-40 border-t border-border/50 bg-card/95 px-4 py-3 backdrop-blur-xl md:hidden">
        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant={saved ? "secondary" : "primary"}
            size="default"
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
            size="default"
            disabled={isTogglingSeen}
            onClick={() => toggleSeen()}
          >
            {seen ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
          </Button>
          {safeHref(listing.page_link) ? (
            <Button asChild variant="secondary" size="default">
              <a
                href={safeHref(listing.page_link)!}
                target="_blank"
                rel="noopener noreferrer"
              >
                <ExternalLink className="h-4 w-4" />
              </a>
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
  label,
  value,
}: {
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  value: string;
}) {
  return (
    <div className="flex items-center gap-2.5 rounded-xl border border-border/50 bg-card px-3 py-2.5">
      <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
        <Icon className="h-4 w-4" />
      </span>
      <div className="min-w-0">
        <p className="text-[10px] uppercase tracking-wide text-muted-foreground/70">
          {label}
        </p>
        <p className="truncate text-sm font-semibold text-foreground tabular-nums">
          {value}
        </p>
      </div>
    </div>
  );
}
