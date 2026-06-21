import { useCallback, useEffect, useId, useRef, useState } from "react";
import { Link } from "react-router";
import { Car, Sun, Moon, Menu, X } from "lucide-react";
import { useTheme } from "@/contexts/ThemeContext";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export function LandingNav() {
  const { theme, toggle } = useTheme();
  const mobileMenuId = useId();
  const [scrolled, setScrolled] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);
  const menuToggleRef = useRef<HTMLButtonElement>(null);
  const mobileMenuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const fn = () => setScrolled(window.scrollY > 20);
    fn();
    window.addEventListener("scroll", fn);
    return () => window.removeEventListener("scroll", fn);
  }, []);

  useEffect(() => {
    if (!mobileOpen || !mobileMenuRef.current) return;

    const menu = mobileMenuRef.current;
    const focusable = menu.querySelectorAll<HTMLElement>(
      'a[href], button, [tabindex]:not([tabindex="-1"])',
    );
    if (focusable.length > 0) focusable[0].focus();

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setMobileOpen(false);
        menuToggleRef.current?.focus();
        return;
      }
      if (e.key !== "Tab" || focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    };

    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [mobileOpen]);

  const closeMobileMenu = useCallback(() => {
    setMobileOpen(false);
    menuToggleRef.current?.focus();
  }, []);

  const links = [
    { label: "תכונות", href: "#features" },
    { label: "איך זה עובד", href: "#how" },
    { label: "סטטיסטיקות", href: "#stats" },
  ];

  return (
    <header
      className={cn(
        "fixed inset-x-0 top-0 z-50 transition-all duration-300",
        scrolled
          ? "border-border/50 border-b bg-background/85 shadow-[0_8px_30px_-12px_rgba(0,0,0,0.12)] dark:shadow-[0_8px_40px_-16px_rgba(0,0,0,0.45)]"
          : "bg-transparent",
      )}
    >
      <div className="mx-auto flex h-16 max-w-6xl items-center justify-between gap-3 px-4 sm:px-6">
        <Link to="/" className="group flex min-w-0 items-center gap-2.5">
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary shadow-lg shadow-primary/35 ring-1 ring-white/15 transition-transform group-hover:scale-[1.03]">
            <Car size={17} className="text-white" />
          </div>
          <span className="truncate text-lg font-bold tracking-tight text-foreground">
            CarWatch
          </span>
        </Link>

        <nav
          className="hidden items-center gap-1 lg:flex"
          aria-label="ניווט דף נחיתה"
        >
          {links.map((l) => (
            <a
              key={l.href}
              href={l.href}
              className="rounded-lg px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-secondary/80 hover:text-foreground"
            >
              {l.label}
            </a>
          ))}
        </nav>

        <div className="flex shrink-0 items-center gap-1.5 sm:gap-2">
          <button
            type="button"
            onClick={toggle}
            className="flex h-9 w-9 items-center justify-center rounded-lg border border-transparent text-muted-foreground transition-all hover:border-border/80 hover:bg-secondary hover:text-foreground"
            aria-label={theme === "dark" ? "מצב בהיר" : "מצב כהה"}
          >
            {theme === "dark" ? <Sun size={16} /> : <Moon size={16} />}
          </button>
          <Button asChild className="hidden sm:flex shadow-lg shadow-primary/25">
            <Link to="/signup">התחל עכשיו</Link>
          </Button>
          <button
            ref={menuToggleRef}
            type="button"
            onClick={() => setMobileOpen((o) => !o)}
            className="flex h-8 w-8 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:text-foreground lg:hidden"
            aria-label={mobileOpen ? "סגור תפריט" : "פתח תפריט"}
            aria-expanded={mobileOpen}
            aria-controls={mobileMenuId}
          >
            {mobileOpen ? <X size={18} /> : <Menu size={18} />}
          </button>
        </div>
      </div>

      <div
        ref={mobileMenuRef}
        id={mobileMenuId}
        role="navigation"
        aria-label="ניווט ראשי — נייד"
        hidden={!mobileOpen}
        className="border-border border-b bg-background/95 px-6 py-4 lg:hidden"
      >
        <div className="space-y-3">
          {links.map((l) => (
            <a
              key={l.href}
              href={l.href}
              onClick={closeMobileMenu}
              className="block py-2 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
            >
              {l.label}
            </a>
          ))}
          <Link
            to="/signup"
            onClick={closeMobileMenu}
            className="mt-2 block w-full rounded-xl bg-primary py-2.5 text-center text-sm font-semibold text-white"
          >
            התחל עכשיו
          </Link>
        </div>
      </div>
    </header>
  );
}
