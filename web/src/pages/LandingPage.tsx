import { useEffect, useState } from "react";
import { Link } from "react-router";
import {
  Car,
  Bell,
  TrendingDown,
  Search,
  Zap,
  Shield,
  ChevronLeft,
  Globe,
} from "lucide-react";

function useAppVersion() {
  const [version, setVersion] = useState<string | null>(null);
  useEffect(() => {
    fetch("/healthz")
      .then((r) => r.json())
      .then((d) => {
        if (d?.version) setVersion(d.version);
      })
      .catch(() => {});
  }, []);
  return version;
}

const features = [
  {
    icon: Bell,
    title: "התראות בזמן אמת",
    desc: "קבל התראה מיידית בטלגרם ברגע שרכב חדש מתאים לחיפוש שלך מופיע.",
  },
  {
    icon: TrendingDown,
    title: "מעקב ירידת מחירים",
    desc: "נעקוב אחרי שינויי מחיר ונודיע לך כשרכב שמעניין אותך יורד במחיר.",
  },
  {
    icon: Search,
    title: "סינון חכם",
    desc: "הגדר חיפוש מדויק — יצרן, דגם, שנה, מחיר, קילומטראז׳, יד ועוד.",
  },
  {
    icon: Globe,
    title: "ריבוי מקורות",
    desc: "סורקים את Yad2 ו-WinWin במקביל כדי שלא תפספס אף מודעה.",
  },
  {
    icon: Zap,
    title: "ציון התאמה",
    desc: "כל מודעה מקבלת ציון התאמה לקריטריונים שלך — כדי שתראה קודם את הטובות ביותר.",
  },
  {
    icon: Shield,
    title: "ממשק מאובטח",
    desc: "כניסה מאובטחת עם Google ו-Firebase. הנתונים שלך פרטיים ומוגנים.",
  },
];

const steps = [
  {
    num: "1",
    title: "הירשם",
    desc: "צור חשבון בשניות עם Google או אימייל.",
  },
  {
    num: "2",
    title: "הגדר חיפוש",
    desc: "בחר יצרן, דגם, טווח שנים ומחיר — ותן לנו לעבוד.",
  },
  {
    num: "3",
    title: "קבל התראות",
    desc: "נשלח לך הודעה בטלגרם או בדאשבורד ברגע שמשהו חדש צץ.",
  },
];

