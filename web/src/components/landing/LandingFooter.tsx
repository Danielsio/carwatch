import { Link } from "react-router";
import { Car } from "lucide-react";

const footerLinks = [
  { label: "תכונות", href: "#features" },
  { label: "איך זה עובד", href: "#how" },
  { label: "סטטיסטיקות", href: "#stats" },
];

export function LandingFooter({ version }: { version?: string | null }) {
  const year = new Date().getFullYear();
  return (
    <footer className="border-border border-t bg-secondary/20 px-4 py-12 sm:px-6">
      <div className="mx-auto max-w-5xl">
        <div className="flex flex-col gap-10 md:flex-row md:items-start md:justify-between">
          <div className="max-w-md space-y-3 text-center md:text-start">
            <Link to="/" className="inline-flex items-center gap-2.5">
              <div className="flex h-8 w-8 items-center justify-center rounded-xl bg-primary shadow-md ring-1 ring-white/15">
                <Car size={15} className="text-white" />
              </div>
              <span className="text-base font-bold text-foreground">CarWatch</span>
            </Link>
            <p className="text-sm leading-relaxed text-muted-foreground">
              מערכת מודיעין לרכישת רכב יד שנייה בישראל — סריקה רב־מקורית, ציון התאמה
              חכם והתראות בזמן אמת, כדי שתגיע לעסקאות לפני ההמון.
            </p>
            <p className="text-xs leading-relaxed text-muted-foreground/90">
              מוצר עצמאי לחיפוש וניטור חכם — לא מייצג את אתרי הליסינג או המוכרים.
            </p>
          </div>

          <nav
            aria-label="קישורי דף הנחיתה"
            className="flex flex-wrap items-center justify-center gap-x-6 gap-y-2 md:justify-end"
          >
            {footerLinks.map((l) => (
              <a
                key={l.href}
                href={l.href}
                className="text-sm font-medium text-muted-foreground transition-colors hover:text-primary"
              >
                {l.label}
              </a>
            ))}
            <Link
              to="/login"
              className="text-sm font-medium text-primary transition-colors hover:text-primary/85"
            >
              כניסה
            </Link>
          </nav>
        </div>

        <div className="mt-10 flex flex-col items-center justify-between gap-3 border-border/80 border-t pt-8 text-center text-xs text-muted-foreground sm:flex-row sm:text-start">
          <p>
            © {year} CarWatch · כל הזכויות שמורות
            {version ? ` · גרסה ${version}` : ""}
          </p>
          <p className="max-w-sm sm:max-w-none">
            מעקב רכבים חכם בישראל — מהיר, שקוף ומותאם לך
          </p>
        </div>
      </div>
    </footer>
  );
}
