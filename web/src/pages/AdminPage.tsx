import { useEffect, useRef, useState } from "react";
import {
  AlertCircle,
  Car,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Cpu,
  Database,
  ExternalLink,
  FileSearch,
  HardDrive,
  Loader2,
  RefreshCw,
  ScrollText,
  Search,
  Shield,
  Table,
  Trash2,
  Users,
  X,
} from "lucide-react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { AnimatePresence, motion } from "motion/react";
import { useAdminStats } from "@/hooks/useAdmin";
import { useLogStream } from "@/hooks/useLogStream";
import {
  adminApi,
  type AdminListing,
  type AdminSearch,
  type AdminUser,
  type LogEntry,
} from "@/lib/api";
import { EmptyState, Skeleton, Badge } from "@/components/ui";
import { useToast } from "@/components/ui/Toast";
import { cn, formatKm, formatPrice, relativeTime, safeHref } from "@/lib/utils";

const TABLE_LABELS: Record<string, string> = {
  users: "משתמשים",
  searches: "חיפושים",
  listing_history: "מודעות",
  price_history: "היסטוריית מחירים",
  dedup_seen: "מודעות שזוהו",
  seen_listings: "מודעות שנצפו",
  notifications: "התראות",
  pending_notifications: "התראות ממתינות",
  catalog: "קטלוג",
  saved_listings: "מודעות שמורות",
  hidden_listings: "מודעות מוסתרות",
  pending_digest: "תקצירים ממתינים",
};

const PURGEABLE = new Set([
  "listing_history",
  "price_history",
  "dedup_seen",
  "seen_listings",
  "notifications",
  "pending_notifications",
  "saved_listings",
  "hidden_listings",
  "pending_digest",
]);

type TabKey = "overview" | "listings" | "searches" | "users" | "logs";

const TABS: { key: TabKey; label: string; icon: typeof Car }[] = [
  { key: "overview", label: "סקירה כללית", icon: Database },
  { key: "listings", label: "מודעות", icon: Car },
  { key: "searches", label: "חיפושים", icon: FileSearch },
  { key: "users", label: "משתמשים", icon: Users },
  { key: "logs", label: "לוגים", icon: ScrollText },
];

// ── Confirm Modal ─────────────────────────────────────────────────────────────

function ConfirmModal({
  message,
  onConfirm,
  onCancel,
  loading,
}: {
  message: string;
  onConfirm: () => void;
  onCancel: () => void;
  loading?: boolean;
}) {
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      role="dialog"
      aria-modal="true"
      aria-labelledby="confirm-title"
      aria-describedby="confirm-desc"
      onKeyDown={(e) => e.key === "Escape" && onCancel()}
    >
      <motion.div
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        exit={{ opacity: 0 }}
        className="absolute inset-0 bg-black/60 backdrop-blur-sm"
        onClick={onCancel}
      />
      <motion.div
        initial={{ opacity: 0, scale: 0.95, y: 8 }}
        animate={{ opacity: 1, scale: 1, y: 0 }}
        exit={{ opacity: 0, scale: 0.95, y: 8 }}
        transition={{ type: "spring", duration: 0.3, bounce: 0.15 }}
        className="relative bg-card border border-border rounded-2xl p-6 max-w-sm w-full shadow-2xl"
      >
        <div className="flex items-center gap-3 mb-4">
          <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-destructive/15">
            <Trash2 className="h-[18px] w-[18px] text-destructive" />
          </div>
          <h3 id="confirm-title" className="font-bold text-foreground">אישור פעולה</h3>
        </div>
        <p id="confirm-desc" className="text-sm text-muted-foreground mb-6 leading-relaxed">
          {message}
        </p>
        <div className="flex gap-3">
          <button
            type="button"
            onClick={onCancel}
            className="flex-1 py-2.5 rounded-xl border border-border text-sm font-medium text-muted-foreground hover:bg-secondary transition-colors"
          >
            ביטול
          </button>
          <button
            type="button"
            onClick={onConfirm}
            disabled={loading}
            className="flex-1 py-2.5 rounded-xl bg-destructive hover:bg-destructive/90 text-white text-sm font-semibold transition-colors disabled:opacity-50"
          >
            {loading ? (
              <Loader2 className="mx-auto h-4 w-4 animate-spin" />
            ) : (
              "אישור"
            )}
          </button>
        </div>
      </motion.div>
    </div>
  );
}

// ── Detail Modal ──────────────────────────────────────────────────────────────

function DetailModal({
  title,
  fields,
  onClose,
}: {
  title: string;
  fields: { label: string; value: string | number | null | undefined }[];
  onClose: () => void;
}) {
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      role="dialog"
      aria-modal="true"
      aria-labelledby="detail-title"
      onKeyDown={(e) => e.key === "Escape" && onClose()}
    >
      <motion.div
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        exit={{ opacity: 0 }}
        className="absolute inset-0 bg-black/60 backdrop-blur-sm"
        onClick={onClose}
      />
      <motion.div
        initial={{ opacity: 0, y: 24 }}
        animate={{ opacity: 1, y: 0 }}
        exit={{ opacity: 0, y: 24 }}
        transition={{ type: "spring", duration: 0.35, bounce: 0.1 }}
        className="relative bg-card border border-border rounded-2xl p-6 max-w-lg w-full shadow-2xl max-h-[80vh] overflow-y-auto"
      >
        <div className="flex items-center justify-between mb-5">
          <h3 id="detail-title" className="font-bold text-foreground">{title}</h3>
          <button
            type="button"
            onClick={onClose}
            className="flex h-8 w-8 items-center justify-center rounded-lg bg-secondary text-muted-foreground hover:text-foreground transition-colors"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
        <div className="space-y-0.5">
          {fields.map((f) => {
            if (f.value === undefined || f.value === null || f.value === "")
              return null;
            return (
              <div
                key={f.label}
                className="flex gap-3 py-2.5 border-b border-border/50 last:border-0"
              >
                <span className="text-xs text-muted-foreground w-28 flex-shrink-0 mt-0.5">
                  {f.label}
                </span>
                <span className="text-sm text-foreground font-medium break-all">
                  {String(f.value)}
                </span>
              </div>
            );
          })}
        </div>
      </motion.div>
    </div>
  );
}

