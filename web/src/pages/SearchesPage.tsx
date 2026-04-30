import { useState } from "react";
import { Link } from "react-router";
import {
  Plus,
  Play,
  Pause,
  Trash2,
  List,
  Pencil,
  Search as SearchIcon,
  Activity,
  Bell,
  Car,
} from "lucide-react";
import { motion } from "motion/react";
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
import { formatPrice, formatKm, relativeTime, cn } from "@/lib/utils";
import type { Search, Listing } from "@/lib/api";
import { Button } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";
import { EmptyState } from "@/components/ui/EmptyState";
import { Skeleton } from "@/components/ui/Skeleton";
import { SectionHeader } from "@/components/ui/SectionHeader";
import { useToast } from "@/components/ui/Toast";

const STAGGER_DELAY = 0.06;

const fadeUp = {
  hidden: { opacity: 0, y: 18 },
  visible: (i: number) => ({
    opacity: 1,
    y: 0,
    transition: {
      delay: i * STAGGER_DELAY,
      duration: 0.35,
      ease: [0, 0, 0.2, 1] as const,
    },
  }),
};

export function SearchesPage() {
  const { toast } = useToast();
  const { data: searches, isLoading, isError } = useSearches();
  const { data: notifCount } = useNotificationCount();
  const { data: recentListings } = useNotifications(5, 0);
  const deleteSearch = useDeleteSearch();
  const pauseSearch = usePauseSearch();
  const resumeSearch = useResumeSearch();
  const isMutating =
    deleteSearch.isPending || pauseSearch.isPending || resumeSearch.isPending;
  const [confirmDelete, setConfirmDelete] = useState<number | null>(null);

  const unread = notifCount?.count ?? 0;
  const activeCount = searches?.filter((s) => s.active).length ?? 0;
  const totalSearches = searches?.length ?? 0;

  if (isLoading) {
    return (
      <div className="space-y-8">
        <div className="flex items-start justify-between gap-4">
          <div className="space-y-2">
            <Skeleton className="h-8 w-40 rounded-lg" />
            <Skeleton className="h-4 w-56 rounded-md" />
          </div>
          <Skeleton className="h-10 w-36 rounded-xl" />
        </div>
        <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
          {[1, 2, 3, 4].map((i) => (
            <Skeleton key={i} className="h-[100px] rounded-xl" />
          ))}
        </div>
        <Skeleton className="h-5 w-32 rounded-md" />
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
        <div className="rounded-2xl border border-destructive/20 bg-destructive/5 p-8 text-center">
          <p className="text-destructive font-medium">
            שגיאה בטעינת החיפושים
          </p>
          <p className="text-sm text-muted-foreground mt-1">נסה לרענן את הדף</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-8">
      <DashboardHeader />

      {/* Stats row */}
      <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
        {[
          {
            icon: SearchIcon,
            label: "חיפושים פעילים",
            value: activeCount,
            color: "text-primary",
            bg: "bg-primary/12",
            glow: "shadow-[0_0_24px_-4px_rgba(59,130,246,0.3)]",
          },
          {
            icon: Car,
            label: "מודעות שנמצאו",
            value: recentListings?.total ?? 0,
            color: "text-score-great",
            bg: "bg-score-great/12",
            glow: "shadow-[0_0_24px_-4px_rgba(16,185,129,0.25)]",
          },
          {
            icon: Bell,
            label: "מודעות חדשות",
            value: unread,
            color: "text-score-good",
            bg: "bg-score-good/12",
            glow: unread > 0
              ? "shadow-[0_0_24px_-4px_rgba(245,158,11,0.3)]"
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
          <motion.div
            key={stat.label}
            custom={i}
            initial="hidden"
            animate="visible"
            variants={fadeUp}
          >
            <StatCard {...stat} />
          </motion.div>
        ))}
      </div>

      {/* Saved searches */}
      <section className="space-y-4">
        <SectionHeader title="חיפושים שמורים" />

        {!searches || searches.length === 0 ? (
          <EmptyState
            icon={SearchIcon}
            title="אין חיפושים פעילים עדיין"
            description="צור חיפוש ראשון כדי להתחיל לעקוב אחר מודעות רכבים"
            action={
              <Button asChild>
                <Link to="/searches/new">
                  <Plus className="h-4 w-4" />
                  צור חיפוש
                </Link>
              </Button>
            }
          />
        ) : (
          <div className="grid gap-4 sm:grid-cols-2">
            {searches.map((search, i) => (
              <motion.div
                key={search.id}
                custom={i}
                initial="hidden"
                animate="visible"
                variants={fadeUp}
              >
                <SearchCard
                  search={search}
                  disabled={isMutating}
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
              </motion.div>
            ))}
          </div>
        )}
      </section>

      {/* Recent listings feed */}
      {recentListings && recentListings.items.length > 0 && (
        <section className="space-y-4">
          <SectionHeader
            title="מודעות אחרונות"
            linkTo="/notifications"
            linkLabel="הצג הכל"
          />
          <div className="space-y-2">
            {recentListings.items.map((listing, i) => (
              <motion.div
                key={listing.token}
                custom={i}
                initial="hidden"
                animate="visible"
                variants={fadeUp}
              >
                <RecentListingRow listing={listing} />
              </motion.div>
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
    <div className="flex items-start justify-between gap-4">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">לוח בקרה</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          מעקב אחר חיפושי רכבים שלך
        </p>
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
        "rounded-xl border border-border/50 bg-card p-5 transition-all duration-200 hover:border-border hover:-translate-y-0.5",
        glow,
      )}
    >
      <div className="flex items-center justify-between mb-3">
        <span className="text-sm text-muted-foreground">{label}</span>
        <div
          className={cn(
            "flex h-10 w-10 shrink-0 items-center justify-center rounded-lg",
            bg,
          )}
        >
          <Icon className={cn("h-[18px] w-[18px]", color)} />
        </div>
      </div>
      <p className="text-3xl font-bold tabular-nums text-foreground">{value}</p>
    </div>
  );
}

function SearchCard({
  search,
  disabled,
  onPause,
  onResume,
  onDelete,
  isConfirmingDelete,
  onCancelDelete,
}: {
  search: Search;
  disabled: boolean;
  onPause: () => void;
  onResume: () => void;
  onDelete: () => void;
  isConfirmingDelete: boolean;
  onCancelDelete: () => void;
}) {
  return (
    <div className="group rounded-2xl border border-border/50 bg-card p-5 transition-all duration-200 hover:border-border hover:shadow-[0_4px_24px_rgba(0,0,0,0.3)]">
      <div className="flex items-start justify-between mb-3">
        <div>
          <h3 className="text-lg font-semibold">
            {search.manufacturer_name} {search.model_name}
          </h3>
          <span className="text-xs text-muted-foreground">{search.source}</span>
        </div>
        <Badge variant={search.active ? "success" : "warning"}>
          {search.active ? "פעיל" : "מושהה"}
        </Badge>
      </div>

      <div className="grid grid-cols-2 gap-2 text-sm text-muted-foreground mb-4">
        <div>
          שנים: {search.year_min}–{search.year_max}
        </div>
        {search.price_max > 0 && (
          <div className="tabular-nums">עד {formatPrice(search.price_max)}</div>
        )}
        {search.max_km > 0 && (
          <div className="tabular-nums">עד {formatKm(search.max_km)}</div>
        )}
        {search.max_hand > 0 && <div>עד יד {search.max_hand}</div>}
      </div>

      <div className="flex items-center gap-2 border-t border-border/50 pt-3">
        <Button
          as={Link}
          to={`/searches/${search.id}/listings`}
          variant="secondary"
          size="sm"
        >
          <List className="h-3.5 w-3.5" />
          תוצאות
        </Button>

        <Button
          as={Link}
          to={`/searches/${search.id}/edit`}
          variant="secondary"
          size="sm"
        >
          <Pencil className="h-3.5 w-3.5" />
          ערוך
        </Button>

        {search.active ? (
          <Button
            type="button"
            variant="secondary"
            size="sm"
            onClick={onPause}
            disabled={disabled}
          >
            <Pause className="h-3.5 w-3.5" />
            השהה
          </Button>
        ) : (
          <Button
            type="button"
            variant="secondary"
            size="sm"
            onClick={onResume}
            disabled={disabled}
          >
            <Play className="h-3.5 w-3.5" />
            חדש
          </Button>
        )}

        <div className="mr-auto">
          {isConfirmingDelete ? (
            <div className="flex items-center gap-1">
              <Button
                type="button"
                variant="destructive"
                size="sm"
                onClick={onDelete}
              >
                אישור
              </Button>
              <Button
                type="button"
                variant="secondary"
                size="sm"
                onClick={onCancelDelete}
              >
                ביטול
              </Button>
            </div>
          ) : (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={onDelete}
              className="text-destructive hover:bg-destructive/10"
            >
              <Trash2 className="h-3.5 w-3.5" />
              מחק
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}

function RecentListingRow({ listing }: { listing: Listing }) {
  return (
    <Link
      to={`/listings/${listing.token}`}
      state={{ listing }}
      className="flex items-center gap-4 rounded-xl border border-border/50 bg-card p-4 transition-all duration-200 hover:border-primary/40 hover:shadow-[0_2px_16px_rgba(59,130,246,0.08)]"
    >
      <div className="h-12 w-16 shrink-0 overflow-hidden rounded-lg bg-secondary">
        {listing.image_url ? (
          <img
            src={listing.image_url}
            alt={`${listing.manufacturer} ${listing.model}`}
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
      <span className="shrink-0 text-sm font-bold tabular-nums text-primary">
        {formatPrice(listing.price)}
      </span>
    </Link>
  );
}
