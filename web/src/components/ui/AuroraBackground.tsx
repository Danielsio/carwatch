import { cn } from "@/lib/utils";

export type AuroraBackgroundProps = {
  /** "app" = subtle ambient wash, "hero" = bolder landing/auth treatment. */
  variant?: "app" | "hero";
  className?: string;
};

/**
 * Fixed, GPU-friendly animated gradient mesh that sits behind all content.
 * Colors are driven by the `--aurora-*` CSS vars, so it adapts to light/dark
 * automatically. Motion is frozen via the reduced-motion media query in
 * index.css. Purely decorative — hidden from assistive tech.
 */
export function AuroraBackground({
  variant = "app",
  className,
}: AuroraBackgroundProps) {
  const hero = variant === "hero";

  return (
    <div
      aria-hidden
      className={cn(
        "pointer-events-none fixed inset-0 -z-10 overflow-hidden",
        className,
      )}
      data-variant={variant}
    >
      <div
        className={cn(
          "aurora-blob animate-aurora-a",
          hero ? "opacity-90" : "opacity-60",
        )}
        style={{
          insetInlineStart: "-8%",
          top: "-12%",
          width: "56vw",
          height: "56vw",
          background:
            "radial-gradient(circle at center, var(--aurora-1), transparent 70%)",
        }}
      />
      <div
        className={cn(
          "aurora-blob animate-aurora-b",
          hero ? "opacity-90" : "opacity-55",
        )}
        style={{
          insetInlineEnd: "-10%",
          top: "8%",
          width: "48vw",
          height: "48vw",
          background:
            "radial-gradient(circle at center, var(--aurora-2), transparent 70%)",
        }}
      />
      <div
        className={cn(
          "aurora-blob animate-aurora-c",
          hero ? "opacity-80" : "opacity-50",
        )}
        style={{
          insetInlineStart: "20%",
          bottom: "-18%",
          width: "52vw",
          height: "52vw",
          background:
            "radial-gradient(circle at center, var(--aurora-3), transparent 72%)",
        }}
      />
      {/* Focus vignette — keeps edges calm and content legible */}
      <div className="absolute inset-0 bg-[radial-gradient(ellipse_80%_60%_at_50%_40%,transparent,var(--color-background)_92%)]" />
    </div>
  );
}