// ── Stat Card ─────────────────────────────────────────────────────────────────

function StatCard({
  label,
  value,
  icon: Icon,
  color,
  subtitle,
}: {
  label: string;
  value: string | number;
  icon: typeof Car;
  color: string;
  subtitle?: string;
}) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 12 }}
      animate={{ opacity: 1, y: 0 }}
      className="rounded-2xl border border-border/50 bg-card p-5 card-hover"
    >
      <div className="flex items-center justify-between mb-3">
        <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
          {label}
        </span>
        <div
          className={cn(
            "flex h-9 w-9 items-center justify-center rounded-xl",
            color,
          )}
        >
          <Icon className="h-[18px] w-[18px]" />
        </div>
      </div>
      <p className="text-3xl font-bold tabular-nums text-foreground">{value}</p>
      {subtitle && (
        <p className="text-xs text-muted-foreground mt-1">{subtitle}</p>
      )}
    </motion.div>
  );
}

// ── Overview Tab ──────────────────────────────────────────────────────────────

function OverviewTab({
  data,
  onRefresh,
}: {
  data: NonNullable<ReturnType<typeof useAdminStats>["data"]>;
  onRefresh: () => void;
}) {
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [confirmPurge, setConfirmPurge] = useState<string | null>(null);

  const purgeMutation = useMutation({
    mutationFn: (table: string) => adminApi.purgeTable(table),
    onSuccess: (result) => {
      toast(
        `נמחקו ${result.deleted} רשומות מ-${TABLE_LABELS[result.table] ?? result.table}`,
        "success",
      );
      setConfirmPurge(null);
      void queryClient.invalidateQueries({ queryKey: ["admin"] });
      onRefresh();
    },
    onError: () => {
      toast("שגיאה במחיקת הטבלה", "error");
    },
  });

  const vacuumMutation = useMutation({
    mutationFn: () => adminApi.vacuum(),
    onSuccess: (result) => {
      toast(
        `דחיסת מסד נתונים הושלמה${result.size_after ? ` — ${result.size_after}` : ""}`,
        "success",
      );
      void queryClient.invalidateQueries({ queryKey: ["admin"] });
      onRefresh();
    },
    onError: () => {
      toast("שגיאה בדחיסת מסד הנתונים", "error");
    },
  });

  const mb = data.db.file_size_bytes / (1024 * 1024);

  return (
    <div className="space-y-6">
      {/* DB Storage + Runtime — two-column */}
      <div className="grid gap-6 lg:grid-cols-2">
        {/* DB Storage card */}
        <div className="rounded-2xl border border-border/50 bg-card p-6">
          <div className="flex items-center justify-between mb-5">
            <div className="flex items-center gap-2.5">
              <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-primary/10">
                <Database className="h-[18px] w-[18px] text-primary" />
              </div>
              <h2 className="text-base font-semibold">אחסון</h2>
            </div>
            <button
              type="button"
              onClick={() => void vacuumMutation.mutate()}
              disabled={vacuumMutation.isPending}
              className="inline-flex items-center gap-1.5 rounded-xl border border-border bg-secondary px-3 py-2 text-xs font-medium transition-colors hover:bg-muted disabled:opacity-50"
            >
              {vacuumMutation.isPending ? (
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
              ) : (
                <HardDrive className="h-3.5 w-3.5" />
              )}
              דחיסת DB
            </button>
          </div>
          <div className="flex items-baseline gap-2.5 mb-3">
            <span className="text-4xl font-bold tabular-nums">
              {data.db.file_size_human}
            </span>
          </div>
          <div className="flex items-center gap-2 mb-4">
            <StorageIndicator sizeBytes={data.db.file_size_bytes} />
            <span className="text-xs text-muted-foreground tabular-nums">
              ({data.db.file_size_bytes.toLocaleString("he-IL")} bytes)
            </span>
          </div>

          {/* Storage bar */}
          <div className="h-2.5 rounded-full bg-secondary overflow-hidden">
            <motion.div
              className={cn(
                "h-full rounded-full",
                mb > 400
                  ? "bg-destructive"
                  : mb > 200
                    ? "bg-score-good"
                    : "bg-primary",
              )}
              initial={{ width: 0 }}
              animate={{ width: `${Math.min(100, (mb / 500) * 100)}%` }}
              transition={{ duration: 0.8, ease: "easeOut" }}
            />
          </div>
          <p className="text-[11px] text-muted-foreground mt-2">
            מתוך ~500 MB מקסימום מומלץ
          </p>
        </div>

        {/* Runtime card */}
        <div className="rounded-2xl border border-border/50 bg-card p-6">
          <div className="flex items-center gap-2.5 mb-5">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-chart-4/10">
              <Cpu className="h-[18px] w-[18px] text-chart-4" />
            </div>
            <h2 className="text-base font-semibold">סטטוס מערכת</h2>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <RuntimeStat label="זמן פעילות" value={data.runtime.uptime} />
            <RuntimeStat
              label="Goroutines"
              value={String(data.runtime.goroutines)}
            />
            <RuntimeStat
              label="זיכרון (Alloc)"
              value={`${data.runtime.mem_alloc_mb.toFixed(1)} MB`}
            />
            <RuntimeStat
              label="זיכרון (Sys)"
              value={`${data.runtime.mem_sys_mb.toFixed(1)} MB`}
            />
          </div>
        </div>
      </div>

      {/* Table sizes */}
      <div className="rounded-2xl border border-border/50 bg-card p-6">
        <div className="flex items-center gap-2.5 mb-5">
          <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-chart-2/10">
            <Table className="h-[18px] w-[18px] text-chart-2" />
          </div>
          <h2 className="text-base font-semibold">טבלאות</h2>
        </div>

        <div className="grid gap-2 sm:grid-cols-2">
          {Object.entries(data.tables)
            .sort(([, a], [, b]) => b - a)
            .map(([table, count]) => {
              const canPurge = PURGEABLE.has(table) && count > 0;
              const isConfirming = confirmPurge === table;

              return (
                <div
                  key={table}
                  className="flex items-center justify-between rounded-xl bg-secondary/50 px-4 py-3 transition-colors duration-200 hover:bg-secondary"
                >
                  <div className="flex items-center gap-2.5 min-w-0">
                    <div
                      className={cn(
                        "h-2 w-2 rounded-full flex-shrink-0",
                        count > 0 ? "bg-primary" : "bg-muted-foreground/30",
                      )}
                    />
                    <span className="text-sm font-medium truncate">
                      {TABLE_LABELS[table] ?? table}
                    </span>
                  </div>
                  <div className="flex items-center gap-3 flex-shrink-0">
                    <span className="text-sm font-mono font-semibold tabular-nums text-muted-foreground">
                      {count.toLocaleString("he-IL")}
                    </span>
                    {canPurge && !isConfirming && (
                      <button
                        type="button"
                        onClick={() => setConfirmPurge(table)}
                        aria-label={`מחק את כל ${TABLE_LABELS[table] ?? table}`}
                        className="rounded-lg p-1.5 text-muted-foreground/50 transition-colors hover:text-destructive hover:bg-destructive/10"
                        title={`מחק את כל ${TABLE_LABELS[table] ?? table}`}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </button>
                    )}
                    {isConfirming && (
                      <div className="flex items-center gap-1.5">
                        <button
                          type="button"
                          onClick={() => purgeMutation.mutate(table)}
                          disabled={purgeMutation.isPending}
                          className="rounded-lg bg-destructive px-2.5 py-1 text-[11px] font-medium text-white transition-opacity hover:opacity-90 disabled:opacity-50"
                        >
                          {purgeMutation.isPending ? "מוחק..." : "אישור"}
                        </button>
                        <button
                          type="button"
                          onClick={() => setConfirmPurge(null)}
                          className="rounded-lg border border-border px-2.5 py-1 text-[11px] font-medium text-muted-foreground transition-colors hover:text-foreground"
                        >
                          ביטול
                        </button>
                      </div>
                    )}
                  </div>
                </div>
              );
            })}
        </div>
      </div>
    </div>
  );
}

