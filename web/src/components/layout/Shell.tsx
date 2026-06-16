import { useState } from "react";
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
  Menu,
} from "lucide-react";
import { useNotificationCount } from "@/hooks/useNotifications";
import { useAppVersion } from "@/hooks/useAppVersion";
import { useAuth } from "@/contexts/AuthContext";
import { useTheme } from "@/contexts/ThemeContext";
import { useHealthCheck } from "@/hooks/useHealthCheck";
import { useMe } from "@/hooks/useMe";
import { useKeyboardShortcuts } from "@/hooks/useKeyboardShortcuts";
import { ConnectionBanner } from "@/components/ui/ConnectionBanner";
import { AuroraBackground } from "@/components/ui/AuroraBackground";
import { AppCommandPalette } from "@/components/AppCommandPalette";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { Button } from "@/components/ui/button";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuBadge,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  SidebarTrigger,
} from "@/components/ui/sidebar";
import { Separator } from "@/components/ui/separator";
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
  {
    path: "/notifications",
    label: "התראות",
    icon: Bell,
    badge: true,
    authOnly: true,
  },
];

const systemNav: NavItem[] = [
  { path: "/settings", label: "הגדרות", icon: Settings, authOnly: true },
  { path: "/admin", label: "ניהול", icon: Wrench, adminOnly: true },
  { path: "/try", label: "נסה חיפוש", icon: Search, guestOnly: true },
];

function isNavActive(pathname: string, path: string): boolean {
  if (path === "/dashboard") return pathname === "/dashboard";
  return pathname === path || pathname.startsWith(`${path}/`);
}

function filterNavItems(
  items: NavItem[],
  isAdmin: boolean,
  isAuthenticated: boolean,
): NavItem[] {
  return items.filter(
    (item) =>
      (!item.adminOnly || isAdmin) &&
      (!item.authOnly || isAuthenticated) &&
      (!item.guestOnly || !isAuthenticated),
  );
}

