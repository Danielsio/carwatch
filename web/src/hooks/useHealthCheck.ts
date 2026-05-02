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

    async function check() {
      abortCtrl = new AbortController();
      const timeout = window.setTimeout(
        () => abortCtrl?.abort(),
        TIMEOUT_MS,
      );
      try {
        const res = await fetch("/healthz", { signal: abortCtrl.signal });
        window.clearTimeout(timeout);
        if (res.ok) {
          const data = await res.json().catch(() => null);
          consecutiveFailures.current = 0;
          setStatus(
            data?.status === "degraded" ? "degraded" : "connected",
          );
        } else {
          consecutiveFailures.current++;
          setStatus(
            consecutiveFailures.current >= 2 ? "disconnected" : "degraded",
          );
        }
      } catch {
        window.clearTimeout(timeout);
        consecutiveFailures.current++;
        setStatus(
          consecutiveFailures.current >= 2 ? "disconnected" : "degraded",
        );
      }
      timer = window.setTimeout(check, POLL_INTERVAL_MS);
    }

    check();
    return () => {
      window.clearTimeout(timer);
      abortCtrl?.abort();
    };
  }, []);

  return status;
}
