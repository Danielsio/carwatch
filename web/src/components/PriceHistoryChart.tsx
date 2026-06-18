import { useState, useRef, useCallback } from "react";
import { TrendingDown, TrendingUp, Minus } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { formatPrice } from "@/lib/utils";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

interface PriceHistoryChartProps {
  token: string;
  currentPrice: number;
}

interface ChartPoint {
  date: string;
  label: string;
  fullLabel: string;
  price: number;
  x: number;
  y: number;
}

function formatDateHe(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleDateString("he-IL", { day: "numeric", month: "short" });
}

function formatFullDateHe(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleDateString("he-IL", {
    day: "numeric",
    month: "long",
    year: "numeric",
  });
}

const CHART_W = 400;
const CHART_H = 160;
const PAD = { top: 12, right: 16, bottom: 28, left: 52 };
const PLOT_W = CHART_W - PAD.left - PAD.right;
const PLOT_H = CHART_H - PAD.top - PAD.bottom;

function buildStepPath(points: ChartPoint[]): string {
  if (points.length === 0) return "";
  let d = `M${points[0].x},${points[0].y}`;
  for (let i = 1; i < points.length; i++) {
    d += `H${points[i].x}V${points[i].y}`;
  }
  return d;
}

function formatAxisPrice(v: number): string {
  return v >= 1000 ? `${Math.round(v / 1000)}K` : String(v);
}

