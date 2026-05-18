import type { Metric } from "web-vitals";

/**
 * Beacon web-vitals to the server when the API endpoint exists,
 * otherwise log to the console in development.
 *
 * Metrics collected: CLS, FCP, INP, LCP, TTFB.
 */

const VITALS_ENDPOINT = "/api/v1/vitals";

function sendToServer(metric: Metric) {
  const body = JSON.stringify({
    name: metric.name,
    value: metric.value,
    rating: metric.rating,
    delta: metric.delta,
    id: metric.id,
    navigationType: metric.navigationType,
  });

  if (navigator.sendBeacon) {
    navigator.sendBeacon(VITALS_ENDPOINT, body);
  } else {
    fetch(VITALS_ENDPOINT, {
      body,
      method: "POST",
      keepalive: true,
      headers: { "Content-Type": "application/json" },
    }).catch(() => {});
  }
}

function logToConsole(metric: Metric) {
  const color =
    metric.rating === "good"
      ? "color: #10B981"
      : metric.rating === "needs-improvement"
        ? "color: #F59E0B"
        : "color: #EF4444";

  console.debug(
    `%c[Web Vital] ${metric.name}: ${metric.value.toFixed(1)} (${metric.rating})`,
    color,
  );
}

export async function reportWebVitals() {
  const { onCLS, onFCP, onINP, onLCP, onTTFB } = await import("web-vitals");

  const report = import.meta.env.DEV ? logToConsole : sendToServer;

  onCLS(report);
  onFCP(report);
  onINP(report);
  onLCP(report);
  onTTFB(report);
}
