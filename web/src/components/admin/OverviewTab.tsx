import { useState } from "react";
import {
  Cpu,
  Database,
  HardDrive,
  Loader2,
  Table,
  Trash2,
} from "lucide-react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { motion } from "motion/react";
import type { useAdminStats } from "@/hooks/useAdmin";
import { adminApi } from "@/lib/api";
import { useToast } from "@/components/ui/Toast";
import { cn } from "@/lib/utils";

const TABLE_LABELS: Record<string, string> = {
  users: "משתמשים",
  searches: "חיפושים",
  listing_history: "היסטוריית מודעות",
  price_history: "היסטוריית מחירים",
  dedup_seen: "סינון כפילויות",
  seen_listings: "מודעות שנצפו",
  notifications: "התראות",
  pending_notifications: "התראות ממתינות",
  catalog: "קטלוג",
  saved_listings: "מודעות שמורות",
  hidden_listings: "מודעות מוסתרות",
  pending_digest: "תקצירים ממתינים",
  link_tokens: "טוקנים לקישור",
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

export function OverviewTab({
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
