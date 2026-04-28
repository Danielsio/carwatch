import {
  Database,
  Cpu,
  Table,
  RefreshCw,
} from "lucide-react";
import { useAdminStats } from "@/hooks/useAdmin";

const TABLE_LABELS: Record<string, string> = {
  users: "משתמשים",
  searches: "חיפושים",
  listing_history: "מודעות",
  price_history: "היסטוריית מחירים",
  dedup_seen: "מודעות שזוהו",
  notifications: "התראות",
  market_cache: "מטמון שוק",
  catalog: "קטלוג",
};

export function AdminPage() {
  const { data, isLoading, isError, dataUpdatedAt } = useAdminStats();

  if (isLoading) {
    return (
      <div className="space-y-4">
        <h1 className="text-2xl font-bold">ניהול מערכת</h1>
        <div className="grid gap-4 sm:grid-cols-2">
          {[1, 2, 3].map((i) => (
            <div
              key={i}
              className="h-40 animate-pulse rounded-xl bg-muted"
            />
          ))}
        </div>
      </div>
    );
  }

  if (isError || !data) {
    return (
      <div className="space-y-4">
        <h1 className="text-2xl font-bold">ניהול מערכת</h1>
        <div className="rounded-xl border border-destructive/50 bg-destructive/10 p-6 text-center">
          <p className="text-destructive font-medium">שגיאה בטעינת הנתונים</p>
          <p className="text-sm text-muted-foreground mt-1">ייתכן שאין הרשאת גישה</p>
        </div>
      </div>
    );
  }

  const lastUpdated = new Date(dataUpdatedAt);

  return (
    <div className="space-y-6 pb-20 md:pb-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">ניהול מערכת</h1>
        <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
          <RefreshCw className="h-3 w-3" />
          עדכון אחרון: {lastUpdated.toLocaleTimeString("he-IL")}
        </div>
      </div>

      {/* DB Storage */}
      <div className="rounded-xl border border-border bg-card p-6">
        <div className="flex items-center gap-2 mb-4">
          <Database className="h-5 w-5 text-primary" />
          <h2 className="text-lg font-semibold">אחסון בסיס נתונים</h2>
        </div>
        <div className="flex items-baseline gap-2 mb-3">
          <span className="text-3xl font-bold">{data.db.file_size_human}</span>
          <span className="text-sm text-muted-foreground">
            ({data.db.file_size_bytes.toLocaleString("he-IL")} bytes)
          </span>
        </div>
        <StorageBar sizeBytes={data.db.file_size_bytes} />
      </div>

      {/* Table sizes */}
      <div className="rounded-xl border border-border bg-card p-6">
        <div className="flex items-center gap-2 mb-4">
          <Table className="h-5 w-5 text-primary" />
          <h2 className="text-lg font-semibold">גודל טבלאות</h2>
        </div>
        <div className="space-y-2">
          {Object.entries(data.tables)
            .sort(([, a], [, b]) => b - a)
            .map(([table, count]) => (
              <div
                key={table}
                className="flex items-center justify-between rounded-lg bg-muted/50 px-4 py-2.5"
              >
                <span className="text-sm font-medium">
                  {TABLE_LABELS[table] ?? table}
                </span>
                <span className="text-sm font-mono font-semibold">
                  {count.toLocaleString("he-IL")}
                </span>
              </div>
            ))}
        </div>
      </div>

      {/* Runtime */}
      <div className="rounded-xl border border-border bg-card p-6">
        <div className="flex items-center gap-2 mb-4">
          <Cpu className="h-5 w-5 text-primary" />
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
    </div>
  );
}

function StorageBar({ sizeBytes }: { sizeBytes: number }) {
  const maxBytes = 500 * 1024 * 1024; // 500 MB reference cap
  const percent = Math.min((sizeBytes / maxBytes) * 100, 100);
  const color =
    percent > 80
      ? "bg-destructive"
      : percent > 50
        ? "bg-yellow-500"
        : "bg-green-500";

  return (
    <div>
      <div className="h-3 w-full rounded-full bg-muted overflow-hidden">
        <div
          className={`h-full rounded-full transition-all ${color}`}
          style={{ width: `${Math.max(percent, 1)}%` }}
        />
      </div>
      <p className="text-xs text-muted-foreground mt-1 text-left" dir="ltr">
        {percent.toFixed(1)}% of 500 MB
      </p>
    </div>
  );
}

function StatBox({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl border border-border bg-background p-3 text-center">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="text-sm font-semibold mt-1 font-mono">{value}</p>
    </div>
  );
}
