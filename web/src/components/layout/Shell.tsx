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
} from "lucide-react";
import { useNotificationCount } from "@/hooks/useNotifications";
import { useAuth } from "@/contexts/AuthContext";
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
  const emailInitial =
    user?.email?.trim().charAt(0)?.toLocaleUpperCase("he-IL") ?? "?";

  return (
    <div className="min-h-screen bg-background">
      <aside className="fixed inset-y-0 right-0 z-50 hidden w-64 flex-col border-l border-border/40 bg-card/85 backdrop-blur-xl md:flex">
        {/* Animated gradient accent on inner (left) edge */}
        <span
          aria-hidden
          className="pointer-events-none absolute inset-y-3 left-0 w-px overflow-hidden rounded-full"
        >
          <span className="absolute inset-y-0 left-0 w-[3px] -translate-x-px bg-gradient-to-b from-primary/70 via-primary/35 to-primary/60 opacity-80 blur-[0.5px]" />
          <span className="absolute inset-0 motion-safe:animate-pulse bg-gradient-to-b from-transparent via-primary/50 to-transparent opacity-40 motion-safe:[animation-duration:3.25s]" />
        </span>

        <div className="relative shrink-0 bg-gradient-to-b from-primary/5 to-transparent">
          <div className="flex h-20 shrink-0 items-center gap-3 px-6">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-primary/12 ring-1 ring-primary/15 shadow-[0_0_20px_-4px_rgba(59,130,246,0.35)]">
              <Car className="h-4.5 w-4.5 text-primary" />
            </div>
            <div className="flex flex-col">
              <span className="text-lg font-semibold tracking-tight leading-tight">
                CarWatch
              </span>
              <span className="text-[11px] text-muted-foreground leading-tight">
                מעקב רכבים
              </span>
            </div>
          </div>
          <div
            className="h-px w-full bg-gradient-to-l from-transparent via-primary/25 to-transparent opacity-80"
            aria-hidden
          />
        </div>

        <nav className="mt-2 flex flex-1 flex-col gap-1 overflow-y-auto p-3">
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
                  "group relative flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium transition-all duration-200",
                  isActive
                    ? "bg-primary/[0.22] text-primary shadow-[inset_0_0_0_1px_rgba(59,130,246,0.28),0_1px_24px_-8px_rgba(59,130,246,0.25)]"
                    : "text-muted-foreground hover:bg-secondary hover:text-foreground",
                )}
              >
                <span className="relative">
                  <Icon className="h-[18px] w-[18px]" />
                  {showBadge && (
                    <span className="absolute -top-1.5 -right-1.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-destructive px-1 text-[10px] font-bold text-white animate-pulse-soft">
                      {unread > 99 ? "99+" : unread}
                    </span>
                  )}
                </span>
                {item.label}
                {isActive && (
                  <>
                    <span
                      className="absolute left-0 top-1/2 h-6 w-[3px] -translate-y-1/2 rounded-full bg-primary shadow-[0_0_16px_4px_rgba(59,130,246,0.55),0_0_6px_2px_rgba(59,130,246,0.35)]"
                      aria-hidden
                    />
                    <span
                      className="absolute left-0 top-1/2 h-8 w-5 -translate-y-1/2 rounded-full bg-primary/20 blur-md"
                      aria-hidden
                    />
                  </>
                )}
              </Link>
            );
          })}
        </nav>

        <div className="relative shrink-0 border-t border-border/50 bg-gradient-to-t from-card/95 to-card/70 p-4 backdrop-blur-sm">
          <div className="mb-3 flex items-center gap-3">
            <div
              className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-primary/30 via-primary/10 to-primary/5 text-sm font-semibold text-primary shadow-inner ring-1 ring-primary/25"
              aria-hidden
            >
              {emailInitial}
            </div>
            <p
              className="min-w-0 flex-1 truncate text-xs text-muted-foreground"
              title={user?.email ?? undefined}
            >
              {user?.email ?? ""}
            </p>
          </div>
          <button
            type="button"
            onClick={() => void signOut()}
            className={cn(
              "flex w-full items-center justify-center gap-2 rounded-xl border border-border/60 bg-secondary/50 px-3 py-2.5 text-sm font-medium text-foreground transition-colors",
              "hover:bg-secondary hover:border-border",
            )}
          >
            <LogOut className="h-4 w-4 text-muted-foreground" aria-hidden />
            התנתק
          </button>
        </div>
      </aside>

      <main className="h-[100dvh] overflow-y-auto scroll-smooth pb-[5.5rem] md:mr-64 md:pb-0">
        <div className="mx-auto max-w-5xl px-4 py-8 sm:px-6 lg:px-8">
          <div key={location.pathname} className="animate-fade-in">
            <Outlet />
          </div>
        </div>
      </main>

      <nav className="fixed inset-x-0 bottom-0 z-50 border-t border-white/[0.06] bg-card/55 shadow-[0_-12px_40px_-12px_rgba(0,0,0,0.55)] backdrop-blur-2xl backdrop-saturate-150 md:hidden">
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
        </div>
      </nav>
    </div>
  );
}
