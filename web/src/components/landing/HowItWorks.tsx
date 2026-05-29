import { Search, Bell, Car } from "lucide-react";
import { FadeUp } from "./FadeUp";

const steps = [
  {
    num: "01",
    icon: Search,
    title: "הגדר חיפוש",
    desc: "בחר יצרן, דגם, טווח מחירים, שנה וקילומטרז. קח שתי דקות — פעם אחת.",
    color: "text-primary",
    bg: "bg-primary/10",
    glow: "shadow-primary/20",
  },
  {
    num: "02",
    icon: Bell,
    title: "קבל התראות",
    desc: "כשמופיעה מודעה תואמת — תקבל הודעה מיידית. אתה תמיד ראשון.",
    color: "text-warning",
    bg: "bg-warning/10",
    glow: "shadow-warning/20",
  },
  {
    num: "03",
    icon: Car,
    title: "קנה חכם",
    desc: "ראה את כל הפרטים, השווה למחיר שוק, ועשה עסקה מעמדת כוח.",
    color: "text-success",
    bg: "bg-success/10",
    glow: "shadow-success/20",
  },
];

export function HowItWorks() {
  return (
    <section id="how" className="relative scroll-mt-28 overflow-hidden px-4 py-24 sm:px-6">
      <div className="pointer-events-none absolute inset-0 bg-gradient-to-b from-transparent via-secondary/15 to-transparent" />
      <div className="relative mx-auto max-w-5xl">
        <FadeUp>
          <div className="mb-16 text-center">
            <span className="mb-3 block text-xs font-semibold tracking-widest text-primary uppercase">
              שלושה צעדים
            </span>
            <h2 className="text-3xl font-bold text-foreground md:text-4xl">
              פשוט כמו שנשמע
            </h2>
          </div>
        </FadeUp>

        <div className="relative grid gap-6 md:grid-cols-3">
          <div className="pointer-events-none absolute top-14 end-[16.5%] start-[16.5%] hidden h-px bg-gradient-to-l from-transparent via-border to-transparent md:block" dir="ltr" />

          {steps.map((step, i) => {
            const Icon = step.icon;
            return (
              <FadeUp key={step.num} delay={i * 0.15}>
                <div className="relative flex flex-col items-center text-center">
                  <div className="relative mb-6">
                    <div
                      className={`relative z-10 flex h-16 w-16 items-center justify-center rounded-2xl border border-border/50 ${step.bg} shadow-xl ${step.glow}`}
                    >
                      <Icon size={26} className={step.color} />
                    </div>
                    <span className="absolute -top-2 -end-2 z-20 flex h-6 w-6 items-center justify-center rounded-full border border-border bg-background text-[10px] font-bold text-muted-foreground">
                      {step.num}
                    </span>
                  </div>
                  <h3 className="mb-2 text-lg font-bold text-foreground">
                    {step.title}
                  </h3>
                  <p className="max-w-xs text-sm leading-relaxed text-muted-foreground">
                    {step.desc}
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
