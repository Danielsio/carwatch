import { AlertCircle, Cpu, Database, RefreshCw, Table } from "lucide-react";
import { useAdminStats } from "@/hooks/useAdmin";
import { EmptyState, PageHeader, Skeleton } from "@/components/ui";

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
          description="ייתכן שאין הרשאת גישה"
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

      {/* DB Storage */}
      <div className="rounded-2xl border border-border/50 bg-card p-6">
        <div className="flex items-center gap-2.5 mb-5">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary/10">
            <Database className="h-4 w-4 text-primary" />
          </div>
          <h2 className="text-lg font-semibold">אחסון בסיס נתונים</h2>
        </div>
        <div className="flex items-baseline gap-2 mb-4">
          <span className="text-3xl font-bold tabular-nums">
            {data.db.file_size_human}
          </span>
          <span className="text-sm text-muted-foreground tabular-nums">
            ({data.db.file_size_bytes.toLocaleString("he-IL")} bytes)
          </span>
        </div>
        <StorageBar sizeBytes={data.db.file_size_bytes} />
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
            .map(([table, count]) => (
              <div
                key={table}
                className="flex items-center justify-between rounded-xl bg-secondary/50 px-4 py-2.5 transition-colors duration-200 hover:bg-secondary"
              >
                <span className="text-sm font-medium">
                  {TABLE_LABELS[table] ?? table}
                </span>
                <span className="text-sm font-mono font-semibold tabular-nums text-muted-foreground">
                  {count.toLocaleString("he-IL")}
                </span>
              </div>
            ))}
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
    </div>
  );
}

function StorageBar({ sizeBytes }: { sizeBytes: number }) {
  const maxBytes = 500 * 1024 * 1024;
  const percent = Math.min((sizeBytes / maxBytes) * 100, 100);

  const color =
    percent > 80
      ? "bg-destructive shadow-[0_0_8px_rgba(239,68,68,0.4)]"
      : percent > 50
        ? "bg-score-good shadow-[0_0_8px_rgba(245,158,11,0.3)]"
        : "bg-score-great shadow-[0_0_8px_rgba(16,185,129,0.3)]";

  return (
    <div>
      <div className="h-2.5 w-full rounded-full bg-secondary overflow-hidden">
        <div
          className={`h-full rounded-full transition-all duration-700 ease-out ${color}`}
          style={{ width: `${percent}%` }}
        />
      </div>
      <p
        className="text-xs text-muted-foreground mt-1.5 text-left tabular-nums"
        dir="ltr"
      >
        {percent.toFixed(1)}% of 500 MB
      </p>
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
