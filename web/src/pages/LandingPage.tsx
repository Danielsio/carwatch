import { Link } from "react-router";
import {
  Search,
  Bell,
  TrendingDown,
  Shield,
  Zap,
  BarChart3,
  ArrowLeft,
  Star,
  Sun,
  Moon,
  Monitor,
} from "lucide-react";
import { useTheme } from "@/hooks/useTheme";
import { cn } from "@/lib/utils";

export function LandingPage() {
  return (
    <div className="min-h-screen bg-background transition-colors duration-300">
      <Header />
      <Hero />
      <Features />
      <HowItWorks />
      <Stats />
      <CTA />
      <Footer />
    </div>
  );
}

function Header() {
  const { theme, setTheme } = useTheme();

  const nextTheme = theme === "light" ? "dark" : theme === "dark" ? "system" : "light";
  const ThemeIcon = theme === "light" ? Sun : theme === "dark" ? Moon : Monitor;

  return (
    <header className="sticky top-0 z-50 border-b border-border/50 glass-heavy">
      <div className="mx-auto max-w-6xl px-4 sm:px-6 lg:px-8">
        <div className="flex h-16 items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl gradient-primary shadow-md">
              <span className="text-sm font-black text-white">CW</span>
            </div>
            <span className="text-xl font-bold tracking-tight">CarWatch</span>
          </div>

          <div className="flex items-center gap-3">
            <button
              onClick={() => setTheme(nextTheme)}
              className="flex h-9 w-9 items-center justify-center rounded-xl text-muted-foreground hover:bg-accent hover:text-foreground transition-all"
              aria-label="שנה מצב תצוגה"
            >
              <ThemeIcon className="h-4 w-4" />
            </button>
            <Link
              to="/"
              className="inline-flex items-center gap-2 rounded-xl gradient-primary px-5 py-2.5 text-sm font-semibold text-white shadow-md hover:shadow-lg hover:brightness-110 transition-all"
            >
              התחל עכשיו
              <ArrowLeft className="h-4 w-4" />
            </Link>
          </div>
        </div>
      </div>
    </header>
  );
}

function Hero() {
  return (
    <section className="relative overflow-hidden">
      <div className="absolute inset-0 gradient-hero opacity-5" />
      <div className="absolute top-20 -right-40 h-80 w-80 rounded-full bg-primary/10 blur-3xl" />
      <div className="absolute bottom-10 -left-40 h-60 w-60 rounded-full bg-purple-500/10 blur-3xl" />

      <div className="relative mx-auto max-w-6xl px-4 py-24 sm:px-6 sm:py-32 lg:px-8 lg:py-40 text-center">
        <div className="animate-slide-up">
          <div className="inline-flex items-center gap-2 rounded-full border border-border bg-card px-4 py-1.5 text-sm font-medium text-muted-foreground mb-8 shadow-sm">
            <Zap className="h-3.5 w-3.5 text-primary" />
            מוצא את הרכב המושלם בשבילך
          </div>

          <h1 className="text-4xl font-black tracking-tight sm:text-5xl lg:text-6xl leading-[1.1]">
            מעקב חכם אחרי
            <br />
            <span className="text-gradient">רכבים יד שנייה</span>
          </h1>

          <p className="mx-auto mt-6 max-w-2xl text-lg text-muted-foreground sm:text-xl leading-relaxed">
            CarWatch סורק את אתרי הרכב המובילים, מזהה עסקאות משתלמות ושולח
            לך התראות בזמן אמת. חסוך שעות של חיפוש ידני.
          </p>

          <div className="mt-10 flex flex-col sm:flex-row items-center justify-center gap-4">
            <Link
              to="/"
              className="inline-flex items-center gap-2.5 rounded-2xl gradient-primary px-8 py-4 text-base font-bold text-white shadow-lg hover:shadow-xl hover:brightness-110 transition-all"
            >
              התחל לחפש
              <ArrowLeft className="h-5 w-5" />
            </Link>
            <a
              href="#features"
              className="inline-flex items-center gap-2 rounded-2xl border border-border bg-card px-8 py-4 text-base font-semibold text-foreground hover:bg-accent transition-all shadow-sm"
            >
              איך זה עובד?
            </a>
          </div>
        </div>
      </div>
    </section>
  );
}

