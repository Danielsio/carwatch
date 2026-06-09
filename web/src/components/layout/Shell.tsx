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
  LogIn,
  Sun,
  Moon,
  User,
  Search,
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
  badge?: boolean;
  adminOnly?: boolean;
  authOnly?: boolean;
  guestOnly?: boolean;
}

const mainNav: NavItem[] = [
  { path: "/dashboard", label: "לוח בקרה", icon: LayoutDashboard },
  { path: "/searches/new", label: "חיפוש חדש", icon: Plus, authOnly: true },
];

const libraryNav: NavItem[] = [
  { path: "/saved", label: "מועדפים", icon: Bookmark, authOnly: true },
  { path: "/history", label: "היסטוריה", icon: History, authOnly: true },
  { path: "/notifications", label: "התראות", icon: Bell, badge: true, authOnly: true },
];

const systemNav: NavItem[] = [
  { path: "/settings", label: "הגדרות", icon: Settings, authOnly: true },
  { path: "/admin", label: "ניהול", icon: Wrench, adminOnly: true },
  { path: "/try", label: "נסה חיפוש", icon: Search, guestOnly: true },
];

interface MobileNavItem {
  path: string;
  label: string;
  icon: typeof LayoutDashboard;
  badge?: boolean;
  fab?: boolean;
  authOnly?: boolean;
}

const mobileNav: MobileNavItem[] = [
  { path: "/dashboard", label: "ראשי", icon: LayoutDashboard, authOnly: true },
  { path: "/saved", label: "מועדפים", icon: Bookmark, authOnly: true },
  { path: "/searches/new", label: "חדש", icon: Plus, fab: true, authOnly: true },
  { path: "/notifications", label: "התראות", icon: Bell, badge: true, authOnly: true },
  { path: "/settings", label: "הגדרות", icon: Settings, authOnly: true },
];

function isNavActive(pathname: string, path: string): boolean {
  if (path === "/dashboard") return pathname === "/dashboard";
  return pathname === path || pathname.startsWith(`${path}/`);
}

function SidebarNavItem({
  item,
  active,
  unread,
}: {
  item: NavItem;
  active: boolean;
  unread: number;
}) {
  const Icon = item.icon;
  const showBadge = item.badge && unread > 0;

  return (
    <Link
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
          "pointer-events-none absolute start-0 top-1/2 h-6 w-[3px] -translate-y-1/2 rounded-r-full transition-all duration-150",
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
        <div className="relative z-[1] ms-auto h-1.5 w-1.5 rounded-full bg-primary shadow-[0_0_8px_2px_var(--color-glow-primary)]" />
      ) : null}
    </Link>
  );
}

function SidebarSection({
  label,
  items,
  pathname,
  unread,
  isAdmin,
  isAuthenticated,
}: {
  label: string;
  items: NavItem[];
  pathname: string;
  unread: number;
  isAdmin: boolean;
  isAuthenticated: boolean;
}) {
  const visibleItems = items.filter(
    (item) => (!item.adminOnly || isAdmin) && (!item.authOnly || isAuthenticated) && (!item.guestOnly || !isAuthenticated),
  );
  if (visibleItems.length === 0) return null;

  return (
    <div>
      <p className="mb-1 px-4 text-[11px] font-semibold tracking-wide text-sidebar-muted uppercase">
        {label}
      </p>
      <div className="space-y-0.5">
        {visibleItems.map((item) => (
          <SidebarNavItem
            key={item.path}
            item={item}
            active={isNavActive(pathname, item.path)}
            unread={unread}
          />
        ))}
      </div>
    </div>
  );
}

