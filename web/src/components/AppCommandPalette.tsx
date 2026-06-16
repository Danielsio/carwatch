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
import type { LucideIcon } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { useTheme } from "@/contexts/ThemeContext";
import { useMe } from "@/hooks/useMe";
import { useSearches } from "@/hooks/useSearches";
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from "@/components/ui/command";

interface CmdItem {
  id: string;
  group: string;
  label: string;
  icon: LucideIcon;
  keywords?: string;
  hint?: string;
  perform: () => void;
}

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

  const run = useCallback(
    (fn: () => void) => {
      setOpen(false);
      fn();
    },
    [],
  );

  const commands = useMemo<CmdItem[]>(() => {
    const go = (
      path: string,
      label: string,
      icon: LucideIcon,
      hint?: string,
    ): CmdItem => ({
      id: `nav:${path}`,
      group: "ניווט",
      label,
      icon,
      hint,
      keywords: path,
      perform: () => navigate(path),
    });

    const list: CmdItem[] = [
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
        perform: () => void signOut(),
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

  const grouped = useMemo(() => {
    const map = new Map<string, CmdItem[]>();
    for (const cmd of commands) {
      const arr = map.get(cmd.group) ?? [];
      arr.push(cmd);
      map.set(cmd.group, arr);
    }
    return map;
  }, [commands]);

  return (
    <CommandDialog open={open} onOpenChange={setOpen}>
      <CommandInput placeholder="חפש פקודה..." dir="rtl" />
      <CommandList>
        <CommandEmpty>לא נמצאו תוצאות</CommandEmpty>
        {Array.from(grouped.entries()).map(([group, items], i) => (
          <div key={group}>
            {i > 0 && <CommandSeparator />}
            <CommandGroup heading={group}>
              {items.map((cmd) => {
                const Icon = cmd.icon;
                return (
                  <CommandItem
                    key={cmd.id}
                    value={`${cmd.label} ${cmd.keywords ?? ""}`}
                    onSelect={() => run(cmd.perform)}
                  >
                    <Icon className="h-4 w-4 text-muted-foreground" />
                    <span>{cmd.label}</span>
                    {cmd.hint && (
                      <kbd className="ms-auto rounded bg-muted px-1.5 py-0.5 text-[10px] font-mono text-muted-foreground">
                        {cmd.hint}
                      </kbd>
                    )}
                  </CommandItem>
                );
              })}
            </CommandGroup>
          </div>
        ))}
      </CommandList>
    </CommandDialog>
  );
}
