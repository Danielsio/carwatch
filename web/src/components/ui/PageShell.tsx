import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

export type PageShellProps = {
  children: ReactNode;
  gap?: "sm" | "md" | "lg";
  className?: string;
};

const gapClass = {
  sm: "space-y-4",
  md: "space-y-6",
  lg: "space-y-8",
} as const;

export function PageShell({
  children,
  gap = "md",
  className,
}: PageShellProps) {
  return (
    <div className={cn(gapClass[gap], className)}>
      {children}
    </div>
  );
}
