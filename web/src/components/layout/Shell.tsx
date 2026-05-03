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
  { path: "/dashboard", label: "לוח בקרה", icon: LayoutDashboard, mobile: true },
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
  if (path === "/dashboard") return pathname === "/dashboard";
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
      <aside className="fixed inset-y-0 right-0 z-40 hidden h-full w-64 flex-col border-l border-[color-mix(in_srgb,var(--color-sidebar-border)_100%,transparent)] bg-sidebar md:flex">
        {/* Base44-style header */}
        <div className="flex items-center gap-3 border-b border-[color-mix(in_srgb,white_6%,transparent)] px-5 py-5">
          <div className="relative flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-[#3b82f6] via-[#3b82f6] to-[#2563eb] text-white shadow-[0_8px_28px_-6px_rgba(59,130,246,0.55),inset_0_1px_0_rgba(255,255,255,0.12)]">
            <Car className="relative z-[1] h-5 w-5 text-white drop-shadow-sm" />
          </div>
          <div className="min-w-0 flex-1">
            <h1 className="text-base leading-none font-bold tracking-tight text-white">
              CarWatch
            </h1>
            <p className="mt-0.5 text-xs text-[color-mix(in_srgb,var(--color-sidebar-foreground)_72%,transparent)]">
              מעקב רכבים חכם
            </p>
          </div>
          <button
            type="button"
            onClick={toggleTheme}
            aria-label={theme === "dark" ? "הפעל מצב בהיר" : "הפעל מצב כהה"}
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-[color-mix(in_srgb,white_6%,transparent)] text-[color-mix(in_srgb,var(--color-sidebar-foreground)_85%,transparent)] transition-all duration-200 hover:bg-[color-mix(in_srgb,white_10%,transparent)] hover:text-white active:scale-[0.96] motion-reduce:active:scale-100"
            title={theme === "dark" ? "מצב בהיר" : "מצב כהה"}
          >
            {theme === "dark" ? <Sun size={15} /> : <Moon size={15} />}
          </button>
        </div>

        <nav className="flex-1 space-y-1 overflow-y-auto px-2.5 py-4">
          {navItems.map((item) => {
            const Icon = item.icon;
            const active = isNavActive(location.pathname, item.path);
            const showBadge = item.badge && unread > 0;

            return (
              <Link
                key={item.path}
                to={item.path}
                className={cn(
                  "group relative flex items-center gap-3 rounded-xl py-2.5 pe-3 ps-4 text-sm font-medium outline-none transition-[background-color,color,box-shadow] duration-200 ease-out",
                  "focus-visible:ring-2 focus-visible:ring-[color-mix(in_srgb,var(--color-sidebar-primary)_38%,transparent)] focus-visible:ring-offset-2 focus-visible:ring-offset-sidebar",
                  active
                    ? "text-white shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--color-sidebar-primary)_24%,transparent)] [background-image:linear-gradient(270deg,color-mix(in_oklab,var(--color-sidebar-primary)_20%,transparent)_0%,color-mix(in_oklab,var(--color-sidebar-primary)_6%,transparent)_55%,transparent_100%)]"
                    : "text-sidebar-foreground hover:bg-[color-mix(in_srgb,white_4.5%,transparent)] hover:text-white",
                )}
              >
                {/* Accent rail — Base44-style strip toward main content */}
                <span
                  className={cn(
                    "pointer-events-none absolute left-0 top-1/2 h-7 w-[3px] -translate-y-1/2 rounded-r-full transition-all duration-200 ease-out",
                    active
                      ? "bg-sidebar-primary shadow-[0_0_14px_rgba(59,130,246,0.55)]"
                      : "bg-[color-mix(in_srgb,var(--color-sidebar-primary)_48%,transparent)] opacity-90 group-hover:bg-sidebar-primary group-hover:opacity-100 group-hover:shadow-[0_0_12px_rgba(59,130,246,0.38)]",
                  )}
                  aria-hidden
                />
                <Icon
                  size={17}
                  className={cn(
                    "relative z-[1] shrink-0 transition-colors duration-200",
                    active
                      ? "text-sidebar-primary drop-shadow-[0_0_10px_rgba(59,130,246,0.35)]"
                      : "text-[color-mix(in_srgb,var(--color-sidebar-foreground)_78%,transparent)] group-hover:text-sidebar-primary",
                  )}
                />
                <span className="relative z-[1]">
                  {item.label}
                  {showBadge ? (
                    <span className="absolute -top-1.5 -right-4 flex h-4 min-w-4 items-center justify-center rounded-full bg-destructive px-1 text-[10px] font-bold text-white animate-pulse-soft">
                      {unread > 99 ? "99+" : unread}
                    </span>
                  ) : null}
                </span>
                {active ? (
                  <div className="relative z-[1] mr-auto h-1.5 w-1.5 rounded-full bg-sidebar-primary shadow-[0_0_12px_2px_rgba(59,130,246,0.65)]" />
                ) : null}
              </Link>
            );
          })}
        </nav>

        {unread > 0 ? (
          <div className="border-t border-[color-mix(in_srgb,white_6%,transparent)] px-3 py-4">
            <Link
              to="/notifications"
              className="flex items-center gap-2.5 rounded-xl border border-[color-mix(in_srgb,var(--color-sidebar-primary)_28%,transparent)] bg-[linear-gradient(270deg,color-mix(in_oklab,var(--color-sidebar-primary)_14%,transparent),color-mix(in_oklab,var(--color-sidebar-primary)_6%,transparent))] px-3 py-2.5 shadow-[inset_0_1px_0_color-mix(in_srgb,white_8%,transparent)] transition-all duration-200 hover:border-[color-mix(in_srgb,var(--color-sidebar-primary)_40%,transparent)] hover:bg-[linear-gradient(270deg,color-mix(in_oklab,var(--color-sidebar-primary)_20%,transparent),color-mix(in_oklab,var(--color-sidebar-primary)_9%,transparent))]"
            >
              <div className="relative shrink-0">
                <Bell className="h-4 w-4 text-sidebar-primary" />
                <span className="absolute -top-1 -right-1 h-2 w-2 animate-pulse-glow rounded-full bg-sidebar-primary" />
              </div>
              <span className="text-xs font-medium text-[#93c5fd]">
                התראות פעילות
              </span>
              <span className="mr-auto flex h-5 min-w-5 shrink-0 items-center justify-center rounded-full bg-sidebar-primary px-1 text-xs font-bold text-white shadow-[0_4px_14px_-4px_rgba(59,130,246,0.65)]">
                {unread > 99 ? "99+" : unread}
              </span>
            </Link>
          </div>
        ) : null}

        <div className="shrink-0 border-t border-[color-mix(in_srgb,white_6%,transparent)] p-4">
          <div className="mb-3 flex items-center gap-3">
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-[color-mix(in_srgb,white_6%,transparent)] text-sm font-semibold text-[color-mix(in_srgb,var(--color-sidebar-foreground)_92%,transparent)] ring-1 ring-[color-mix(in_srgb,white_10%,transparent)]">
              {emailInitial}
            </div>
            <p
              className="min-w-0 flex-1 truncate text-xs text-[color-mix(in_srgb,var(--color-sidebar-foreground)_68%,transparent)]"
              title={user?.email ?? undefined}
            >
              {user?.email ?? ""}
            </p>
          </div>
          <button
            type="button"
            onClick={() => void signOut()}
            className="flex w-full items-center justify-center gap-2 rounded-xl border border-[color-mix(in_srgb,white_8%,transparent)] bg-[color-mix(in_srgb,white_3%,transparent)] px-3 py-2.5 text-sm font-medium text-sidebar-foreground transition-all duration-200 hover:border-[color-mix(in_srgb,white_14%,transparent)] hover:bg-[color-mix(in_srgb,white_7%,transparent)] hover:text-white active:scale-[0.99] motion-reduce:active:scale-100"
          >
            <LogOut className="h-4 w-4" aria-hidden />
            התנתק
          </button>
          {appVersion ? (
            <p
              className="mt-3 rounded-xl border border-[color-mix(in_srgb,var(--color-sidebar-primary)_22%,transparent)] bg-[color-mix(in_srgb,var(--color-sidebar-primary)_8%,transparent)] px-2 py-2 text-center text-sm font-semibold tracking-wide text-[#60a5fa] tabular-nums"
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
