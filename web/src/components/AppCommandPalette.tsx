import { useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router";
import {
  LayoutDashboard,
  Plus,
  Bookmark,
  History,
  Bell,
  Settings,
  Wrench,
  Car,
  Sun,
  Moon,
  LogOut,
} from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { useTheme } from "@/contexts/ThemeContext";
import { useMe } from "@/hooks/useMe";
import { useSearches } from "@/hooks/useSearches";
import { CommandPalette, type CommandItem } from "@/components/CommandPalette";

/**
 * App-level wiring for the command palette: builds the command list from the
 * router, theme, auth and saved searches, and owns open state + the global
 * ⌘K / Ctrl+K / "/" hotkey. Mounted once inside the authenticated Shell.
 */
export function AppCommandPalette() {
  const [open, setOpen] = useState(false);
  const navigate = useNavigate();
  const { theme, toggle } = useTheme();
  const { user, signOut } = useAuth();
  const { data: me } = useMe(!!user);
  const { data: searches } = useSearches(!!user);

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if ((e.key === "k" || e.key === "K") && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        setOpen((o) => !o);
        return;
      }
      if (e.key === "/" && !open) {
        const target = e.target as HTMLElement | null;
        const tag = target?.tagName;
        if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return;
        if (target?.isContentEditable) return;
        e.preventDefault();
        setOpen(true);
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open]);

  const close = useCallback(() => setOpen(false), []);

  const commands = useMemo<CommandItem[]>(() => {
    const NAV = "ניווט";
    const go = (
      path: string,
      label: string,
      icon: CommandItem["icon"],
      hint?: string,
    ): CommandItem => ({
      id: `nav:${path}`,
      group: NAV,
      label,
      icon,
      hint,
      keywords: path,
      perform: () => navigate(path),
    });

    const list: CommandItem[] = [
      go("/dashboard", "לוח בקרה", LayoutDashboard, "D"),
      go("/searches/new", "חיפוש חדש", Plus),
      go("/saved", "מועדפים", Bookmark),
      go("/history", "היסטוריה", History, "H"),
      go("/notifications", "התראות", Bell, "N"),
      go("/settings", "הגדרות", Settings, "S"),
    ];

    if (me?.is_admin) list.push(go("/admin", "ניהול", Wrench));

    list.push({
      id: "action:theme",
      group: "פעולות",
      label: theme === "dark" ? "מצב בהיר" : "מצב כהה",
      icon: theme === "dark" ? Sun : Moon,
      keywords: "theme dark light מצב כהה בהיר ערכת נושא",
      perform: toggle,
    });

    if (user) {
      list.push({
        id: "action:signout",
        group: "פעולות",
        label: "התנתק",
        icon: LogOut,
        keywords: "logout sign out יציאה",
        perform: () => {
          void signOut();
        },
      });
    }

    (searches ?? []).forEach((s) => {
      list.push({
        id: `search:${s.id}`,
        group: "החיפושים שלי",
        label: s.name,
        icon: Car,
        keywords: `${s.manufacturer_name} ${s.model_name} listings search`,
        perform: () => navigate(`/searches/${s.id}/listings`),
      });
    });

    return list;
  }, [navigate, me, theme, toggle, user, signOut, searches]);

  return <CommandPalette open={open} onClose={close} commands={commands} />;
}