// ── Listings Tab ──────────────────────────────────────────────────────────────

function ListingsTab() {
  const [page, setPage] = useState(0);
  const [searchQuery, setSearchQuery] = useState("");
  const [detailListing, setDetailListing] = useState<AdminListing | null>(null);
  const [confirmDelete, setConfirmDelete] = useState<AdminListing | null>(null);
  const pageSize = 20;
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ["admin", "listings", page],
    queryFn: () =>
      adminApi.listings({ limit: pageSize, offset: page * pageSize }),
  });

  const deleteMutation = useMutation({
    mutationFn: ({ token, chatId }: { token: string; chatId: number }) =>
      adminApi.deleteListing(token, chatId),
    onSuccess: () => {
      toast("המודעה נמחקה", "success");
      setConfirmDelete(null);
      void queryClient.invalidateQueries({ queryKey: ["admin"] });
    },
    onError: () => {
      toast("שגיאה במחיקת המודעה", "error");
    },
  });

  const totalPages = data ? Math.ceil(data.total / pageSize) : 0;

  const filtered =
    searchQuery && data
      ? data.items.filter(
          (l) =>
            l.manufacturer?.toLowerCase().includes(searchQuery.toLowerCase()) ||
            l.model?.toLowerCase().includes(searchQuery.toLowerCase()) ||
            l.city?.toLowerCase().includes(searchQuery.toLowerCase()),
        )
      : data?.items ?? [];

  return (
    <div className="space-y-4">
      {/* Header with search */}
      <div className="flex flex-col sm:flex-row gap-3">
        <div className="relative flex-1">
          <Search className="absolute right-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="חפש לפי יצרן, דגם, עיר..."
            className="w-full bg-secondary/50 border border-border rounded-xl pr-10 pl-4 py-2.5 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring transition-colors"
          />
        </div>
        <button
          type="button"
          onClick={() => void refetch()}
          className="inline-flex items-center gap-2 px-4 py-2.5 rounded-xl bg-secondary hover:bg-secondary/80 text-sm font-medium text-foreground transition-colors"
        >
          <RefreshCw className="h-3.5 w-3.5" />
          רענן
        </button>
      </div>

      {/* Listings */}
      <div className="rounded-2xl border border-border/50 bg-card overflow-hidden">
        <div className="flex items-center justify-between px-6 py-4 border-b border-border/50">
          <h2 className="text-base font-semibold">כל המודעות</h2>
          {data && (
            <span className="text-sm text-muted-foreground tabular-nums">
              {data.total.toLocaleString("he-IL")} סה״כ
            </span>
          )}
        </div>

        <div className="p-3">
          {isLoading ? (
            <div className="space-y-2 p-3">
              {Array.from({ length: 5 }).map((_, i) => (
                <Skeleton key={i} className="h-[72px] rounded-xl" />
              ))}
            </div>
          ) : isError ? (
            <EmptyState
              icon={AlertCircle}
              title="שגיאה בטעינת המודעות"
              className="border-0 bg-transparent"
            />
          ) : filtered.length === 0 ? (
            <EmptyState
              icon={Car}
              title="אין מודעות"
              description={
                searchQuery
                  ? "לא נמצאו מודעות התואמות לחיפוש"
                  : "עדיין לא נמצאו מודעות במערכת"
              }
              className="border-0 bg-transparent"
            />
          ) : (
            <div className="space-y-1.5">
              <AnimatePresence mode="popLayout">
                {filtered.map((listing) => (
                  <motion.div
                    key={`${listing.token}-${listing.chat_id}`}
                    layout
                    initial={{ opacity: 0, y: 4 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, scale: 0.95 }}
                    className="flex items-center gap-3 rounded-xl bg-secondary/40 px-4 py-3 transition-colors hover:bg-secondary group"
                  >
                    {listing.image_url ? (
                      <img
                        src={listing.image_url}
                        alt=""
                        className="h-12 w-16 rounded-lg object-cover bg-muted flex-shrink-0"
                        referrerPolicy="no-referrer"
                      />
                    ) : (
                      <div className="h-12 w-16 rounded-lg bg-muted flex items-center justify-center flex-shrink-0">
                        <Car className="h-5 w-5 text-muted-foreground/30" />
                      </div>
                    )}
                    <div className="min-w-0 flex-1">
                      <p className="text-sm font-medium truncate">
                        {listing.manufacturer} {listing.model} {listing.year}
                      </p>
                      <div className="flex items-center gap-2 mt-0.5 flex-wrap">
                        <span className="text-xs text-muted-foreground">
                          {formatPrice(listing.price)}
                        </span>
                        <span className="text-xs text-muted-foreground">
                          · {formatKm(listing.km)}
                        </span>
                        {listing.city && (
                          <span className="text-xs text-muted-foreground">
                            · {listing.city}
                          </span>
                        )}
                        {listing.fitness_score != null && (
                          <Badge
                            variant={
                              listing.fitness_score >= 0.7
                                ? "success"
                                : listing.fitness_score >= 0.4
                                  ? "warning"
                                  : "destructive"
                            }
                            className="text-[10px] px-1.5 py-0"
                          >
                            {(listing.fitness_score * 100).toFixed(0)}%
                          </Badge>
                        )}
                      </div>
                    </div>
                    <div className="flex items-center gap-1.5 flex-shrink-0 opacity-100 sm:opacity-0 sm:group-hover:opacity-100 sm:group-focus-within:opacity-100 transition-opacity">
                      <button
                        type="button"
                        onClick={() => setDetailListing(listing)}
                        className="flex h-8 w-8 items-center justify-center rounded-lg bg-card border border-border text-muted-foreground hover:text-foreground transition-colors"
                        title="פרטים"
                      >
                        <Search className="h-3.5 w-3.5" />
                      </button>
                      {safeHref(listing.page_link) && (
                        <a
                          href={safeHref(listing.page_link)!}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="flex h-8 w-8 items-center justify-center rounded-lg bg-card border border-border text-muted-foreground hover:text-primary transition-colors"
                          title="צפה במודעה"
                        >
                          <ExternalLink className="h-3.5 w-3.5" />
                        </a>
                      )}
                      <button
                        type="button"
                        onClick={() => setConfirmDelete(listing)}
                        className="flex h-8 w-8 items-center justify-center rounded-lg bg-destructive/10 border border-destructive/20 text-destructive hover:bg-destructive/20 transition-colors"
                        title="מחק"
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </button>
                    </div>
                  </motion.div>
                ))}
              </AnimatePresence>
            </div>
          )}
        </div>

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="flex items-center justify-center gap-4 px-6 py-4 border-t border-border/50">
            <button
              type="button"
              onClick={() => setPage((p) => Math.max(0, p - 1))}
              disabled={page === 0}
              className="flex h-9 w-9 items-center justify-center rounded-lg border border-border transition-colors hover:bg-muted disabled:opacity-30"
            >
              <ChevronRight className="h-4 w-4" />
            </button>
            <span className="text-sm tabular-nums text-muted-foreground">
              עמוד {page + 1} מתוך {totalPages}
            </span>
            <button
              type="button"
              onClick={() =>
                setPage((p) => Math.min(totalPages - 1, p + 1))
              }
              disabled={page >= totalPages - 1}
              className="flex h-9 w-9 items-center justify-center rounded-lg border border-border transition-colors hover:bg-muted disabled:opacity-30"
            >
              <ChevronLeft className="h-4 w-4" />
            </button>
          </div>
        )}
      </div>

      <AnimatePresence>
        {confirmDelete && (
          <ConfirmModal
            message={`האם למחוק את "${confirmDelete.manufacturer} ${confirmDelete.model} ${confirmDelete.year}"?`}
            onConfirm={() =>
              deleteMutation.mutate({
                token: confirmDelete.token,
                chatId: confirmDelete.chat_id,
              })
            }
            onCancel={() => setConfirmDelete(null)}
            loading={deleteMutation.isPending}
          />
        )}
        {detailListing && (
          <DetailModal
            title={`${detailListing.manufacturer} ${detailListing.model} ${detailListing.year}`}
            fields={[
              { label: "מחיר", value: formatPrice(detailListing.price) },
              { label: "קילומטרז'", value: formatKm(detailListing.km) },
              { label: "יד", value: detailListing.hand || null },
              { label: "עיר", value: detailListing.city },
              {
                label: "ציון התאמה",
                value: detailListing.fitness_score != null
                  ? `${(detailListing.fitness_score * 100).toFixed(0)}%`
                  : null,
              },
              {
                label: "תאריך",
                value: detailListing.first_seen_at
                  ? relativeTime(detailListing.first_seen_at)
                  : null,
              },
              { label: "חיפוש", value: detailListing.search_name },
              { label: "Token", value: detailListing.token },
              { label: "Chat ID", value: detailListing.chat_id },
            ]}
            onClose={() => setDetailListing(null)}
          />
        )}
      </AnimatePresence>
    </div>
  );
}

