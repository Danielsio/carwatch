import { Card, CardContent } from "@/components/ui/card";
import { FadeUp } from "./FadeUp";

const stats = [
  {
    value: "200+",
    label: "מודעות נסרקות בכל סריקה",
  },
  {
    value: "15 דק'",
    label: "בין סריקות",
  },
  {
    value: "24/7",
    label: "מעקב רציף",
  },
  {
    value: "0₪",
    label: "חינם לגמרי",
  },
];

export function StatsSection() {
  return (
    <section
      id="stats"
      className="scroll-mt-28 px-4 py-24 sm:px-6"
    >
      <div className="relative mx-auto max-w-5xl">

        <FadeUp>
          <div className="mb-14 text-center">
            <span className="mb-3 block text-xs font-semibold tracking-widest text-primary uppercase">
              במספרים
            </span>
            <h2 className="text-3xl font-bold text-foreground md:text-4xl">
              הנתונים מדברים
            </h2>
            <p className="mx-auto mt-3 max-w-lg text-sm text-muted-foreground md:text-base">
              CarWatch עובדת ברקע מסביב לשעון כדי שלא תפספס אף עסקה
            </p>
          </div>
        </FadeUp>

        <div className="relative grid grid-cols-2 gap-3 sm:gap-4 md:grid-cols-4">
          {stats.map((s, i) => (
            <FadeUp key={s.label} delay={i * 0.1}>
              <Card className="card-hover group text-center">
                <CardContent className="p-6">
                  <p className="mb-2 text-3xl font-extrabold tabular-nums text-primary md:text-4xl">
                    {s.value}
                  </p>
                  <p className="text-sm leading-relaxed text-muted-foreground">
                    {s.label}
                  </p>
                </CardContent>
              </Card>
            </FadeUp>
          ))}
        </div>
      </div>
    </section>
  );
}
