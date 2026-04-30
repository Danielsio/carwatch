import * as React from "react";
import { cn } from "@/lib/utils";

export type InputProps = React.InputHTMLAttributes<HTMLInputElement> & {
  error?: boolean;
};

const inputStyles =
  "w-full bg-card border rounded-xl px-4 py-3 text-sm outline-none focus:border-primary focus:ring-2 focus:ring-primary/20 placeholder:text-muted-foreground transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed";

export const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, error, ...props }, ref) => (
    <input
      ref={ref}
      className={cn(
        inputStyles,
        error ? "border-destructive/60" : "border-border",
        className,
      )}
      {...props}
    />
  ),
);

Input.displayName = "Input";
