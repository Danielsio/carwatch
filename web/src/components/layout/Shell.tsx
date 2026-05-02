import { Outlet, Link, useLocation } from "react-router";
import {
  Search,
  Plus,
  Car,
  Settings,
  Bookmark,
  Clock,
  Bell,
  LogOut,
  Sun,
  Moon,
} from "lucide-react";
import { useNotificationCount } from "@/hooks/useNotifications";
import { useAppVersion } from "@/hooks/useAppVersion";
import { useAuth } from "@/contexts/AuthContext";
import { useTheme } from "@/contexts/ThemeContext";
import { cn } from "@/lib/utils";

const navItems = [
  { path: "/", label: "חיפושים", icon: Search, mobile: true },
  { path: "/searches/new", label: "חיפוש חדש", icon: Plus, mobile: false },
  { path: "/saved", label: "שמורים", icon: Bookmark, mobile: true },
  { path: "/history", label: "היסטוריה", icon: Clock, mobile: true },
  { path: "/notifications", label: "התראות", icon: Bell, badge: true, mobile: true },
  { path: "/admin", label: "ניהול", icon: Settings, mobile: false },
];

export function Shell() {
  const location = useLocation();
  const { data: notifCount } = useNotificationCount();
  const unread = notifCount?.count ?? 0;
  const { user, signOut } = useAuth();
  const { theme, toggle: toggleTheme } = useTheme();
  const appVersion = useAppVersion();
  const emailInitial =
    user?.email?.trim().charAt(0)?.toLocaleUpperCase("he-IL") || "?";

  return (
    <div className="min-h-screen bg-background">
      <aside className="fixed inset-y-0 right-0 z-50 hidden w-64 flex-col border-l border-sidebar-border bg-sidebar md:flex">
        <div className="flex items-center gap-3 px-5 py-5 border-b border-sidebar-border">
          <div className="relative w-10 h-10 shrink-0 rounded-xl bg-sidebar-primary flex items-center justify-center shadow-lg shadow-sidebar-primary/40">
            <Car className="w-5 h-5 text-white" />
            <div className="absolute inset-0 rounded-xl bg-gradient-to-br from-white/20 to-transparent" />
          </div>
          <div className="flex-1 min-w-0">
            <h1 className="font-bold text-base text-white leading-none tracking-tight">
              CarWatch
            </h1>
            <p className="text-xs text-sidebar-foreground/60 mt-0.5">
              מעקב רכבים חכם
            </p>
          </div>
          <button
            type="button"
            onClick={toggleTheme}
            aria-label={theme === "dark" ? "הפעל מצב בהיר" : "הפעל מצב כהה"}
            className="w-7 h-7 shrink-0 rounded-lg bg-sidebar-accent flex items-center justify-center text-sidebar-foreground/60 hover:text-sidebar-foreground hover:bg-sidebar-accent/80 transition-all"
            title={theme === "dark" ? "מצב בהיר" : "מצב כהה"}
          >
            {theme === "dark" ? <Sun size={14} /> : <Moon size={14} />}
          </button>
        </div>

        <nav className="flex-1 px-3 py-4 space-y-0.5 overflow-y-auto">
          {navItems.map((item) => {
            const Icon = item.icon;
            const isActive =
              item.path === "/"
                ? location.pathname === "/"
                : location.pathname.startsWith(item.path);
            const showBadge = item.badge && unread > 0;

            return (
              <Link
                key={item.path}
                to={item.path}
                className={cn(
                  "flex items-center gap-3 px-4 py-2.5 rounded-lg text-sm font-medium transition-all duration-150",
                  isActive
                    ? "bg-sidebar-primary/15 text-white"
                    : "text-sidebar-foreground hover:bg-sidebar-accent hover:text-white",
                )}
              >
                <Icon
                  size={17}
                  className={cn(
                    "shrink-0 transition-colors",
                    isActive
                      ? "text-sidebar-primary"
                      : "text-sidebar-foreground/70",
                  )}
                />
                <span className="relative">
                  {item.label}
                  {showBadge && (
                    <span className="absolute -top-1.5 -right-4 flex h-4 min-w-4 items-center justify-center rounded-full bg-destructive px-1 text-[10px] font-bold text-white animate-pulse-soft">
                      {unread > 99 ? "99+" : unread}
                    </span>
                  )}
                </span>
                {isActive && (
                  <div className="mr-auto w-1.5 h-1.5 rounded-full bg-sidebar-primary shadow-sm shadow-sidebar-primary/60" />
                )}
              </Link>
            );
          })}
        </nav>

        {unread > 0 && (
          <div className="px-4 py-4 border-t border-sidebar-border">
            <Link
              to="/notifications"
              className="flex items-center gap-2.5 px-3 py-2.5 rounded-xl bg-sidebar-primary/10 border border-sidebar-primary/20 transition-colors hover:bg-sidebar-primary/15"
            >
              <div className="relative">
                <Bell className="w-4 h-4 text-sidebar-primary" />
                <span className="absolute -top-1 -right-1 w-2 h-2 rounded-full bg-sidebar-primary animate-pulse-soft" />
              </div>
              <span className="text-xs text-sidebar-primary font-medium">
                התראות פעילות
              </span>
              <span className="mr-auto flex h-5 min-w-5 items-center justify-center rounded-full bg-sidebar-primary px-1 text-xs font-bold text-white shadow shadow-sidebar-primary/30">
                {unread > 99 ? "99+" : unread}
              </span>
            </Link>
          </div>
        )}

        <div className="shrink-0 border-t border-sidebar-border p-4">
          <div className="mb-3 flex items-center gap-3">
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-sidebar-accent text-sm font-semibold text-sidebar-foreground ring-1 ring-sidebar-border">
              {emailInitial}
            </div>
            <p
              className="min-w-0 flex-1 truncate text-xs text-sidebar-foreground/60"
              title={user?.email ?? undefined}
            >
              {user?.email ?? ""}
            </p>
          </div>
          <button
            type="button"
            onClick={() => void signOut()}
            className="flex w-full items-center justify-center gap-2 rounded-lg border border-sidebar-border bg-sidebar-accent/50 px-3 py-2.5 text-sm font-medium text-sidebar-foreground transition-colors hover:bg-sidebar-accent hover:text-white"
          >
            <LogOut className="h-4 w-4" aria-hidden />
            התנתק
          </button>
          {appVersion && (
            <p className="mt-2 text-center text-[10px] text-sidebar-foreground/30 tabular-nums">
              v{appVersion}
            </p>
          )}
        </div>
      </aside>

      <main className="h-[100dvh] overflow-y-auto scroll-smooth pb-[5.5rem] md:mr-64 md:pb-0">
        <div className="mx-auto max-w-5xl px-4 py-8 sm:px-6 lg:px-8">
          <div key={location.pathname} className="animate-fade-in">
            <Outlet />
          </div>
        </div>
      </main>

      <nav className="fixed inset-x-0 bottom-0 z-50 border-t border-border/40 bg-card/80 shadow-[0_-4px_24px_-8px_rgba(0,0,0,0.15)] dark:shadow-[0_-12px_40px_-12px_rgba(0,0,0,0.55)] backdrop-blur-2xl backdrop-saturate-150 md:hidden">
        <div className="flex justify-around px-1 py-3">
          {navItems.filter((item) => item.mobile).map((item) => {
            const Icon = item.icon;
            const isActive =
              item.path === "/"
                ? location.pathname === "/"
                : location.pathname.startsWith(item.path);
            const showBadge = item.badge && unread > 0;

            return (
              <Link
                key={item.path}
                to={item.path}
                className={cn(
                  "group flex min-w-0 flex-1 flex-col items-center gap-1 px-2 py-1 text-[11px] font-medium transition-all duration-200",
                  "active:scale-[0.94] motion-reduce:active:scale-100",
                  isActive ? "text-primary" : "text-muted-foreground",
                )}
              >
                <span className="flex flex-col items-center gap-1">
                  <span className="relative">
                    <Icon className="h-5 w-5 transition-transform duration-200 group-active:scale-90" />
                    {showBadge && (
                      <span className="absolute -top-1 -right-2 flex h-3.5 min-w-3.5 items-center justify-center rounded-full bg-destructive px-0.5 text-[9px] font-bold text-white animate-pulse-soft">
                        {unread > 99 ? "99+" : unread}
                      </span>
                    )}
                  </span>
                  {isActive && (
                    <span
                      className="h-1.5 w-1.5 rounded-full bg-primary shadow-[0_0_10px_2px_rgba(59,130,246,0.65)]"
                      aria-hidden
                    />
                  )}
                </span>
                <span className="line-clamp-1 text-center leading-tight">
                  {item.label}
                </span>
              </Link>
            );
          })}
          <button
            type="button"
            onClick={toggleTheme}
            aria-label={theme === "dark" ? "הפעל מצב בהיר" : "הפעל מצב כהה"}
            className="flex min-w-0 flex-1 flex-col items-center gap-1 px-2 py-1 text-[11px] font-medium text-muted-foreground transition-all duration-200 active:scale-[0.94]"
          >
            <span className="flex flex-col items-center gap-1">
              {theme === "dark" ? (
                <Sun className="h-5 w-5" />
              ) : (
                <Moon className="h-5 w-5" />
              )}
            </span>
            <span className="line-clamp-1 text-center leading-tight">
              {theme === "dark" ? "בהיר" : "כהה"}
            </span>
          </button>
        </div>
      </nav>
    </div>
  );
}
