import { useState, useEffect } from "react";
import { Timer, Loader2, CheckCircle2 } from "lucide-react";
import { cn } from "@/lib/utils";
import { useSchedulerStatus } from "@/hooks/useSchedulerStatus";

export function NextScanCountdown() {
  const { data: status } = useSchedulerStatus();
  const [remaining, setRemaining] = useState<number | null>(null);

  useEffect(() => {
    if (!status?.next_cycle_at) return;

    const target = new Date(status.next_cycle_at).getTime();

    const tick = () => {
      setRemaining(target - Date.now());
    };
    tick();
    const id = setInterval(tick, 1000);
    return () => clearInterval(id);
  }, [status?.next_cycle_at]);

  if (!status?.next_cycle_at || remaining === null) return null;

  const isOverdue = remaining <= 0;
  const totalMs = (status.polling_interval_seconds ?? 900) * 1000;
  const elapsed = totalMs - remaining;
  const progress = isOverdue ? 100 : Math.min(100, Math.max(0, (elapsed / totalMs) * 100));

  const mins = Math.max(0, Math.floor(remaining / 60_000));
  const secs = Math.max(0, Math.floor((remaining % 60_000) / 1000));

  return (
    <div
      className={cn(
        "relative overflow-hidden rounded-xl border bg-card/60 backdrop-blur-sm px-4 py-3",
        "transition-all duration-500",
        isOverdue
          ? "border-primary/30 shadow-[0_0_20px_-6px_var(--color-glow-primary)]"
          : "border-border/30",
      )}
    >
      {/* Progress bar */}
      <div className="absolute inset-x-0 bottom-0 h-[2px] bg-border/20">
        <div
          className={cn(
            "h-full transition-all duration-1000 ease-linear rounded-full",
            isOverdue
              ? "bg-primary animate-pulse"
              : "bg-gradient-to-r from-primary/60 to-primary",
          )}
          style={{ width: `${progress}%` }}
        />
      </div>

      <div className="flex items-center gap-3">
        <div
          className={cn(
            "flex h-8 w-8 shrink-0 items-center justify-center rounded-lg transition-colors duration-500",
            isOverdue ? "bg-primary/15" : "bg-muted/60",
          )}
        >
          {isOverdue ? (
            <Loader2 className="h-4 w-4 text-primary animate-spin" />
          ) : remaining < 60_000 ? (
            <CheckCircle2 className="h-4 w-4 text-success" />
          ) : (
            <Timer className="h-4 w-4 text-muted-foreground" />
          )}
        </div>

        <div className="flex-1 min-w-0">
          <p className="text-xs text-muted-foreground leading-tight">
            {isOverdue ? "סורק עכשיו..." : "סריקה הבאה"}
          </p>
        </div>

        {!isOverdue && (
          <p className="text-sm font-semibold tabular-nums text-foreground tracking-tight">
            {mins}:{String(secs).padStart(2, "0")}
          </p>
        )}
      </div>
    </div>
  );
}