function Features() {
  const features = [
    {
      icon: Search,
      title: "חיפוש מותאם אישית",
      description:
        "הגדר חיפוש לפי יצרן, דגם, שנה, מחיר, קילומטראז' ועוד. CarWatch עושה את העבודה בשבילך.",
      gradient: "from-blue-500/10 to-cyan-500/10",
      iconColor: "text-blue-600 dark:text-blue-400",
      iconBg: "bg-blue-500/10 dark:bg-blue-500/20",
    },
    {
      icon: Bell,
      title: "התראות בזמן אמת",
      description:
        "קבל התראות מיידיות כשמתפרסמת מודעה חדשה שמתאימה לקריטריונים שלך. לא תפספס עסקה.",
      gradient: "from-purple-500/10 to-pink-500/10",
      iconColor: "text-purple-600 dark:text-purple-400",
      iconBg: "bg-purple-500/10 dark:bg-purple-500/20",
    },
    {
      icon: TrendingDown,
      title: "מעקב ירידות מחיר",
      description:
        "CarWatch עוקב אחרי שינויי מחירים ומיידע אותך כשמחיר יורד. זהה הזדמנויות לפני כולם.",
      gradient: "from-emerald-500/10 to-green-500/10",
      iconColor: "text-emerald-600 dark:text-emerald-400",
      iconBg: "bg-emerald-500/10 dark:bg-emerald-500/20",
    },
    {
      icon: BarChart3,
      title: "ציון התאמה חכם",
      description:
        "אלגוריתם שמנתח כל מודעה ומדרג אותה לפי איכות העסקה — מחיר, קילומטראז', שנה ומצב.",
      gradient: "from-amber-500/10 to-orange-500/10",
      iconColor: "text-amber-600 dark:text-amber-400",
      iconBg: "bg-amber-500/10 dark:bg-amber-500/20",
    },
    {
      icon: Shield,
      title: "סינון חכם",
      description:
        "סנן מודעות לפי מילות מפתח, מנוע מינימלי ועוד. ראה רק את מה שרלוונטי לך.",
      gradient: "from-red-500/10 to-rose-500/10",
      iconColor: "text-red-600 dark:text-red-400",
      iconBg: "bg-red-500/10 dark:bg-red-500/20",
    },
    {
      icon: Star,
      title: "שמירת מועדפים",
      description:
        "שמור מודעות מעניינות לצפייה מאוחרת. גש אליהן בכל עת מדף השמורים.",
      gradient: "from-indigo-500/10 to-violet-500/10",
      iconColor: "text-indigo-600 dark:text-indigo-400",
      iconBg: "bg-indigo-500/10 dark:bg-indigo-500/20",
    },
  ];

  return (
    <section id="features" className="py-20 sm:py-28">
      <div className="mx-auto max-w-6xl px-4 sm:px-6 lg:px-8">
        <div className="text-center mb-14">
          <h2 className="text-3xl font-black tracking-tight sm:text-4xl">
            הכלים שלך למציאת <span className="text-gradient">העסקה הבאה</span>
          </h2>
          <p className="mt-4 text-lg text-muted-foreground max-w-2xl mx-auto">
            כל מה שצריך כדי למצוא את הרכב הנכון במחיר הנכון, בלי לבזבז שעות
            בגלילה.
          </p>
        </div>

        <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
          {features.map((feature, i) => {
            const Icon = feature.icon;
            return (
              <div
                key={feature.title}
                className={cn(
                  "rounded-2xl border border-border bg-gradient-to-br p-6 hover-lift gradient-card card-shine",
                  feature.gradient,
                  `stagger-${Math.min(i + 1, 6)} animate-slide-up`,
                )}
              >
                <div
                  className={cn(
                    "flex h-11 w-11 items-center justify-center rounded-xl mb-4",
                    feature.iconBg,
                  )}
                >
                  <Icon className={cn("h-5 w-5", feature.iconColor)} />
                </div>
                <h3 className="text-lg font-bold mb-2">{feature.title}</h3>
                <p className="text-sm text-muted-foreground leading-relaxed">
                  {feature.description}
                </p>
              </div>
            );
          })}
        </div>
      </div>
    </section>
  );
}

