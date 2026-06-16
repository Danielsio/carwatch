import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  ReferenceDot,
} from "recharts";
import { TrendingDown, TrendingUp, Minus } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { formatPrice } from "@/lib/utils";
import { Skeleton } from "@/components/ui/skeleton";

interface PriceHistoryChartProps {
  token: string;
  currentPrice: number;
}

interface ChartPoint {
  date: string;
  label: string;
  price: number;
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

  if (loading) {
    return (
      <div className="rounded-2xl border border-border/50 bg-card p-5 space-y-3">
        <Skeleton className="h-5 w-32" />
        <Skeleton className="h-40 w-full rounded-xl" />
      </div>
    );
  }

  if (error || records.length === 0) {
    return (
      <div className="rounded-2xl border border-border/50 bg-card p-5 space-y-2">
        <h2 className="text-sm font-semibold text-foreground flex items-center gap-2">
          <Minus className="h-4 w-4 text-muted-foreground" />
          היסטוריית מחירים
        </h2>
        <p className="text-sm text-muted-foreground">
          אין היסטוריית מחירים
        </p>
      </div>
    );
  }

  // Single data point: show static message
  if (records.length === 1) {
    return (
      <div className="rounded-2xl border border-border/50 bg-card p-5 space-y-2">
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
      </div>
    );
  }

  // Multiple data points: build chart
  const sorted = [...records].sort(
    (a, b) =>
      new Date(a.observed_at).getTime() - new Date(b.observed_at).getTime(),
  );

  const chartData: ChartPoint[] = sorted.map((r) => ({
    date: r.observed_at,
    label: formatDateHe(r.observed_at),
    price: r.price,
  }));

  const firstPrice = sorted[0].price;
  const lastPrice = sorted[sorted.length - 1].price;
  const trend = lastPrice < firstPrice ? "down" : lastPrice > firstPrice ? "up" : "flat";

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

  const prices = sorted.map((r) => r.price);
  const minPrice = Math.min(...prices);
  const maxPrice = Math.max(...prices);
  const padding = Math.max(Math.round((maxPrice - minPrice) * 0.15), 1000);

  return (
    <div className="rounded-2xl border border-border/50 bg-card p-5 space-y-3">
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold text-foreground flex items-center gap-2">
          <TrendIcon className={`h-4 w-4 ${trendColor}`} />
          היסטוריית מחירים
        </h2>
        <span className={`text-xs font-medium ${trendColor}`}>
          {trendLabel}
        </span>
      </div>

      <div className="h-44 w-full" dir="ltr" role="img" aria-label="גרף היסטוריית מחירים">
        <ResponsiveContainer width="100%" height="100%">
          <LineChart
            data={chartData}
            margin={{ top: 8, right: 16, left: 8, bottom: 0 }}
          >
            <XAxis
              dataKey="label"
              tick={{ fontSize: 11 }}
              className="fill-muted-foreground"
            />
            <YAxis
              domain={[minPrice - padding, maxPrice + padding]}
              tick={{ fontSize: 11 }}
              className="fill-muted-foreground"
              tickFormatter={(v: number) =>
                v >= 1000 ? `${Math.round(v / 1000)}K` : String(v)
              }
            />
            <Tooltip
              formatter={(value) => [formatPrice(value as number), "מחיר"]}
              labelFormatter={(label) => String(label)}
              contentStyle={{
                backgroundColor: "var(--color-popover)",
                border: "1px solid var(--color-border)",
                borderRadius: "0.5rem",
                color: "var(--color-popover-foreground)",
                fontSize: 13,
                direction: "rtl",
              }}
            />
            <Line
              type="stepAfter"
              dataKey="price"
              stroke={lineColor}
              strokeWidth={2.5}
              dot={{ r: 4, fill: lineColor, strokeWidth: 0 }}
              activeDot={{ r: 6, fill: lineColor, strokeWidth: 0 }}
            />
            {/* Highlight current price point */}
            <ReferenceDot
              x={chartData[chartData.length - 1].label}
              y={currentPrice}
              r={0}
            />
          </LineChart>
        </ResponsiveContainer>
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
    </div>
  );
}
