import type { ReactNode } from "react";
import {
  Bell,
  TrendingDown,
  SlidersHorizontal,
  Zap,
  Bookmark,
  BarChart3,
  Sparkles,
} from "lucide-react";
import { motion } from "motion/react";
import { useInView } from "@/hooks/useInView";

const features = [
  {
    icon: Bell,
    title: "התראות מיידיות",
    desc: "קבל התראה בטלגרם או בממשק תוך דקות מרגע שמודעה תואמת לחיפוש שלך.",
    color: "text-primary",
    bg: "bg-primary/10",
    border: "border-primary/20",
  },
  {
    icon: TrendingDown,
    title: "מעקב מחירים",
    desc: "CarWatch עוקבת אחר שינויי מחיר — תדע מיד כשמוכר מוריד מחיר.",
    color: "text-success",
    bg: "bg-success/10",
    border: "border-success/20",
  },
  {
    icon: SlidersHorizontal,
    title: "פילטרים חכמים",
    desc: "סנן לפי יצרן, דגם, שנה, מחיר, קילומטרז, יד ועוד — מדויק לצרכים שלך.",
    color: "text-chart-4",
    bg: "bg-chart-4/10",
    border: "border-chart-4/20",
  },
  {
    icon: Zap,
    title: "סריקה אוטומטית",
    desc: "המערכת סורקת את Yad2 ו-WinWin במקביל — ללא פעולה ידנית מצדך.",
    color: "text-warning",
    bg: "bg-warning/10",
    border: "border-warning/20",
  },
  {
    icon: Bookmark,
    title: "מועדפים וארכיון",
    desc: "שמור מודעות מעניינות לבדיקה מאוחרת, ועיין בהיסטוריית כל מה שנמצא.",
    color: "text-chart-5",
    bg: "bg-chart-5/10",
    border: "border-chart-5/20",
  },
  {
    icon: BarChart3,
    title: "ניתוח שוק",
    desc: "ראה כמה מודעות קיימות, מה הטווח הממוצע, ואיפה המחיר שאתה רואה ממוקם.",
    color: "text-chart-2",
    bg: "bg-chart-2/10",
    border: "border-chart-2/20",
  },
  {
    icon: Sparkles,
    title: "Smart Match Score",
    desc: "אלגוריתם חכם מדרג כל מודעה 0–10 לפי מחיר, ק\"מ, שנה ומצב — כדי שתראה קודם את הכי טוב.",
    color: "text-primary",
    bg: "bg-primary/10",
    border: "border-primary/30",
    highlight: true,
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
      initial={{ opacity: 0, y: 28 }}
      animate={inView ? { opacity: 1, y: 0 } : {}}
      transition={{ delay, duration: 0.6 }}
    >
      {children}
    </motion.div>
  );
}

export function FeaturesSection() {
  return (
    <section id="features" className="scroll-mt-24 px-6 py-24">
      <div className="mx-auto max-w-5xl">
        <FadeUp>
          <div className="mb-16 text-center">
            <span className="mb-3 block text-xs font-semibold tracking-widest text-primary uppercase">
              יכולות
            </span>
            <h2 className="mb-4 text-3xl font-bold text-foreground md:text-4xl">
              כל מה שצריך. כלום מיותר.
            </h2>
            <p className="mx-auto max-w-xl text-base text-muted-foreground">
              כל הכלים שאתה צריך כדי למצוא את הרכב הבא שלך מהר יותר וחכם יותר.
            </p>
          </div>
        </FadeUp>

        <div className="grid grid-cols-1 gap-5 md:grid-cols-2 lg:grid-cols-3">
          {features.map((f, i) => {
            const Icon = f.icon;
            return (
              <FadeUp key={f.title} delay={i * 0.07}>
                <div
                  className={`card-hover group relative h-full overflow-hidden rounded-2xl border ${f.border} bg-card p-6 ${f.highlight ? "border-primary/40" : ""}`}
                >
                  {f.highlight ? (
                    <div className="pointer-events-none absolute inset-0 bg-primary/3" />
                  ) : null}
                  <div
                    className={`mb-4 flex h-11 w-11 items-center justify-center rounded-2xl ${f.bg} transition-transform group-hover:scale-110`}
                  >
                    <Icon size={20} className={f.color} />
                  </div>
                  <h3 className="mb-2 font-bold text-foreground">{f.title}</h3>
                  <p className="text-sm leading-relaxed text-muted-foreground">
                    {f.desc}
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