// ── Searches Tab ──────────────────────────────────────────────────────────────

function SearchesTab() {
  const [detailSearch, setDetailSearch] = useState<AdminSearch | null>(null);

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ["admin", "searches"],
    queryFn: adminApi.searches,
  });

  return (
    <div className="space-y-4">
      <div className="flex gap-3 justify-end">
        <button
          type="button"
          onClick={() => void refetch()}
          className="inline-flex items-center gap-2 px-4 py-2.5 rounded-xl bg-secondary hover:bg-secondary/80 text-sm font-medium text-foreground transition-colors"
        >
          <RefreshCw className="h-3.5 w-3.5" />
          רענן
        </button>
      </div>

      <div className="rounded-2xl border border-border/50 bg-card overflow-hidden">
        <div className="flex items-center justify-between px-6 py-4 border-b border-border/50">
          <h2 className="text-base font-semibold">כל החיפושים</h2>
          {data && (
            <span className="text-sm text-muted-foreground tabular-nums">
              {data.total} סה״כ
            </span>
          )}
        </div>

        <div className="p-3">
          {isLoading ? (
            <div className="space-y-2 p-3">
              {Array.from({ length: 3 }).map((_, i) => (
                <Skeleton key={i} className="h-16 rounded-xl" />
              ))}
            </div>
          ) : isError ? (
            <EmptyState
              icon={AlertCircle}
              title="שגיאה בטעינת החיפושים"
              className="border-0 bg-transparent"
            />
          ) : !data || data.items.length === 0 ? (
            <EmptyState
              icon={FileSearch}
              title="אין חיפושים"
              description="עדיין לא נוצרו חיפושים במערכת"
              className="border-0 bg-transparent"
            />
          ) : (
            <div className="space-y-1.5">
              {data.items.map((search) => (
                <motion.div
                  key={search.id}
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  className="flex items-center gap-3 rounded-xl bg-secondary/40 px-4 py-3 transition-colors hover:bg-secondary cursor-pointer group focus-visible:ring-2 focus-visible:ring-ring outline-none"
                  tabIndex={0}
                  role="button"
                  onClick={() => setDetailSearch(search)}
                  onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); setDetailSearch(search); } }}
                >
                  <div
                    className={cn(
                      "h-2.5 w-2.5 rounded-full flex-shrink-0",
                      search.active
                        ? "bg-success animate-pulse-soft"
                        : "bg-muted-foreground/40",
                    )}
                  />
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium truncate">
                      {search.name || `חיפוש #${search.id}`}
                    </p>
                    <div className="flex items-center gap-2 mt-0.5 flex-wrap">
                      <span className="text-xs text-muted-foreground">
                        מקור: {search.source}
                      </span>
                      {search.price_max > 0 && (
                        <span className="text-xs text-muted-foreground">
                          · עד {formatPrice(search.price_max)}
                        </span>
                      )}
                      <span className="text-xs text-muted-foreground">
                        · {relativeTime(search.created_at)}
                      </span>
                    </div>
                  </div>
                  <Badge variant={search.active ? "success" : "default"}>
                    {search.active ? "פעיל" : "מושהה"}
                  </Badge>
                  <span className="text-xs text-muted-foreground tabular-nums flex-shrink-0">
                    #{search.chat_id}
                  </span>
                </motion.div>
              ))}
            </div>
          )}
        </div>
      </div>

      <AnimatePresence>
        {detailSearch && (
          <DetailModal
            title={detailSearch.name || `חיפוש #${detailSearch.id}`}
            fields={[
              { label: "מזהה", value: detailSearch.id },
              { label: "שם", value: detailSearch.name },
              { label: "מקור", value: detailSearch.source },
              { label: "יצרן", value: detailSearch.manufacturer || null },
              { label: "דגם", value: detailSearch.model || null },
              { label: "שנה מ", value: detailSearch.year_min || null },
              { label: "שנה עד", value: detailSearch.year_max || null },
              {
                label: "מחיר מקס",
                value: detailSearch.price_max
                  ? formatPrice(detailSearch.price_max)
                  : null,
              },
              {
                label: "ק״מ מקס",
                value: detailSearch.max_km
                  ? detailSearch.max_km.toLocaleString("he-IL")
                  : null,
              },
              { label: "יד מקס", value: detailSearch.max_hand || null },
              {
                label: "סטטוס",
                value: detailSearch.active ? "פעיל" : "מושהה",
              },
              { label: "Chat ID", value: detailSearch.chat_id },
              {
                label: "נוצר",
                value: relativeTime(detailSearch.created_at),
              },
            ]}
            onClose={() => setDetailSearch(null)}
          />
        )}
      </AnimatePresence>
    </div>
  );
}

