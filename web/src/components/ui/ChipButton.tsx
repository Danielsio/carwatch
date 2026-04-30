import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

export interface ChipButtonProps {
  selected: boolean;
  onClick: () => void;
  children: ReactNode;
}

export function ChipButton({ selected, onClick, children }: ChipButtonProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={selected}
      className={cn(
        "rounded-xl border px-3.5 py-2 text-sm transition-all duration-200 active:scale-[0.97]",
        selected
          ? "border-primary bg-primary/10 text-primary ring-1 ring-primary/20"
          : "border-border/50 bg-card hover:border-border hover:bg-surface-hover text-secondary-foreground",
      )}
    >
      {children}
    </button>
  );
}
