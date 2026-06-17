import { useMemo, useState } from "react";
import { Check, ChevronsUpDown } from "lucide-react";
import {
  Command,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { buildTrie } from "@/lib/trie";
import { cn } from "@/lib/utils";

export interface ComboboxOption {
  value: number | string;
  /** Text shown in the trigger and list row. */
  label: string;
  /** Extra strings to match on (e.g. an English name alongside a Hebrew label). */
  keywords?: string[];
}

interface ComboboxProps {
  options: ComboboxOption[];
  value: ComboboxOption["value"];
  onChange: (value: ComboboxOption["value"]) => void;
  /** Trigger text when nothing is selected. */
  placeholder?: string;
  searchPlaceholder?: string;
  emptyText?: string;
  disabled?: boolean;
  id?: string;
  dir?: "rtl" | "ltr";
  className?: string;
}

/**
 * A searchable single-select. Typing filters the options through a word-level
 * prefix {@link buildTrie | trie} (cmdk's built-in fuzzy filter is disabled), so
 * a prefix of any word — in either the label or its keywords — matches.
 */
export function Combobox({
  options,
  value,
  onChange,
  placeholder = "בחר…",
  searchPlaceholder = "חיפוש…",
  emptyText = "לא נמצאו תוצאות",
  disabled,
  id,
  dir = "rtl",
  className,
}: ComboboxProps) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");

  const trie = useMemo(
    () => buildTrie(options, (o) => [o.label, ...(o.keywords ?? [])]),
    [options],
  );
  const results = useMemo(() => trie.search(query), [trie, query]);

  const selected = options.find((o) => o.value === value);

  function close() {
    setOpen(false);
    setQuery("");
  }

  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (!next) setQuery("");
      }}
    >
      <PopoverTrigger
        id={id}
        disabled={disabled}
        dir={dir}
        aria-expanded={open}
        className={cn(
          "flex w-full items-center justify-between gap-2 rounded-xl border border-border bg-background py-2.5 ps-4 pe-3 text-sm outline-none transition-all duration-200 focus:border-primary focus:ring-2 focus:ring-ring disabled:cursor-not-allowed disabled:opacity-50",
          className,
        )}
      >
        <span className={cn("truncate", !selected && "text-muted-foreground")}>
          {selected ? selected.label : placeholder}
        </span>
        <ChevronsUpDown
          className="h-4 w-4 shrink-0 text-muted-foreground"
          aria-hidden
        />
      </PopoverTrigger>
      <PopoverContent
        align="start"
        sideOffset={4}
        className="w-(--anchor-width) min-w-48 p-0"
      >
        <Command shouldFilter={false} dir={dir}>
          <CommandInput
            value={query}
            onValueChange={setQuery}
            placeholder={searchPlaceholder}
            dir={dir}
            autoFocus
          />
          <CommandList>
            {results.length === 0 ? (
              <div className="py-6 text-center text-sm text-muted-foreground">
                {emptyText}
              </div>
            ) : (
              results.map((opt) => (
                <CommandItem
                  key={opt.value}
                  value={String(opt.value)}
                  onSelect={() => {
                    onChange(opt.value);
                    close();
                  }}
                >
                  <Check
                    className={cn(
                      "h-4 w-4",
                      opt.value === value ? "opacity-100" : "opacity-0",
                    )}
                    aria-hidden
                  />
                  <span className="truncate">{opt.label}</span>
                </CommandItem>
              ))
            )}
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}
