import { useId, useState } from "react";
import { Search, SlidersHorizontal, X, Flame, EyeOff, Image as ImageIcon } from "lucide-react";
import { Toggle } from "@/components/ui/toggle";
import { RangeSlider } from "@/components/ui/RangeSlider";
import { formatKm, formatPrice, cn } from "@/lib/utils";
import {
  activeFilterCount,
  type ListingFacets,
  type ListingFilters,
} from "@/hooks/useListingFilters";

export type ListingsFilterBarProps = {
  filters: ListingFilters;
  onChange: (next: ListingFilters) => void;
  onClear: () => void;
  facets: ListingFacets;
  resultCount: number;
  totalCount: number;
};

function toggleInArray(arr: string[], value: string): string[] {
  return arr.includes(value)
    ? arr.filter((v) => v !== value)
    : [...arr, value];
}

export function ListingsFilterBar({
  filters,
  onChange,
  onClear,
  facets,
  resultCount,
  totalCount,
}: ListingsFilterBarProps) {
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const advancedId = useId();
  const active = activeFilterCount(filters);

  const set = (partial: Partial<ListingFilters>) =>
    onChange({ ...filters, ...partial });

  const hasPriceRange = facets.priceMax > 0;
  const hasYearRange = facets.yearMax > facets.yearMin && facets.yearMin > 0;
  const hasKmRange = facets.kmMax > 0;
  const hasAdvanced = hasPriceRange || hasYearRange || hasKmRange;

  return (
    <div
      role="group"
      aria-label="סינון תוצאות"
      className="space-y-3 rounded-2xl border border-border/50 bg-card/70 p-3 backdrop-blur-sm dir-rtl"
    >
      {/* Search + advanced toggle + count */}
      <div className="flex items-center gap-2">
        <div className="relative flex-1 min-w-0">
          <Search
            className="pointer-events-none absolute top-1/2 start-3 h-4 w-4 -translate-y-1/2 text-muted-foreground"
            aria-hidden
          />
          <input
            type="search"
            value={filters.text}
            onChange={(e) => set({ text: e.target.value })}
            placeholder="חפש לפי דגם, עיר, או תיאור…"
            aria-label="חיפוש חופשי בתוצאות"
            className="w-full rounded-xl border border-input bg-secondary/60 py-2.5 ps-9 pe-3 text-sm text-foreground outline-none transition-colors placeholder:text-muted-foreground/60 focus:border-primary/50 focus:bg-secondary"
          />
        </div>

        {hasAdvanced ? (
          <button
            type="button"
            onClick={() => setAdvancedOpen((v) => !v)}
            aria-expanded={advancedOpen}
            aria-controls={advancedId}
            className={cn(
              "flex shrink-0 items-center gap-1.5 rounded-xl border px-3 py-2.5 text-sm font-medium transition-colors",
              advancedOpen
                ? "border-primary/40 bg-primary/10 text-primary"
                : "border-border/50 bg-secondary/60 text-muted-foreground hover:text-foreground",
            )}
          >
            <SlidersHorizontal className="h-4 w-4" aria-hidden />
            <span className="hidden sm:inline">טווחים</span>
          </button>
        ) : null}
      </div>

      {/* Quick chips */}
      <div className="flex flex-wrap items-center gap-1.5">
        <Toggle
          variant="outline"
          size="sm"
          pressed={filters.dealsOnly}
          onPressedChange={() => set({ dealsOnly: !filters.dealsOnly })}
        >
          <Flame className="h-3.5 w-3.5" aria-hidden />
          מציאות
        </Toggle>
        <Toggle
          variant="outline"
          size="sm"
          pressed={filters.unseenOnly}
          onPressedChange={() => set({ unseenOnly: !filters.unseenOnly })}
        >
          <EyeOff className="h-3.5 w-3.5" aria-hidden />
          חדשות
        </Toggle>
        <Toggle
          variant="outline"
          size="sm"
          pressed={filters.photoOnly}
          onPressedChange={() => set({ photoOnly: !filters.photoOnly })}
        >
          <ImageIcon className="h-3.5 w-3.5" aria-hidden />
          עם תמונה
        </Toggle>

        {facets.fuels.map((fuel) => (
          <Toggle
            key={fuel}
            variant="outline"
            size="sm"
            pressed={filters.fuels.includes(fuel)}
            onPressedChange={() => set({ fuels: toggleInArray(filters.fuels, fuel) })}
          >
            {fuel}
          </Toggle>
        ))}
        {facets.gearboxes.map((gb) => (
          <Toggle
            key={gb}
            variant="outline"
            size="sm"
            pressed={filters.gearboxes.includes(gb)}
            onPressedChange={() =>
              set({ gearboxes: toggleInArray(filters.gearboxes, gb) })
            }
          >
            {gb}
          </Toggle>
        ))}
      </div>

      {/* Advanced ranges */}
      {advancedOpen && hasAdvanced ? (
        <div
          id={advancedId}
          className="grid gap-4 rounded-xl bg-secondary/40 p-4 sm:grid-cols-3"
        >
          {hasPriceRange ? (
            <RangeSlider
              aria-label="מחיר מקסימלי"
              min={0}
              max={facets.priceMax}
              step={1000}
              value={filters.priceMax ?? facets.priceMax}
              onChange={(v) =>
                set({ priceMax: v >= facets.priceMax ? null : v })
              }
              formatLabel={(v) => `עד ${formatPrice(v)}`}
            />
          ) : null}
          {hasYearRange ? (
            <RangeSlider
              aria-label="שנה מינימלית"
              min={facets.yearMin}
              max={facets.yearMax}
              step={1}
              value={filters.yearMin ?? facets.yearMin}
              onChange={(v) =>
                set({ yearMin: v <= facets.yearMin ? null : v })
              }
              formatLabel={(v) => `משנת ${v}`}
            />
          ) : null}
          {hasKmRange ? (
            <RangeSlider
              aria-label="קילומטראז׳ מקסימלי"
              min={0}
              max={facets.kmMax}
              step={5000}
              value={filters.kmMax ?? facets.kmMax}
              onChange={(v) => set({ kmMax: v >= facets.kmMax ? null : v })}
              formatLabel={(v) => `עד ${formatKm(v)}`}
            />
          ) : null}
        </div>
      ) : null}

      {/* Result count + clear */}
      {active > 0 ? (
        <div className="flex items-center justify-between gap-2 border-t border-border/40 pt-2.5">
          <p className="text-xs text-muted-foreground tabular-nums" aria-live="polite">
            {resultCount.toLocaleString("he-IL")} מתוך{" "}
            {totalCount.toLocaleString("he-IL")} מודעות
          </p>
          <button
            type="button"
            onClick={onClear}
            className="flex items-center gap-1 rounded-lg px-2 py-1 text-xs font-medium text-primary transition-colors hover:bg-primary/10"
          >
            <X className="h-3.5 w-3.5" aria-hidden />
            נקה ({active})
          </button>
        </div>
      ) : null}
    </div>
  );
}
