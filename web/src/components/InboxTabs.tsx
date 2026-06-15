import type { ReactNode } from "react";
import { Link, useLocation } from "react-router";
import { Bell, Bookmark, History } from "lucide-react";
import { useNotificationCount } from "@/hooks/useNotifications";
import { cn } from "@/lib/utils";

const TABS = [
  { to: "/notifications", label: "התראות", icon: Bell, badge: true },
  { to: "/saved", label: "מועדפים", icon: Bookmark, badge: false },
  { to: "/history", label: "היסטוריה", icon: History, badge: false },
] as const;

/**
 * Unified header for the inbox surfaces (Notifications / Saved / History).
 * Renders the tab bar (with the live unread badge) plus an optional per-page
 * action slot, replacing the standalone PageHeader on those pages.
 */
export function InboxTabs({ action }: { action?: ReactNode }) {
  const { pathname } = useLocation();
  const { data: count } = useNotificationCount();
  const unread = count?.count ?? 0;

  return (
    <header className="flex flex-col gap-3 border-b border-border/50 pb-4 dir-rtl">
      <div className="flex items-center justify-between gap-3">
        <nav
          className="flex gap-1 overflow-x-auto scrollbar-hide"
          aria-label="תיבת דואר"
        >
          {TABS.map((t) => {
            const active = pathname === t.to;
            const Icon = t.icon;
            return (
              <Link
                key={t.to}
                to={t.to}
                aria-current={active ? "page" : undefined}
                className={cn(
                  "relative flex shrink-0 items-center gap-2 rounded-xl px-3.5 py-2 text-sm font-medium transition-all duration-150",
                  active
                    ? "bg-primary/10 text-primary ring-1 ring-primary/20"
                    : "text-muted-foreground hover:bg-surface-hover hover:text-foreground",
                )}
              >
                <Icon className="h-4 w-4" aria-hidden />
                {t.label}
                {t.badge && unread > 0 ? (
                  <span className="flex h-4 min-w-4 items-center justify-center rounded-full bg-primary px-1 text-[10px] font-bold text-white tabular-nums">
                    {unread > 99 ? "99+" : unread}
                  </span>
                ) : null}
              </Link>
            );
          })}
        </nav>
        {action ? <div className="shrink-0">{action}</div> : null}
      </div>
    </header>
  );
}
