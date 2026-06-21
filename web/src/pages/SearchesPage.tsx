import { useState } from "react";
import { Link } from "react-router";
import { usePageTitle } from "@/hooks/usePageTitle";
import { useAuth } from "@/contexts/AuthContext";
import { Plus, Search as SearchIcon, Activity, Bell, Car, Sparkles } from "lucide-react";
import {
  useSearches,
  useDeleteSearch,
  usePauseSearch,
  useResumeSearch,
} from "@/hooks/useSearches";
import {
  useNotificationCount,
  useNotifications,
} from "@/hooks/useNotifications";
import { useSearchCycleStats } from "@/hooks/useSearchCycleStats";
import { formatPrice, relativeTime, cn } from "@/lib/utils";
import type { Listing } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { PageHeader } from "@/components/ui/PageHeader";
import { ErrorState } from "@/components/ui/ErrorState";
import { SearchCard } from "@/components/SearchCard";
import { Skeleton } from "@/components/ui/skeleton";
import { SectionHeader } from "@/components/ui/SectionHeader";
import { NumberTicker } from "@/components/ui/number-ticker";
import { NextScanCountdown } from "@/components/NextScanCountdown";
import { useToast } from "@/components/ui/Toast";

const STAGGER_MS = 60;

export function SearchesPage() {
  usePageTitle("לוח בקרה");
  const { user } = useAuth();
  const { toast } = useToast();
  const { data: searches, isLoading, isError } = useSearches(!!user);
  const { data: notifCount } = useNotificationCount(!!user);
  const { data: recentListings } = useNotifications(5, 0, !!user);
  const { data: cycleStats } = useSearchCycleStats(!!user);
  const cycleStatsMap = new Map(cycleStats?.map(s => [s.search_id, s]) ?? []);
  const deleteSearch = useDeleteSearch();
  const pauseSearch = usePauseSearch();
  const resumeSearch = useResumeSearch();
  const isMutating =
    deleteSearch.isPending || pauseSearch.isPending || resumeSearch.isPending;
  const [confirmDelete, setConfirmDelete] = useState<number | null>(null);

  const unread = notifCount?.count ?? 0;
  const activeCount = searches?.filter((s) => s.active).length ?? 0;
  const totalSearches = searches?.length ?? 0;

  if (!user) {
    return (
      <div className="space-y-8 animate-fade-in">
        <PageHeader title="לוח בקרה" />
        <div className="rounded-2xl border border-border bg-card p-6 sm:p-10 text-center space-y-6">
          <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-2xl bg-muted text-muted-foreground">
            <Sparkles className="h-8 w-8" />
          </div>
          <div className="space-y-2">
            <h2 className="text-xl font-bold text-foreground">ברוך הבא ל-CarWatch!</h2>
            <p className="mx-auto max-w-md text-sm text-muted-foreground leading-relaxed">
              הירשם בחינם כדי ליצור חיפושים, לעקוב אחר מודעות רכב ולקבל התראות בזמן אמת. תוכל גם לנסות חיפוש מהיר בלי הרשמה.
            </p>
          </div>
          <div className="flex flex-col sm:flex-row items-center justify-center gap-3">
            <Button asChild size="lg">
              <Link to="/login">
                <Plus className="h-4 w-4" />
                הירשם בחינם
              </Link>
            </Button>
            <Button asChild variant="secondary" size="lg">
              <Link to="/try">
                <SearchIcon className="h-4 w-4" />
                נסה חיפוש מהיר
              </Link>
            </Button>
          </div>
        </div>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="space-y-8 animate-fade-in motion-reduce:animate-none">
        <div className="flex items-start justify-between gap-4">
          <div className="space-y-2">
            <Skeleton className="h-8 w-40" />
            <Skeleton className="h-4 w-56" />
          </div>
          <Skeleton className="h-10 w-36 rounded-xl" />
        </div>
        <div className="grid grid-cols-2 gap-3 sm:gap-4 md:grid-cols-4">
          {[1, 2, 3, 4].map((i) => (
            <Skeleton key={i} className="h-[100px] rounded-xl" />
          ))}
        </div>
        <Skeleton className="h-5 w-32" />
        <div className="grid gap-4 sm:grid-cols-2">
          {[1, 2].map((i) => (
            <Skeleton key={i} className="h-52 rounded-2xl" />
          ))}
        </div>
      </div>
    );
  }

  if (isError) {
    return (
      <div className="space-y-8">
        <DashboardHeader />
        <ErrorState
          title="שגיאה בטעינת החיפושים"
          description="נסה לרענן את הדף"
          onRetry={() => window.location.reload()}
        />
      </div>
    );
  }

  return (
    <div className="space-y-8 pb-4 animate-fade-in motion-reduce:animate-none">
      <div className="animate-slide-up motion-reduce:animate-none">
        <DashboardHeader />
      </div>

      {/* Stats */}
      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        {[
          { icon: SearchIcon, label: "חיפושים פעילים", value: activeCount },
          {
            icon: Car,
            label: "מודעות שנמצאו",
            value:
              searches?.reduce((sum, s) => sum + (s.listings_count ?? 0), 0) ?? 0,
          },
          { icon: Bell, label: "מודעות חדשות", value: unread, highlight: unread > 0 },
          { icon: Activity, label: "סה״כ חיפושים", value: totalSearches },
        ].map((stat) => (
          <StatCard key={stat.label} {...stat} />
        ))}
      </div>

      {/* Next scan countdown */}
      {totalSearches > 0 && (
        <div className="animate-slide-up motion-reduce:animate-none" style={{ animationDelay: "40ms", animationFillMode: "backwards" }}>
          <NextScanCountdown />
        </div>
      )}

      {/* Daily digest */}
      {totalSearches > 0 && (
        <div className="animate-slide-up motion-reduce:animate-none" style={{ animationDelay: "60ms", animationFillMode: "backwards" }}>
          <Card>
            <CardContent className="p-5">
            <div className="flex items-center justify-between mb-3">
              <h2 className="text-sm font-semibold text-foreground">סיכום יומי</h2>
              {unread > 0 && (
                <Link
                  to="/notifications"
                  className="text-xs font-medium text-primary hover:underline"
                >
                  צפה בכל ההתראות
                </Link>
              )}
            </div>
            <div className="grid grid-cols-3 gap-4 text-center">
              <div>
                <p className="text-2xl font-bold tabular-nums text-foreground">{activeCount}</p>
                <p className="text-xs text-muted-foreground mt-0.5">חיפושים פעילים</p>
              </div>
              <div>
                <p className="text-2xl font-bold tabular-nums text-foreground">{recentListings?.total ?? 0}</p>
                <p className="text-xs text-muted-foreground mt-0.5">מודעות שנמצאו</p>
              </div>
              <div>
                <p className={cn("text-2xl font-bold tabular-nums", unread > 0 ? "text-primary" : "text-foreground")}>{unread}</p>
                <p className="text-xs text-muted-foreground mt-0.5">חדשות היום</p>
              </div>
            </div>
            </CardContent>
          </Card>
        </div>
      )}

      {/* Saved searches */}
      <section className="space-y-4 animate-slide-up motion-reduce:animate-none" style={{ animationDelay: "80ms", animationFillMode: "backwards" }}>
        <SectionHeader title="חיפושים שמורים" />

        {!searches || searches.length === 0 ? (
          <div className="rounded-2xl border border-border bg-card p-6 sm:p-8 text-center space-y-5">
            <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-2xl bg-muted text-muted-foreground">
              <Car className="h-8 w-8" />
            </div>
            <div className="space-y-2">
              <h2 className="text-xl font-bold text-foreground">ברוך הבא ל-CarWatch!</h2>
              <p className="mx-auto max-w-md text-sm text-muted-foreground leading-relaxed">
                צור את החיפוש הראשון שלך ונתחיל לעקוב אחר מודעות רכב בשבילך. הגדרה לוקחת פחות מ-2 דקות.
              </p>
            </div>
            <div className="flex flex-col sm:flex-row items-center justify-center gap-3">
              <Button asChild size="lg">
                <Link to="/searches/new">
                  <Plus className="h-4 w-4" />
                  צור חיפוש ראשון
                </Link>
              </Button>
            </div>
            <div className="grid grid-cols-3 gap-4 pt-4 border-t border-border/30">
              <div className="text-center">
                <p className="text-xs font-semibold text-primary">01</p>
                <p className="text-xs text-muted-foreground mt-1">בחר יצרן ודגם</p>
              </div>
              <div className="text-center">
                <p className="text-xs font-semibold text-primary">02</p>
                <p className="text-xs text-muted-foreground mt-1">הגדר תקציב</p>
              </div>
              <div className="text-center">
                <p className="text-xs font-semibold text-primary">03</p>
                <p className="text-xs text-muted-foreground mt-1">קבל התראות</p>
              </div>
            </div>
          </div>
        ) : (
          <div className={cn("grid gap-4", searches.length > 1 && "sm:grid-cols-2")}>
            {searches.map((search, i) => (
              <div
                key={search.id}
                className="animate-slide-up motion-reduce:animate-none"
                style={{ animationDelay: `${i * STAGGER_MS}ms`, animationFillMode: "backwards" }}
              >
                <SearchCard
                  search={search}
                  disabled={isMutating}
                  cycleStats={cycleStatsMap.get(search.id)}
                  onPause={() =>
                    pauseSearch.mutate(search.id, {
                      onSuccess: () => toast("החיפוש הושהה", "info"),
                    })
                  }
                  onResume={() =>
                    resumeSearch.mutate(search.id, {
                      onSuccess: () => toast("החיפוש חודש", "success"),
                    })
                  }
                  onDelete={() => {
                    if (confirmDelete === search.id) {
                      deleteSearch.mutate(search.id, {
                        onSuccess: () => toast("החיפוש נמחק", "success"),
                      });
                      setConfirmDelete(null);
                    } else {
                      setConfirmDelete(search.id);
                    }
                  }}
                  isConfirmingDelete={confirmDelete === search.id}
                  onCancelDelete={() => setConfirmDelete(null)}
                />
              </div>
            ))}
          </div>
        )}
      </section>

      {/* Recent listings feed */}
      {recentListings && recentListings.items.length > 0 && (
        <section className="space-y-4 animate-slide-up motion-reduce:animate-none" style={{ animationDelay: "120ms", animationFillMode: "backwards" }}>
          <SectionHeader
            title="מודעות אחרונות"
            linkTo="/notifications"
            linkLabel="הצג הכל"
          />
          <div className="space-y-2">
            {recentListings.items.map((listing, i) => (
              <div
                key={listing.token}
                className="animate-slide-up motion-reduce:animate-none"
                style={{ animationDelay: `${i * STAGGER_MS}ms`, animationFillMode: "backwards" }}
              >
                <RecentListingRow listing={listing} />
              </div>
            ))}
          </div>
        </section>
      )}
    </div>
  );
}

