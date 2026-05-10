import { useLocation, Link, useNavigate, useParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { useState, useEffect, type ReactNode } from "react";
import type { ComponentType } from "react";
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
} from "lucide-react";
import { formatPrice, formatKm, relativeTime, safeHref } from "@/lib/utils";
import { api, ApiError } from "@/lib/api";
import type { Listing } from "@/lib/api";
import { Button } from "@/components/ui/Button";
import { MatchScoreBox } from "@/components/ui/MatchScoreBox";
import { scoreHsl, scoreLabel } from "@/lib/scoringAlgorithm";
import { useMarkListingSeen, useUnmarkListingSeen } from "@/hooks/useListingSeen";
import { Skeleton } from "@/components/ui/Skeleton";
import { EmptyState } from "@/components/ui/EmptyState";

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
            <Skeleton key={i} className="h-24 rounded-2xl" />
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

  return (
    <ListingDetailContent
      listing={listing}
      backButton={
        <Button variant="ghost" size="sm" onClick={() => navigate(-1)} className="text-muted-foreground hover:text-foreground -mr-2">
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
  const markSeen = useMarkListingSeen();
  const unmarkSeen = useUnmarkListingSeen();
  const [seen, setSeen] = useState(() => listing.seen ?? false);

  useEffect(() => {
    setSeen(listing.seen ?? false);
  }, [listing.token, listing.seen]);

  const hasVehicleSpecs =
    listing.engine_volume || listing.horse_power || listing.engine_type || listing.gear_box;

  return (
    <div className="space-y-6">
      {backButton}

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

      {/* Title + score + Price */}
      <div className="flex items-start gap-4">
        {listing.fitness_score != null ? (
          <MatchScoreBox score={listing.fitness_score} size="lg" />
        ) : null}
        <div className="min-w-0 flex-1">
          <h1 className="text-2xl font-bold tracking-tight">
            {listing.manufacturer} {listing.model}
          </h1>
          {listing.sub_model && (
            <p className="mt-0.5 text-sm text-muted-foreground font-medium">
              {listing.sub_model}
              {listing.engine_volume ? ` ${listing.engine_volume}` : ""}
              {listing.horse_power ? ` (${listing.horse_power} כ"ס)` : ""}
            </p>
          )}
          <p className="mt-0.5 text-muted-foreground">{listing.year}</p>
          {listing.fitness_score != null ? (
            <p
              className="mt-1 text-sm font-medium"
              style={{ color: scoreHsl(listing.fitness_score) }}
            >
              {scoreLabel(listing.fitness_score)}
            </p>
          ) : null}
        </div>
        <span className="shrink-0 text-2xl font-bold tabular-nums text-primary">
          {formatPrice(listing.price)}
        </span>
      </div>

      {/* Primary specs grid */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <SpecCard icon={Calendar} label="שנה" value={String(listing.year)} />
        <SpecCard icon={MapPin} label="עיר" value={listing.city || "—"} />
        <SpecCard icon={Hand} label="יד" value={String(listing.hand)} />
        <SpecCard icon={Gauge} label='ק"מ' value={formatKm(listing.km)} />
      </div>

      {/* Vehicle specs grid */}
      {hasVehicleSpecs && (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          {listing.engine_volume ? (
            <SpecCard icon={Fuel} label="נפח מנוע" value={`${listing.engine_volume}`} />
          ) : null}
          {listing.horse_power ? (
            <SpecCard icon={Zap} label='כ"ס' value={String(listing.horse_power)} />
          ) : null}
          {listing.engine_type ? (
            <SpecCard icon={Fuel} label="סוג מנוע" value={listing.engine_type} />
          ) : null}
          {listing.gear_box ? (
            <SpecCard icon={Cog} label="תיבת הילוכים" value={listing.gear_box} />
          ) : null}
        </div>
      )}

      {/* Description */}
      {listing.description && (
        <div className="rounded-2xl border border-border/50 bg-card p-5">
          <h2 className="text-sm font-semibold text-foreground mb-2">תיאור</h2>
          <p className="text-sm text-muted-foreground leading-relaxed whitespace-pre-line break-words">
            {listing.description}
          </p>
        </div>
      )}

      <div className="flex items-center gap-1.5 text-sm text-muted-foreground">
        <Clock className="h-4 w-4" />
        {relativeTime(listing.first_seen_at)}
      </div>

      <div className="flex flex-wrap gap-3">
        <Button
          type="button"
          variant="secondary"
          size="lg"
          disabled={seen ? unmarkSeen.isPending : markSeen.isPending}
          onClick={() => {
            const next = !seen;
            setSeen(next);
            const mutation = next ? markSeen : unmarkSeen;
            mutation.mutate(listing.token, { onError: () => setSeen(!next) });
          }}
        >
          {seen ? (
            <>
              <EyeOff className="h-4 w-4" />
              החזר לחדשות
            </>
          ) : (
            <>
              <Eye className="h-4 w-4" />
              סמן כנצפה
            </>
          )}
        </Button>
        {safeHref(listing.page_link) ? (
          <Button
            as="a"
            href={safeHref(listing.page_link)!}
            target="_blank"
            rel="noopener noreferrer"
            size="lg"
          >
            <ExternalLink className="h-4 w-4" />
            צפה במודעה המקורית
          </Button>
        ) : null}
      </div>
    </div>
  );
}

function SpecCard({
  icon: Icon,
  label,
  value,
}: {
  icon: ComponentType<{ className?: string }>;
  label: string;
  value: string;
}) {
  return (
    <div className="rounded-2xl border border-border/50 bg-card p-4 text-center transition-colors duration-200 hover:border-border">
      <Icon className="mx-auto h-5 w-5 text-muted-foreground mb-1.5" />
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="text-sm font-semibold mt-0.5 tabular-nums">{value}</p>
    </div>
  );
}
