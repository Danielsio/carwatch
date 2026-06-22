import { useState } from "react";
import {
  Activity,
  Cpu,
  Database,
  HardDrive,
  Trash2,
} from "lucide-react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { motion } from "motion/react";
import type { useAdminStats } from "@/hooks/useAdmin";
import { adminApi } from "@/lib/api";
import { ActivityChart } from "./ActivityChart";
import { useToast } from "@/components/ui/Toast";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

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
    <Card className="transition-colors duration-200 hover:border-border">
      <CardContent className="p-4">
        <p className="text-xs text-muted-foreground mb-1">{label}</p>
        <p className="text-sm font-semibold font-mono tabular-nums">{value}</p>
      </CardContent>
    </Card>
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
  const vacuumMutation = useMutation({
    mutationFn: () => adminApi.vacuum(),
    meta: { suppressToast: true },
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

  const [showResetConfirm, setShowResetConfirm] = useState(false);
  const resetMutation = useMutation({
    mutationFn: () => adminApi.resetAll(),
    meta: { suppressToast: true },
    onSuccess: (result) => {
      toast(`איפוס הושלם — ${result.total.toLocaleString("he-IL")} שורות נמחקו`, "success");
      void queryClient.invalidateQueries({ queryKey: ["admin"] });
      onRefresh();
    },
    onError: () => {
      toast("שגיאה באיפוס המערכת", "error");
    },
  });

  const mb = data.db.file_size_bytes / (1024 * 1024);

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">פעילות יומית (30 ימים)</CardTitle>
        </CardHeader>
        <CardContent>
          <ActivityChart />
        </CardContent>
      </Card>

      {/* DB Storage + Runtime — two-column */}
      <div className="grid gap-6 lg:grid-cols-2">
        {/* DB Storage card */}
        <Card>
          <CardContent className="p-6">
          <div className="flex items-center justify-between mb-5">
            <div className="flex items-center gap-2.5">
              <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-primary/10">
                <Database className="h-[18px] w-[18px] text-primary" />
              </div>
              <h2 className="text-base font-semibold">אחסון</h2>
            </div>
            <div className="flex gap-2">
              <Button
                variant="destructive"
                size="sm"
                onClick={() => setShowResetConfirm(true)}
                loading={resetMutation.isPending}
              >
                <Trash2 className="h-3.5 w-3.5" />
                איפוס מערכת
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => void vacuumMutation.mutate()}
                loading={vacuumMutation.isPending}
              >
                <HardDrive className="h-3.5 w-3.5" />
                דחיסת DB
              </Button>
            </div>
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
          </CardContent>
        </Card>

        <Card>
          <CardContent className="p-6">
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
          </CardContent>
        </Card>
      </div>

      {data.pool && (
        <Card>
          <CardHeader>
            <CardTitle className="text-sm">מאגר חיבורים</CardTitle>
          </CardHeader>
          <CardContent>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
            <div className="rounded-xl bg-secondary/40 p-3">
              <p className="text-xs text-muted-foreground">פעיל</p>
              <p className="text-lg font-bold">{data.pool.in_use}</p>
            </div>
            <div className="rounded-xl bg-secondary/40 p-3">
              <p className="text-xs text-muted-foreground">במנוחה</p>
              <p className="text-lg font-bold">{data.pool.idle}</p>
            </div>
            <div className="rounded-xl bg-secondary/40 p-3">
              <p className="text-xs text-muted-foreground">מקסימום</p>
              <p className="text-lg font-bold">{data.pool.max_open_connections}</p>
            </div>
          </div>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardContent className="p-6">
        <div className="flex items-center gap-2.5 mb-5">
          <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-chart-3/10">
            <Activity className="h-[18px] w-[18px] text-chart-3" />
          </div>
          <h2 className="text-base font-semibold">בקשות API (מאז הפעלת השרת)</h2>
        </div>
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
          <RuntimeStat
            label="סה״כ בקשות"
            value={data.http.requests_total.toLocaleString("he-IL")}
          />
          <RuntimeStat
            label="2xx"
            value={data.http.status_2xx.toLocaleString("he-IL")}
          />
          <RuntimeStat
            label="4xx"
            value={data.http.status_4xx.toLocaleString("he-IL")}
          />
          <RuntimeStat
            label="5xx"
            value={data.http.status_5xx.toLocaleString("he-IL")}
          />
          <RuntimeStat
            label="זמן תגובה ממוצע"
            value={`${data.http.avg_duration_ms.toFixed(2)} ms`}
          />
        </div>
        </CardContent>
      </Card>

      {showResetConfirm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <Card className="w-full max-w-md shadow-xl">
            <CardContent className="p-6 space-y-4">
              <h3 className="text-lg font-bold text-destructive">איפוס מערכת</h3>
              <p className="text-sm text-muted-foreground">
                כל הנתונים יימחקו — היסטוריית מודעות, מחירים, סטטיסטיקות, ומועדפים.
                <br />
                <strong>משתמשים וחיפושים יישמרו.</strong>
              </p>
              <div className="flex gap-3 justify-end">
                <Button
                  variant="outline"
                  onClick={() => setShowResetConfirm(false)}
                >
                  ביטול
                </Button>
                <Button
                  variant="destructive"
                  onClick={() => {
                    setShowResetConfirm(false);
                    void resetMutation.mutate();
                  }}
                >
                  איפוס הכל
                </Button>
              </div>
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  );
}
