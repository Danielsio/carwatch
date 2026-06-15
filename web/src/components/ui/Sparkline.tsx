import { useId } from "react";
import { cn } from "@/lib/utils";

export type SparklineProps = {
  data: number[];
  width?: number;
  height?: number;
  className?: string;
  ariaLabel?: string;
};

/**
 * Tiny inline SVG trend chart (area + line + endpoint dot). Uses currentColor,
 * so the parent's text color drives the hue. Decorative when no ariaLabel.
 */
export function Sparkline({
  data,
  width = 100,
  height = 28,
  className,
  ariaLabel,
}: SparklineProps) {
  const gradId = useId();
  if (!data || data.length < 2) return null;

  const max = Math.max(1, ...data);
  const last = data.length - 1;
  const stepX = width / last;
  const yOf = (v: number) => height - 2 - (v / max) * (height - 4);

  const pts = data.map((v, i) => `${(i * stepX).toFixed(1)},${yOf(v).toFixed(1)}`);
  const line = pts.join(" ");
  const area = `0,${height} ${line} ${width},${height}`;
  const lastX = (last * stepX).toFixed(1);
  const lastY = yOf(data[last]).toFixed(1);

  return (
    <svg
      width={width}
      height={height}
      viewBox={`0 0 ${width} ${height}`}
      className={cn("text-primary", className)}
      preserveAspectRatio="none"
      role="img"
      aria-label={ariaLabel}
      aria-hidden={ariaLabel ? undefined : true}
    >
      <defs>
        <linearGradient id={gradId} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="currentColor" stopOpacity="0.25" />
          <stop offset="100%" stopColor="currentColor" stopOpacity="0" />
        </linearGradient>
      </defs>
      <polygon points={area} fill={`url(#${gradId})`} />
      <polyline
        points={line}
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinejoin="round"
        strokeLinecap="round"
        vectorEffect="non-scaling-stroke"
      />
      <circle cx={lastX} cy={lastY} r="2" fill="currentColor" />
    </svg>
  );
}
