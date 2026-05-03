import { Link } from "react-router";
import { Car } from "lucide-react";

export function LandingFooter({ version }: { version?: string | null }) {
  return (
    <footer className="border-border border-t px-6 py-10">
      <div className="mx-auto flex max-w-5xl flex-col items-center justify-between gap-4 md:flex-row">
        <Link to="/welcome" className="flex items-center gap-2.5">
          <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-primary shadow shadow-primary/30">
            <Car size={13} className="text-white" />
          </div>
          <span className="text-sm font-bold text-foreground">CarWatch</span>
        </Link>
        <p className="text-center text-xs text-muted-foreground md:text-start">
          © 2026 CarWatch · מעקב רכבים חכם בישראל
          {version ? ` · גרסה ${version}` : ""}
        </p>
        <Link
          to="/login"
          className="text-xs font-medium text-primary transition-colors hover:text-primary/80"
        >
          כניסה לאפליקציה ←
        </Link>
      </div>
    </footer>
  );
}
