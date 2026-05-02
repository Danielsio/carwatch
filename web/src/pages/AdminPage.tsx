import { useState } from "react";
import {
  AlertCircle,
  Cpu,
  Database,
  RefreshCw,
  Table,
  Trash2,
  HardDrive,
  Loader2,
  ChevronLeft,
  ChevronRight,
} from "lucide-react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useAdminStats } from "@/hooks/useAdmin";
import { adminApi, type AdminListing } from "@/lib/api";
import { EmptyState, PageHeader, Skeleton } from "@/components/ui";
import { useToast } from "@/components/ui/Toast";
import { cn } from "@/lib/utils";
import { formatKm, formatPrice } from "@/lib/utils";

const TABLE_LABELS: Record<string, string> = {
  users: "משתמשים",
  searches: "חיפושים",
  listing_history: "מודעות",
  price_history: "היסטוריית מחירים",
  dedup_seen: "מודעות שזוהו",
  seen_listings: "מודעות שנצפו",
  notifications: "התראות",
  pending_notifications: "התראות ממתינות",
  market_cache: "מטמון שוק",
  catalog: "קטלוג",
  catalog_cache: "מטמון קטלוג",
  saved_listings: "מודעות שמורות",
  hidden_listings: "מודעות מוסתרות",
  pending_digest: "תקצירים ממתינים",
};

const NON_PURGEABLE = new Set(["users", "searches", "catalog"]);

export function AdminPage() {
  const { data, isLoading, isError, dataUpdatedAt, refetch } = useAdminStats();
  const [activeTab, setActiveTab] = useState<"stats" | "listings">("stats");

  if (isLoading) {
    return (
      <div className="space-y-6 pb-20 md:pb-4">
        <PageHeader title="ניהול" />
        <div className="grid gap-4 sm:grid-cols-2">
          {[1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-44 rounded-2xl" />
          ))}
        </div>
      </div>
    );
  }

  if (isError || !data) {
    return (
      <div className="space-y-6 pb-20 md:pb-4">
        <PageHeader title="ניהול" />
        <EmptyState
          icon={AlertCircle}
          title="שגיאה בטעינת הנתונים"
          description="ייתכן שאין לך הרשאות מנהל. ודא שה-admin_email מוגדר בקובץ ההגדרות."
        />
      </div>
    );
  }

  const lastUpdated = new Date(dataUpdatedAt);

  return (
    <div className="space-y-6 pb-20 md:pb-4">
      <PageHeader
        title="ניהול"
        action={
          <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <RefreshCw className="h-3 w-3" />
            עדכון אחרון: {lastUpdated.toLocaleTimeString("he-IL")}
          </div>
        }
      />

      {/* Tabs */}
      <div className="flex gap-1 rounded-xl bg-secondary/50 p-1">
        <button
          type="button"
          onClick={() => setActiveTab("stats")}
          className={cn(
            "flex-1 rounded-lg px-4 py-2 text-sm font-medium transition-colors",
            activeTab === "stats"
              ? "bg-card text-foreground shadow-sm"
              : "text-muted-foreground hover:text-foreground",
          )}
        >
          סטטיסטיקות
        </button>
        <button
          type="button"
          onClick={() => setActiveTab("listings")}
          className={cn(
            "flex-1 rounded-lg px-4 py-2 text-sm font-medium transition-colors",
            activeTab === "listings"
              ? "bg-card text-foreground shadow-sm"
              : "text-muted-foreground hover:text-foreground",
          )}
        >
          מודעות
        </button>
      </div>

      {activeTab === "stats" ? (
        <StatsTab data={data} onRefresh={() => void refetch()} />
      ) : (
        <ListingsTab />
      )}
    </div>
  );
}

