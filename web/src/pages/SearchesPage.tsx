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
import { Button } from "@/components/ui/Button";
import { PageHeader } from "@/components/ui/PageHeader";
import { ErrorState } from "@/components/ui/ErrorState";
import { SearchCard } from "@/components/SearchCard";
import { Skeleton } from "@/components/ui/Skeleton";
import { SectionHeader } from "@/components/ui/SectionHeader";
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
        <div className="rounded-2xl border border-primary/20 bg-gradient-to-br from-primary/5 to-transparent p-6 sm:p-10 text-center space-y-6">
          <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-2xl bg-primary/10 text-primary">
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

      {/* Stats row */}
      <div className="grid grid-cols-2 gap-3 sm:gap-4 md:grid-cols-4">
        {[
          {
            icon: SearchIcon,
            label: "חיפושים פעילים",
            value: activeCount,
            color: "text-primary",
            bg: "bg-primary/12",
            glow: "shadow-[0_4px_20px_-4px_var(--color-glow-primary)]",
          },
          {
            icon: Car,
            label: "מודעות שנמצאו",
            value: searches?.reduce((sum, s) => sum + (s.listings_count ?? 0), 0) ?? 0,
            color: "text-primary",
            bg: "bg-primary/12",
            glow: "shadow-[0_4px_20px_-4px_var(--color-glow-primary)]",
          },
          {
            icon: Bell,
            label: "מודעות חדשות",
            value: unread,
            color: "text-score-good",
            bg: "bg-score-good/12",
            glow: unread > 0
              ? "shadow-[0_4px_20px_-4px_rgba(217,119,6,0.25)]"
              : "",
          },
          {
            icon: Activity,
            label: "סה״כ חיפושים",
            value: totalSearches,
            color: "text-chart-purple",
            bg: "bg-chart-purple/12",
            glow: "shadow-[0_0_24px_-4px_var(--color-glow-chart-purple)]",
          },
        ].map((stat, i) => (
          <div
            key={stat.label}
            className="animate-slide-up motion-reduce:animate-none"
            style={{ animationDelay: `${i * STAGGER_MS}ms`, animationFillMode: "backwards" }}
          >
            <StatCard {...stat} />
          </div>
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
          <div className="rounded-2xl border border-border/50 bg-card p-5">
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
          </div>
        </div>
      )}

      {/* Saved searches */}
      <section className="space-y-4 animate-slide-up motion-reduce:animate-none" style={{ animationDelay: "80ms", animationFillMode: "backwards" }}>
        <SectionHeader title="חיפושים שמורים" />

        {!searches || searches.length === 0 ? (
          <div className="rounded-2xl border border-primary/20 bg-gradient-to-br from-primary/5 to-transparent p-6 sm:p-8 text-center space-y-5">
            <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-2xl bg-primary/10 text-primary">
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
    <div className="relative flex items-start justify-between gap-4">
      <div className="relative">
        <p className="text-[11px] font-semibold tracking-[0.2em] text-primary uppercase mb-1.5">Dashboard</p>
        <h1 className="text-2xl sm:text-3xl font-extrabold tracking-tight">לוח בקרה</h1>
        <p className="mt-1.5 text-sm text-muted-foreground">
          מעקב אחר חיפושי רכבים שלך
        </p>
      </div>
      <div className="relative shrink-0">
        <Button asChild>
          <Link to="/searches/new">
            <Plus className="h-4 w-4" />
            חיפוש חדש
          </Link>
        </Button>
      </div>
    </div>
  );
}

function StatCard({
  icon: Icon,
  value,
  label,
  color,
  bg,
  glow,
}: {
  icon: React.ComponentType<{ className?: string }>;
  value: number;
  label: string;
  color: string;
  bg: string;
  glow?: string;
}) {
  return (
    <div
      className={cn(
        "group relative overflow-hidden rounded-2xl border border-border/40 bg-gradient-to-b from-card to-card/80 p-4 sm:p-5 backdrop-blur-sm transition-all duration-300",
        "hover:border-primary/30 hover:shadow-[0_8px_32px_-8px_var(--color-glow-primary)] hover:-translate-y-1 active:scale-[0.98]",
        "dark:from-[#0d1017] dark:to-[#0a0d14] dark:border-white/[0.06]",
        "dark:hover:border-white/[0.12] dark:hover:shadow-[0_8px_40px_-8px_rgba(59,130,246,0.2)]",
        "motion-reduce:hover:translate-y-0 motion-reduce:active:scale-100",
        glow,
      )}
    >
      {/* Ambient glow blob */}
      <div className="pointer-events-none absolute -end-6 -top-6 h-24 w-24 rounded-full bg-gradient-to-br from-primary/20 to-violet-500/10 opacity-0 blur-2xl transition-opacity duration-500 group-hover:opacity-100" />
      {/* Top edge shine */}
      <div className="pointer-events-none absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-white/10 to-transparent dark:via-white/[0.06]" />
      {/* Hover overlay */}
      <div className="pointer-events-none absolute inset-0 rounded-2xl bg-gradient-to-br from-white/[0.02] to-transparent opacity-0 transition-opacity duration-300 group-hover:opacity-100" />

      <div className="relative flex items-center justify-between mb-3 sm:mb-4">
        <div
          className={cn(
            "flex h-10 w-10 sm:h-11 sm:w-11 shrink-0 items-center justify-center rounded-xl",
            "ring-1 ring-white/[0.08] dark:ring-white/[0.06]",
            bg,
          )}
        >
          <Icon className={cn("h-4 w-4 sm:h-5 sm:w-5", color)} />
        </div>
      </div>
      <p className="relative text-3xl sm:text-4xl font-extrabold tabular-nums text-foreground tracking-tighter">
        {value}
      </p>
      <span className="relative mt-1 block text-[11px] sm:text-xs font-medium text-muted-foreground tracking-wide uppercase">{label}</span>
    </div>
  );
}

function RecentListingRow({ listing }: { listing: Listing }) {
  return (
    <div>
      <Link
        to={`/listings/${listing.token}`}
        state={{ listing }}
        className="flex items-center gap-4 rounded-xl border border-border/50 bg-card p-4 transition-all duration-150 hover:border-primary/30 hover:shadow-[0_4px_20px_-8px_var(--color-glow-primary)]"
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
        <span className="shrink-0 text-sm font-bold tabular-nums text-amber-500 dark:text-amber-400">
          {formatPrice(listing.price)}
        </span>
      </Link>
    </div>
  );
}
