import { Link } from "react-router";
import { cn } from "@/lib/utils";

export type SectionHeaderProps = {
  title: string;
  linkTo?: string;
  linkLabel?: string;
  className?: string;
};

export function SectionHeader({
  title,
  linkTo,
  linkLabel = "הצג הכל",
  className,
}: SectionHeaderProps) {
  return (
    <div className={cn("flex items-center justify-between", className)}>
      <h2 className="text-base font-semibold text-foreground">{title}</h2>
      {linkTo && (
        <Link
          to={linkTo}
          className="text-sm font-medium text-primary transition-colors hover:text-primary/80"
        >
          {linkLabel}
        </Link>
      )}
    </div>
  );
}