// ── Users Tab ─────────────────────────────────────────────────────────────────

function UsersTab() {
  const [detailUser, setDetailUser] = useState<AdminUser | null>(null);

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ["admin", "users"],
    queryFn: adminApi.users,
  });

  return (
    <div className="space-y-4">
      <div className="flex gap-3 justify-end">
        <button
          type="button"
          onClick={() => void refetch()}
          className="inline-flex items-center gap-2 px-4 py-2.5 rounded-xl bg-secondary hover:bg-secondary/80 text-sm font-medium text-foreground transition-colors"
        >
          <RefreshCw className="h-3.5 w-3.5" />
          רענן
        </button>
      </div>

      <div className="rounded-2xl border border-border/50 bg-card overflow-hidden">
        <div className="flex items-center justify-between px-6 py-4 border-b border-border/50">
          <h2 className="text-base font-semibold">כל המשתמשים</h2>
          {data && (
            <span className="text-sm text-muted-foreground tabular-nums">
              {data.total} סה״כ
            </span>
          )}
        </div>

        <div className="p-3">
          {isLoading ? (
            <div className="space-y-2 p-3">
              {Array.from({ length: 3 }).map((_, i) => (
                <Skeleton key={i} className="h-16 rounded-xl" />
              ))}
            </div>
          ) : isError ? (
            <EmptyState
              icon={AlertCircle}
              title="שגיאה בטעינת המשתמשים"
              className="border-0 bg-transparent"
            />
          ) : !data || data.items.length === 0 ? (
            <EmptyState
              icon={Users}
              title="אין משתמשים"
              description="עדיין לא נרשמו משתמשים למערכת"
              className="border-0 bg-transparent"
            />
          ) : (
            <div className="space-y-1.5">
              {data.items.map((user) => (
                <motion.div
                  key={user.chat_id}
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  className="flex items-center gap-3 rounded-xl bg-secondary/40 px-4 py-3 transition-colors hover:bg-secondary cursor-pointer focus-visible:ring-2 focus-visible:ring-ring outline-none"
                  tabIndex={0}
                  role="button"
                  onClick={() => setDetailUser(user)}
                  onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); setDetailUser(user); } }}
                >
                  <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-primary/10 text-primary font-bold text-sm flex-shrink-0">
                    {(user.username || user.channel || "?")[0].toUpperCase()}
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium truncate">
                      {user.username || `User #${user.chat_id}`}
                    </p>
                    <div className="flex items-center gap-2 mt-0.5 flex-wrap">
                      {user.channel && (
                        <span className="text-xs text-muted-foreground">
                          {user.channel}
                        </span>
                      )}
                      <span className="text-xs text-muted-foreground">
                        · {relativeTime(user.created_at)}
                      </span>
                    </div>
                  </div>
                  <div className="flex items-center gap-2 flex-shrink-0">
                    {user.tier && user.tier !== "free" && (
                      <Badge variant="success">{user.tier}</Badge>
                    )}
                    <Badge variant={user.active ? "success" : "default"}>
                      {user.active ? "פעיל" : "לא פעיל"}
                    </Badge>
                  </div>
                </motion.div>
              ))}
            </div>
          )}
        </div>
      </div>

      <AnimatePresence>
        {detailUser && (
          <DetailModal
            title={detailUser.username || `User #${detailUser.chat_id}`}
            fields={[
              { label: "Chat ID", value: detailUser.chat_id },
              { label: "שם משתמש", value: detailUser.username },
              { label: "ערוץ", value: detailUser.channel },
              { label: "מזהה ערוץ", value: detailUser.channel_id },
              { label: "שפה", value: detailUser.language },
              { label: "דרגה", value: detailUser.tier },
              {
                label: "סטטוס",
                value: detailUser.active ? "פעיל" : "לא פעיל",
              },
              { label: "נוצר", value: relativeTime(detailUser.created_at) },
            ]}
            onClose={() => setDetailUser(null)}
          />
        )}
      </AnimatePresence>
    </div>
  );
}

