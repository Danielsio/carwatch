import { useEffect, useState } from "react";
import { AlertCircle, CheckCircle2, Clock, Search, Bell, Car } from "lucide-react";
import { adminApi, type AdminCycleEntry } from "@/lib/api";
import { cn } from "@/lib/utils";

export function CyclesTab() {
  const [cycles, setCycles] = useState<AdminCycleEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  useEffect(() => {
    let cancelled = false;
    const load = () => {
      adminApi.cycles(50).then((res) => {
        if (!cancelled) {
          setCycles(res.items ?? []);
          setError(false);
          setLoading(false);
        }
      }).catch(() => {
        if (!cancelled) {
          setError(true);
          setLoading(false);
        }
      });
    };
    load();
    const interval = setInterval(load, 30_000);
    return () => { cancelled = true; clearInterval(interval); };
  }, []);

  if (loading) {
    return (
      <div className="flex h-64 items-center justify-center text-muted-foreground">
        טוען לוג סריקות...
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex h-64 items-center justify-center text-destructive">
        שגיאה בטעינת לוג סריקות
      </div>
    );
  }

  if (cycles.length === 0) {
    return (
      <div className="flex h-64 items-center justify-center text-muted-foreground">
        אין נתוני סריקות עדיין. סריקה ראשונה תתחיל בקרוב.
      </div>
    );
  }

  return (
    <div className="space-y-3">
      <p className="text-xs text-muted-foreground">
        {cycles.length} סריקות אחרונות · מתעדכן אוטומטית כל 30 שניות
      </p>
      <div className="space-y-2">
        {cycles.map((c) => {
          const isError = c.status === "error";
          const time = new Date(c.started_at);
          const durationSec = (c.duration_ms / 1000).toFixed(1);

          return (
            <div
              key={c.id}
              className={cn(
                "rounded-xl border p-4 transition-colors",
                isError
                  ? "border-destructive/30 bg-destructive/5"
                  : "border-border/50 bg-card hover:border-border",
              )}
            >
              <div className="flex items-start justify-between gap-3">
                <div className="flex items-center gap-2 min-w-0">
                  {isError ? (
                    <AlertCircle className="h-4 w-4 text-destructive flex-shrink-0" />
                  ) : (
                    <CheckCircle2 className="h-4 w-4 text-score-great flex-shrink-0" />
                  )}
                  <span className="text-sm font-medium truncate">
                    {time.toLocaleDateString("he-IL")} {time.toLocaleTimeString("he-IL")}
                  </span>
                </div>
                <div className="flex items-center gap-1 text-xs text-muted-foreground flex-shrink-0">
                  <Clock className="h-3 w-3" />
                  {durationSec}s
                </div>
              </div>

              <div className="flex flex-wrap gap-3 mt-3 text-xs">
                <span className="inline-flex items-center gap-1 text-muted-foreground">
                  <Search className="h-3 w-3" />
                  {c.searches} חיפושים
                </span>
                <span className="inline-flex items-center gap-1 text-muted-foreground">
                  <Car className="h-3 w-3" />
                  {c.listings_fetched} מודעות
                </span>
                <span className={cn(
                  "inline-flex items-center gap-1 font-medium",
                  c.listings_matched > 0 ? "text-primary" : "text-muted-foreground",
                )}>
                  ✓ {c.listings_matched} התאמות
                </span>
                {c.notifications > 0 && (
                  <span className="inline-flex items-center gap-1 text-chart-4 font-medium">
                    <Bell className="h-3 w-3" />
                    {c.notifications} התראות
                  </span>
                )}
              </div>

              {isError && c.error_message && (
                <p className="mt-2 text-xs text-destructive/80 font-mono bg-destructive/5 rounded-lg p-2 break-all">
                  {c.error_message}
                </p>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
