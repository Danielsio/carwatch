import { useLocation, Link, useNavigate } from "react-router";
import {
  ArrowRight,
  ExternalLink,
  Calendar,
  Gauge,
  Hand,
  MapPin,
  Clock,
} from "lucide-react";
import { formatPrice, formatKm, relativeTime, cn } from "@/lib/utils";
import type { Listing } from "@/lib/api";

export function ListingDetailPage() {
  const location = useLocation();
  const navigate = useNavigate();
  const listing = location.state?.listing as Listing | undefined;

  if (!listing) {
    return (
      <div className="flex flex-col items-center justify-center py-20">
        <p className="text-lg font-medium mb-2">המודעה לא נמצאה</p>
        <p className="text-sm text-muted-foreground mb-6">
          ניתן לגשת למודעה דרך רשימת התוצאות
        </p>
        <Link
          to="/"
          className="inline-flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 transition-colors"
        >
          <ArrowRight className="h-4 w-4" />
          חזרה לחיפושים
        </Link>
      </div>
    );
  }

  return (
    <div className="space-y-6 pb-20 md:pb-4">
      {/* Back button */}
      <button
        onClick={() => navigate(-1)}
        className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground transition-colors"
      >
        <ArrowRight className="h-4 w-4" />
        חזרה לתוצאות
      </button>

      {/* Cover image */}
      {listing.image_url ? (
        <div className="aspect-video w-full overflow-hidden rounded-xl bg-muted">
          <img
            src={listing.image_url}
            alt={`${listing.manufacturer} ${listing.model}`}
            className="h-full w-full object-cover"
          />
        </div>
      ) : (
        <div className="aspect-video w-full rounded-xl bg-muted flex items-center justify-center">
          <span className="text-6xl text-muted-foreground/20">🚗</span>
        </div>
      )}

      {/* Title + Price */}
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold">
            {listing.manufacturer} {listing.model}
          </h1>
          <p className="text-muted-foreground">{listing.year}</p>
        </div>
        <span className="text-2xl font-bold text-primary">
          {formatPrice(listing.price)}
        </span>
      </div>

      {/* Specs grid */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <SpecCard
          icon={Gauge}
          label='ק"מ'
          value={formatKm(listing.km)}
        />
        <SpecCard
          icon={Hand}
          label="יד"
          value={String(listing.hand)}
        />
        <SpecCard
          icon={MapPin}
          label="עיר"
          value={listing.city || "—"}
        />
        <SpecCard
          icon={Calendar}
          label="שנה"
          value={String(listing.year)}
        />
      </div>

      {/* Score + Seen */}
      <div className="flex items-center gap-4">
        {listing.fitness_score != null && (
          <div className="flex items-center gap-2">
            <span className="text-sm text-muted-foreground">ציון התאמה:</span>
            <span
              className={cn(
                "inline-flex items-center rounded-full px-3 py-1 text-sm font-medium",
                listing.fitness_score >= 7
                  ? "bg-green-100 text-green-800"
                  : listing.fitness_score >= 5
                    ? "bg-yellow-100 text-yellow-800"
                    : "bg-gray-100 text-gray-600",
              )}
            >
              {listing.fitness_score.toFixed(1)}
            </span>
          </div>
        )}
        <div className="flex items-center gap-1.5 text-sm text-muted-foreground">
          <Clock className="h-4 w-4" />
          {relativeTime(listing.first_seen_at)}
        </div>
      </div>

      {/* External link */}
      <a
        href={listing.page_link}
        target="_blank"
        rel="noopener noreferrer"
        className="inline-flex items-center gap-2 rounded-lg bg-primary px-6 py-3 text-sm font-medium text-primary-foreground hover:bg-primary/90 transition-colors"
      >
        <ExternalLink className="h-4 w-4" />
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
    <div className="rounded-xl border border-border bg-card p-4 text-center">
      <Icon className="mx-auto h-5 w-5 text-muted-foreground mb-1" />
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="text-sm font-semibold mt-0.5">{value}</p>
    </div>
  );
}
