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
        "relative flex flex-col items-center justify-center gap-4 rounded-2xl border border-border/50 bg-card/50 px-6 py-20 text-center overflow-hidden",
        className,
      )}
    >
      <div
        className="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_60%_40%_at_50%_40%,rgba(59,130,246,0.06),transparent)]"
        aria-hidden
      />
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
