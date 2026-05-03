import { Outlet, Link, useLocation } from "react-router";
import {
  LayoutDashboard,
  Plus,
  Car,
  Settings,
  Wrench,
  Bookmark,
  History,
  Bell,
  LogOut,
  Sun,
  Moon,
} from "lucide-react";
import { useNotificationCount } from "@/hooks/useNotifications";
import { useAppVersion } from "@/hooks/useAppVersion";
import { useAuth } from "@/contexts/AuthContext";
import { useTheme } from "@/contexts/ThemeContext";
import { useHealthCheck } from "@/hooks/useHealthCheck";
import { ConnectionBanner } from "@/components/ui/ConnectionBanner";
import { cn } from "@/lib/utils";

const navItems = [
  { path: "/", label: "לוח בקרה", icon: LayoutDashboard, mobile: true },
  { path: "/searches/new", label: "חיפוש חדש", icon: Plus, mobile: true },
  { path: "/saved", label: "מועדפים", icon: Bookmark, mobile: true },
  { path: "/history", label: "היסטוריה", icon: History, mobile: true },
  {
    path: "/notifications",
    label: "התראות",
    icon: Bell,
    badge: true,
    mobile: true,
  },
  { path: "/settings", label: "הגדרות", icon: Settings, mobile: true },
  { path: "/admin", label: "ניהול", icon: Wrench, mobile: false },
];

function isNavActive(pathname: string, path: string): boolean {
  if (path === "/") return pathname === "/";
  return pathname === path || pathname.startsWith(`${path}/`);
}

