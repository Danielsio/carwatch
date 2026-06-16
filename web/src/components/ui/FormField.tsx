import type { ReactNode } from "react";
import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";

export type FormFieldProps = {
  label: string;
  htmlFor?: string;
  hint?: string;
  error?: string;
  children: ReactNode;
  className?: string;
};

export function FormField({
  label,
  htmlFor,
  hint,
  error,
  children,
  className,
}: FormFieldProps) {
  const errorId = htmlFor ? `${htmlFor}-error` : undefined;
  const hintId = htmlFor ? `${htmlFor}-hint` : undefined;
  const describedBy = [error && errorId, hint && hintId].filter(Boolean).join(" ") || undefined;

  return (
    <div className={cn("space-y-1.5 dir-rtl", className)} data-describedby={describedBy}>
      <Label htmlFor={htmlFor}>{label}</Label>
      {children}
      {error ? (
        <p id={errorId} className="text-xs text-destructive" role="alert">
          {error}
        </p>
      ) : null}
      {hint && !error ? (
        <p id={hintId} className="text-xs text-muted-foreground">{hint}</p>
      ) : null}
    </div>
  );
}
