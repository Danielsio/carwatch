import type { ReactNode } from "react";
import { useEffect, useRef, useState } from "react";
import { motion } from "motion/react";
import { useInView } from "@/hooks/useInView";

/** Illustrative counters for marketing; replace when real metrics exist. */
const stats = [
  { value: 12000, suffix: "+", label: "מודעות נסרקות מדי יום", decimals: 0 },
  {
    value: 98,
    suffix: "%",
    label: "מהמשתמשים מוצאים רכב תוך שבועיים",
    decimals: 0,
  },
  {
    value: 3.5,
    suffix: "K",
    label: "חיפושים שמורים פעילים",
    decimals: 1,
  },
  {
    value: 4,
    suffix: " דק׳",
    label: "זמן ממוצע מפרסום לקבלת התראה",
    decimals: 0,
  },
];

function Counter({
  target,
  suffix,
  decimals,
  active,
}: {
  target: number;
  suffix: string;
  decimals: number;
  active: boolean;
}) {
  const [val, setVal] = useState(0);
  const rafRef = useRef<number | null>(null);

  useEffect(() => {
    if (!active) return;
    const duration = 1800;
    const start = performance.now();
    const tick = (now: number) => {
      const elapsed = now - start;
      const progress = Math.min(elapsed / duration, 1);
      const eased = 1 - Math.pow(1 - progress, 3);
      setVal(eased * target);
      if (progress < 1) {
        rafRef.current = requestAnimationFrame(tick);
      }
    };
    rafRef.current = requestAnimationFrame(tick);
    return () => {
      if (rafRef.current != null) cancelAnimationFrame(rafRef.current);
    };
  }, [active, target]);

  const display =
    decimals > 0
      ? val.toFixed(decimals)
      : Math.floor(val).toLocaleString("he-IL");
  return (
    <span>
      {display}
      {suffix}
    </span>
  );
}

function FadeUp({
  children,
  delay = 0,
}: {
  children: ReactNode;
  delay?: number;
}) {
  const { ref, inView } = useInView();
  return (
    <motion.div
      ref={ref}
      initial={{ opacity: 0, y: 20 }}
      animate={inView ? { opacity: 1, y: 0 } : {}}
      transition={{ delay, duration: 0.5 }}
    >
      {children}
    </motion.div>
  );
}

export function StatsSection() {
  const { ref, inView } = useInView(0.3);

  return (
    <section id="stats" ref={ref} className="scroll-mt-24 px-6 py-24">
      <div className="mx-auto max-w-5xl">
        <FadeUp>
          <div className="mb-14 text-center">
            <span className="mb-3 block text-xs font-semibold tracking-widest text-primary uppercase">
              במספרים
            </span>
            <h2 className="text-3xl font-bold text-foreground md:text-4xl">
              פלטפורמה שעובדת
            </h2>
          </div>
        </FadeUp>

        <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
          {stats.map((s, i) => (
            <FadeUp key={s.label} delay={i * 0.1}>
              <div className="card-hover rounded-2xl border border-border bg-card p-6 text-center">
                <div className="gradient-text mb-2 text-3xl font-bold tabular-nums md:text-4xl">
                  <Counter
                    target={s.value}
                    suffix={s.suffix}
                    decimals={s.decimals}
                    active={inView}
                  />
                </div>
                <p className="text-xs leading-relaxed text-muted-foreground">
                  {s.label}
                </p>
              </div>
            </FadeUp>
          ))}
        </div>
      </div>
    </section>
  );
}