function HowItWorks() {
  const steps = [
    {
      number: "1",
      title: "הגדר חיפוש",
      description: "בחר יצרן, דגם, טווח שנים ומחיר. הוסף סינונים מתקדמים.",
    },
    {
      number: "2",
      title: "CarWatch סורק",
      description:
        "המערכת סורקת את אתרי הרכב באופן אוטומטי ומנתחת כל מודעה חדשה.",
    },
    {
      number: "3",
      title: "קבל התראות",
      description: "קבל התראות על מודעות רלוונטיות עם ציון התאמה וניתוח מחיר.",
    },
  ];

  return (
    <section className="py-20 sm:py-28 bg-muted/30">
      <div className="mx-auto max-w-6xl px-4 sm:px-6 lg:px-8">
        <div className="text-center mb-14">
          <h2 className="text-3xl font-black tracking-tight sm:text-4xl">
            איך זה עובד?
          </h2>
          <p className="mt-4 text-lg text-muted-foreground">
            שלושה צעדים פשוטים למציאת הרכב המושלם
          </p>
        </div>

        <div className="grid gap-8 sm:grid-cols-3">
          {steps.map((step, i) => (
            <div
              key={step.number}
              className={cn(
                "text-center",
                `stagger-${i + 1} animate-slide-up`,
              )}
            >
              <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-2xl gradient-primary text-2xl font-black text-white shadow-lg mb-5">
                {step.number}
              </div>
              <h3 className="text-xl font-bold mb-2">{step.title}</h3>
              <p className="text-muted-foreground leading-relaxed">
                {step.description}
              </p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

function Stats() {
  const stats = [
    { value: "24/7", label: "סריקה רציפה" },
    { value: "< 1 דקה", label: "זמן תגובה" },
    { value: "100%", label: "חינם" },
  ];

  return (
    <section className="py-20 sm:py-28">
      <div className="mx-auto max-w-4xl px-4 sm:px-6 lg:px-8">
        <div className="rounded-3xl gradient-primary p-10 sm:p-14 text-center text-white shadow-xl relative overflow-hidden">
          <div className="absolute top-0 right-0 h-40 w-40 rounded-full bg-white/5 -translate-y-1/2 translate-x-1/2" />
          <div className="absolute bottom-0 left-0 h-32 w-32 rounded-full bg-white/5 translate-y-1/2 -translate-x-1/2" />

          <div className="relative grid gap-8 sm:grid-cols-3">
            {stats.map((stat) => (
              <div key={stat.label}>
                <p className="text-4xl font-black tracking-tight mb-1">
                  {stat.value}
                </p>
                <p className="text-sm text-white/70 font-medium">
                  {stat.label}
                </p>
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}

function CTA() {
  return (
    <section className="py-20 sm:py-28 bg-muted/30">
      <div className="mx-auto max-w-3xl px-4 sm:px-6 lg:px-8 text-center">
        <h2 className="text-3xl font-black tracking-tight sm:text-4xl mb-4">
          מוכן למצוא את הרכב <span className="text-gradient">הבא שלך</span>?
        </h2>
        <p className="text-lg text-muted-foreground mb-10">
          הצטרף ל-CarWatch וקבל התראות על עסקאות שמתאימות בדיוק לך.
        </p>
        <Link
          to="/"
          className="inline-flex items-center gap-2.5 rounded-2xl gradient-primary px-10 py-4 text-lg font-bold text-white shadow-lg hover:shadow-xl hover:brightness-110 transition-all"
        >
          התחל בחינם
          <ArrowLeft className="h-5 w-5" />
        </Link>
      </div>
    </section>
  );
}

function Footer() {
  return (
    <footer className="border-t border-border py-8">
      <div className="mx-auto max-w-6xl px-4 sm:px-6 lg:px-8">
        <div className="flex flex-col sm:flex-row items-center justify-between gap-4">
          <div className="flex items-center gap-2.5">
            <div className="flex h-7 w-7 items-center justify-center rounded-lg gradient-primary">
              <span className="text-xs font-black text-white">CW</span>
            </div>
            <span className="text-sm font-bold">CarWatch</span>
          </div>
          <p className="text-xs text-muted-foreground">
            &copy; {new Date().getFullYear()} CarWatch. כל הזכויות שמורות.
          </p>
        </div>
      </div>
    </footer>
  );
}
