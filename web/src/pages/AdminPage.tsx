import { Database, Cpu, Table, RefreshCw, HardDrive, Activity } from "lucide-react";
import { useAdminStats } from "@/hooks/useAdmin";
import { cn } from "@/lib/utils";

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
      <div className="space-y-6 animate-fade-in">
        <div className="h-8 w-40 rounded-lg skeleton" />
        <div className="grid gap-4 sm:grid-cols-2">
          {[1, 2, 3].map((i) => (
            <div key={i} className="h-44 rounded-2xl skeleton" />
          ))}
        </div>
      </div>
    );
  }

  if (isError || !data) {
    return (
      <div className="space-y-6 animate-fade-in">
        <h1 className="text-2xl font-bold">ניהול מערכת</h1>
        <div className="rounded-2xl border border-destructive/30 bg-destructive/5 p-8 text-center">
          <p className="text-destructive font-semibold text-lg">
            שגיאה בטעינת הנתונים
          </p>
          <p className="text-sm text-muted-foreground mt-2">
            ייתכן שאין הרשאת גישה
          </p>
        </div>
      </div>
    );
  }

  const lastUpdated = new Date(dataUpdatedAt);
  const sortedTables = Object.entries(data.tables).sort(
    ([, a], [, b]) => b - a,
  );
  const maxCount = sortedTables.length > 0 ? sortedTables[0][1] : 1;

  return (
    <div className="space-y-6 pb-20 md:pb-4 animate-fade-in">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2.5">
          <div className="flex h-8 w-8 items-center justify-center rounded-xl gradient-primary">
            <Activity className="h-4 w-4 text-white" />
          </div>
          <h1 className="text-2xl font-bold tracking-tight">ניהול מערכת</h1>
        </div>
        <div className="flex items-center gap-1.5 text-xs text-muted-foreground bg-secondary rounded-lg px-3 py-1.5">
          <RefreshCw className="h-3 w-3" />
          {lastUpdated.toLocaleTimeString("he-IL")}
        </div>
      </div>

      {/* DB Storage */}
      <div className="rounded-2xl border border-border bg-card p-6 gradient-card card-shine animate-slide-up stagger-1">
        <div className="flex items-center gap-2.5 mb-5">
          <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-blue-500/10 dark:bg-blue-500/20">
            <Database className="h-5 w-5 text-blue-600 dark:text-blue-400" />
          </div>
          <h2 className="text-lg font-bold">אחסון בסיס נתונים</h2>
        </div>
        <div className="flex items-baseline gap-2 mb-4">
          <span className="text-4xl font-bold tracking-tight">
            {data.db.file_size_human}
          </span>
          <span className="text-sm text-muted-foreground font-medium">
            ({data.db.file_size_bytes.toLocaleString("he-IL")} bytes)
          </span>
        </div>
        <StorageBar sizeBytes={data.db.file_size_bytes} />
      </div>

      {/* Tables */}
      <div className="rounded-2xl border border-border bg-card p-6 gradient-card animate-slide-up stagger-2">
        <div className="flex items-center gap-2.5 mb-5">
          <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-purple-500/10 dark:bg-purple-500/20">
            <Table className="h-5 w-5 text-purple-600 dark:text-purple-400" />
          </div>
          <h2 className="text-lg font-bold">גודל טבלאות</h2>
        </div>
        <div className="space-y-2">
          {sortedTables.map(([table, count]) => (
            <div
              key={table}
              className="flex items-center gap-3 rounded-xl bg-muted/50 px-4 py-3 group hover:bg-muted transition-colors"
            >
              <span className="text-sm font-semibold min-w-[100px]">
                {TABLE_LABELS[table] ?? table}
              </span>
              <div className="flex-1 h-2 rounded-full bg-border overflow-hidden">
                <div
                  className="h-full rounded-full gradient-primary transition-all duration-700"
                  style={{
                    width: `${Math.max(2, (count / maxCount) * 100)}%`,
                  }}
                />
              </div>
              <span className="text-sm font-mono font-bold tabular-nums min-w-[60px] text-left">
                {count.toLocaleString("he-IL")}
              </span>
            </div>
          ))}
        </div>
      </div>

      {/* Runtime */}
      <div className="rounded-2xl border border-border bg-card p-6 gradient-card animate-slide-up stagger-3">
        <div className="flex items-center gap-2.5 mb-5">
          <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-emerald-500/10 dark:bg-emerald-500/20">
            <Cpu className="h-5 w-5 text-emerald-600 dark:text-emerald-400" />
          </div>
          <h2 className="text-lg font-bold">סטטוס מערכת</h2>
        </div>
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <RuntimeStat
            icon={HardDrive}
            label="זמן פעילות"
            value={data.runtime.uptime}
          />
          <RuntimeStat
            icon={Activity}
            label="Goroutines"
            value={String(data.runtime.goroutines)}
          />
          <RuntimeStat
            icon={Cpu}
            label="זיכרון (Alloc)"
            value={`${data.runtime.mem_alloc_mb.toFixed(1)} MB`}
          />
          <RuntimeStat
            icon={Database}
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
      ? "from-red-500 to-red-600"
      : percent > 50
        ? "from-amber-500 to-orange-500"
        : "from-emerald-500 to-green-500";

  return (
    <div>
      <div className="h-3 w-full rounded-full bg-muted overflow-hidden">
        <div
          className={cn(
            "h-full rounded-full bg-gradient-to-r transition-all duration-700",
            color,
          )}
          style={{ width: `${percent}%` }}
        />
      </div>
      <p className="text-xs text-muted-foreground mt-1.5 text-left font-medium" dir="ltr">
        {percent.toFixed(1)}% of 500 MB
      </p>
    </div>
  );
}

function RuntimeStat({
  icon: Icon,
  label,
  value,
}: {
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  value: string;
}) {
  return (
    <div className="rounded-xl border border-border bg-background p-4 text-center card-shine">
      <Icon className="mx-auto h-4 w-4 text-muted-foreground mb-1.5" />
      <p className="text-xs text-muted-foreground font-medium">{label}</p>
      <p className="text-sm font-bold mt-1 font-mono tracking-tight">
        {value}
      </p>
    </div>
  );
}
