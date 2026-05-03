import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { motion } from "motion/react";
import { Activity, Clock, Search, Sparkles } from "lucide-react";
import { useInView } from "@/hooks/useInView";

/** Illustrative counters for marketing; replace when real metrics exist. */
const stats: {
  value: number;
  suffix: string;
  label: string;
  decimals: number;
  icon: LucideIcon;
}[] = [
  {
    value: 12000,
    suffix: "+",
    label: "מודעות נסרקות מדי יום",
    decimals: 0,
    icon: Search,
  },
  {
    value: 98,
    suffix: "%",
    label: "מהמשתמשים מוצאים רכב תוך שבועיים",
    decimals: 0,
    icon: Sparkles,
  },
  {
    value: 3.5,
    suffix: "K",
    label: "חיפושים שמורים פעילים",
    decimals: 1,
    icon: Activity,
  },
  {
    value: 4,
    suffix: " דק׳",
    label: "זמן ממוצע מפרסום לקבלת התראה",
    decimals: 0,
    icon: Clock,
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
    <section
      id="stats"
      ref={ref}
      className="scroll-mt-28 px-4 py-24 sm:px-6"
    >
      <div className="relative mx-auto max-w-5xl">
        <div className="pointer-events-none absolute -top-24 start-1/2 h-64 w-[min(90%,28rem)] -translate-x-1/2 rounded-full bg-primary/8 blur-[100px]" />
        <div className="pointer-events-none absolute -bottom-16 -start-20 h-40 w-40 rounded-full bg-purple-500/10 blur-[80px]" />

        <FadeUp>
          <div className="mb-14 text-center">
            <span className="mb-3 block text-xs font-semibold tracking-widest text-primary uppercase">
              במספרים
            </span>
            <h2 className="text-3xl font-bold text-foreground md:text-4xl">
              בנוי לקצב השוק המקומי
            </h2>
            <p className="mx-auto mt-3 max-w-lg text-sm text-muted-foreground md:text-base">
              נתונים להמחשה — בקרוב נחבר מדדים חיים כשיהיו זמינים במוצר.
            </p>
          </div>
        </FadeUp>

        <div className="relative grid grid-cols-2 gap-3 sm:gap-4 md:grid-cols-4">
          {stats.map((s, i) => {
            const Icon = s.icon;
            return (
              <FadeUp key={s.label} delay={i * 0.1}>
                <div className="group card-hover relative overflow-hidden rounded-2xl border border-border/80 bg-card p-5 text-center shadow-[0_1px_0_0_rgba(255,255,255,0.04)_inset] dark:shadow-[0_1px_0_0_rgba(255,255,255,0.06)_inset]">
                  <div className="pointer-events-none absolute -end-6 -top-6 h-24 w-24 rounded-full bg-primary/10 opacity-70 blur-2xl transition-opacity group-hover:opacity-100" />
                  <div className="relative mx-auto mb-3 flex h-10 w-10 items-center justify-center rounded-xl border border-border/50 bg-secondary/50">
                    <Icon className="h-[18px] w-[18px] text-primary" aria-hidden />
                  </div>
                  <div className="relative mb-2 text-2xl font-bold tabular-nums md:text-3xl">
                    <span className="gradient-text">
                      <Counter
                        target={s.value}
                        suffix={s.suffix}
                        decimals={s.decimals}
                        active={inView}
                      />
                    </span>
                  </div>
                  <p className="relative text-[11px] leading-snug text-muted-foreground sm:text-xs">
                    {s.label}
                  </p>
                </div>
              </FadeUp>
            );
          })}
        </div>
      </div>
    </section>
  );
}