export function Shell() {
  const location = useLocation();
  const { data: notifCount } = useNotificationCount();
  const unread = notifCount?.count ?? 0;
  const { user, signOut } = useAuth();
  const { theme, toggle: toggleTheme } = useTheme();
  const appVersion = useAppVersion();
  const connectionStatus = useHealthCheck();
  const emailInitial =
    user?.email?.trim().charAt(0)?.toLocaleUpperCase("he-IL") || "?";

  return (
    <div className="min-h-screen bg-background">
      <ConnectionBanner status={connectionStatus} />
      <aside className="border-sidebar-border bg-sidebar fixed inset-y-0 right-0 z-40 hidden h-full w-64 flex-col border-l md:flex">
        <div className="border-sidebar-border flex items-center gap-3 border-b px-5 py-5">
          <div className="bg-sidebar-primary relative flex h-10 w-10 shrink-0 items-center justify-center rounded-xl shadow-lg shadow-sidebar-primary/40">
            <Car className="h-5 w-5 text-white" />
            <div className="absolute inset-0 rounded-xl bg-gradient-to-br from-white/20 to-transparent" />
          </div>
          <div className="min-w-0 flex-1">
            <h1 className="text-base leading-none font-bold tracking-tight text-white">
              CarWatch
            </h1>
            <p className="mt-0.5 text-xs text-sidebar-foreground/60">
              מעקב רכבים חכם
            </p>
          </div>
          <button
            type="button"
            onClick={toggleTheme}
            aria-label={theme === "dark" ? "הפעל מצב בהיר" : "הפעל מצב כהה"}
            className="bg-sidebar-accent text-sidebar-foreground/60 hover:bg-sidebar-accent/80 hover:text-sidebar-foreground flex h-7 w-7 shrink-0 items-center justify-center rounded-lg transition-all"
            title={theme === "dark" ? "מצב בהיר" : "מצב כהה"}
          >
            {theme === "dark" ? <Sun size={14} /> : <Moon size={14} />}
          </button>
        </div>

        <nav className="flex-1 space-y-0.5 overflow-y-auto px-3 py-4">
          {navItems.map((item) => {
            const Icon = item.icon;
            const active = isNavActive(location.pathname, item.path);
            const showBadge = item.badge && unread > 0;

            return (
              <Link
                key={item.path}
                to={item.path}
                className={cn(
                  "nav-active-pill flex items-center gap-3 px-4 py-2.5 text-sm font-medium transition-all duration-150",
                  active
                    ? "bg-sidebar-primary/15 text-white"
                    : "text-sidebar-foreground hover:bg-sidebar-accent hover:text-white",
                )}
              >
                <Icon
                  size={17}
                  className={cn(
                    "shrink-0 transition-colors",
                    active
                      ? "text-sidebar-primary"
                      : "text-sidebar-foreground/70",
                  )}
                />
                <span className="relative">
                  {item.label}
                  {showBadge ? (
                    <span className="absolute -top-1.5 -right-4 flex h-4 min-w-4 items-center justify-center rounded-full bg-destructive px-1 text-[10px] font-bold text-white animate-pulse-soft">
                      {unread > 99 ? "99+" : unread}
                    </span>
                  ) : null}
                </span>
                {active ? (
                  <div className="bg-sidebar-primary mr-auto h-1.5 w-1.5 rounded-full shadow-sm shadow-primary/60" />
                ) : null}
              </Link>
            );
          })}
        </nav>

        {unread > 0 ? (
          <div className="border-sidebar-border border-t px-4 py-4">
            <Link
              to="/notifications"
              className="border-sidebar-primary/20 bg-sidebar-primary/10 hover:bg-sidebar-primary/15 flex items-center gap-2.5 rounded-xl border px-3 py-2.5 transition-colors"
            >
              <div className="relative">
                <Bell className="text-sidebar-primary h-4 w-4" />
                <span className="bg-sidebar-primary absolute -top-1 -right-1 h-2 w-2 animate-pulse-glow rounded-full" />
              </div>
              <span className="text-sidebar-primary text-xs font-medium">
                התראות פעילות
              </span>
              <span className="bg-sidebar-primary mr-auto flex h-5 min-w-5 items-center justify-center rounded-full px-1 text-xs font-bold text-white shadow shadow-primary/30">
                {unread > 99 ? "99+" : unread}
              </span>
            </Link>
          </div>
        ) : null}

        <div className="border-sidebar-border shrink-0 border-t p-4">
          <div className="mb-3 flex items-center gap-3">
            <div className="bg-sidebar-accent text-sidebar-foreground ring-sidebar-border flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-sm font-semibold ring-1">
              {emailInitial}
            </div>
            <p
              className="text-sidebar-foreground/60 min-w-0 flex-1 truncate text-xs"
              title={user?.email ?? undefined}
            >
              {user?.email ?? ""}
            </p>
          </div>
          <button
            type="button"
            onClick={() => void signOut()}
            className="border-sidebar-border bg-sidebar-accent/50 text-sidebar-foreground hover:bg-sidebar-accent hover:text-white flex w-full items-center justify-center gap-2 rounded-lg border px-3 py-2.5 text-sm font-medium transition-colors"
          >
            <LogOut className="h-4 w-4" aria-hidden />
            התנתק
          </button>
          {appVersion ? (
            <p
              className="mt-3 rounded-lg bg-sidebar-accent/80 px-2 py-1.5 text-center text-sm font-semibold tracking-wide text-sidebar-primary tabular-nums ring-1 ring-sidebar-border"
              title={`גרסה ${appVersion}`}
            >
              גרסה {appVersion}
            </p>
          ) : null}
        </div>
      </aside>

      <main className="h-[100dvh] overflow-y-auto scroll-smooth pb-[5.5rem] landscape:pb-16 md:mr-64 md:pb-0">
        <div className="mx-auto max-w-5xl px-4 py-8 landscape:py-4 sm:px-6 lg:px-8 md:py-8">
          <div key={location.pathname} className="animate-fade-in">
            <Outlet />
          </div>
        </div>
      </main>

      <nav className="fixed inset-x-0 bottom-0 z-50 border-t border-border/40 bg-card/80 shadow-[0_-4px_24px_-8px_rgba(0,0,0,0.15)] dark:shadow-[0_-12px_40px_-12px_rgba(0,0,0,0.55)] backdrop-blur-2xl backdrop-saturate-150 md:hidden">
        <div className="flex justify-around px-1 py-3 landscape:py-1.5">
          {navItems.filter((item) => item.mobile).map((item) => {
            const Icon = item.icon;
            const isActive = isNavActive(location.pathname, item.path);
            const showBadge = item.badge && unread > 0;

            return (
              <Link
                key={item.path}
                to={item.path}
                className={cn(
                  "group flex min-w-0 flex-1 flex-col items-center gap-1 landscape:gap-0.5 px-2 py-1 landscape:py-0.5 text-[11px] font-medium transition-all duration-200",
                  "active:scale-[0.94] motion-reduce:active:scale-100",
                  isActive ? "text-primary" : "text-muted-foreground",
                )}
              >
                <span className="flex flex-col items-center gap-1 landscape:gap-0">
                  <span className="relative">
                    <Icon className="h-5 w-5 landscape:h-4 landscape:w-4 transition-transform duration-200 group-active:scale-90" />
                    {showBadge && (
                      <span className="absolute -top-1 -right-2 flex h-3.5 min-w-3.5 items-center justify-center rounded-full bg-destructive px-0.5 text-[9px] font-bold text-white animate-pulse-soft">
                        {unread > 99 ? "99+" : unread}
                      </span>
                    )}
                  </span>
                  {isActive && (
                    <span
                      className="h-1.5 w-1.5 landscape:h-1 landscape:w-1 rounded-full bg-primary shadow-[0_0_10px_2px_rgba(59,130,246,0.65)]"
                      aria-hidden
                    />
                  )}
                </span>
                <span className="line-clamp-1 text-center leading-tight landscape:text-[10px]">
                  {item.label}
                </span>
              </Link>
            );
          })}
          <button
            type="button"
            onClick={toggleTheme}
            aria-label={theme === "dark" ? "הפעל מצב בהיר" : "הפעל מצב כהה"}
            className="flex min-w-0 flex-1 flex-col items-center gap-1 landscape:gap-0.5 px-2 py-1 landscape:py-0.5 text-[11px] font-medium text-muted-foreground transition-all duration-200 active:scale-[0.94]"
          >
            <span className="flex flex-col items-center gap-1 landscape:gap-0">
              {theme === "dark" ? (
                <Sun className="h-5 w-5 landscape:h-4 landscape:w-4" />
              ) : (
                <Moon className="h-5 w-5 landscape:h-4 landscape:w-4" />
              )}
            </span>
            <span className="line-clamp-1 text-center leading-tight landscape:text-[10px]">
              {theme === "dark" ? "בהיר" : "כהה"}
            </span>
          </button>
        </div>
      </nav>
    </div>
  );
}
