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
  Star,
} from "lucide-react";
import { formatPrice, formatKm, relativeTime, cn } from "@/lib/utils";
import { api } from "@/lib/api";
import type { Listing } from "@/lib/api";

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
  }, [token, stateListingForToken]);

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
      <div className="space-y-6 animate-fade-in">
        <div className="h-6 w-32 rounded-lg skeleton" />
        <div className="aspect-video w-full rounded-2xl skeleton" />
        <div className="flex justify-between">
          <div className="space-y-2">
            <div className="h-8 w-48 rounded-lg skeleton" />
            <div className="h-5 w-24 rounded-lg skeleton" />
          </div>
          <div className="h-10 w-32 rounded-lg skeleton" />
        </div>
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="h-24 rounded-2xl skeleton" />
          ))}
        </div>
      </div>
    );
  }

  if (error || !listing) {
    return (
      <div className="flex flex-col items-center justify-center py-24 animate-fade-in">
        <div className="flex h-16 w-16 items-center justify-center rounded-2xl bg-muted mb-4">
          <span className="text-3xl opacity-30">🔍</span>
        </div>
        <p className="text-xl font-bold mb-2">המודעה לא נמצאה</p>
        <p className="text-sm text-muted-foreground mb-8">
          ניתן לגשת למודעה דרך רשימת התוצאות
        </p>
        <Link
          to="/"
          className="inline-flex items-center gap-2 rounded-xl gradient-primary px-6 py-3 text-sm font-semibold text-white shadow-md hover:shadow-lg hover:brightness-110 transition-all"
        >
          <ArrowRight className="h-4 w-4" />
          חזרה לחיפושים
        </Link>
      </div>
    );
  }

  return (
    <div className="space-y-6 pb-20 md:pb-4 animate-fade-in">
      {/* Back */}
      <button
        onClick={() => navigate(-1)}
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors font-medium"
      >
        <ArrowRight className="h-4 w-4" />
        חזרה לתוצאות
      </button>

      {/* Cover image */}
      {listing.image_url ? (
        <div className="aspect-video w-full overflow-hidden rounded-2xl bg-muted shadow-lg relative">
          <img
            src={listing.image_url}
            alt={`${listing.manufacturer} ${listing.model}`}
            className="h-full w-full object-cover"
          />
          <div className="absolute inset-0 bg-gradient-to-t from-black/30 via-transparent to-transparent pointer-events-none" />
        </div>
      ) : (
        <div className="aspect-video w-full rounded-2xl bg-gradient-to-br from-muted to-muted/50 flex items-center justify-center shadow-lg">
          <span className="text-7xl opacity-15">🚗</span>
        </div>
      )}

      {/* Title + Price */}
      <div className="flex items-start justify-between animate-slide-up stagger-1">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">
            {listing.manufacturer} {listing.model}
          </h1>
          <p className="text-lg text-muted-foreground">{listing.year}</p>
        </div>
        <div className="text-left">
          <span className="text-3xl font-bold text-gradient">
            {formatPrice(listing.price)}
          </span>
        </div>
      </div>

      {/* Specs grid */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4 animate-slide-up stagger-2">
        <SpecCard icon={Gauge} label='ק"מ' value={formatKm(listing.km)} />
        <SpecCard icon={Hand} label="יד" value={String(listing.hand)} />
        <SpecCard icon={MapPin} label="עיר" value={listing.city || "—"} />
        <SpecCard icon={Calendar} label="שנה" value={String(listing.year)} />
      </div>

      {/* Score + Seen */}
      <div className="flex flex-wrap items-center gap-4 animate-slide-up stagger-3">
        {listing.fitness_score != null && (
          <div className="flex items-center gap-2.5">
            <span className="text-sm text-muted-foreground font-medium">
              ציון התאמה:
            </span>
            <span
              className={cn(
                "inline-flex items-center gap-1 rounded-full px-3.5 py-1.5 text-sm font-bold",
                listing.fitness_score >= 7
                  ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-400"
                  : listing.fitness_score >= 5
                    ? "bg-amber-100 text-amber-700 dark:bg-amber-500/20 dark:text-amber-400"
                    : "bg-secondary text-muted-foreground",
              )}
            >
              {listing.fitness_score >= 7 && <Star className="h-3.5 w-3.5" />}
              {listing.fitness_score.toFixed(1)}
            </span>
          </div>
        )}
        <div className="flex items-center gap-1.5 text-sm text-muted-foreground">
          <Clock className="h-4 w-4" />
          {relativeTime(listing.first_seen_at)}
        </div>
      </div>

      {/* CTA */}
      <a
        href={listing.page_link}
        target="_blank"
        rel="noopener noreferrer"
        className="inline-flex items-center gap-2.5 rounded-xl gradient-primary px-8 py-3.5 text-base font-semibold text-white shadow-lg hover:shadow-xl hover:brightness-110 transition-all duration-200 animate-slide-up stagger-4"
      >
        <ExternalLink className="h-5 w-5" />
        צפה במודעה המקורית
      </a>
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
    <div className="rounded-2xl border border-border bg-card p-4 text-center gradient-card card-shine">
      <Icon className="mx-auto h-5 w-5 text-primary/70 mb-1.5" />
      <p className="text-xs text-muted-foreground font-medium">{label}</p>
      <p className="text-sm font-bold mt-0.5 tracking-tight">{value}</p>
    </div>
  );
}