function StatsTab({
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
      toast(`נמחקו ${result.deleted} רשומות מ-${TABLE_LABELS[result.table] ?? result.table}`, "success");
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
      toast(`דחיסת מסד נתונים הושלמה — ${result.size_after}`, "success");
      void queryClient.invalidateQueries({ queryKey: ["admin"] });
      onRefresh();
    },
    onError: () => {
      toast("שגיאה בדחיסת מסד הנתונים", "error");
    },
  });

  return (
    <>
      {/* DB Storage */}
      <div className="rounded-2xl border border-border/50 bg-card p-6">
        <div className="flex items-center justify-between mb-5">
          <div className="flex items-center gap-2.5">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary/10">
              <Database className="h-4 w-4 text-primary" />
            </div>
            <h2 className="text-lg font-semibold">אחסון בסיס נתונים</h2>
          </div>
          <button
            type="button"
            onClick={() => void vacuumMutation.mutate()}
            disabled={vacuumMutation.isPending}
            className="inline-flex items-center gap-1.5 rounded-lg border border-border bg-secondary px-3 py-1.5 text-xs font-medium transition-colors hover:bg-muted disabled:opacity-50"
          >
            {vacuumMutation.isPending ? (
              <Loader2 className="h-3 w-3 animate-spin" />
            ) : (
              <HardDrive className="h-3 w-3" />
            )}
            דחיסת DB
          </button>
        </div>
        <div className="flex items-baseline gap-2 mb-4">
          <span className="text-3xl font-bold tabular-nums">
            {data.db.file_size_human}
          </span>
          <span className="text-sm text-muted-foreground tabular-nums">
            ({data.db.file_size_bytes.toLocaleString("he-IL")} bytes)
          </span>
        </div>
        <StorageIndicator sizeBytes={data.db.file_size_bytes} />
      </div>

      {/* Table sizes */}
      <div className="rounded-2xl border border-border/50 bg-card p-6">
        <div className="flex items-center gap-2.5 mb-5">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary/10">
            <Table className="h-4 w-4 text-primary" />
          </div>
          <h2 className="text-lg font-semibold">גודל טבלאות</h2>
        </div>
        <div className="space-y-1.5">
          {Object.entries(data.tables)
            .sort(([, a], [, b]) => b - a)
            .map(([table, count]) => {
              const canPurge = !NON_PURGEABLE.has(table) && count > 0;
              const isConfirming = confirmPurge === table;

              return (
                <div
                  key={table}
                  className="flex items-center justify-between rounded-xl bg-secondary/50 px-4 py-2.5 transition-colors duration-200 hover:bg-secondary"
                >
                  <span className="text-sm font-medium">
                    {TABLE_LABELS[table] ?? table}
                  </span>
                  <div className="flex items-center gap-3">
                    <span className="text-sm font-mono font-semibold tabular-nums text-muted-foreground">
                      {count.toLocaleString("he-IL")}
                    </span>
                    {canPurge && !isConfirming && (
                      <button
                        type="button"
                        onClick={() => setConfirmPurge(table)}
                        className="rounded p-1 text-muted-foreground/50 transition-colors hover:text-destructive"
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
                          className="rounded-md bg-destructive px-2 py-0.5 text-[11px] font-medium text-white transition-opacity hover:opacity-90 disabled:opacity-50"
                        >
                          {purgeMutation.isPending ? "מוחק..." : "אישור"}
                        </button>
                        <button
                          type="button"
                          onClick={() => setConfirmPurge(null)}
                          className="rounded-md border border-border px-2 py-0.5 text-[11px] font-medium text-muted-foreground transition-colors hover:text-foreground"
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

      {/* Runtime */}
      <div className="rounded-2xl border border-border/50 bg-card p-6">
        <div className="flex items-center gap-2.5 mb-5">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary/10">
            <Cpu className="h-4 w-4 text-primary" />
          </div>
          <h2 className="text-lg font-semibold">סטטוס מערכת</h2>
        </div>
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <StatBox label="זמן פעילות" value={data.runtime.uptime} />
          <StatBox
            label="Goroutines"
            value={String(data.runtime.goroutines)}
          />
          <StatBox
            label="זיכרון (Alloc)"
            value={`${data.runtime.mem_alloc_mb.toFixed(1)} MB`}
          />
          <StatBox
            label="זיכרון (Sys)"
            value={`${data.runtime.mem_sys_mb.toFixed(1)} MB`}
          />
        </div>
      </div>
    </>
  );
}

function ListingsTab() {
  const [page, setPage] = useState(0);
  const pageSize = 20;
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const { data, isLoading, isError } = useQuery({
    queryKey: ["admin", "listings", page],
    queryFn: () => adminApi.listings({ limit: pageSize, offset: page * pageSize }),
  });

  const deleteMutation = useMutation({
    mutationFn: ({ token, chatId }: { token: string; chatId: number }) =>
      adminApi.deleteListing(token, chatId),
    onSuccess: () => {
      toast("המודעה נמחקה", "success");
      void queryClient.invalidateQueries({ queryKey: ["admin"] });
    },
  });

  const totalPages = data ? Math.ceil(data.total / pageSize) : 0;

  return (
    <div className="space-y-4">
      <div className="rounded-2xl border border-border/50 bg-card p-6">
        <div className="flex items-center justify-between mb-5">
          <div className="flex items-center gap-2.5">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary/10">
              <Database className="h-4 w-4 text-primary" />
            </div>
            <h2 className="text-lg font-semibold">כל המודעות</h2>
          </div>
          {data && (
            <span className="text-sm text-muted-foreground tabular-nums">
              {data.total.toLocaleString("he-IL")} סה״כ
            </span>
          )}
        </div>

        {isLoading ? (
          <div className="space-y-2">
            {Array.from({ length: 5 }).map((_, i) => (
              <Skeleton key={i} className="h-16 rounded-xl" />
            ))}
          </div>
        ) : isError ? (
          <p className="text-sm text-destructive text-center py-8">
            שגיאה בטעינת המודעות
          </p>
        ) : !data || data.items.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-8">
            אין מודעות
          </p>
        ) : (
          <div className="space-y-1.5">
            {data.items.map((listing) => (
              <AdminListingRow
                key={`${listing.token}-${listing.chat_id}`}
                listing={listing}
                onDelete={(token, chatId) => deleteMutation.mutate({ token, chatId })}
                deleting={deleteMutation.isPending}
              />
            ))}
          </div>
        )}

        {totalPages > 1 && (
          <div className="mt-4 flex items-center justify-center gap-3">
            <button
              type="button"
              onClick={() => setPage((p) => Math.max(0, p - 1))}
              disabled={page === 0}
              className="rounded-lg border border-border p-2 transition-colors hover:bg-muted disabled:opacity-30"
            >
              <ChevronRight className="h-4 w-4" />
            </button>
            <span className="text-sm tabular-nums text-muted-foreground">
              {page + 1} / {totalPages}
            </span>
            <button
              type="button"
              onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
              disabled={page >= totalPages - 1}
              className="rounded-lg border border-border p-2 transition-colors hover:bg-muted disabled:opacity-30"
            >
              <ChevronLeft className="h-4 w-4" />
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

function AdminListingRow({
  listing,
  onDelete,
  deleting,
}: {
  listing: AdminListing;
  onDelete: (token: string, chatId: number) => void;
  deleting: boolean;
}) {
  const [confirm, setConfirm] = useState(false);

  return (
    <div className="flex items-center gap-3 rounded-xl bg-secondary/50 px-4 py-3 transition-colors hover:bg-secondary">
      {listing.image_url ? (
        <img
          src={listing.image_url}
          alt=""
          className="h-10 w-14 rounded-lg object-cover bg-muted flex-shrink-0"
          referrerPolicy="no-referrer"
        />
      ) : (
        <div className="h-10 w-14 rounded-lg bg-muted flex items-center justify-center flex-shrink-0 text-lg opacity-20">
          🚗
        </div>
      )}
      <div className="min-w-0 flex-1">
        <p className="text-sm font-medium truncate">
          {listing.manufacturer} {listing.model} {listing.year}
        </p>
        <p className="text-xs text-muted-foreground truncate">
          {formatPrice(listing.price)} · {formatKm(listing.km)} · {listing.city}
        </p>
      </div>
      <a
        href={listing.page_link}
        target="_blank"
        rel="noopener noreferrer"
        className="text-xs text-primary hover:underline flex-shrink-0"
      >
        צפה
      </a>
      {!confirm ? (
        <button
          type="button"
          onClick={() => setConfirm(true)}
          className="rounded p-1 text-muted-foreground/50 transition-colors hover:text-destructive flex-shrink-0"
        >
          <Trash2 className="h-3.5 w-3.5" />
        </button>
      ) : (
        <div className="flex items-center gap-1.5 flex-shrink-0">
          <button
            type="button"
            onClick={() => { onDelete(listing.token, listing.chat_id); setConfirm(false); }}
            disabled={deleting}
            className="rounded-md bg-destructive px-2 py-0.5 text-[11px] font-medium text-white transition-opacity hover:opacity-90 disabled:opacity-50"
          >
            מחק
          </button>
          <button
            type="button"
            onClick={() => setConfirm(false)}
            className="rounded-md border border-border px-2 py-0.5 text-[11px] font-medium text-muted-foreground transition-colors hover:text-foreground"
          >
            ביטול
          </button>
        </div>
      )}
    </div>
  );
}

function StorageIndicator({ sizeBytes }: { sizeBytes: number }) {
  const mb = sizeBytes / (1024 * 1024);
  const color =
    mb > 400
      ? "bg-destructive"
      : mb > 200
        ? "bg-score-good"
        : "bg-score-great";

  return (
    <div className="flex items-center gap-2">
      <span
        className={`inline-block h-2.5 w-2.5 rounded-full ${color}`}
        aria-hidden
      />
      <span className="text-xs text-muted-foreground tabular-nums" dir="ltr">
        {mb.toFixed(1)} MB
      </span>
    </div>
  );
}

function StatBox({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl border border-border/50 bg-secondary/50 p-3.5 text-center transition-colors duration-200 hover:border-border">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="text-sm font-semibold mt-1 font-mono tabular-nums">
        {value}
      </p>
    </div>
  );
}
