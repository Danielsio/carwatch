import { useEffect, useMemo, useRef, useState } from "react";
import {
  ChevronDown,
  Download,
  ScrollText,
  Trash2,
} from "lucide-react";
import { adminApi, type LogEntry } from "@/lib/api";
import { useLogStream } from "@/hooks/useLogStream";
import { EmptyState } from "@/components/ui";
import { cn } from "@/lib/utils";

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

const ALL_COMPONENTS = [
  "scheduler", "yad2", "winwin", "enricher",
  "bot", "telegram", "notifier",
  "api-pricelist", "circuit_breaker",
] as const;
const ALL_LEVELS = ["DEBUG", "INFO", "WARN", "ERROR"] as const;

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
        <span className="text-muted-foreground/60 flex-shrink-0 tabular-nums w-[135px]">
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

export function LogsTab({ active }: { active: boolean }) {
  const { logs, connected, clear } = useLogStream(active);
  const [componentFilter, setComponentFilter] = useState<Set<string>>(
    new Set(ALL_COMPONENTS),
  );
  const [levelFilter, setLevelFilter] = useState<Set<string>>(
    new Set(ALL_LEVELS),
  );
  const [expandedKey, setExpandedKey] = useState<string | null>(null);
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

  const filtered = useMemo(
    () =>
      logs.filter(
        (e) => componentFilter.has(e.component) && levelFilter.has(e.level),
      ),
    [logs, componentFilter, levelFilter],
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
      const d = new Date(iso);
      const pad = (n: number) => String(n).padStart(2, "0");
      return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
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
            onClick={() => {
              const blob = new Blob([JSON.stringify(filtered, null, 2)], { type: "application/json" });
              const url = URL.createObjectURL(blob);
              const a = document.createElement("a");
              a.href = url;
              a.download = `carwatch-logs-${new Date().toISOString().slice(0, 19).replace(/:/g, "-")}.json`;
              document.body.appendChild(a);
              a.click();
              a.remove();
              setTimeout(() => URL.revokeObjectURL(url), 0);
            }}
            disabled={filtered.length === 0}
            className="inline-flex items-center gap-1.5 px-3 py-2 rounded-xl bg-secondary hover:bg-secondary/80 text-xs font-medium text-foreground transition-colors disabled:opacity-40 disabled:pointer-events-none"
          >
            <Download className="h-3 w-3" />
            ייצוא JSON
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
              {filtered.map((entry, idx) => {
                const key = `${entry.time}-${entry.level}-${entry.component}-${entry.message}`;
                return (
                  <LogLine
                    key={`${entry.time}-${idx}`}
                    entry={entry}
                    expanded={expandedKey === key}
                    onToggle={() =>
                      setExpandedKey(expandedKey === key ? null : key)
                    }
                    formatTime={formatTime}
                  />
                );
              })}
              <div ref={bottomRef} />
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
