import { cn } from "@/lib/utils";
import { scoreBgColor, scoreColor } from "@/lib/scoringAlgorithm";

export type MatchScoreBoxProps = {
  score: number;
  size?: "sm" | "md" | "lg";
  className?: string;
};

const sizeClass: Record<NonNullable<MatchScoreBoxProps["size"]>, string> = {
  sm: "h-11 w-11 text-lg [&_.denom]:text-[8px]",
  md: "h-14 w-14 text-xl [&_.denom]:text-[9px]",
  lg: "h-16 w-16 text-2xl [&_.denom]:text-[10px]",
};

/**
 * Rounded score tile matching the landing “Smart Match Score” demo (/10, tier border + fill).
 */
export function MatchScoreBox({ score, size = "md", className }: MatchScoreBoxProps) {
  const normalized = Number.isFinite(score)
    ? Math.min(10, Math.max(0, score))
    : 0;
  const formatted = normalized.toFixed(1);

  return (
    <div
      className={cn(
        "flex shrink-0 flex-col items-center justify-center rounded-2xl border-2 font-bold leading-none",
        scoreBgColor(normalized),
        scoreColor(normalized),
        sizeClass[size],
        className,
      )}
      aria-label={`ציון ${formatted} מתוך 10`}
    >
      <span className="tabular-nums">{formatted}</span>
      <span className="denom mt-0.5 font-semibold opacity-65">/10</span>
    </div>
  );
}
