import type { ReactNode } from "react";
import { Link } from "react-router";
import { ArrowLeft, Bell, TrendingDown } from "lucide-react";

function FloatingCard({
  className,
  children,
  delay = 0,
}: {
  className?: string;
  children: ReactNode;
  delay?: number;
}) {
  return (
    <div
      className={`glass-card absolute rounded-2xl p-3.5 shadow-xl animate-slide-up motion-reduce:animate-none ${className ?? ""}`}
      style={delay > 0 ? { animationDelay: `${delay}s`, animationFillMode: "backwards" } : undefined}
    >
      {children}
    </div>
  );
}

export function HeroSection() {
  return (
    <section className="relative flex min-h-screen items-center justify-center pt-16">
      <div className="pointer-events-none absolute inset-0 overflow-hidden">
        <div className="absolute top-1/4 end-1/4 h-96 w-96 rounded-full bg-primary/10 blur-[120px]" />
        <div className="absolute bottom-1/3 start-1/4 h-72 w-72 rounded-full bg-purple-500/8 blur-[100px]" />
        <div className="absolute top-1/2 left-1/2 h-[600px] w-[600px] -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary/5 blur-[150px]" />
      </div>

      <div className="landing-grid-bg pointer-events-none absolute inset-0 opacity-[0.03]" />

      <div className="relative z-10 mx-auto max-w-5xl px-4 text-center sm:px-6">
        <div
          lang="he"
          className="mb-8 inline-flex items-center gap-2 rounded-full border border-primary/20 bg-primary/10 px-4 py-1.5 text-sm font-medium text-primary animate-fade-in motion-reduce:animate-none"
        >
          <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-primary motion-reduce:animate-none" />
          מעקב רכבים חכם בישראל
        </div>

        <h1
          lang="he"
          className="mb-6 text-4xl leading-[1.12] font-bold text-foreground sm:text-5xl md:text-6xl lg:text-7xl animate-slide-up motion-reduce:animate-none"
          style={{ animationDelay: "0.1s", animationFillMode: "backwards" }}
        >
          עקוב אחרי המודעות
          <br />
          <span className="gradient-text">וקנה לפני שכולם הספיקו.</span>
        </h1>

        <p
          lang="he"
          className="mx-auto mb-2 max-w-2xl text-lg leading-relaxed text-muted-foreground md:text-xl animate-slide-up motion-reduce:animate-none"
          style={{ animationDelay: "0.2s", animationFillMode: "backwards" }}
        >
          CarWatch סורקת מקורות מרובים בישראל, מדרגת מודעות בציון חכם ושולחת התראות
          בזמן אמת — כדי שתאסוף את הרכב הנכון במחיר הנכון.
        </p>
        <p
          className="mx-auto mb-10 max-w-2xl text-sm font-medium text-muted-foreground/90 md:text-base animate-slide-up motion-reduce:animate-none"
          lang="en"
          dir="ltr"
          style={{ animationDelay: "0.25s", animationFillMode: "backwards" }}
        >
          Multi-source monitoring · Smart Match Score · Real-time alerts
        </p>

        <div
          className="mb-16 flex flex-col items-center justify-center gap-4 sm:flex-row animate-slide-up motion-reduce:animate-none"
          style={{ animationDelay: "0.3s", animationFillMode: "backwards" }}
        >
          <Link
            to="/signup"
            className="flex items-center gap-2.5 rounded-2xl bg-primary px-7 py-3.5 text-base font-bold text-white shadow-2xl shadow-primary/25 transition-all hover:bg-primary/90 hover:shadow-primary/40 hover:active:translate-y-0 md:hover:-translate-y-0.5"
          >
            התחל לעקוב עכשיו
            <ArrowLeft size={18} />
          </Link>
          <Link
            to="/try"
            className="flex items-center gap-2 rounded-2xl border border-primary/30 bg-primary/5 px-6 py-3.5 text-sm font-medium text-primary transition-colors hover:border-primary/50 hover:bg-primary/10"
          >
            נסה חיפוש חינם
          </Link>
          <a
            href="#how"
            className="flex items-center gap-2 rounded-2xl border border-border px-6 py-3.5 text-sm font-medium text-muted-foreground transition-colors hover:border-border/80 hover:bg-secondary/50 hover:text-foreground"
          >
            איך זה עובד?
          </a>
        </div>

        <div
          className="relative mx-auto max-w-3xl pb-10 animate-slide-up motion-reduce:animate-none"
          style={{ animationDelay: "0.4s", animationFillMode: "backwards" }}
        >
          <div className="glass-card rounded-3xl border border-border/80 p-5 shadow-2xl sm:p-6">
            <div className="mb-4 flex flex-wrap items-center justify-between gap-2">
              <div className="flex items-center gap-2">
                <div className="h-2 w-2 animate-pulse rounded-full bg-success motion-reduce:animate-none" />
                <span className="text-xs font-medium text-muted-foreground">
                  לוח בקרה · 3 חיפושים פעילים
                </span>
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <span className="rounded-full border border-border/60 bg-secondary/40 px-2.5 py-1 text-[10px] font-semibold text-muted-foreground">
                  Yad2
                </span>
                <span className="rounded-full bg-primary/10 px-3 py-1 text-xs font-medium text-primary">
                  סנכרון: לפני רגע
                </span>
              </div>
            </div>

            <div className="space-y-3">
              {[
                {
                  title: "טויוטה קורולה 2020–2022",
                  count: 47,
                  new: 3,
                  price: "עד ₪120,000",
                  score: "8.6",
                  tag: null as string | null,
                },
                {
                  title: "קיה ספורטאג׳ יד 2",
                  count: 31,
                  new: 1,
                  price: "עד ₪180,000",
                  score: "7.9",
                  tag: "ירידת מחיר",
                },
                {
                  title: "מזדה 3 אוטומט",
                  count: 22,
                  new: 0,
                  price: "עד ₪140,000",
                  score: "7.2",
                  tag: null as string | null,
                },
              ].map((item, i) => (
                <div
                  key={i}
                  className="flex flex-col gap-3 rounded-xl border border-border/40 bg-secondary/40 px-3 py-3 sm:flex-row sm:items-center sm:justify-between sm:px-4"
                >
                  <div className="min-w-0 flex-1 text-right">
                    <div className="flex flex-wrap items-center justify-end gap-2">
                      <p className="text-sm font-semibold text-foreground">
                        {item.title}
                      </p>
                      {item.tag ? (
                        <span className="inline-flex items-center gap-1 rounded-md bg-success/15 px-2 py-0.5 text-[10px] font-bold text-success ring-1 ring-success/20">
                          <TrendingDown className="h-3 w-3" aria-hidden />
                          {item.tag}
                        </span>
                      ) : null}
                    </div>
                    <p className="mt-0.5 text-xs text-muted-foreground">
                      {item.price}
                    </p>
                  </div>
                  <div className="flex flex-wrap items-center justify-between gap-2 sm:justify-end sm:gap-3">
                    <span className="rounded-lg bg-card/80 px-2 py-1 text-[10px] font-bold tabular-nums text-primary ring-1 ring-primary/15">
                      ציון {item.score}
                    </span>
                    <span className="text-xs text-muted-foreground">
                      {item.count} מודעות
                    </span>
                    {item.new > 0 ? (
                      <span className="rounded-full bg-primary px-2.5 py-0.5 text-xs font-bold text-white">
                        {item.new} חדשות
                      </span>
                    ) : (
                      <span className="rounded-full bg-muted/80 px-2 py-0.5 text-[10px] text-muted-foreground">
                        עדכני
                      </span>
                    )}
                  </div>
                </div>
              ))}
            </div>

            <div className="mt-4 flex flex-wrap items-center justify-between gap-2 rounded-2xl border border-dashed border-primary/25 bg-primary/5 px-3 py-2.5">
              <div className="flex items-center gap-2 text-xs font-medium text-foreground">
                <Bell className="h-3.5 w-3.5 shrink-0 text-primary" aria-hidden />
                <span>סיכום יומי · 08:00 — 6 התאמות חדשות מאתמול</span>
              </div>
              <span className="text-[10px] font-semibold text-primary/90" lang="en" dir="ltr">
                digest
              </span>
            </div>
          </div>

          <FloatingCard
            className="-top-5 max-w-[min(18rem,calc(100vw-3rem))] start-2 sm:-start-8 md:-start-14"
            delay={0.8}
          >
            <div className="flex items-start gap-2.5">
              <div className="flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-lg bg-primary/20">
                <Bell size={13} className="text-primary" />
              </div>
              <div className="text-right">
                <p className="text-xs font-semibold text-foreground">
                  התראה חדשה!
                </p>
                <p className="mt-0.5 text-[11px] text-muted-foreground">
                  טויוטה קורולה 2021 — ₪109,000
                </p>
                <p className="mt-1 text-[10px] text-primary">
                  ₪11,000 מתחת לממוצע שוק
                </p>
              </div>
            </div>
          </FloatingCard>

          <FloatingCard className="-bottom-5 max-w-[min(18rem,calc(100vw-3rem))] end-2 sm:-end-8 md:-end-14" delay={1.0}>
            <div className="flex items-center gap-2.5">
              <div className="flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-lg bg-success/20">
                <TrendingDown size={13} className="text-success" />
              </div>
              <div className="text-right">
                <p className="text-xs font-semibold text-foreground">ירידת מחיר</p>
                <p className="mt-0.5 text-[11px] text-muted-foreground">
                  מ-₪135,000 → ₪119,000
                </p>
              </div>
            </div>
          </FloatingCard>
        </div>
      </div>

      <div
        className="absolute bottom-8 left-1/2 flex -translate-x-1/2 flex-col items-center gap-1.5 animate-fade-in motion-reduce:animate-none"
        style={{ animationDelay: "1.5s", animationFillMode: "backwards" }}
      >
        <span className="text-xs text-muted-foreground/50">גלול למטה</span>
        <div className="flex h-8 w-5 items-start justify-center rounded-full border-2 border-border/50 p-1">
          <div
            className="h-1.5 w-1 rounded-full bg-muted-foreground/50"
            style={{ animation: "scroll-dot 1.5s ease-in-out infinite" }}
          />
        </div>
      </div>
    </section>
  );
}