// ── Logs Tab ──────────────────────────────────────────────────────────────────

const LEVEL_STYLES: Record<
  string,
  { bg: string; text: string; label: string }
> = {
  DEBUG: {
    bg: "bg-muted-foreground/10",
    text: "text-muted-foreground",
    label: "DBG",
  },
  INFO: { bg: "bg-primary/10", text: "text-primary", label: "INF" },
  WARN: { bg: "bg-warning/10", text: "text-warning", label: "WRN" },
  ERROR: { bg: "bg-destructive/10", text: "text-destructive", label: "ERR" },
};

const ALL_COMPONENTS = ["yad2", "winwin", "scheduler", "enricher"] as const;
const ALL_LEVELS = ["DEBUG", "INFO", "WARN", "ERROR"] as const;

function LogsTab({ active }: { active: boolean }) {
  const { logs, connected, clear } = useLogStream(active);
  const [componentFilter, setComponentFilter] = useState<Set<string>>(
    new Set(ALL_COMPONENTS),
  );
  const [levelFilter, setLevelFilter] = useState<Set<string>>(
    new Set(ALL_LEVELS),
  );
  const [expandedIdx, setExpandedIdx] = useState<number | null>(null);
  const [autoScroll, setAutoScroll] = useState(true);
  const bottomRef = useRef<HTMLDivElement>(null);
  const [backendLevel, setBackendLevel] = useState("INFO");

  useEffect(() => {
    if (!active) return;
    adminApi.getLogLevel().then((r) => setBackendLevel(r.level)).catch(() => {});
  }, [active]);

  async function changeBackendLevel(level: string) {
    try {
      const { level: newLevel } = await adminApi.setLogLevel(level);
      setBackendLevel(newLevel);
    } catch {
      // non-critical
    }
  }

  const filtered = logs.filter(
    (e) => componentFilter.has(e.component) && levelFilter.has(e.level),
  );

  useEffect(() => {
    if (autoScroll && bottomRef.current) {
      bottomRef.current.scrollIntoView({ behavior: "smooth" });
    }
  }, [filtered.length, autoScroll]);

  function toggleFilter(
    set: Set<string>,
    setter: (s: Set<string>) => void,
    value: string,
  ) {
    const next = new Set(set);
    if (next.has(value)) {
      if (next.size > 1) next.delete(value);
    } else {
      next.add(value);
    }
    setter(next);
  }

  function formatTime(iso: string) {
    try {
      return new Date(iso).toLocaleTimeString("he-IL", {
        hour: "2-digit",
        minute: "2-digit",
        second: "2-digit",
      });
    } catch {
      return "";
    }
  }

  return (
    <div className="space-y-4">
      {/* Toolbar */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-3 flex-wrap">
          {/* Connection indicator */}
          <div className="flex items-center gap-1.5">
            <div
              className={cn(
                "h-2 w-2 rounded-full flex-shrink-0",
                connected
                  ? "bg-success animate-pulse-soft"
                  : "bg-muted-foreground/40",
              )}
            />
            <span className="text-xs text-muted-foreground">
              {connected ? "מחובר" : "מנותק"}
            </span>
          </div>

          {/* Component filters */}
          <div className="flex gap-1">
            {ALL_COMPONENTS.map((c) => (
              <button
                key={c}
                type="button"
                onClick={() =>
                  toggleFilter(componentFilter, setComponentFilter, c)
                }
                className={cn(
                  "px-2 py-1 rounded-md text-[11px] font-medium transition-colors",
                  componentFilter.has(c)
                    ? "bg-primary/10 text-primary"
                    : "bg-secondary text-muted-foreground/50",
                )}
              >
                {c}
              </button>
            ))}
          </div>

          {/* Level filters */}
          <div className="flex gap-1">
            {ALL_LEVELS.map((l) => {
              const style = LEVEL_STYLES[l];
              return (
                <button
                  key={l}
                  type="button"
                  onClick={() => toggleFilter(levelFilter, setLevelFilter, l)}
                  className={cn(
                    "px-2 py-1 rounded-md text-[11px] font-medium transition-colors",
                    levelFilter.has(l)
                      ? `${style.bg} ${style.text}`
                      : "bg-secondary text-muted-foreground/50",
                  )}
                >
                  {style.label}
                </button>
              );
            })}
          </div>
        </div>

        <div className="flex items-center gap-2">
          {/* Backend log level selector */}
          <div className="flex items-center gap-1.5">
            <span className="text-[11px] text-muted-foreground">רמה:</span>
            <div className="flex gap-0.5 rounded-lg bg-secondary/50 p-0.5">
              {ALL_LEVELS.map((l) => {
                const style = LEVEL_STYLES[l];
                return (
                  <button
                    key={l}
                    type="button"
                    onClick={() => changeBackendLevel(l)}
                    className={cn(
                      "px-2 py-1 rounded-md text-[11px] font-medium transition-all",
                      backendLevel === l
                        ? `${style.bg} ${style.text} ring-1 ring-current/20`
                        : "text-muted-foreground/40 hover:text-muted-foreground/70",
                    )}
                  >
                    {style.label}
                  </button>
                );
              })}
            </div>
          </div>

          <div className="h-4 w-px bg-border/50" />

          <button
            type="button"
            onClick={() => setAutoScroll((p) => !p)}
            className={cn(
              "inline-flex items-center gap-1.5 px-3 py-2 rounded-xl text-xs font-medium transition-colors",
              autoScroll
                ? "bg-primary/10 text-primary"
                : "bg-secondary text-muted-foreground",
            )}
          >
            <ChevronDown className="h-3 w-3" />
            גלילה אוטומטית
          </button>
          <button
            type="button"
            onClick={clear}
            className="inline-flex items-center gap-1.5 px-3 py-2 rounded-xl bg-secondary hover:bg-secondary/80 text-xs font-medium text-foreground transition-colors"
          >
            <Trash2 className="h-3 w-3" />
            נקה
          </button>
        </div>
      </div>

      {/* Log entries */}
      <div className="rounded-2xl border border-border/50 bg-card overflow-hidden">
        <div className="flex items-center justify-between px-6 py-4 border-b border-border/50">
          <h2 className="text-base font-semibold">
            לוגים ({filtered.length})
          </h2>
          <span className="text-xs text-muted-foreground tabular-nums">
            {logs.length} סה״כ
          </span>
        </div>

        <div dir="ltr" className="max-h-[60vh] overflow-y-auto p-2 font-mono text-xs">
          {filtered.length === 0 ? (
            <EmptyState
              icon={ScrollText}
              title="אין לוגים"
              description="לוגים מה-fetchers יופיעו כאן בזמן אמת"
              className="border-0 bg-transparent"
            />
          ) : (
            <div className="space-y-px">
              {filtered.map((entry, idx) => (
                <LogLine
                  key={`${entry.time}-${idx}`}
                  entry={entry}
                  expanded={expandedIdx === idx}
                  onToggle={() =>
                    setExpandedIdx(expandedIdx === idx ? null : idx)
                  }
                  formatTime={formatTime}
                />
              ))}
              <div ref={bottomRef} />
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function LogLine({
  entry,
  expanded,
  onToggle,
  formatTime,
}: {
  entry: LogEntry;
  expanded: boolean;
  onToggle: () => void;
  formatTime: (iso: string) => string;
}) {
  const style = LEVEL_STYLES[entry.level] ?? LEVEL_STYLES.INFO;
  const hasAttrs = entry.attrs && Object.keys(entry.attrs).length > 0;

  return (
    <div
      className={cn(
        "rounded-lg px-3 py-2 transition-colors",
        expanded ? "bg-secondary" : "hover:bg-secondary/50",
        hasAttrs && "cursor-pointer focus-visible:ring-2 focus-visible:ring-ring outline-none",
      )}
      tabIndex={hasAttrs ? 0 : undefined}
      role={hasAttrs ? "button" : undefined}
      aria-expanded={hasAttrs ? expanded : undefined}
      onClick={hasAttrs ? onToggle : undefined}
      onKeyDown={hasAttrs ? (e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onToggle(); } } : undefined}
    >
      <div className="flex items-start gap-2">
        <span className="text-muted-foreground/60 flex-shrink-0 tabular-nums w-[60px]">
          {formatTime(entry.time)}
        </span>
        <span
          className={cn(
            "inline-flex items-center justify-center rounded px-1.5 py-0.5 text-[10px] font-semibold flex-shrink-0 w-8",
            style.bg,
            style.text,
          )}
        >
          {style.label}
        </span>
        <span className="text-primary/70 flex-shrink-0 w-[70px] truncate">
          {entry.component}
        </span>
        <span className="text-foreground break-all flex-1">{entry.message}</span>
        {hasAttrs && (
          <ChevronDown
            className={cn(
              "h-3 w-3 text-muted-foreground/40 flex-shrink-0 mt-0.5 transition-transform",
              expanded && "rotate-180",
            )}
          />
        )}
      </div>
      {expanded && hasAttrs && (
        <div className="mt-2 ml-[132px] space-y-0.5 border-l border-border/50 pl-3">
          {Object.entries(entry.attrs!).map(([k, v]) => (
            <div key={k} className="flex gap-2">
              <span className="text-muted-foreground/60">{k}:</span>
              <span className="text-foreground/80 break-all">{v}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function StorageIndicator({ sizeBytes }: { sizeBytes: number }) {
  const mb = sizeBytes / (1024 * 1024);
  const color =
    mb > 400
      ? "bg-destructive"
      : mb > 200
        ? "bg-score-good"
        : "bg-score-great";

  return (
    <span
      className={`inline-block h-2.5 w-2.5 rounded-full ${color}`}
      aria-hidden
    />
  );
}

function RuntimeStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl border border-border/50 bg-secondary/50 p-4 transition-colors duration-200 hover:border-border">
      <p className="text-xs text-muted-foreground mb-1">{label}</p>
      <p className="text-sm font-semibold font-mono tabular-nums">{value}</p>
    </div>
  );
}

// ── Main Admin Page ───────────────────────────────────────────────────────────

export function AdminPage() {
  const { data, isLoading, isError, dataUpdatedAt, refetch } = useAdminStats();
  const [activeTab, setActiveTab] = useState<TabKey>("overview");

  if (isLoading) {
    return (
      <div className="space-y-6 pb-20 md:pb-4">
        <AdminHeader />
        <div className="grid gap-4 grid-cols-2 md:grid-cols-4">
          {[1, 2, 3, 4].map((i) => (
            <Skeleton key={i} className="h-28 rounded-2xl" />
          ))}
        </div>
        <Skeleton className="h-10 w-80 rounded-xl" />
        <div className="grid gap-4 sm:grid-cols-2">
          {[1, 2].map((i) => (
            <Skeleton key={i} className="h-52 rounded-2xl" />
          ))}
        </div>
      </div>
    );
  }

  if (isError || !data) {
    return (
      <div className="space-y-6 pb-20 md:pb-4">
        <AdminHeader />
        <EmptyState
          icon={AlertCircle}
          title="שגיאה בטעינת הנתונים"
          description="ייתכן שאין לך הרשאות מנהל. ודא שה-admin_email מוגדר בקובץ ההגדרות."
        />
      </div>
    );
  }

  const listingsCount =
    data.tables["listing_history"] ?? 0;
  const searchesCount = data.tables["searches"] ?? 0;
  const usersCount = data.tables["users"] ?? 0;

  const lastUpdated = new Date(dataUpdatedAt);

  return (
    <div className="space-y-6 pb-20 md:pb-4">
      <AdminHeader
        action={
          <div className="flex items-center gap-2">
            <span className="text-xs text-muted-foreground tabular-nums">
              עדכון: {lastUpdated.toLocaleTimeString("he-IL")}
            </span>
            <button
              type="button"
              onClick={() => void refetch()}
              className="flex h-8 w-8 items-center justify-center rounded-lg bg-secondary text-muted-foreground hover:text-foreground transition-colors"
              title="רענן נתונים"
            >
              <RefreshCw className="h-3.5 w-3.5" />
            </button>
          </div>
        }
      />

      {/* Stat Cards */}
      <div className="grid gap-4 grid-cols-2 md:grid-cols-4">
        <StatCard
          label="מודעות"
          value={listingsCount.toLocaleString("he-IL")}
          icon={Car}
          color="bg-primary/10 text-primary"
        />
        <StatCard
          label="חיפושים"
          value={searchesCount.toLocaleString("he-IL")}
          icon={FileSearch}
          color="bg-chart-2/10 text-chart-2"
        />
        <StatCard
          label="משתמשים"
          value={usersCount.toLocaleString("he-IL")}
          icon={Users}
          color="bg-chart-4/10 text-chart-4"
        />
        <StatCard
          label="אחסון"
          value={data.db.file_size_human}
          icon={Database}
          color="bg-warning/10 text-warning"
          subtitle={data.runtime.uptime + " פעיל"}
        />
      </div>

      {/* Tabs */}
      <div className="flex gap-1 rounded-xl bg-secondary/50 p-1 w-fit flex-wrap">
        {TABS.map((t) => (
          <button
            key={t.key}
            type="button"
            onClick={() => setActiveTab(t.key)}
            className={cn(
              "flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-medium transition-all",
              activeTab === t.key
                ? "bg-card text-foreground shadow-sm border border-border"
                : "text-muted-foreground hover:text-foreground",
            )}
          >
            <t.icon className="h-4 w-4" />
            {t.label}
          </button>
        ))}
      </div>

      {/* Tab Content */}
      <AnimatePresence mode="wait">
        <motion.div
          key={activeTab}
          initial={{ opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.15 }}
        >
          {activeTab === "overview" && (
            <OverviewTab data={data} onRefresh={() => void refetch()} />
          )}
          {activeTab === "listings" && <ListingsTab />}
          {activeTab === "searches" && <SearchesTab />}
          {activeTab === "users" && <UsersTab />}
          {activeTab === "logs" && (
            <LogsTab active={activeTab === "logs"} />
          )}
        </motion.div>
      </AnimatePresence>
    </div>
  );
}

function AdminHeader({ action }: { action?: React.ReactNode }) {
  return (
    <motion.div
      initial={{ opacity: 0, y: -8 }}
      animate={{ opacity: 1, y: 0 }}
      className="flex items-center justify-between border-b border-border/50 pb-6"
    >
      <div className="flex items-center gap-3">
        <div className="flex h-11 w-11 items-center justify-center rounded-xl bg-destructive/10">
          <Shield className="h-5 w-5 text-destructive" />
        </div>
        <div>
          <h1 className="text-2xl font-bold text-foreground">ניהול מערכת</h1>
          <p className="text-sm text-muted-foreground mt-0.5">
            גישת מנהל · צפה וערוך את כל הנתונים
          </p>
        </div>
      </div>
      {action}
    </motion.div>
  );
}
