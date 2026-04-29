import { Outlet, Link, useLocation } from "react-router";
import {
  Search,
  Plus,
  Car,
  Settings,
  Bookmark,
  Clock,
  Bell,
} from "lucide-react";
import { useNotificationCount } from "@/hooks/useNotifications";
import { cn } from "@/lib/utils";

const navItems = [
  { path: "/", label: "חיפושים", icon: Search },
  { path: "/searches/new", label: "חיפוש חדש", icon: Plus },
  { path: "/saved", label: "שמורים", icon: Bookmark },
  { path: "/history", label: "היסטוריה", icon: Clock },
  { path: "/notifications", label: "התראות", icon: Bell, badge: true },
  { path: "/admin", label: "ניהול", icon: Settings },
];

export function Shell() {
  const location = useLocation();
  const { data: notifCount } = useNotificationCount();
  const unread = notifCount?.count ?? 0;

  return (
    <div className="min-h-screen bg-background">
      {/* Desktop sidebar */}
      <aside className="fixed inset-y-0 right-0 z-50 hidden w-64 border-l border-border/50 bg-card/80 backdrop-blur-xl md:block">
        <div className="flex h-16 items-center gap-3 border-b border-border/50 px-6">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary/10">
            <Car className="h-4.5 w-4.5 text-primary" />
          </div>
          <span className="text-lg font-semibold tracking-tight">
            CarWatch
          </span>
        </div>

        <nav className="flex flex-col gap-1 p-3 mt-2">
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
                    ? "bg-primary/10 text-primary shadow-[inset_0_0_0_1px_rgba(59,130,246,0.15)]"
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
                  <span className="absolute left-0 top-1/2 -translate-y-1/2 h-5 w-[3px] rounded-full bg-primary" />
                )}
              </Link>
            );
          })}
        </nav>
      </aside>

      {/* Main content */}
      <main className="pb-20 md:mr-64 md:pb-0">
        <div className="mx-auto max-w-5xl px-4 py-8 sm:px-6 lg:px-8 animate-fade-in">
          <Outlet />
        </div>
      </main>

      {/* Mobile bottom nav */}
      <nav className="fixed inset-x-0 bottom-0 z-50 border-t border-border/50 bg-card/80 backdrop-blur-xl md:hidden">
        <div className="flex justify-around py-1.5">
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
                  "flex flex-col items-center gap-0.5 px-3 py-1.5 text-[11px] font-medium transition-colors duration-200",
                  isActive ? "text-primary" : "text-muted-foreground",
                )}
              >
                <span className="relative">
                  <Icon className="h-5 w-5" />
                  {showBadge && (
                    <span className="absolute -top-1 -right-1.5 flex h-3.5 min-w-3.5 items-center justify-center rounded-full bg-destructive px-0.5 text-[9px] font-bold text-white animate-pulse-soft">
                      {unread > 99 ? "99+" : unread}
                    </span>
                  )}
                </span>
                {item.label}
                {isActive && (
                  <span className="h-[3px] w-4 rounded-full bg-primary mt-0.5" />
                )}
              </Link>
            );
          })}
        </div>
      </nav>
    </div>
  );
}
