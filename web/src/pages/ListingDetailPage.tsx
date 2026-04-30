import { useEffect, useState } from "react";
import { useLocation, Link, useNavigate, useParams } from "react-router";
import {
  ArrowRight,
  ExternalLink,
  Calendar,
  Gauge,
  Hand,
  MapPin,
  Clock,
  Car,
} from "lucide-react";
import { formatPrice, formatKm, relativeTime, safeHref } from "@/lib/utils";
import { api } from "@/lib/api";
import type { Listing } from "@/lib/api";
import { Button } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";
import { Skeleton } from "@/components/ui/Skeleton";
import { EmptyState } from "@/components/ui/EmptyState";

export function ListingDetailPage() {
  const location = useLocation();
  const navigate = useNavigate();
  const { token } = useParams();
  const stateListingRaw = location.state?.listing as Listing | undefined;
  const stateListingForToken =
    stateListingRaw?.token === token ? stateListingRaw : undefined;

  const [listing, setListing] = useState<Listing | undefined>(
    stateListingForToken,
  );
  const [loading, setLoading] = useState(!stateListingForToken && !!token);
  const [error, setError] = useState(false);

  useEffect(() => {
    setListing(stateListingForToken);
    setError(false);
    setLoading(!stateListingForToken && !!token);
  }, [token]);

  useEffect(() => {
    if (listing || !token) return;
    setLoading(true);
    api
      .listing(token)
      .then((data) => setListing(data))
      .catch(() => setError(true))
      .finally(() => setLoading(false));
  }, [listing, token]);

  if (loading) {
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

  if (error || !listing) {
    return (
      <EmptyState
        icon={Car}
        title="המודעה לא נמצאה"
        description="ניתן לגשת למודעה דרך רשימת התוצאות"
        action={
          <Button asChild>
            <Link to="/">
              <ArrowRight className="h-4 w-4" />
              חזרה לחיפושים
            </Link>
          </Button>
        }
      />
    );
  }

  return (
    <div className="space-y-6 pb-20 md:pb-4">
      <Button variant="ghost" size="sm" onClick={() => navigate(-1)} className="text-muted-foreground hover:text-foreground -mr-2">
        <ArrowRight className="h-4 w-4" />
        חזרה לתוצאות
      </Button>

      {/* Hero image */}
      {listing.image_url ? (
        <div className="aspect-video w-full overflow-hidden rounded-2xl bg-secondary">
          <img
            src={listing.image_url}
            alt={`${listing.manufacturer} ${listing.model}`}
            className="h-full w-full object-cover"
          />
        </div>
      ) : (
        <div className="aspect-video w-full rounded-2xl bg-secondary flex items-center justify-center">
          <span className="text-6xl opacity-15">🚗</span>
        </div>
      )}

      {/* Title + Price */}
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">
            {listing.manufacturer} {listing.model}
          </h1>
          <p className="text-muted-foreground mt-0.5">{listing.year}</p>
        </div>
        <span className="text-2xl font-bold text-primary tabular-nums">
          {formatPrice(listing.price)}
        </span>
      </div>

      {/* Specs grid */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <SpecCard icon={Gauge} label='ק"מ' value={formatKm(listing.km)} />
        <SpecCard icon={Hand} label="יד" value={String(listing.hand)} />
        <SpecCard icon={MapPin} label="עיר" value={listing.city || "—"} />
        <SpecCard
          icon={Calendar}
          label="שנה"
          value={String(listing.year)}
        />
      </div>

      <div className="flex items-center gap-4">
        {listing.fitness_score != null && (
          <div className="flex items-center gap-2">
            <span className="text-sm text-muted-foreground">ציון התאמה:</span>
            <Badge
              variant={
                listing.fitness_score >= 7
                  ? "success"
                  : listing.fitness_score >= 5
                    ? "warning"
                    : "default"
              }
              className="text-sm tabular-nums"
            >
              {listing.fitness_score.toFixed(1)}
            </Badge>
          </div>
        )}
        <div className="flex items-center gap-1.5 text-sm text-muted-foreground">
          <Clock className="h-4 w-4" />
          {relativeTime(listing.first_seen_at)}
        </div>
      </div>

      {safeHref(listing.page_link) && (
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
      )}
    </div>
  );
}

function SpecCard({
  icon: Icon,
  label,
  value,
}: {
  icon: React.ComponentType<{ className?: string }>;
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
