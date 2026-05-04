import { useCallback, useEffect, useRef, useState } from "react";
import { getAuthToken } from "@/lib/auth-token";
import { adminApi, type LogEntry } from "@/lib/api";

const MAX_ENTRIES = 500;
const STYLE_BY_LEVEL: Record<string, string> = {
  DEBUG: "color: #8b8b8b",
  INFO: "color: #3b82f6",
  WARN: "color: #f59e0b; font-weight: bold",
  ERROR: "color: #ef4444; font-weight: bold",
};

export function useLogStream(enabled: boolean) {
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [connected, setConnected] = useState(false);
  const esRef = useRef<EventSource | null>(null);

  useEffect(() => {
    if (!enabled) {
      if (esRef.current) {
        esRef.current.close();
        esRef.current = null;
        setConnected(false);
      }
      return;
    }

    let cancelled = false;

    async function connect() {
      // Load recent logs first
      try {
        const { items } = await adminApi.logs(200);
        if (!cancelled) {
          setLogs(items);
        }
      } catch {
        // non-critical
      }

      const token = await getAuthToken().catch(() => null);
      if (cancelled) return;

      const url = `/api/v1/admin/logs/stream${token ? `?token=${encodeURIComponent(token)}` : ""}`;
      const es = new EventSource(url);
      esRef.current = es;

      es.onopen = () => {
        if (!cancelled) setConnected(true);
      };

      es.onmessage = (event) => {
        try {
          const entry: LogEntry = JSON.parse(event.data);

          const style = STYLE_BY_LEVEL[entry.level] ?? "";
          console.log(
            `%c[${entry.component}] ${entry.level}: ${entry.message}`,
            style,
            entry.attrs ?? "",
          );

          if (!cancelled) {
            setLogs((prev) => {
              const next = [...prev, entry];
              return next.length > MAX_ENTRIES
                ? next.slice(next.length - MAX_ENTRIES)
                : next;
            });
          }
        } catch {
          // ignore malformed events
        }
      };

      es.onerror = () => {
        if (!cancelled) setConnected(false);
      };
    }

    void connect();

    return () => {
      cancelled = true;
      if (esRef.current) {
        esRef.current.close();
        esRef.current = null;
      }
      setConnected(false);
    };
  }, [enabled]);

  const clear = useCallback(() => setLogs([]), []);

  return { logs, connected, clear };
}
