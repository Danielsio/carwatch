import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

export type EmptyStateProps = {
  icon: LucideIcon;
  title: string;
  description?: string;
  action?: ReactNode;
  className?: string;
};

export function EmptyState({
  icon: Icon,
  title,
  description,
  action,
  className,
}: EmptyStateProps) {
  return (
    <div
      className={cn(
        "relative flex flex-col items-center justify-center gap-4 rounded-2xl border border-border bg-card px-6 py-10 sm:py-14 md:py-16 text-center",
        className,
      )}
    >
      <div className="relative flex h-16 w-16 items-center justify-center rounded-2xl bg-primary/8 text-primary ring-1 ring-primary/10">
        <Icon className="h-7 w-7" aria-hidden />
      </div>
      <div className="relative space-y-1.5">
        <p className="text-lg font-semibold text-foreground">{title}</p>
        {description ? (
          <p className="max-w-sm text-sm text-muted-foreground leading-relaxed">
            {description}
          </p>
        ) : null}
      </div>
      {action ? <div className="relative pt-2">{action}</div> : null}
    </div>
  );
}
