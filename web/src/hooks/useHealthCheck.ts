import { useEffect, useRef, useState } from "react";

type ConnectionStatus = "connected" | "degraded" | "disconnected";

const POLL_INTERVAL_MS = 30_000;
const TIMEOUT_MS = 8_000;

export function useHealthCheck(): ConnectionStatus {
  const [status, setStatus] = useState<ConnectionStatus>("connected");
  const consecutiveFailures = useRef(0);

  useEffect(() => {
    let timer: number | undefined;
    let abortCtrl: AbortController | undefined;
    let disposed = false;

    async function check() {
      abortCtrl = new AbortController();
      const timeout = window.setTimeout(
        () => abortCtrl?.abort(),
        TIMEOUT_MS,
      );
      try {
        const res = await fetch("/healthz", { signal: abortCtrl.signal });
        window.clearTimeout(timeout);
        if (disposed) return;

        const data = await res.json().catch(() => null);
        if (data?.status === "degraded") {
          consecutiveFailures.current = 0;
          setStatus("degraded");
        } else if (res.ok) {
          consecutiveFailures.current = 0;
          setStatus("connected");
        } else {
          consecutiveFailures.current++;
          setStatus(
            consecutiveFailures.current >= 2 ? "disconnected" : "degraded",
          );
        }
      } catch {
        window.clearTimeout(timeout);
        if (disposed) return;
        consecutiveFailures.current++;
        setStatus(
          consecutiveFailures.current >= 2 ? "disconnected" : "degraded",
        );
      }
      if (!disposed) {
        timer = window.setTimeout(check, POLL_INTERVAL_MS);
      }
    }

    check();
    return () => {
      disposed = true;
      window.clearTimeout(timer);
      abortCtrl?.abort();
    };
  }, []);

  return status;
}