function NavSection({
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
  const visible = filterNavItems(items, isAdmin, isAuthenticated);
  if (visible.length === 0) return null;

  return (
    <SidebarGroup>
      <SidebarGroupLabel>{label}</SidebarGroupLabel>
      <SidebarMenu>
        {visible.map((item) => {
          const Icon = item.icon;
          const active = isNavActive(pathname, item.path);
          const showBadge = item.badge && unread > 0;
          return (
            <SidebarMenuItem key={item.path}>
              <SidebarMenuButton
                isActive={active}
                tooltip={item.label}
                render={<Link to={item.path} />}
              >
                <Icon />
                <span>{item.label}</span>
              </SidebarMenuButton>
              {showBadge && (
                <SidebarMenuBadge className="bg-destructive text-white text-[10px] font-bold animate-pulse-soft">
                  {unread > 99 ? "99+" : unread}
                </SidebarMenuBadge>
              )}
            </SidebarMenuItem>
          );
        })}
      </SidebarMenu>
    </SidebarGroup>
  );
}

export function Shell() {
  const location = useLocation();
  const { user, signOut } = useAuth();
  const [signOutOpen, setSignOutOpen] = useState(false);
  const { data: notifCount } = useNotificationCount(!!user);
  const unread = notifCount?.count ?? 0;
  const { theme, toggle: toggleTheme } = useTheme();
  const appVersion = useAppVersion();
  const connectionStatus = useHealthCheck();
  const { data: me } = useMe(!!user);
  useKeyboardShortcuts();
  const isAdmin = me?.is_admin ?? false;
  const emailInitial =
    user?.email?.trim().charAt(0)?.toLocaleUpperCase("he-IL") || "?";

  return (
    <SidebarProvider>
      <div className="flex min-h-screen w-full">
        <AuroraBackground />
        <AppCommandPalette />
        <ConfirmDialog
          open={signOutOpen}
          title="האם לצאת מהחשבון?"
          description="תוכל להתחבר מחדש בכל עת."
          confirmLabel="התנתק"
          variant="destructive"
          onConfirm={() => {
            setSignOutOpen(false);
            void signOut();
          }}
          onCancel={() => setSignOutOpen(false)}
        />
        <a
          href="#main-content"
          className="sr-only focus:not-sr-only focus:fixed focus:top-4 focus:right-4 focus:z-[60] focus:rounded-xl focus:bg-primary focus:px-4 focus:py-2 focus:text-sm focus:font-semibold focus:text-white focus:shadow-lg"
        >
          דלג לתוכן
        </a>
        <ConnectionBanner status={connectionStatus} />

        <Sidebar side="right" collapsible="offcanvas">
          <SidebarHeader>
            <div className="flex items-center gap-3 px-2">
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-primary via-primary to-blue-400 text-white shadow-[0_4px_16px_-4px_var(--color-glow-primary)]">
                <Car className="h-5 w-5" />
              </div>
              <div className="min-w-0 flex-1">
                <h1 className="text-[15px] leading-none font-extrabold tracking-tight text-foreground">
                  CarWatch
                </h1>
                <p className="mt-1 text-[10px] font-medium tracking-widest text-muted-foreground uppercase">
                  Smart Tracking
                </p>
              </div>
              <button
                type="button"
                onClick={toggleTheme}
                aria-label={
                  theme === "dark" ? "הפעל מצב בהיר" : "הפעל מצב כהה"
                }
                className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-sidebar-accent text-sidebar-foreground transition-all duration-150 hover:bg-sidebar-accent-hover hover:text-foreground active:scale-[0.96]"
              >
                {theme === "dark" ? <Sun size={14} /> : <Moon size={14} />}
              </button>
            </div>
          </SidebarHeader>

          <SidebarContent>
            <NavSection
              label="ראשי"
              items={mainNav}
              pathname={location.pathname}
              unread={unread}
              isAdmin={isAdmin}
              isAuthenticated={!!user}
            />
            <NavSection
              label="ספריה"
              items={libraryNav}
              pathname={location.pathname}
              unread={unread}
              isAdmin={isAdmin}
              isAuthenticated={!!user}
            />
            <NavSection
              label="מערכת"
              items={systemNav}
              pathname={location.pathname}
              unread={unread}
              isAdmin={isAdmin}
              isAuthenticated={!!user}
            />

            {unread > 0 && (
              <div className="px-3 pt-2">
                <Link
                  to="/notifications"
                  className="flex items-center gap-2.5 rounded-lg border border-primary/20 bg-primary/5 px-3 py-2 transition-all duration-150 hover:bg-primary/10"
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
            )}

            {!user && (
              <div className="mx-3 mt-2 rounded-xl border border-primary/20 bg-primary/5 p-3 text-center text-xs text-muted-foreground">
                <Link
                  to="/login"
                  className="font-medium text-primary hover:underline"
                >
                  הירשם בחינם
                </Link>{" "}
                כדי לשמור חיפושים ולקבל התראות
              </div>
            )}
          </SidebarContent>

          <SidebarFooter>
            <Separator />
            <div className="p-2">
              <div className="mb-2.5 flex items-center gap-2.5">
                <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-sidebar-accent text-xs font-semibold text-sidebar-foreground ring-1 ring-sidebar-border">
                  {user ? emailInitial : <User className="h-4 w-4" />}
                </div>
                <p
                  className="min-w-0 flex-1 truncate text-xs text-muted-foreground"
                  title={user?.email ?? undefined}
                >
                  {user?.email ?? "אורח"}
                </p>
              </div>
              {user ? (
                <Button
                  variant="outline"
                  size="sm"
                  className="w-full"
                  onClick={() => setSignOutOpen(true)}
                >
                  <LogOut className="h-3.5 w-3.5" />
                  התנתק
                </Button>
              ) : (
                <Button asChild variant="default" size="sm" className="w-full">
                  <Link to="/login">
                    <LogIn className="h-3.5 w-3.5" />
                    התחבר
                  </Link>
                </Button>
              )}
              {appVersion && (
                <p className="mt-1.5 text-center text-[10px] text-muted-foreground tabular-nums">
                  v{appVersion}
                </p>
              )}
            </div>
          </SidebarFooter>
        </Sidebar>

        <main
          id="main-content"
          className={cn(
            "flex-1 overflow-y-auto scroll-smooth",
            "h-[100dvh]",
          )}
        >
          {/* Mobile header with sidebar trigger */}
          <div className="sticky top-0 z-30 flex h-12 items-center gap-2 border-b border-border/50 bg-background/80 px-4 backdrop-blur-xl md:hidden">
            <SidebarTrigger>
              <Menu className="h-5 w-5" />
            </SidebarTrigger>
            <div className="flex items-center gap-2">
              <Car className="h-4 w-4 text-primary" />
              <span className="text-sm font-bold">CarWatch</span>
            </div>
            {unread > 0 && (
              <Link
                to="/notifications"
                className="ms-auto relative"
                aria-label={`${unread} התראות חדשות`}
              >
                <Bell className="h-5 w-5 text-muted-foreground" />
                <span className="absolute -top-1 -right-1 flex h-3.5 min-w-3.5 items-center justify-center rounded-full bg-destructive px-0.5 text-[9px] font-bold text-white">
                  {unread > 99 ? "99+" : unread}
                </span>
              </Link>
            )}
          </div>

          <div className="mx-auto max-w-5xl px-4 py-6 sm:px-6 md:py-8 lg:px-8">
            <div key={location.pathname} className="animate-fade-in">
              <Outlet />
            </div>
          </div>
        </main>
      </div>
    </SidebarProvider>
  );
}
