import { Link } from "react-router";
import { ArrowLeft, Zap } from "lucide-react";
import { FadeUp } from "./FadeUp";

export function FinalCTA() {
  return (
    <section className="px-4 py-24 sm:px-6">
      <div className="mx-auto max-w-3xl">
        <FadeUp>
          <div className="relative overflow-hidden rounded-3xl border border-border bg-card p-10 text-center md:p-16">
          <div className="pointer-events-none absolute inset-0">
          </div>

          <div className=" pointer-events-none absolute inset-0 opacity-[0.025]" />

          <div className="relative z-10">
            <div className="mx-auto mb-6 flex h-14 w-14 items-center justify-center rounded-2xl border border-border bg-muted">
              <Zap size={24} className="text-primary" />
            </div>
            <h2 className="mb-4 text-3xl leading-tight font-bold text-foreground md:text-5xl">
              מוכן למצוא את
              <br />
              <span className="text-primary">הרכב שלך?</span>
            </h2>
            <p className="mx-auto mb-10 max-w-md text-base leading-relaxed text-muted-foreground">
              חסוך זמן וכסף עם CarWatch — מעקב חכם אחרי רכבים. הגדרה תוך 2 דקות,
              בלי כרטיס אשראי.
            </p>
            <div className="flex flex-col items-center justify-center gap-4 sm:flex-row">
              <Link
                to="/signup"
                className="flex items-center gap-2.5 rounded-2xl bg-primary px-8 py-4 text-base font-bold text-white transition-all hover:bg-primary/90 hover:active:translate-y-0 md:"
              >
                התחל לעקוב עכשיו
                <ArrowLeft size={18} />
              </Link>
              <p className="text-sm text-muted-foreground">
                חינמי לחלוטין · ללא הגבלת חיפושים
              </p>
            </div>
          </div>
        </div>
        </FadeUp>
      </div>
    </section>
  );
}