export function Shell() {
  const location = useLocation();
  const { user, signOut } = useAuth();
  const { data: notifCount } = useNotificationCount(!!user);
  const unread = notifCount?.count ?? 0;
  const { theme, toggle: toggleTheme } = useTheme();
  const appVersion = useAppVersion();
  const connectionStatus = useHealthCheck();
  const { data: me } = useMe(!!user);
  const isAdmin = me?.is_admin ?? false;
  const emailInitial =
    user?.email?.trim().charAt(0)?.toLocaleUpperCase("he-IL") || "?";

  return (
    <div className="min-h-screen bg-background">
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:fixed focus:top-4 focus:right-4 focus:z-[60] focus:rounded-xl focus:bg-primary focus:px-4 focus:py-2 focus:text-sm focus:font-semibold focus:text-white focus:shadow-lg"
      >
        דלג לתוכן
      </a>
      <ConnectionBanner status={connectionStatus} />

      {/* ── Desktop Sidebar ── */}
      <aside
        aria-label="ניווט ראשי"
        className="fixed inset-y-0 right-0 z-40 hidden h-full w-60 flex-col border-l border-sidebar-border bg-sidebar md:flex"
      >
        {/* Brand */}
        <div className="flex items-center gap-3 border-b border-sidebar-border px-4 py-5">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-primary via-primary to-blue-400 text-white shadow-[0_4px_16px_-4px_var(--color-glow-primary)]">
            <Car className="h-5 w-5 text-white" />
          </div>
          <div className="min-w-0 flex-1">
            <h1 className="text-[15px] leading-none font-extrabold tracking-tight text-foreground dark:text-white">
              CarWatch
            </h1>
            <p className="mt-1 text-[10px] font-medium tracking-widest text-sidebar-muted uppercase">
              Smart Tracking
            </p>
          </div>
          <button
            type="button"
            onClick={toggleTheme}
            aria-label={theme === "dark" ? "הפעל מצב בהיר" : "הפעל מצב כהה"}
            className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-sidebar-accent text-sidebar-foreground transition-all duration-150 hover:bg-sidebar-accent-hover hover:text-foreground dark:hover:text-white active:scale-[0.96] motion-reduce:active:scale-100"
            title={theme === "dark" ? "מצב בהיר" : "מצב כהה"}
          >
            {theme === "dark" ? <Sun size={14} /> : <Moon size={14} />}
          </button>
        </div>

        {/* Navigation */}
        <nav className="flex-1 space-y-5 overflow-y-auto px-2.5 py-4">
          <SidebarSection label="ראשי" items={mainNav} pathname={location.pathname} unread={unread} isAdmin={isAdmin} isAuthenticated={!!user} />
          <SidebarSection label="ספריה" items={libraryNav} pathname={location.pathname} unread={unread} isAdmin={isAdmin} isAuthenticated={!!user} />
          <SidebarSection label="מערכת" items={systemNav} pathname={location.pathname} unread={unread} isAdmin={isAdmin} isAuthenticated={!!user} />
        </nav>

        {/* Notification Banner */}
        {unread > 0 ? (
          <div className="border-t border-sidebar-border px-3 py-3">
            <Link
              to="/notifications"
              className="flex items-center gap-2.5 rounded-lg border border-sidebar-active-edge bg-sidebar-active-fade px-3 py-2 transition-all duration-150 hover:bg-primary/10"
            >
              <div className="relative shrink-0">
                <Bell className="h-4 w-4 text-primary" />
                <span className="absolute -top-1 -right-1 h-2 w-2 animate-pulse-glow rounded-full bg-primary" />
              </div>
              <span className="text-xs font-medium text-primary">
                התראות חדשות
              </span>
              <span className="ms-auto flex h-5 min-w-5 shrink-0 items-center justify-center rounded-full bg-primary px-1 text-xs font-bold text-white shadow-sm">
                {unread > 99 ? "99+" : unread}
              </span>
            </Link>
          </div>
        ) : null}

        {/* Guest signup banner */}
        {!user && (
          <div className="mx-4 mb-4 rounded-xl border border-primary/20 bg-primary/5 p-3 text-center text-xs text-muted-foreground">
            <Link to="/login" className="font-medium text-primary hover:underline">
              הירשם בחינם
            </Link>{" "}
            כדי לשמור חיפושים ולקבל התראות
          </div>
        )}

        {/* User */}
        <div className="shrink-0 border-t border-sidebar-border p-3">
          <div className="mb-2.5 flex items-center gap-2.5">
            <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-sidebar-accent text-xs font-semibold text-sidebar-foreground ring-1 ring-sidebar-border">
              {user ? emailInitial : <User className="h-4 w-4" />}
            </div>
            <p
              className="min-w-0 flex-1 truncate text-xs text-sidebar-muted"
              title={user?.email ?? undefined}
            >
              {user?.email ?? "אורח"}
            </p>
          </div>
          {user ? (
            <button
              type="button"
              onClick={() => void signOut()}
              className="flex w-full items-center justify-center gap-2 rounded-lg border border-sidebar-border bg-sidebar-accent px-3 py-2 text-sm font-medium text-sidebar-foreground transition-all duration-150 hover:bg-sidebar-accent-hover hover:text-foreground dark:hover:text-white active:scale-[0.99] motion-reduce:active:scale-100"
            >
              <LogOut className="h-3.5 w-3.5" aria-hidden />
              התנתק
            </button>
          ) : (
            <Link
              to="/login"
              className="flex w-full items-center justify-center gap-2 rounded-lg border border-primary/30 bg-primary/10 px-3 py-2 text-sm font-medium text-primary transition-all duration-150 hover:bg-primary/20 active:scale-[0.99] motion-reduce:active:scale-100"
            >
              <LogIn className="h-3.5 w-3.5" aria-hidden />
              התחבר
            </Link>
          )}
          {appVersion ? (
            <p
              className="mt-1.5 text-center text-[10px] text-sidebar-muted/50 tabular-nums"
              title={`v${appVersion}`}
            >
              v{appVersion}
            </p>
          ) : null}
        </div>
      </aside>

      {/* ── Main Content ── */}
      <main id="main-content" className="h-[100dvh] overflow-y-auto scroll-smooth pb-[var(--bottom-nav-h)] landscape:pb-16 md:me-60 md:pb-0">
        <div className="mx-auto max-w-5xl px-4 py-6 landscape:py-4 sm:px-6 md:py-8 lg:px-8">
          <div key={location.pathname} className="animate-fade-in">
            <Outlet />
          </div>
        </div>
      </main>

      {/* ── Mobile Bottom Nav ── */}
      <nav
        aria-label="ניווט"
        className="fixed inset-x-0 bottom-0 z-50 border-t border-border/50 bg-card/90 shadow-[0_-2px_16px_-6px_rgba(0,0,0,0.08)] dark:shadow-[0_-4px_24px_-8px_rgba(0,0,0,0.4)] backdrop-blur-xl backdrop-saturate-150 pb-[env(safe-area-inset-bottom,0px)] md:hidden"
      >
        <div className="flex items-end justify-around px-2 py-1.5 landscape:py-1">
          {mobileNav.filter((item) => !item.authOnly || !!user).map((item) => {
            const Icon = item.icon;
            const isActive = isNavActive(location.pathname, item.path);
            const showBadge = item.badge && unread > 0;

            if (item.fab) {
              return (
                <Link
                  key={item.path}
                  to={item.path}
                  aria-label={item.label}
                  aria-current={isActive ? "page" : undefined}
                  className="group relative -mt-4 flex flex-col items-center"
                >
                  <span className="flex h-12 w-12 items-center justify-center rounded-full bg-primary text-white shadow-[0_4px_14px_-2px_var(--color-glow-primary)] transition-all duration-150 active:scale-[0.92] motion-reduce:active:scale-100">
                    <Plus className="h-5.5 w-5.5" />
                  </span>
                  <span className="mt-0.5 text-[10px] font-medium text-primary landscape:hidden">
                    {item.label}
                  </span>
                </Link>
              );
            }

            return (
              <Link
                key={item.path}
                to={item.path}
                aria-label={item.label}
                aria-current={isActive ? "page" : undefined}
                className={cn(
                  "group flex min-w-0 flex-1 flex-col items-center gap-0.5 px-1 py-1.5 landscape:py-0.5 text-[10px] font-medium transition-all duration-150",
                  "active:scale-[0.94] motion-reduce:active:scale-100",
                  isActive ? "text-primary" : "text-muted-foreground",
                )}
              >
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
                    className="h-1 w-1 rounded-full bg-primary shadow-[0_0_6px_1px_var(--color-glow-primary)]"
                    aria-hidden
                  />
                )}
                <span className="line-clamp-1 text-center leading-tight landscape:text-[9px]">
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
