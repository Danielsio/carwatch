import type { ReactNode } from "react";
import { motion } from "motion/react";
import { X, Check } from "lucide-react";
import { useInView } from "@/hooks/useInView";

const problems = [
  "רצת לאתר יד2 כל שעה כדי לא לפספס הזדמנות",
  "מצאת רכב מצוין — אבל הוא כבר נמכר אתמול",
  "שילמת יותר כי לא ידעת שהמחיר ירד",
  "בזבזת שעות על מיון ידני של מאות מודעות",
];

const solutions = [
  "CarWatch סורקת אוטומטית — אתה ישן בשקט",
  "קבל התראה תוך דקות מרגע פרסום המודעה",
  "מעקב מחירים חכם — תדע כשמחיר יורד",
  "פילטרים מדויקים — רק מה שרלוונטי אליך",
];

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
      initial={{ opacity: 0, y: 24 }}
      animate={inView ? { opacity: 1, y: 0 } : {}}
      transition={{ delay, duration: 0.6 }}
    >
      {children}
    </motion.div>
  );
}

export function ProblemSolution() {
  return (
    <section className="relative overflow-hidden px-6 py-24">
      <div className="pointer-events-none absolute inset-0 bg-gradient-to-b from-transparent via-secondary/20 to-transparent" />
      <div className="mx-auto max-w-5xl">
        <FadeUp>
          <div className="mb-16 text-center">
            <span className="mb-3 block text-xs font-semibold tracking-widest text-primary uppercase">
              הבעיה שפתרנו
            </span>
            <h2 className="text-3xl font-bold text-foreground md:text-4xl">
              חיפוש רכב זה ניהול משרה נוספת
            </h2>
          </div>
        </FadeUp>

        <div className="grid gap-6 md:grid-cols-2">
          <FadeUp delay={0.1}>
            <div className="rounded-3xl border border-destructive/15 bg-destructive/5 p-7">
              <div className="mb-6 flex items-center gap-2.5">
                <div className="flex h-8 w-8 items-center justify-center rounded-xl bg-destructive/15">
                  <X size={16} className="text-destructive" />
                </div>
                <h3 className="text-base font-bold text-foreground">בלי CarWatch</h3>
              </div>
              <ul className="space-y-3.5">
                {problems.map((p, i) => (
                  <li
                    key={i}
                    className="flex items-start gap-3 text-sm text-muted-foreground"
                  >
                    <X
                      size={14}
                      className="mt-0.5 flex-shrink-0 text-destructive/70"
                    />
                    {p}
                  </li>
                ))}
              </ul>
            </div>
          </FadeUp>

          <FadeUp delay={0.2}>
            <div className="relative overflow-hidden rounded-3xl border border-success/20 bg-success/5 p-7">
              <div className="absolute top-0 right-0 h-32 w-32 translate-x-1/2 -translate-y-1/2 rounded-full bg-success/5" />
              <div className="mb-6 flex items-center gap-2.5">
                <div className="flex h-8 w-8 items-center justify-center rounded-xl bg-success/15">
                  <Check size={16} className="text-success" />
                </div>
                <h3 className="text-base font-bold text-foreground">עם CarWatch</h3>
              </div>
              <ul className="space-y-3.5">
                {solutions.map((s, i) => (
                  <li key={i} className="flex items-start gap-3 text-sm text-foreground">
                    <Check size={14} className="mt-0.5 flex-shrink-0 text-success" />
                    {s}
                  </li>
                ))}
              </ul>
            </div>
          </FadeUp>
        </div>
      </div>
    </section>
  );
}
