import { useEffect, useRef, useState } from "react";
import { cn } from "@/lib/utils";

export function NumberTicker({
  value,
  direction = "up",
  duration = 800,
  className,
}: {
  value: number;
  direction?: "up" | "down";
  duration?: number;
  className?: string;
}) {
  const [displayValue, setDisplayValue] = useState(direction === "up" ? 0 : value);
  const startTime = useRef<number | null>(null);
  const rafId = useRef<number>(0);
  const startValue = useRef(direction === "up" ? 0 : value);
  const targetValue = useRef(value);

  useEffect(() => {
    targetValue.current = value;
    startValue.current = displayValue;
    startTime.current = null;

    function tick(now: number) {
      if (startTime.current === null) startTime.current = now;
      const elapsed = now - startTime.current;
      const progress = Math.min(elapsed / duration, 1);
      const eased = 1 - Math.pow(1 - progress, 3);
      const current = Math.round(
        startValue.current + (targetValue.current - startValue.current) * eased,
      );
      setDisplayValue(current);
      if (progress < 1) {
        rafId.current = requestAnimationFrame(tick);
      }
    }

    rafId.current = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(rafId.current);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value, duration]);

  return (
    <span className={cn("tabular-nums", className)}>
      {displayValue.toLocaleString("he-IL")}
    </span>
  );
}
