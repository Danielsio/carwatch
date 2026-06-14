import { Fragment, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Search } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";

export type CommandItem = {
  /** Stable unique id. */
  id: string;
  /** Visible label. */
  label: string;
  /** Group heading the item is filed under. */
  group: string;
  icon: LucideIcon;
  /** Optional trailing hint (e.g. a single-key shortcut). */
  hint?: string;
  /** Extra text matched against the query but not displayed. */
  keywords?: string;
  /** Invoked when the item is chosen. */
  perform: () => void;
};

export type CommandPaletteProps = {
  open: boolean;
  onClose: () => void;
  commands: CommandItem[];
};

/**
 * Presentational, fully-controlled command palette. Knows nothing about the
 * router or app contexts — every item carries its own `perform` callback — so
 * it can be unit-tested in isolation. The container (`AppCommandPalette`)
 * supplies commands and owns open/close + the global hotkey.
 */
export function CommandPalette({ open, onClose, commands }: CommandPaletteProps) {
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLUListElement>(null);
  const restoreFocusRef = useRef<HTMLElement | null>(null);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return commands;
    return commands.filter((c) =>
      `${c.label} ${c.keywords ?? ""} ${c.group}`.toLowerCase().includes(q),
    );
  }, [query, commands]);

  // Reset query + highlight on open; restore focus to the trigger on close.
  useEffect(() => {
    if (open) {
      restoreFocusRef.current = document.activeElement as HTMLElement | null;
      setQuery("");
      setActive(0);
      const raf = requestAnimationFrame(() => inputRef.current?.focus());
      return () => cancelAnimationFrame(raf);
    }
    restoreFocusRef.current?.focus?.();
  }, [open]);

  // Keep the highlight in range as the query narrows results.
  useEffect(() => {
    setActive(0);
  }, [query]);

  // Lock body scroll while open.
  useEffect(() => {
    if (!open) return;
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = prev;
    };
  }, [open]);

  // Keep the active option scrolled into view.
  useEffect(() => {
    if (!open) return;
    const el = listRef.current?.querySelector(`[data-idx="${active}"]`);
    el?.scrollIntoView?.({ block: "nearest" });
  }, [active, open]);

  const run = useCallback(
    (cmd?: CommandItem) => {
      if (!cmd) return;
      onClose();
      cmd.perform();
    },
    [onClose],
  );

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActive((a) => (filtered.length ? (a + 1) % filtered.length : 0));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActive((a) =>
        filtered.length ? (a - 1 + filtered.length) % filtered.length : 0,
      );
    } else if (e.key === "Enter") {
      e.preventDefault();
      run(filtered[active]);
    } else if (e.key === "Escape") {
      e.preventDefault();
      onClose();
    } else if (e.key === "Tab") {
      // Trap focus on the input — the list is keyboard-driven.
      e.preventDefault();
    }
  };

  if (!open) return null;

  // Group while preserving the commands' original order.
  const groups: { name: string; items: { cmd: CommandItem; index: number }[] }[] =
    [];
  filtered.forEach((cmd, index) => {
    let g = groups.find((x) => x.name === cmd.group);
    if (!g) {
      g = { name: cmd.group, items: [] };
      groups.push(g);
    }
    g.items.push({ cmd, index });
  });

  return (
    <div className="fixed inset-0 z-[200]" role="presentation">
      <div
        className="absolute inset-0 animate-fade-in bg-background/70 backdrop-blur-sm"
        onClick={onClose}
        aria-hidden
      />
      <div
        role="dialog"
        aria-modal="true"
        aria-label="לוח פקודות"
        dir="rtl"
        className="glass-card glow-border absolute inset-x-0 top-[10vh] mx-auto w-[92vw] max-w-xl animate-scale-in overflow-hidden rounded-2xl shadow-2xl"
      >
        {/* Search input */}
        <div className="flex items-center gap-3 border-b border-border/50 px-4">
          <Search className="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden />
          <input
            ref={inputRef}
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={onKeyDown}
            role="combobox"
            aria-expanded
            aria-controls="cmdk-list"
            aria-activedescendant={
              filtered[active] ? `cmdk-opt-${active}` : undefined
            }
            aria-label="חיפוש פקודה"
            placeholder="חפש פקודה, חיפוש שמור, או נווט…"
            className="flex-1 bg-transparent py-4 text-sm text-foreground outline-none placeholder:text-muted-foreground"
          />
          <kbd className="hidden rounded border border-border/60 bg-secondary px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground sm:inline">
            ESC
          </kbd>
        </div>

        {/* Results */}
        <ul
          id="cmdk-list"
          ref={listRef}
          role="listbox"
          aria-label="פקודות זמינות"
          className="max-h-[55vh] overflow-y-auto p-2"
        >
          {filtered.length === 0 ? (
            <li
              role="presentation"
              className="px-3 py-10 text-center text-sm text-muted-foreground"
            >
              לא נמצאו תוצאות
            </li>
          ) : (
            groups.map((group) => (
              <Fragment key={group.name}>
                <li
                  role="presentation"
                  className="px-3 pt-3 pb-1 text-[11px] font-semibold tracking-wide text-muted-foreground uppercase"
                >
                  {group.name}
                </li>
                {group.items.map(({ cmd, index }) => {
                  const Icon = cmd.icon;
                  const selected = index === active;
                  return (
                    <li
                      key={cmd.id}
                      id={`cmdk-opt-${index}`}
                      data-idx={index}
                      role="option"
                      aria-selected={selected}
                      onMouseMove={() => setActive(index)}
                      onClick={() => run(cmd)}
                      className={cn(
                        "flex cursor-pointer items-center gap-3 rounded-lg px-3 py-2.5 text-sm transition-colors",
                        selected
                          ? "bg-primary/12 text-foreground"
                          : "text-muted-foreground",
                      )}
                    >
                      <span
                        className={cn(
                          "flex h-7 w-7 shrink-0 items-center justify-center rounded-md transition-colors",
                          selected
                            ? "bg-primary/15 text-primary"
                            : "bg-secondary text-muted-foreground",
                        )}
                      >
                        <Icon className="h-4 w-4" aria-hidden />
                      </span>
                      <span className="min-w-0 flex-1 truncate text-foreground">
                        {cmd.label}
                      </span>
                      {cmd.hint ? (
                        <kbd className="shrink-0 rounded border border-border/60 bg-secondary px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
                          {cmd.hint}
                        </kbd>
                      ) : null}
                    </li>
                  );
                })}
              </Fragment>
            ))
          )}
        </ul>

        {/* Footer hints */}
        <div className="flex items-center gap-4 border-t border-border/50 px-4 py-2 text-[11px] text-muted-foreground">
          <span className="flex items-center gap-1">
            <Kbd>↑</Kbd>
            <Kbd>↓</Kbd>
            ניווט
          </span>
          <span className="flex items-center gap-1">
            <Kbd>↵</Kbd>
            בחירה
          </span>
          <span className="ms-auto flex items-center gap-1">
            <Kbd>⌘</Kbd>
            <Kbd>K</Kbd>
            פתיחה
          </span>
        </div>
      </div>
    </div>
  );
}

function Kbd({ children }: { children: React.ReactNode }) {
  return (
    <kbd className="inline-flex min-w-4 items-center justify-center rounded border border-border/60 bg-secondary px-1 py-0.5 text-[10px] font-medium text-muted-foreground">
      {children}
    </kbd>
  );
}
