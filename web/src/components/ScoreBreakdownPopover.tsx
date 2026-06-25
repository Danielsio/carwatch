"use client";

import type { ScoreBreakdown } from "@/lib/api";
import { MatchScoreBox } from "@/components/ui/MatchScoreBox";
import {
  Popover,
  PopoverTrigger,
  PopoverContent,
} from "@/components/ui/popover";
import { scoreHsl, scoreHslAlpha } from "@/lib/scoringAlgorithm";
import { cn } from "@/lib/utils";

type ScoreBreakdownPopoverProps = {
  score: number;
  breakdown?: ScoreBreakdown;
  originalOwnership?: string | null;
  size?: "sm" | "md" | "lg";
  className?: string;
};

const DIMENSIONS = [
  { key: "condition" as const, label: "מצב הרכב", weight: "60%" },
  { key: "value" as const, label: "שווי עסקה", weight: "35%" },
  { key: "engine" as const, label: "מנוע", weight: "5%" },
];

const OWNERSHIP_LABELS: Record<string, string> = {
  private: "פרטי",
  lease: "ליסינג",
  rental: "השכרה",
};

export function ScoreBreakdownPopover({
  score,
  breakdown,
  originalOwnership,
  size = "md",
  className,
}: ScoreBreakdownPopoverProps) {
  if (!breakdown) {
    return <MatchScoreBox score={score} size={size} className={className} />;
  }

  return (
    <Popover>
      <PopoverTrigger
        className={cn("cursor-pointer", className)}
        aria-label="הצג פירוט ציון"
      >
        <MatchScoreBox score={score} size={size} />
      </PopoverTrigger>
      <PopoverContent side="bottom" align="start" className="w-[280px] p-0">
        <div className="p-3 space-y-3">
          {/* Header */}
          <div className="flex items-center justify-between">
            <span className="text-sm font-semibold">פירוט ציון</span>
            <span
              className="text-lg font-extrabold tabular-nums"
              style={{ color: scoreHsl(score) }}
            >
              {score.toFixed(1)}
            </span>
          </div>

          {/* Dimension rows */}
          <div className="space-y-2.5">
            {DIMENSIONS.map((dim) => {
              const value = breakdown[dim.key];
              const barColor = scoreHsl(value * 10);
              const barBg = scoreHslAlpha(value * 10, 0.15);
              return (
                <div key={dim.key} className="space-y-1">
                  <div className="flex items-center justify-between text-xs">
                    <div className="flex items-center gap-1.5">
                      <span className="font-medium text-foreground">
                        {dim.label}
                      </span>
                      <span className="rounded bg-muted px-1.5 py-0.5 text-[10px] font-semibold text-muted-foreground">
                        {dim.weight}
                      </span>
                    </div>
                    <span
                      className="font-bold tabular-nums"
                      style={{ color: barColor }}
                    >
                      {Math.round(value * 100)}%
                    </span>
                  </div>
                  <div
                    className="h-1.5 w-full overflow-hidden rounded-full"
                    style={{ backgroundColor: barBg }}
                  >
                    <div
                      className="h-full rounded-full transition-all duration-300"
                      style={{
                        width: `${Math.round(value * 100)}%`,
                        backgroundColor: barColor,
                      }}
                    />
                  </div>
                </div>
              );
            })}
          </div>

          {/* Ownership chip */}
          {originalOwnership && OWNERSHIP_LABELS[originalOwnership] ? (
            <div className="flex items-center gap-1.5 pt-1 border-t border-border/30 text-xs text-muted-foreground">
              <span>בעלות מקורית:</span>
              <span className="rounded-full bg-muted px-2 py-0.5 font-medium text-foreground">
                {OWNERSHIP_LABELS[originalOwnership]}
              </span>
            </div>
          ) : null}
        </div>
      </PopoverContent>
    </Popover>
  );
}
