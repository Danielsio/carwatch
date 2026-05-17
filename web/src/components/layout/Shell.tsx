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
import { useMe } from "@/hooks/useMe";
import { ConnectionBanner } from "@/components/ui/ConnectionBanner";
import { cn } from "@/lib/utils";

interface NavItem {
  path: string;
  label: string;
  icon: typeof LayoutDashboard;
  mobile: boolean;
  badge?: boolean;
  adminOnly?: boolean;
}

const navItems: NavItem[] = [
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
  { path: "/admin", label: "ניהול", icon: Wrench, mobile: false, adminOnly: true },
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
  const { data: me } = useMe();
  const isAdmin = me?.is_admin ?? false;
  const visibleNavItems = navItems.filter(
    (item) => !item.adminOnly || isAdmin,
  );
  const emailInitial =
    user?.email?.trim().charAt(0)?.toLocaleUpperCase("he-IL") || "?";

  return (
    <div className="min-h-screen bg-background">
      <ConnectionBanner status={connectionStatus} />
      <aside
        aria-label="ניווט ראשי"
        className="fixed inset-y-0 right-0 z-40 hidden h-full w-64 flex-col border-l border-sidebar-border bg-sidebar md:flex"
      >
        <div className="flex items-center gap-3 border-b border-sidebar-border px-5 py-5">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-primary to-primary/80 text-white shadow-md">
            <Car className="h-5 w-5 text-white drop-shadow-sm" />
          </div>
          <div className="min-w-0 flex-1">
            <h1 className="text-base leading-none font-bold tracking-tight text-foreground dark:text-white">
              CarWatch
            </h1>
            <p className="mt-0.5 text-xs text-sidebar-muted">
              מעקב רכבים חכם
            </p>
          </div>
          <button
            type="button"
            onClick={toggleTheme}
            aria-label={theme === "dark" ? "הפעל מצב בהיר" : "הפעל מצב כהה"}
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-sidebar-accent text-sidebar-foreground transition-all duration-150 hover:bg-sidebar-accent-hover hover:text-foreground dark:hover:text-white active:scale-[0.96] motion-reduce:active:scale-100"
            title={theme === "dark" ? "מצב בהיר" : "מצב כהה"}
          >
            {theme === "dark" ? <Sun size={15} /> : <Moon size={15} />}
          </button>
        </div>

        <nav className="flex-1 space-y-0.5 overflow-y-auto px-3 py-4">
          {visibleNavItems.map((item) => {
            const Icon = item.icon;
            const active = isNavActive(location.pathname, item.path);
            const showBadge = item.badge && unread > 0;

            return (
              <Link
                key={item.path}
                to={item.path}
                aria-current={active ? "page" : undefined}
                className={cn(
                  "group relative flex items-center gap-3 rounded-lg py-2 pe-3 ps-4 text-sm font-medium outline-none transition-all duration-150",
                  "focus-visible:ring-2 focus-visible:ring-primary/30 focus-visible:ring-offset-2 focus-visible:ring-offset-sidebar",
                  active
                    ? "bg-sidebar-active-fade text-primary dark:text-white"
                    : "text-sidebar-foreground hover:bg-sidebar-accent-hover hover:text-foreground dark:hover:text-white",
                )}
              >
                <span
                  className={cn(
                    "pointer-events-none absolute left-0 top-1/2 h-6 w-[3px] -translate-y-1/2 rounded-r-full transition-all duration-150",
                    active
                      ? "bg-primary shadow-[0_0_10px_var(--color-glow-primary)]"
                      : "bg-transparent group-hover:bg-primary/40",
                  )}
                  aria-hidden
                />
                <Icon
                  size={17}
                  className={cn(
                    "relative z-[1] shrink-0 transition-colors duration-150",
                    active
                      ? "text-primary"
                      : "text-sidebar-muted group-hover:text-primary",
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
                  <div className="relative z-[1] mr-auto h-1.5 w-1.5 rounded-full bg-primary shadow-[0_0_8px_2px_var(--color-glow-primary)]" />
                ) : null}
              </Link>
            );
          })}
        </nav>

        {unread > 0 ? (
          <div className="border-t border-sidebar-border px-3 py-4">
            <Link
              to="/notifications"
              className="flex items-center gap-2.5 rounded-lg border border-sidebar-active-edge bg-sidebar-active-fade px-3 py-2.5 transition-all duration-150 hover:bg-primary/10"
            >
              <div className="relative shrink-0">
                <Bell className="h-4 w-4 text-primary" />
                <span className="absolute -top-1 -right-1 h-2 w-2 animate-pulse-glow rounded-full bg-primary" />
              </div>
              <span className="text-xs font-medium text-primary">
                התראות פעילות
              </span>
              <span className="mr-auto flex h-5 min-w-5 shrink-0 items-center justify-center rounded-full bg-primary px-1 text-xs font-bold text-white shadow-sm">
                {unread > 99 ? "99+" : unread}
              </span>
            </Link>
          </div>
        ) : null}

        <div className="shrink-0 border-t border-sidebar-border p-4">
          <div className="mb-3 flex items-center gap-3">
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-sidebar-accent text-sm font-semibold text-sidebar-foreground ring-1 ring-sidebar-border">
              {emailInitial}
            </div>
            <p
              className="min-w-0 flex-1 truncate text-xs text-sidebar-muted"
              title={user?.email ?? undefined}
            >
              {user?.email ?? ""}
            </p>
          </div>
          <button
            type="button"
            onClick={() => void signOut()}
            className="flex w-full items-center justify-center gap-2 rounded-lg border border-sidebar-border bg-sidebar-accent px-3 py-2.5 text-sm font-medium text-sidebar-foreground transition-all duration-150 hover:bg-sidebar-accent-hover hover:text-foreground dark:hover:text-white active:scale-[0.99] motion-reduce:active:scale-100"
          >
            <LogOut className="h-4 w-4" aria-hidden />
            התנתק
          </button>
          {appVersion ? (
            <p
              className="mt-2 text-center text-[11px] text-sidebar-muted/60 tabular-nums"
              title={`v${appVersion}`}
            >
              v{appVersion}
            </p>
          ) : null}
        </div>
      </aside>

      <main className="h-[100dvh] overflow-y-auto scroll-smooth pb-[calc(4rem+env(safe-area-inset-bottom,0px))] landscape:pb-16 md:mr-64 md:pb-0">
        <div className="mx-auto max-w-5xl px-4 py-6 landscape:py-4 sm:px-6 md:py-8 lg:px-8">
          <div key={location.pathname} className="animate-fade-in">
            <Outlet />
          </div>
        </div>
      </main>

      <nav
        aria-label="ניווט"
        className="fixed inset-x-0 bottom-0 z-50 border-t border-border/50 bg-card/90 shadow-[0_-2px_16px_-6px_rgba(0,0,0,0.1)] dark:shadow-[0_-8px_32px_-8px_rgba(0,0,0,0.4)] backdrop-blur-xl backdrop-saturate-150 pb-[env(safe-area-inset-bottom,0px)] md:hidden"
      >
        <div className="flex justify-around px-1 py-2 landscape:py-1.5">
          {visibleNavItems.filter((item) => item.mobile).map((item) => {
            const Icon = item.icon;
            const isActive = isNavActive(location.pathname, item.path);
            const showBadge = item.badge && unread > 0;

            return (
              <Link
                key={item.path}
                to={item.path}
                aria-current={isActive ? "page" : undefined}
                className={cn(
                  "group flex min-w-0 flex-1 flex-col items-center gap-1 landscape:gap-0.5 px-2 py-2 landscape:py-0.5 text-[11px] font-medium transition-all duration-150",
                  "active:scale-[0.94] motion-reduce:active:scale-100",
                  isActive ? "text-primary" : "text-muted-foreground",
                )}
              >
                <span className="flex flex-col items-center gap-1 landscape:gap-0">
                  <span className="relative">
                    <Icon className="h-5 w-5 landscape:h-4 landscape:w-4 transition-transform duration-150 group-active:scale-90" />
                    {showBadge && (
                      <span className="absolute -top-1 -right-2 flex h-3.5 min-w-3.5 items-center justify-center rounded-full bg-destructive px-0.5 text-[9px] font-bold text-white animate-pulse-soft">
                        {unread > 99 ? "99+" : unread}
                      </span>
                    )}
                  </span>
                  {isActive && (
                    <span
                      className="h-1.5 w-1.5 landscape:h-1 landscape:w-1 rounded-full bg-primary shadow-[0_0_8px_2px_var(--color-glow-primary)]"
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
        </div>
      </nav>
    </div>
  );
}
