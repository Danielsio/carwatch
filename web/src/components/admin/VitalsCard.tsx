import { Gauge } from "lucide-react";
import type { VitalsSummary } from "@/lib/api";

const VITAL_LABELS: Record<string, { label: string; unit: string; thresholds: [number, number] }> = {
  LCP: { label: "Largest Contentful Paint", unit: "ms", thresholds: [2500, 4000] },
  FCP: { label: "First Contentful Paint", unit: "ms", thresholds: [1800, 3000] },
  CLS: { label: "Cumulative Layout Shift", unit: "", thresholds: [0.1, 0.25] },
  INP: { label: "Interaction to Next Paint", unit: "ms", thresholds: [200, 500] },
  TTFB: { label: "Time to First Byte", unit: "ms", thresholds: [800, 1800] },
};

function ratingColor(name: string, value: number): string {
  const t = VITAL_LABELS[name]?.thresholds;
  if (!t) return "text-muted-foreground";
  if (value <= t[0]) return "text-score-great";
  if (value <= t[1]) return "text-score-good";
  return "text-destructive";
}

function ratingBg(name: string, value: number): string {
  const t = VITAL_LABELS[name]?.thresholds;
  if (!t) return "bg-muted";
  if (value <= t[0]) return "bg-score-great/10";
  if (value <= t[1]) return "bg-score-good/10";
  return "bg-destructive/10";
}

function formatValue(name: string, value: number): string {
  if (name === "CLS") return value.toFixed(3);
  return Math.round(value).toLocaleString("he-IL");
}

export function VitalsCard({ vitals }: { vitals: VitalsSummary[] }) {
  if (!vitals || vitals.length === 0) {
    return (
      <div className="rounded-2xl border border-border/50 bg-card p-6">
        <div className="flex items-center gap-2.5 mb-4">
          <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-chart-5/10">
            <Gauge className="h-[18px] w-[18px] text-chart-5" />
          </div>
          <h2 className="text-base font-semibold">Web Vitals</h2>
        </div>
        <p className="text-sm text-muted-foreground">
          אין נתוני ביצועים עדיין. נתונים יופיעו לאחר שמשתמשים יגלשו באתר.
        </p>
      </div>
    );
  }

  const order = ["LCP", "FCP", "INP", "CLS", "TTFB"];
  const sorted = [...vitals].sort(
    (a, b) => (order.indexOf(a.name) === -1 ? 99 : order.indexOf(a.name)) -
              (order.indexOf(b.name) === -1 ? 99 : order.indexOf(b.name)),
  );

  return (
    <div className="rounded-2xl border border-border/50 bg-card p-6">
      <div className="flex items-center gap-2.5 mb-5">
        <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-chart-5/10">
          <Gauge className="h-[18px] w-[18px] text-chart-5" />
        </div>
        <h2 className="text-base font-semibold">Web Vitals</h2>
      </div>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {sorted.map((v) => {
          const meta = VITAL_LABELS[v.name];
          const unit = meta?.unit ?? "";

          return (
            <div
              key={v.name}
              className={`rounded-xl border border-border/50 p-4 transition-colors ${ratingBg(v.name, v.p75)}`}
            >
              <div className="flex items-baseline justify-between mb-1">
                <span className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                  {v.name}
                </span>
                <span className="text-[10px] text-muted-foreground tabular-nums">
                  {v.count} דגימות
                </span>
              </div>
              <p className="text-xs text-muted-foreground mb-2 truncate">
                {meta?.label ?? v.name}
              </p>
              <div className={`text-2xl font-bold tabular-nums ${ratingColor(v.name, v.p75)}`}>
                {formatValue(v.name, v.p75)}
                {unit && <span className="text-sm font-normal ml-0.5">{unit}</span>}
              </div>
              <p className="text-[11px] text-muted-foreground mt-1">p75</p>

              <div className="flex gap-3 mt-3 text-[11px] tabular-nums">
                <span>p50: {formatValue(v.name, v.p50)}</span>
                <span>p95: {formatValue(v.name, v.p95)}</span>
              </div>

              <div className="flex gap-2 mt-2">
                {v.good > 0 && (
                  <span className="inline-flex items-center gap-1 rounded-md bg-score-great/20 px-1.5 py-0.5 text-[10px] font-medium text-score-great">
                    {v.good} טוב
                  </span>
                )}
                {v.needs_improvement > 0 && (
                  <span className="inline-flex items-center gap-1 rounded-md bg-score-good/20 px-1.5 py-0.5 text-[10px] font-medium text-score-good">
                    {v.needs_improvement} סביר
                  </span>
                )}
                {v.poor > 0 && (
                  <span className="inline-flex items-center gap-1 rounded-md bg-destructive/20 px-1.5 py-0.5 text-[10px] font-medium text-destructive">
                    {v.poor} גרוע
                  </span>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
