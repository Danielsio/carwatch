import {
  Bell,
  TrendingDown,
  SlidersHorizontal,
  Bookmark,
  BarChart3,
  Sparkles,
  Layers,
  CalendarSync,
} from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { FadeUp } from "./FadeUp";

const features = [
  {
    icon: Layers,
    title: "מעקב אוטומטי",
    desc: "סריקה רציפה של Yad2 — תמונה מלאה של השוק, בלי ריענון ידני.",
    color: "text-chart-4",
    bg: "bg-chart-4/10",
    border: "border-chart-4/20",
  },
  {
    icon: Bell,
    title: "התראות בזמן אמת",
    desc: "מודעה חדשה שנכנסת לטווח שלך? תקבל התראה בממשק ובערוץ ההודעות שלך תוך דקות — קונים ראשונים נוגעים קודם במחיר.",
    color: "text-primary",
    bg: "bg-primary/10",
    border: "border-primary/20",
  },
  {
    icon: Sparkles,
    title: "Smart Match Score",
    desc: "ציון 0–10 לכל מודעה לפי מחיר, ק״מ, שנה, יד ועוד — המודעות הכי רלוונטיות עולות למעלה כדי שלא תפספס עסקה טובה.",
    color: "text-primary",
    bg: "bg-primary/10",
    border: "border-primary/35",
    highlight: true,
  },
  {
    icon: BarChart3,
    title: "ניתוח שוק ומגמות",
    desc: "הבן את טווח המחירים, כמות המודעות והפוזיציה של כל רכב ביחס לשוק — החלטות רגועות ומבוססות נתונים.",
    color: "text-chart-2",
    bg: "bg-chart-2/10",
    border: "border-chart-2/20",
  },
  {
    icon: Bookmark,
    title: "חיפושים שמורים",
    desc: "נהל עשרות חיפושים במקביל: השהה, חדש או מחק — כל פרופיל עם פילטרים משלו למציאת הרכב המדויק למשפחה, לעבודה או לפרויקט.",
    color: "text-chart-5",
    bg: "bg-chart-5/10",
    border: "border-chart-5/20",
  },
  {
    icon: CalendarSync,
    title: "סיכום יומי (digest)",
    desc: "קיבלו צפייה מהירה על מה שנוסף מאז אתמול — מושלם למי שרוצה לעקוב בלי הצטברות התראות ספאם לאורך היום.",
    color: "text-warning",
    bg: "bg-warning/10",
    border: "border-warning/25",
  },
  {
    icon: TrendingDown,
    title: "מעקב מחירים וירידות",
    desc: "מודיעים כשמחיר יורד או כשמודעה חוזרת עם הצעה אגרסיבית יותר — ניהול משא ומתן חכם מהספה.",
    color: "text-success",
    bg: "bg-success/10",
    border: "border-success/20",
  },
  {
    icon: SlidersHorizontal,
    title: "פילטרים ואוטומציה",
    desc: "שנה, דגם, תקציב, ק״מ, יד ועוד — הגדר פעם אחת ו-CarWatch ממשיכה לסרוק ברקע ומזהה שינויים, בלי ריענון ידני.",
    color: "text-chart-4",
    bg: "bg-chart-4/10",
    border: "border-chart-4/15",
  },
];

export function FeaturesSection() {
  return (
    <section id="features" className="scroll-mt-28 px-4 py-24 sm:px-6">
      <div className="mx-auto max-w-5xl">
        <FadeUp>
          <div className="mb-16 text-center">
            <span className="mb-3 block text-xs font-semibold tracking-widest text-primary uppercase">
              יכולות
            </span>
            <h2 className="mb-4 text-3xl font-bold text-foreground md:text-4xl">
              פלטפורמה מלאה למציאת רכב
            </h2>
            <p className="mx-auto max-w-2xl text-base leading-relaxed text-muted-foreground">
              מאיסוף מודעות ועד דירוג חכם והתראות — הכל במקום אחד, מותאם לשוק הרכבים
              הישראלי ולעבודה ב-RTL.
            </p>
          </div>
        </FadeUp>

        <div className="grid grid-cols-1 gap-4 sm:gap-5 md:grid-cols-2 xl:grid-cols-4">
          {features.map((f, i) => {
            const Icon = f.icon;
            return (
              <FadeUp key={f.title} delay={i * 0.05}>
                <Card
                  className={`card-hover group relative flex h-full flex-col overflow-hidden ${f.highlight ? "border-primary/40 ring-1 ring-primary/10" : ""}`}
                >
                  {f.highlight && (
                    <div className="pointer-events-none absolute inset-0 bg-gradient-to-br from-primary/[0.06] via-transparent to-purple-500/[0.04]" />
                  )}
                  <CardContent className="relative flex h-full flex-col p-6">
                    <div
                      className={`mb-4 flex h-12 w-12 items-center justify-center rounded-2xl ${f.bg} transition-transform duration-300 group-hover:scale-110`}
                    >
                      <Icon size={22} className={f.color} aria-hidden />
                    </div>
                    <h3 className="mb-2 text-base font-bold text-foreground md:text-lg">
                      {f.title}
                    </h3>
                    <p className="text-sm leading-relaxed text-muted-foreground">
                      {f.desc}
                    </p>
                  </CardContent>
                </Card>
              </FadeUp>
            );
          })}
        </div>
      </div>
    </section>
  );
}
