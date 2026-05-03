import type { ReactNode } from "react";
import { Link } from "react-router";
import { ArrowLeft, Bell, TrendingDown } from "lucide-react";
import { motion } from "motion/react";

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
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ delay, duration: 0.6, ease: "easeOut" }}
      className={`glass-card absolute rounded-2xl p-3.5 shadow-xl ${className ?? ""}`}
    >
      {children}
    </motion.div>
  );
}

export function HeroSection() {
  return (
    <section className="relative flex min-h-screen items-center justify-center overflow-hidden pt-16">
      <div className="pointer-events-none absolute inset-0">
        <div className="absolute top-1/4 right-1/4 h-96 w-96 rounded-full bg-primary/10 blur-[120px]" />
        <div className="absolute bottom-1/3 left-1/4 h-72 w-72 rounded-full bg-purple-500/8 blur-[100px]" />
        <div className="absolute top-1/2 left-1/2 h-[600px] w-[600px] -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary/5 blur-[150px]" />
      </div>

      <div className="landing-grid-bg pointer-events-none absolute inset-0 opacity-[0.03]" />

      <div className="relative z-10 mx-auto max-w-5xl px-6 text-center">
        <motion.div
          initial={{ opacity: 0, y: -10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5 }}
          className="mb-8 inline-flex items-center gap-2 rounded-full border border-primary/20 bg-primary/10 px-4 py-1.5 text-sm font-medium text-primary"
        >
          <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-primary" />
          מעקב רכבים חכם בישראל
        </motion.div>

        <motion.h1
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.1, duration: 0.7 }}
          className="mb-6 text-5xl leading-tight font-bold text-foreground md:text-7xl"
        >
          עקוב אחר רכבים.
          <br />
          <span className="gradient-text">קבל התראה ראשון.</span>
        </motion.h1>

        <motion.p
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.2, duration: 0.7 }}
          className="mx-auto mb-10 max-w-2xl text-lg leading-relaxed text-muted-foreground md:text-xl"
        >
          CarWatch סורקת אלפי מודעות רכב בישראל ומתריאה לך מיד כשמופיע הרכב שחיפשת
          — במחיר שתוכל להרשות לעצמך.
        </motion.p>

        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.3, duration: 0.6 }}
          className="mb-16 flex flex-col items-center justify-center gap-4 sm:flex-row"
        >
          <Link
            to="/signup"
            className="flex items-center gap-2.5 rounded-2xl bg-primary px-7 py-3.5 text-base font-bold text-white shadow-2xl shadow-primary/25 transition-all hover:bg-primary/90 hover:shadow-primary/40 hover:active:translate-y-0 md:hover:-translate-y-0.5"
          >
            התחל לעקוב עכשיו
            <ArrowLeft size={18} />
          </Link>
          <a
            href="#how"
            className="flex items-center gap-2 rounded-2xl border border-border px-6 py-3.5 text-sm font-medium text-muted-foreground transition-colors hover:border-border/80 hover:bg-secondary/50 hover:text-foreground"
          >
            איך זה עובד?
          </a>
        </motion.div>

        <motion.div
          initial={{ opacity: 0, y: 40, scale: 0.95 }}
          animate={{ opacity: 1, y: 0, scale: 1 }}
          transition={{ delay: 0.4, duration: 0.8, ease: "easeOut" }}
          className="relative mx-auto max-w-3xl"
        >
          <div className="glass-card rounded-3xl border border-border/80 p-6 shadow-2xl">
            <div className="mb-4 flex items-center justify-between">
              <div className="flex items-center gap-2">
                <div className="h-2 w-2 animate-pulse rounded-full bg-success" />
                <span className="text-xs font-medium text-muted-foreground">
                  3 חיפושים פעילים
                </span>
              </div>
              <span className="rounded-full bg-primary/10 px-3 py-1 text-xs font-medium text-primary">
                עדכון אחרון: כרגע
              </span>
            </div>

            <div className="space-y-3">
              {[
                {
                  title: "טויוטה קורולה 2020–2022",
                  count: 47,
                  new: 3,
                  price: "עד ₪120,000",
                },
                {
                  title: "קיה ספורטאג׳ יד 2",
                  count: 31,
                  new: 1,
                  price: "עד ₪180,000",
                },
                {
                  title: "מזדה 3 אוטומט",
                  count: 22,
                  new: 0,
                  price: "עד ₪140,000",
                },
              ].map((item, i) => (
                <div
                  key={i}
                  className="flex items-center justify-between rounded-xl bg-secondary/50 px-4 py-3"
                >
                  <div className="text-right">
                    <p className="text-sm font-semibold text-foreground">
                      {item.title}
                    </p>
                    <p className="mt-0.5 text-xs text-muted-foreground">
                      {item.price}
                    </p>
                  </div>
                  <div className="flex items-center gap-3">
                    <span className="text-xs text-muted-foreground">
                      {item.count} מודעות
                    </span>
                    {item.new > 0 ? (
                      <span className="rounded-full bg-primary px-2.5 py-0.5 text-xs font-bold text-white">
                        {item.new} חדשות
                      </span>
                    ) : null}
                  </div>
                </div>
              ))}
            </div>
          </div>

          <FloatingCard className="-top-5 -left-8 max-w-xs md:-left-14" delay={0.8}>
            <div className="flex items-start gap-2.5">
              <div className="flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-lg bg-primary/20">
                <Bell size={13} className="text-primary" />
              </div>
              <div>
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

          <FloatingCard
            className="-bottom-5 -right-8 md:-right-14"
            delay={1.0}
          >
            <div className="flex items-center gap-2.5">
              <div className="flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-lg bg-success/20">
                <TrendingDown size={13} className="text-success" />
              </div>
              <div>
                <p className="text-xs font-semibold text-foreground">ירידת מחיר</p>
                <p className="mt-0.5 text-[11px] text-muted-foreground">
                  מ-₪135,000 → ₪119,000
                </p>
              </div>
            </div>
          </FloatingCard>
        </motion.div>
      </div>

      <motion.div
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        transition={{ delay: 1.5 }}
        className="absolute bottom-8 left-1/2 flex -translate-x-1/2 flex-col items-center gap-1.5"
      >
        <span className="text-xs text-muted-foreground/50">גלול למטה</span>
        <div className="flex h-8 w-5 items-start justify-center rounded-full border-2 border-border/50 p-1">
          <motion.div
            animate={{ y: [0, 10, 0] }}
            transition={{ duration: 1.5, repeat: Infinity, ease: "easeInOut" }}
            className="h-1.5 w-1 rounded-full bg-muted-foreground/50"
          />
        </div>
      </motion.div>
    </section>
  );
}
