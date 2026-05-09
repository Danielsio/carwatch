import { useState } from "react";
import { Link } from "react-router";
import { Plus, Search as SearchIcon, Activity, Bell, Car } from "lucide-react";
import { motion, useReducedMotion } from "motion/react";
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
import { formatPrice, relativeTime, cn } from "@/lib/utils";
import type { Listing } from "@/lib/api";
import { Button } from "@/components/ui/Button";
import { EmptyState } from "@/components/ui/EmptyState";
import { ErrorState } from "@/components/ui/ErrorState";
import { SearchCard } from "@/components/SearchCard";
import { Skeleton } from "@/components/ui/Skeleton";
import { SectionHeader } from "@/components/ui/SectionHeader";
import { useToast } from "@/components/ui/Toast";

const STAGGER_DELAY = 0.06;

function useFadeUpVariants() {
  const reduceMotion = useReducedMotion();
  return {
    hidden: { opacity: 0, y: reduceMotion ? 0 : 18 },
    visible: (i: number) => ({
      opacity: 1,
      y: 0,
      transition: {
        delay: reduceMotion ? 0 : i * STAGGER_DELAY,
        duration: reduceMotion ? 0.15 : 0.4,
        ease: [0, 0, 0.2, 1] as const,
      },
    }),
  };
}

export function SearchesPage() {
  const fadeUp = useFadeUpVariants();
  const reduceMotion = useReducedMotion();
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
    <motion.div
      className="space-y-8 pb-4"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ duration: reduceMotion ? 0.15 : 0.35 }}
    >
      <motion.div
        initial={{ opacity: 0, y: reduceMotion ? 0 : 12 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: reduceMotion ? 0.15 : 0.4, ease: [0, 0, 0.2, 1] }}
      >
        <DashboardHeader />
      </motion.div>

      {/* Stats row */}
      <div className="grid grid-cols-2 gap-3 sm:gap-4 md:grid-cols-4">
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
            color: "text-primary",
            bg: "bg-primary/12",
            glow: "shadow-[0_0_24px_-4px_rgba(59,130,246,0.25)]",
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
      <motion.section
        className="space-y-4"
        initial={{ opacity: 0, y: reduceMotion ? 0 : 8 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: reduceMotion ? 0 : 0.08, duration: reduceMotion ? 0.15 : 0.45 }}
      >
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
          <div className={cn("grid gap-4", searches.length > 1 && "sm:grid-cols-2")}>
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
      </motion.section>

      {/* Recent listings feed */}
      {recentListings && recentListings.items.length > 0 && (
        <motion.section
          className="space-y-4"
          initial={{ opacity: 0, y: reduceMotion ? 0 : 8 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: reduceMotion ? 0 : 0.12, duration: reduceMotion ? 0.15 : 0.45 }}
        >
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
        </motion.section>
      )}
    </motion.div>
  );
}

/* ---------- sub-components ---------- */

function DashboardHeader() {
  return (
    <div className="relative flex items-start justify-between gap-4">
      <div
        className="pointer-events-none absolute -inset-x-2 -top-3 h-[4.5rem] rounded-2xl bg-gradient-to-b from-primary/[0.07] to-transparent sm:-inset-x-4 dark:from-primary/[0.09]"
        aria-hidden
      />
      <div className="relative">
        <h1 className="text-2xl font-bold tracking-tight">לוח בקרה</h1>
        <p className="mt-1 text-sm text-muted-foreground">
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
  const reduceMotion = useReducedMotion();
  return (
    <motion.div
      whileHover={
        reduceMotion ? undefined : { y: -3, transition: { duration: 0.2 } }
      }
      whileTap={reduceMotion ? undefined : { scale: 0.985 }}
      className={cn(
        "group relative overflow-hidden rounded-xl border border-border/60 bg-card p-3 sm:p-5 shadow-[0_1px_0_0_rgba(255,255,255,0.04)_inset] transition-[border-color,box-shadow] duration-200 hover:border-primary/25 dark:shadow-[0_1px_0_0_rgba(255,255,255,0.06)_inset]",
        glow,
      )}
    >
      <div className="pointer-events-none absolute -end-8 -top-10 h-28 w-28 rounded-full bg-gradient-to-br from-primary/15 via-transparent to-purple-500/10 opacity-80 blur-2xl transition-opacity group-hover:opacity-100" />
      <div className="pointer-events-none absolute inset-0 bg-gradient-to-t from-transparent via-transparent to-primary/[0.03] opacity-0 transition-opacity group-hover:opacity-100" />
      <div
        className="pointer-events-none absolute start-0 top-0 h-8 w-8 rounded-br-xl border-primary/20 border-s-2 border-t-2 opacity-50"
        aria-hidden
      />
      <div className="pointer-events-none absolute bottom-0 start-0 end-0 h-px bg-gradient-to-r from-transparent via-primary/20 to-transparent opacity-70" />
      <div className="relative flex items-center justify-between mb-2 sm:mb-3">
        <span className="text-xs sm:text-sm text-muted-foreground leading-tight">{label}</span>
        <div
          className={cn(
            "relative flex h-8 w-8 sm:h-10 sm:w-10 shrink-0 items-center justify-center rounded-xl ring-1 ring-white/10",
            bg,
          )}
        >
          <Icon className={cn("h-4 w-4 sm:h-[18px] sm:w-[18px]", color)} />
        </div>
      </div>
      <p className="relative text-2xl sm:text-3xl font-bold tabular-nums text-foreground tracking-tight">
        {value}
      </p>
    </motion.div>
  );
}

function RecentListingRow({ listing }: { listing: Listing }) {
  const reduceMotion = useReducedMotion();
  return (
    <motion.div
      whileHover={reduceMotion ? undefined : { scale: 1.01 }}
      transition={{ type: "spring", stiffness: 400, damping: 28 }}
      className="block"
    >
      <Link
        to={`/listings/${listing.token}`}
        state={{ listing }}
        className="flex items-center gap-4 rounded-xl border border-border/50 bg-card p-4 shadow-[0_1px_0_0_rgba(255,255,255,0.04)_inset] transition-all duration-200 hover:border-primary/40 hover:shadow-[0_8px_28px_-8px_rgba(59,130,246,0.18)] dark:shadow-[0_1px_0_0_rgba(255,255,255,0.06)_inset]"
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
    </motion.div>
  );
}
