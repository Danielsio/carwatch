import { WifiOff, AlertTriangle } from "lucide-react";
import { cn } from "@/lib/utils";

type ConnectionStatus = "connected" | "degraded" | "disconnected";

export function ConnectionBanner({ status }: { status: ConnectionStatus }) {
  if (status === "connected") return null;

  const isDegraded = status === "degraded";

  return (
    <div
      role="alert"
      className={cn(
        "flex items-center justify-center gap-2 px-4 py-2 text-sm font-medium dir-rtl",
        isDegraded
          ? "bg-amber-500/10 text-amber-600 dark:text-amber-400"
          : "bg-destructive/10 text-destructive",
      )}
    >
      {isDegraded ? (
        <AlertTriangle className="h-4 w-4 flex-shrink-0" />
      ) : (
        <WifiOff className="h-4 w-4 flex-shrink-0" />
      )}
      <span>
        {isDegraded
          ? "השרת במצב מופחת — ייתכנו עיכובים"
          : "אין חיבור לשרת — בודק מחדש..."}
      </span>
    </div>
  );
}
