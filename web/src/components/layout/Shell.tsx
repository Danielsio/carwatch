import { Outlet, Link, useLocation } from "react-router";
import { Search, Plus, Bookmark, Clock, Settings, Sun, Moon, Monitor } from "lucide-react";
import { cn } from "@/lib/utils";
import { useTheme } from "@/hooks/useTheme";
import { useState } from "react";

const navItems = [
  { path: "/", label: "חיפושים", icon: Search },
  { path: "/searches/new", label: "חיפוש חדש", icon: Plus },
  { path: "/saved", label: "שמורים", icon: Bookmark },
  { path: "/history", label: "היסטוריה", icon: Clock },
  { path: "/admin", label: "ניהול", icon: Settings },
];

export function Shell() {
  const location = useLocation();

  return (
    <div className="min-h-screen bg-background transition-colors duration-300">
      {/* Desktop sidebar */}
      <aside className="fixed inset-y-0 right-0 z-50 hidden w-64 border-l border-sidebar-border bg-sidebar md:flex md:flex-col transition-colors duration-300">
        <Link to="/" className="flex h-16 items-center gap-3 border-b border-sidebar-border px-6 group">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg gradient-primary shadow-md">
            <span className="text-sm font-black text-white">CW</span>
          </div>
          <span className="text-lg font-bold tracking-tight group-hover:text-gradient transition-all">
            CarWatch
          </span>
        </Link>

        <nav className="flex flex-1 flex-col gap-1 p-3">
          {navItems.map((item) => {
            const Icon = item.icon;
            const isActive =
              item.path === "/"
                ? location.pathname === "/"
                : location.pathname.startsWith(item.path);
            return (
              <Link
                key={item.path}
                to={item.path}
                className={cn(
                  "flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium transition-all duration-200",
                  isActive
                    ? "gradient-primary text-white shadow-md"
                    : "text-muted-foreground hover:bg-accent hover:text-foreground",
                )}
              >
                <Icon className="h-4 w-4" />
                {item.label}
              </Link>
            );
          })}
        </nav>

        <div className="border-t border-sidebar-border p-3">
          <ThemeToggle />
        </div>
      </aside>

      {/* Main content */}
      <main className="pb-20 md:mr-64 md:pb-0 transition-[margin] duration-300">
        <div className="mx-auto max-w-5xl px-4 py-6 sm:px-6 lg:px-8">
          <Outlet />
        </div>
      </main>

      {/* Mobile bottom nav */}
      <nav className="fixed inset-x-0 bottom-0 z-50 border-t border-border glass-heavy md:hidden">
        <div className="flex justify-around py-1.5 safe-area-pb">
          {navItems.slice(0, 4).map((item) => {
            const Icon = item.icon;
            const isActive =
              item.path === "/"
                ? location.pathname === "/"
                : location.pathname.startsWith(item.path);
            return (
              <Link
                key={item.path}
                to={item.path}
                className={cn(
                  "relative flex flex-col items-center gap-0.5 px-3 py-1.5 text-[11px] font-medium transition-colors duration-200",
                  isActive ? "text-primary" : "text-muted-foreground",
                )}
              >
                <Icon className={cn("h-5 w-5", isActive && "drop-shadow-sm")} />
                {item.label}
                {isActive && (
                  <span className="absolute -top-1.5 left-1/2 -translate-x-1/2 h-0.5 w-4 rounded-full gradient-primary" />
                )}
              </Link>
            );
          })}
        </div>
      </nav>
    </div>
  );
}

function ThemeToggle() {
  const { theme, setTheme } = useTheme();
  const [open, setOpen] = useState(false);

  const options = [
    { value: "light" as const, label: "בהיר", icon: Sun },
    { value: "dark" as const, label: "כהה", icon: Moon },
    { value: "system" as const, label: "מערכת", icon: Monitor },
  ];

  const current = options.find((o) => o.value === theme) ?? options[2];
  const CurrentIcon = current.icon;

  return (
    <div className="relative">
      <button
        onClick={() => setOpen(!open)}
        className="flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium text-muted-foreground hover:bg-accent hover:text-foreground transition-all duration-200"
      >
        <CurrentIcon className="h-4 w-4" />
        {current.label}
      </button>

      {open && (
        <>
          <div className="fixed inset-0 z-40" onClick={() => setOpen(false)} />
          <div className="absolute bottom-full right-0 left-0 mb-1 z-50 rounded-xl border border-border bg-popover p-1 shadow-lg animate-scale-in">
            {options.map((opt) => {
              const Icon = opt.icon;
              return (
                <button
                  key={opt.value}
                  onClick={() => {
                    setTheme(opt.value);
                    setOpen(false);
                  }}
                  className={cn(
                    "flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors",
                    theme === opt.value
                      ? "bg-accent text-foreground font-medium"
                      : "text-muted-foreground hover:bg-accent/50",
                  )}
                >
                  <Icon className="h-4 w-4" />
                  {opt.label}
                </button>
              );
            })}
          </div>
        </>
      )}
    </div>
  );
}