/* ---------- sub-components ---------- */

function DashboardHeader() {
  return (
    <div className="flex items-end justify-between gap-4 border-b border-border pb-5">
      <div>
        <p className="mb-1 text-[11px] font-medium uppercase tracking-[0.18em] text-muted-foreground">
          Dashboard
        </p>
        <h1 className="text-2xl font-bold leading-none tracking-tight text-foreground sm:text-[28px]">
          לוח בקרה
        </h1>
      </div>
      <Button asChild>
        <Link to="/searches/new">
          <Plus className="h-4 w-4" />
          חיפוש חדש
        </Link>
      </Button>
    </div>
  );
}

function StatCard({
  icon: Icon,
  value,
  label,
  highlight,
}: {
  icon: React.ComponentType<{ className?: string }>;
  value: number;
  label: string;
  highlight?: boolean;
}) {
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <div className="flex items-center gap-1.5 text-muted-foreground">
        <Icon className="h-3.5 w-3.5 shrink-0" />
        <span className="truncate text-[11px] font-medium uppercase tracking-wider">
          {label}
        </span>
      </div>
      <p
        className={cn(
          "mt-2 text-2xl font-bold tabular-nums tracking-tight sm:text-3xl",
          highlight ? "text-primary" : "text-foreground",
        )}
      >
        <NumberTicker value={value} />
      </p>
    </div>
  );
}

function RecentListingRow({ listing }: { listing: Listing }) {
  return (
    <Card className="hover-tint">
      <Link
        to={`/listings/${listing.token}`}
        state={{ listing }}
        className="flex items-center gap-4 p-4"
      >
        <div className="h-12 w-16 shrink-0 overflow-hidden rounded-lg bg-secondary">
          {listing.image_url ? (
            <img
              src={listing.image_url}
              alt={`${listing.manufacturer} ${listing.model}`}
              referrerPolicy="no-referrer"
              className="h-full w-full object-cover"
              loading="lazy"
            />
          ) : (
            <div className="flex h-full w-full items-center justify-center text-xl opacity-20">
              🚗
            </div>
          )}
        </div>
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-medium text-foreground">
            {listing.manufacturer} {listing.model} {listing.year}
          </p>
          <p className="text-xs text-muted-foreground">
            {listing.city || "—"} · {relativeTime(listing.first_seen_at)}
          </p>
        </div>
        <span className="shrink-0 text-sm font-bold tabular-nums text-foreground">
          {formatPrice(listing.price)}
        </span>
      </Link>
    </Card>
  );
}
