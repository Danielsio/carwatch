import { useCallback, useEffect, useState } from "react";
import { getAuthToken } from "@/lib/auth-token";
import { adminApi, type LogEntry } from "@/lib/api";
import { feedSSE } from "@/lib/sse-parse";

const MAX_ENTRIES = 2000;
const STREAM_URL = "/api/v1/admin/logs/stream";
const RECONNECT_MS = 2000;

const STYLE_BY_LEVEL: Record<string, string> = {
  DEBUG: "color: #8b8b8b",
  INFO: "color: #3b82f6",
  WARN: "color: #f59e0b; font-weight: bold",
  ERROR: "color: #ef4444; font-weight: bold",
};

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function openLogStream(signal: AbortSignal): Promise<Response | null> {
  const headersFor = (token: string) => {
    const h = new Headers();
    h.set("Accept", "text/event-stream");
    h.set("Authorization", `Bearer ${token}`);
    return h;
  };

  let token = await getAuthToken().catch(() => null);
  if (!token) {
    return null;
  }

  let res = await fetch(STREAM_URL, {
    headers: headersFor(token),
    signal,
  });

  if (res.status === 401) {
    const fresh = await getAuthToken(true).catch(() => null);
    if (!fresh) {
      return res;
    }
    res = await fetch(STREAM_URL, {
      headers: headersFor(fresh),
      signal,
    });
  }

  return res;
}

export function useLogStream(enabled: boolean) {
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    if (!enabled) {
      setConnected(false);
      return;
    }

    const ac = new AbortController();
    let cancelled = false;

    async function run() {
      try {
        const { items } = await adminApi.logs(500);
        if (!cancelled) {
          setLogs(items);
        }
      } catch {
        // non-critical
      }

      while (!cancelled && !ac.signal.aborted) {
        let res: Response | null;
        try {
          res = await openLogStream(ac.signal);
        } catch {
          if (cancelled || ac.signal.aborted) {
            return;
          }
          setConnected(false);
          await delay(RECONNECT_MS);
          continue;
        }

        if (cancelled || ac.signal.aborted) {
          return;
        }

        if (!res || !res.ok || !res.body) {
          setConnected(false);
          await delay(RECONNECT_MS);
          continue;
        }

        setConnected(true);

        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let sseBuf = "";

        try {
          while (!cancelled && !ac.signal.aborted) {
            const { done, value } = await reader.read();
            if (done) {
              break;
            }
            const text = decoder.decode(value, { stream: true });
            sseBuf = feedSSE(text, sseBuf, (payload) => {
              try {
                const entry: LogEntry = JSON.parse(payload);
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
            });
          }
        } finally {
          reader.releaseLock();
        }

        if (cancelled || ac.signal.aborted) {
          return;
        }
        setConnected(false);
        await delay(RECONNECT_MS);
      }
    }

    void run();

    return () => {
      cancelled = true;
      ac.abort();
      setConnected(false);
    };
  }, [enabled]);

  const clear = useCallback(() => setLogs([]), []);

  return { logs, connected, clear };
}