export function LandingPage() {
  const version = useAppVersion();

  return (
    <div dir="rtl" className="min-h-screen bg-background text-foreground">
      <div
        className="pointer-events-none fixed inset-0 bg-[radial-gradient(ellipse_80%_50%_at_50%_-20%,rgba(59,130,246,0.12),transparent)]"
        aria-hidden
      />

      {/* Header */}
      <header className="relative z-10 flex items-center justify-between px-6 py-5 sm:px-10">
        <div className="flex items-center gap-3">
          <div className="relative flex h-10 w-10 items-center justify-center rounded-xl bg-primary shadow-lg shadow-primary/30">
            <Car className="h-5 w-5 text-white" />
            <div className="absolute inset-0 rounded-xl bg-gradient-to-br from-white/20 to-transparent" />
          </div>
          <div>
            <span className="text-lg font-bold tracking-tight">CarWatch</span>
            <span className="mr-1 text-xs text-muted-foreground">
              מעקב רכבים חכם
            </span>
          </div>
        </div>
        <Link
          to="/login"
          className="rounded-xl bg-primary px-5 py-2.5 text-sm font-semibold text-primary-foreground transition-opacity hover:opacity-90"
        >
          התחבר
        </Link>
      </header>

      {/* Hero */}
      <section className="relative z-10 mx-auto max-w-4xl px-6 pb-20 pt-16 text-center sm:px-10 sm:pt-24">
        <h1 className="text-4xl font-extrabold leading-tight tracking-tight sm:text-5xl lg:text-6xl">
          <span className="bg-gradient-to-l from-primary to-blue-400 bg-clip-text text-transparent">
            מוצא לך רכב
          </span>
          <br />
          <span>לפני כולם</span>
        </h1>
        <p className="mx-auto mt-6 max-w-2xl text-lg leading-relaxed text-muted-foreground sm:text-xl">
          CarWatch סורק את לוחות הרכב המובילים בישראל, מסנן לפי הקריטריונים
          שלך, ומתריע לך ברגע שעסקה טובה מופיעה — אוטומטית, 24/7.
        </p>

        <div className="mt-10 flex flex-wrap items-center justify-center gap-4">
          <Link
            to="/signup"
            className="group inline-flex items-center gap-2 rounded-xl bg-primary px-7 py-3.5 text-sm font-semibold text-primary-foreground shadow-lg shadow-primary/25 transition-all hover:shadow-primary/40 hover:opacity-95"
          >
            התחל בחינם
            <ChevronLeft className="h-4 w-4 transition-transform group-hover:-translate-x-0.5" />
          </Link>
          <Link
            to="/login"
            className="inline-flex items-center gap-2 rounded-xl border border-border bg-card px-7 py-3.5 text-sm font-medium text-foreground transition-colors hover:bg-muted"
          >
            יש לי חשבון
          </Link>
        </div>
      </section>

      {/* Features */}
      <section className="relative z-10 mx-auto max-w-6xl px-6 py-20 sm:px-10">
        <h2 className="mb-4 text-center text-2xl font-bold sm:text-3xl">
          למה CarWatch?
        </h2>
        <p className="mx-auto mb-14 max-w-xl text-center text-muted-foreground">
          כל הכלים שאתה צריך כדי למצוא את הרכב הבא שלך — במקום אחד.
        </p>

        <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {features.map((f) => {
            const Icon = f.icon;
            return (
              <div
                key={f.title}
                className="group rounded-2xl border border-border/60 bg-card/80 p-6 backdrop-blur-sm transition-colors hover:border-primary/30 hover:bg-card"
              >
                <div className="mb-4 flex h-11 w-11 items-center justify-center rounded-xl bg-primary/10 text-primary transition-colors group-hover:bg-primary/15">
                  <Icon className="h-5 w-5" />
                </div>
                <h3 className="mb-2 text-base font-semibold">{f.title}</h3>
                <p className="text-sm leading-relaxed text-muted-foreground">
                  {f.desc}
                </p>
              </div>
            );
          })}
        </div>
      </section>

      {/* How it works */}
      <section className="relative z-10 mx-auto max-w-4xl px-6 py-20 sm:px-10">
        <h2 className="mb-14 text-center text-2xl font-bold sm:text-3xl">
          איך זה עובד?
        </h2>
        <div className="grid gap-8 sm:grid-cols-3">
          {steps.map((s) => (
            <div key={s.num} className="text-center">
              <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-primary/10 text-xl font-bold text-primary">
                {s.num}
              </div>
              <h3 className="mb-2 text-lg font-semibold">{s.title}</h3>
              <p className="text-sm leading-relaxed text-muted-foreground">
                {s.desc}
              </p>
            </div>
          ))}
        </div>
      </section>

      {/* CTA */}
      <section className="relative z-10 mx-auto max-w-3xl px-6 py-20 text-center sm:px-10">
        <h2 className="mb-4 text-2xl font-bold sm:text-3xl">
          מוכן לחסוך זמן בחיפוש?
        </h2>
        <p className="mx-auto mb-8 max-w-lg text-muted-foreground">
          הצטרף עכשיו וקבל התראות על עסקאות טובות — לפני כולם.
        </p>
        <Link
          to="/signup"
          className="inline-flex items-center gap-2 rounded-xl bg-primary px-8 py-3.5 text-sm font-semibold text-primary-foreground shadow-lg shadow-primary/25 transition-all hover:shadow-primary/40 hover:opacity-95"
        >
          התחל בחינם
        </Link>
      </section>

      {/* Footer */}
      <footer className="relative z-10 border-t border-border/40 px-6 py-8 text-center sm:px-10">
        <div className="flex flex-col items-center gap-2">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Car className="h-4 w-4" />
            <span>CarWatch — מעקב רכבים חכם</span>
          </div>
          {version && (
            <span className="text-xs text-muted-foreground/60">
              גרסה {version}
            </span>
          )}
        </div>
      </footer>
    </div>
  );
}
