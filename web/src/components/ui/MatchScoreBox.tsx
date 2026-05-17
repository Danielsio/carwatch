import { cn } from "@/lib/utils";
import { scoreHsl, scoreHslAlpha } from "@/lib/scoringAlgorithm";

export type MatchScoreBoxProps = {
  score: number;
  size?: "sm" | "md" | "lg";
  className?: string;
};

const sizeClass: Record<NonNullable<MatchScoreBoxProps["size"]>, string> = {
  sm: "h-9 w-9 text-sm [&_.denom]:text-[7px]",
  md: "h-12 w-12 text-lg [&_.denom]:text-[8px]",
  lg: "h-14 w-14 text-xl [&_.denom]:text-[9px]",
};

export function MatchScoreBox({ score, size = "md", className }: MatchScoreBoxProps) {
  const normalized = Number.isFinite(score)
    ? Math.min(10, Math.max(0, score))
    : 0;
  const formatted = normalized.toFixed(1);

  return (
    <div
      className={cn(
        "relative flex shrink-0 flex-col items-center justify-center rounded-xl font-extrabold leading-none overflow-hidden",
        sizeClass[size],
        className,
      )}
      style={{
        color: scoreHsl(normalized),
        backgroundColor: scoreHslAlpha(normalized, 0.10),
        boxShadow: `0 0 16px -4px ${scoreHslAlpha(normalized, 0.3)}, inset 0 0 0 1.5px ${scoreHslAlpha(normalized, 0.25)}`,
      }}
      aria-label={`ציון ${formatted} מתוך 10`}
    >
      <span className="tabular-nums">{formatted}</span>
      <span className="denom font-bold opacity-50">/10</span>
    </div>
  );
}
