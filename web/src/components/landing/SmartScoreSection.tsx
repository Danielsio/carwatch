import type { ReactNode } from "react";
import { motion } from "motion/react";
import { Sparkles, TrendingUp, Gauge, Calendar, Users } from "lucide-react";
import { useInView } from "@/hooks/useInView";
import {
  scoreListingAgainstSearch,
  scoreColor,
  scoreBgColor,
  scoreLabel,
  scoreBarColor,
  type DemoListingInput,
} from "@/lib/scoringAlgorithm";

const demoSearch = {
  year_min: 2018,
  year_max: 2022,
  price_max: 130000,
  mileage_max: 120000,
  hand_max: 2,
};

const demoListings: DemoListingInput[] = [
  {
    title: "טויוטה קורולה 2021",
    year: 2021,
    price: 119000,
    mileage: 38000,
    hand: 1,
    city: "תל אביב",
  },
  {
    title: "יונדאי i35 2020",
    year: 2020,
    price: 108000,
    mileage: 72000,
    hand: 2,
    city: "חיפה",
  },
  {
    title: "קיה ספורטאג׳ 2019",
    year: 2019,
    price: 143000,
    mileage: 55000,
    hand: 1,
    city: "ירושלים",
  },
  {
    title: "מזדה 3 2018",
    year: 2018,
    price: 91000,
    mileage: 148000,
    hand: 3,
    city: 'ראשל"צ',
  },
];

const factors = [
  {
    icon: TrendingUp,
    label: "מחיר מול תקציב",
    desc: "ציון גבוה לרכבים שנמצאים מתחת לתקציב שהגדרת",
    weight: "30%",
  },
  {
    icon: Gauge,
    label: "קילומטרז",
    desc: "פחות ק\"מ = עייפות פחותה = ציון גבוה יותר",
    weight: "30%",
  },
  {
    icon: Calendar,
    label: "שנת ייצור",
    desc: "רכבים חדשים יותר בטווח המבוקש מקבלים עדיפות",
    weight: "20%",
  },
  {
    icon: Users,
    label: "מספר ידיים",
    desc: "בעלים ראשון שווה ציון גבוה משמעותית",
    weight: "20%",
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

function DemoCard({
  listing,
  delay,
}: {
  listing: DemoListingInput;
  delay: number;
}) {
  const { ref, inView } = useInView();
  const { score, breakdown } = scoreListingAgainstSearch(listing, demoSearch);
  const color = scoreColor(score);
  const bg = scoreBgColor(score);
  const label = scoreLabel(score);
  const bar = scoreBarColor(score);

  return (
    <motion.div
      ref={ref}
      initial={{ opacity: 0, x: 20 }}
      animate={inView ? { opacity: 1, x: 0 } : {}}
      transition={{ delay, duration: 0.5 }}
      className="flex items-center gap-4 rounded-2xl border border-border bg-card p-4"
    >
      <div
        className={`flex h-14 w-14 flex-shrink-0 flex-col items-center justify-center rounded-2xl border-2 font-bold ${bg} ${color}`}
      >
        <span className="text-xl leading-none">{score.toFixed(1)}</span>
        <span className="mt-0.5 text-[9px] opacity-60">/10</span>
      </div>

      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-semibold text-foreground">
          {listing.title}
        </p>
        <p className={`mt-0.5 text-xs font-medium ${color}`}>{label}</p>
        <div className="mt-2 flex gap-2">
          {(
            [
              { key: "price" as const, label: "מחיר" },
              { key: "mileage" as const, label: "ק״מ" },
              { key: "year" as const, label: "שנה" },
              { key: "hand" as const, label: "יד" },
            ] as const
          ).map((f) => (
            <div key={f.key} className="flex flex-1 flex-col items-center gap-0.5">
              <div className="h-1 w-full overflow-hidden rounded-full bg-secondary">
                <div
                  className={`h-full rounded-full ${bar}`}
                  style={{ width: `${breakdown[f.key]}%` }}
                />
              </div>
              <span className="text-[9px] text-muted-foreground">{f.label}</span>
            </div>
          ))}
        </div>
      </div>

      <div className="flex-shrink-0 text-sm font-bold text-primary">
        ₪{listing.price.toLocaleString("he-IL")}
      </div>
    </motion.div>
  );
}

export function SmartScoreSection() {
  return (
    <section className="relative overflow-hidden px-6 py-24">
      <div className="pointer-events-none absolute inset-0">
        <div className="absolute top-1/2 left-1/2 h-[300px] w-[500px] -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary/6 blur-[100px]" />
      </div>

      <div className="relative mx-auto max-w-5xl">
        <FadeUp>
          <div className="mb-16 text-center">
            <span className="mb-4 inline-flex items-center gap-2 rounded-full border border-primary/20 bg-primary/10 px-4 py-1.5 text-xs font-semibold tracking-widest text-primary uppercase">
              <Sparkles size={12} />
              חדש · Smart Match Score
            </span>
            <h2 className="mb-4 text-3xl font-bold text-foreground md:text-4xl">
              לא רק לסנן —
              <span className="gradient-text"> לדרג חכם.</span>
            </h2>
            <p className="mx-auto max-w-xl text-base leading-relaxed text-muted-foreground">
              CarWatch מחשבת ציון 0–10 לכל מודעה על בסיס האלגוריתם שלנו — כך שתמיד תראה קודם את העסקאות הכי טובות עבורך.
            </p>
          </div>
        </FadeUp>

        <div className="grid items-start gap-10 md:grid-cols-2">
          <div>
            <FadeUp>
              <div className="mb-4 flex items-center gap-2">
                <span className="text-xs font-medium text-muted-foreground">
                  דוגמה חיה — חיפוש: 2018–2022, עד ₪130K
                </span>
              </div>
            </FadeUp>
            <div className="space-y-3">
              {demoListings.map((l, i) => (
                <DemoCard key={l.title} listing={l} delay={i * 0.1} />
              ))}
            </div>
          </div>

          <div className="space-y-4">
            <FadeUp>
              <h3 className="mb-5 text-base font-bold text-foreground">
                מה נכנס לחישוב?
              </h3>
            </FadeUp>
            {factors.map((f, i) => {
              const Icon = f.icon;
              return (
                <FadeUp key={f.label} delay={0.05 + i * 0.07}>
                  <div className="flex items-start gap-3 rounded-xl border border-border bg-card p-4">
                    <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-xl bg-primary/10">
                      <Icon size={16} className="text-primary" />
                    </div>
                    <div className="flex-1">
                      <div className="mb-0.5 flex items-center justify-between">
                        <span className="text-sm font-semibold text-foreground">
                          {f.label}
                        </span>
                        <span className="rounded-full bg-primary/10 px-2 py-0.5 text-xs font-bold text-primary">
                          {f.weight}
                        </span>
                      </div>
                      <p className="text-xs leading-relaxed text-muted-foreground">
                        {f.desc}
                      </p>
                    </div>
                  </div>
                </FadeUp>
              );
            })}

            <FadeUp delay={0.4}>
              <div className="mt-2 rounded-xl border border-primary/15 bg-primary/5 p-4">
                <p className="text-xs leading-relaxed text-primary/90">
                  <span className="font-bold">ללא סף חריף.</span> כל הגורמים
                  משתמשים בחישוב רציף עם דעיכה חלקה — כך שהציון תמיד מרגיש הגיוני
                  ומוסבר.
                </p>
              </div>
            </FadeUp>
          </div>
        </div>
      </div>
    </section>
  );
}
