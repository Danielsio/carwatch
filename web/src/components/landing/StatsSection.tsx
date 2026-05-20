import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { motion } from "motion/react";
import { Bell, Clock, Search, Shield } from "lucide-react";
import { useInView } from "@/hooks/useInView";

const features: {
  label: string;
  description: string;
  icon: LucideIcon;
}[] = [
  {
    label: "סריקה רציפה",
    description: "Yad2 נסרק כל 30 דקות, אוטומטית",
    icon: Search,
  },
  {
    label: "התראה מיידית",
    description: "מודעה חדשה? תקבל התראה בטלגרם תוך דקות",
    icon: Bell,
  },
  {
    label: "הגדרה ב-2 דקות",
    description: "בחר יצרן, דגם, תקציב — והמערכת עובדת בשבילך",
    icon: Clock,
  },
  {
    label: "ציון התאמה חכם",
    description: "כל מודעה מקבלת ציון 0-10 לפי הקריטריונים שלך",
    icon: Shield,
  },
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
      initial={{ opacity: 0, y: 20 }}
      animate={inView ? { opacity: 1, y: 0 } : {}}
      transition={{ delay, duration: 0.5 }}
    >
      {children}
    </motion.div>
  );
}

export function StatsSection() {
  return (
    <section
      id="stats"
      className="scroll-mt-28 px-4 py-24 sm:px-6"
    >
      <div className="relative mx-auto max-w-5xl">
        <div className="pointer-events-none absolute -top-24 start-1/2 h-64 w-[min(90%,28rem)] -translate-x-1/2 rounded-full bg-primary/8 blur-[100px]" />

        <FadeUp>
          <div className="mb-14 text-center">
            <span className="mb-3 block text-xs font-semibold tracking-widest text-primary uppercase">
              איך זה עובד
            </span>
            <h2 className="text-3xl font-bold text-foreground md:text-4xl">
              מעקב אוטומטי, תוצאות מיידיות
            </h2>
            <p className="mx-auto mt-3 max-w-lg text-sm text-muted-foreground md:text-base">
              המערכת עובדת ברקע — אתה מקבל רק את מה שרלוונטי לך
            </p>
          </div>
        </FadeUp>

        <div className="relative grid grid-cols-2 gap-3 sm:gap-4 md:grid-cols-4">
          {features.map((f, i) => {
            const Icon = f.icon;
            return (
              <FadeUp key={f.label} delay={i * 0.1}>
                <div className="group card-hover relative overflow-hidden rounded-2xl border border-border/80 bg-card p-5 text-center">
                  <div className="relative mx-auto mb-3 flex h-10 w-10 items-center justify-center rounded-xl border border-border/50 bg-secondary/50">
                    <Icon className="h-[18px] w-[18px] text-primary" aria-hidden />
                  </div>
                  <p className="relative mb-1.5 text-sm font-semibold text-foreground">
                    {f.label}
                  </p>
                  <p className="relative text-xs leading-relaxed text-muted-foreground">
                    {f.description}
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