export function PriceHistoryChart({
  token,
  currentPrice,
}: PriceHistoryChartProps) {
  const { data, isLoading: loading, isError: error } = useQuery({
    queryKey: ["price-history", token],
    queryFn: () => api.priceHistory(token),
    staleTime: 5 * 60 * 1000,
  });
  const records = data?.items ?? [];
  const [hover, setHover] = useState<ChartPoint | null>(null);
  const svgRef = useRef<SVGSVGElement>(null);

  const onPointerMove = useCallback(
    (e: React.PointerEvent, points: ChartPoint[]) => {
      const svg = svgRef.current;
      if (!svg || points.length === 0) return;
      const rect = svg.getBoundingClientRect();
      const clientX = e.clientX - rect.left;
      const scale = CHART_W / rect.width;
      const mx = clientX * scale;

      let closest = points[0];
      let minDist = Math.abs(mx - closest.x);
      for (let i = 1; i < points.length; i++) {
        const dist = Math.abs(mx - points[i].x);
        if (dist < minDist) {
          minDist = dist;
          closest = points[i];
        }
      }
      setHover(closest);
    },
    [],
  );

  if (loading) {
    return (
      <Card>
        <CardContent className="p-5 space-y-3">
          <Skeleton className="h-5 w-32" />
          <Skeleton className="h-40 w-full rounded-xl" />
        </CardContent>
      </Card>
    );
  }

  if (error || records.length === 0) {
    return (
      <Card>
        <CardContent className="p-5 space-y-2">
          <h2 className="text-sm font-semibold text-foreground flex items-center gap-2">
            <Minus className="h-4 w-4 text-muted-foreground" />
            היסטוריית מחירים
          </h2>
          <p className="text-sm text-muted-foreground">
            אין היסטוריית מחירים
          </p>
        </CardContent>
      </Card>
    );
  }

  if (records.length === 1) {
    return (
      <Card>
        <CardContent className="p-5 space-y-2">
          <h2 className="text-sm font-semibold text-foreground flex items-center gap-2">
            <Minus className="h-4 w-4 text-muted-foreground" />
            היסטוריית מחירים
          </h2>
          <p className="text-sm text-muted-foreground">
            המחיר לא השתנה מאז שהמודעה הופיעה
          </p>
          <p className="text-xs text-muted-foreground">
            נצפה לראשונה: {formatFullDateHe(records[0].observed_at)}
          </p>
        </CardContent>
      </Card>
    );
  }

  const sorted = [...records].sort(
    (a, b) =>
      new Date(a.observed_at).getTime() - new Date(b.observed_at).getTime(),
  );

  const prices = sorted.map((r) => r.price);
  const minPrice = Math.min(...prices);
  const maxPrice = Math.max(...prices);
  const padding = Math.max(Math.round((maxPrice - minPrice) * 0.15), 1000);
  const yMin = minPrice - padding;
  const yMax = maxPrice + padding;
  const yRange = yMax - yMin;

  const firstPrice = sorted[0].price;
  const lastPrice = sorted[sorted.length - 1].price;
  const trend =
    lastPrice < firstPrice ? "down" : lastPrice > firstPrice ? "up" : "flat";

  const lineColor =
    trend === "down"
      ? "var(--color-emerald-500, #10b981)"
      : trend === "up"
        ? "var(--color-red-500, #ef4444)"
        : "var(--color-muted-foreground, #a1a1aa)";

  const TrendIcon =
    trend === "down" ? TrendingDown : trend === "up" ? TrendingUp : Minus;
  const trendColor =
    trend === "down"
      ? "text-emerald-500"
      : trend === "up"
        ? "text-red-500"
        : "text-muted-foreground";
  const trendLabel =
    trend === "down"
      ? "ירידה במחיר"
      : trend === "up"
        ? "עליה במחיר"
        : "מחיר יציב";

  const points: ChartPoint[] = sorted.map((r, i) => ({
    date: r.observed_at,
    label: formatDateHe(r.observed_at),
    fullLabel: formatFullDateHe(r.observed_at),
    price: r.price,
    x: PAD.left + (sorted.length === 1 ? PLOT_W / 2 : (i / (sorted.length - 1)) * PLOT_W),
    y: PAD.top + PLOT_H - ((r.price - yMin) / yRange) * PLOT_H,
  }));

  const yTicks = 4;
  const yTickValues = Array.from({ length: yTicks }, (_, i) =>
    Math.round(yMin + (i / (yTicks - 1)) * yRange),
  );

  const xTickInterval = Math.max(1, Math.floor(points.length / 4));

  return (
    <Card>
      <CardContent className="p-5 space-y-3">
        <div className="flex items-center justify-between">
          <h2 className="text-sm font-semibold text-foreground flex items-center gap-2">
            <TrendIcon className={`h-4 w-4 ${trendColor}`} />
            היסטוריית מחירים
          </h2>
          <span className={`text-xs font-medium ${trendColor}`}>
            {trendLabel}
          </span>
        </div>

        <div
          className="h-44 w-full"
          dir="ltr"
          role="img"
          aria-label="גרף היסטוריית מחירים"
        >
          <svg
            ref={svgRef}
            viewBox={`0 0 ${CHART_W} ${CHART_H}`}
            className="h-full w-full"
            onPointerMove={(e) => onPointerMove(e, points)}
            onPointerLeave={() => setHover(null)}
          >
            {/* Y-axis grid + labels */}
            {yTickValues.map((v) => {
              const y = PAD.top + PLOT_H - ((v - yMin) / yRange) * PLOT_H;
              return (
                <g key={v}>
                  <line
                    x1={PAD.left}
                    x2={CHART_W - PAD.right}
                    y1={y}
                    y2={y}
                    className="stroke-border"
                    strokeWidth={0.5}
                  />
                  <text
                    x={PAD.left - 6}
                    y={y + 3.5}
                    textAnchor="end"
                    className="fill-muted-foreground"
                    fontSize={10}
                  >
                    {formatAxisPrice(v)}
                  </text>
                </g>
              );
            })}

            {/* X-axis labels */}
            {points
              .filter(
                (_, i) =>
                  i === 0 ||
                  i === points.length - 1 ||
                  i % xTickInterval === 0,
              )
              .map((pt) => (
                <text
                  key={pt.date}
                  x={pt.x}
                  y={CHART_H - 6}
                  textAnchor="middle"
                  className="fill-muted-foreground"
                  fontSize={10}
                >
                  {pt.label}
                </text>
              ))}

            {/* Step line */}
            <path
              d={buildStepPath(points)}
              fill="none"
              stroke={lineColor}
              strokeWidth={2.5}
              strokeLinejoin="round"
            />

            {/* Data dots */}
            {points.map((pt) => (
              <circle
                key={pt.date}
                cx={pt.x}
                cy={pt.y}
                r={hover?.date === pt.date ? 6 : 4}
                fill={lineColor}
              />
            ))}

            {/* Hover crosshair + tooltip */}
            {hover && (
              <g>
                <line
                  x1={hover.x}
                  x2={hover.x}
                  y1={PAD.top}
                  y2={PAD.top + PLOT_H}
                  stroke={lineColor}
                  strokeWidth={1}
                  strokeDasharray="3,3"
                  opacity={0.5}
                />
                <rect
                  x={Math.min(hover.x - 50, CHART_W - PAD.right - 100)}
                  y={Math.max(hover.y - 38, PAD.top)}
                  width={100}
                  height={32}
                  rx={6}
                  className="fill-popover stroke-border"
                  strokeWidth={0.5}
                />
                <text
                  x={Math.min(hover.x, CHART_W - PAD.right - 50)}
                  y={Math.max(hover.y - 22, PAD.top + 16)}
                  textAnchor="middle"
                  className="fill-popover-foreground"
                  fontSize={11}
                  fontWeight={600}
                >
                  {formatPrice(hover.price)}
                </text>
                <text
                  x={Math.min(hover.x, CHART_W - PAD.right - 50)}
                  y={Math.max(hover.y - 10, PAD.top + 28)}
                  textAnchor="middle"
                  className="fill-muted-foreground"
                  fontSize={9}
                >
                  {hover.label}
                </text>
              </g>
            )}
          </svg>
        </div>

        {/* Timeline summary */}
        <div className="flex justify-between text-[11px] text-muted-foreground tabular-nums">
          <span>
            {formatFullDateHe(sorted[0].observed_at)} &mdash;{" "}
            {formatPrice(firstPrice)}
          </span>
          <span>
            {formatFullDateHe(sorted[sorted.length - 1].observed_at)} &mdash;{" "}
            {formatPrice(lastPrice)}
          </span>
        </div>
      </CardContent>
    </Card>
  );
}
