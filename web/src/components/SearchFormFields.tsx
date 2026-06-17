import { useMemo } from "react";
import { formatPrice } from "@/lib/utils";
import { useManufacturers, useModels } from "@/hooks/useCatalog";
import { Toggle } from "@/components/ui/toggle";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { RangeSlider } from "@/components/ui/RangeSlider";
import { Combobox, type ComboboxOption } from "@/components/ui/Combobox";
import { FormField } from "@/components/ui/FormField";
import type { Manufacturer, Model } from "@/lib/api";

/** Map a catalog entry to a searchable combobox option (matches both names). */
function catalogOption(entry: Manufacturer | Model): ComboboxOption {
  const label =
    entry.name_he && entry.name_he !== entry.name
      ? `${entry.name_he} (${entry.name})`
      : entry.name;
  return {
    value: entry.id,
    label,
    keywords: [entry.name, entry.name_he].filter((k): k is string => Boolean(k)),
  };
}

export interface SearchFormData {
  name: string;
  source: string;
  manufacturer: number;
  model: number;
  yearMin: number;
  yearMax: number;
  priceMin: number;
  priceMax: number;
  engineMinCC: number;
  maxKm: number;
  maxHand: number;
  keywords: string;
  excludeKeys: string;
  sellerFilter: "any" | "private" | "commercial";
  gearBox: string;
  priceOnly: boolean;
  photoOnly: boolean;
}

export function defaultFormData(): SearchFormData {
  return {
    name: "",
    source: "yad2",
    manufacturer: 0,
    model: 0,
    yearMin: 2018,
    yearMax: new Date().getFullYear(),
    priceMin: 0,
    priceMax: 0,
    engineMinCC: 0,
    maxKm: 0,
    maxHand: 0,
    keywords: "",
    excludeKeys: "",
    sellerFilter: "any",
    gearBox: "",
    priceOnly: false,
    photoOnly: false,
  };
}

export function normalizeSellerFilter(v: string | undefined): "any" | "private" | "commercial" {
  const s = (v ?? "any").toLowerCase().trim();
  if (s === "private") return "private";
  if (s === "commercial" || s === "dealer" || s === "dealership") return "commercial";
  return "any";
}

export function formToPayload(form: SearchFormData) {
  return {
    name: form.name || undefined,
    source: form.source,
    manufacturer: form.manufacturer,
    model: form.model,
    year_min: form.yearMin,
    year_max: form.yearMax,
    price_min: form.priceMin || undefined,
    price_max: form.priceMax,
    engine_min_cc: form.engineMinCC || undefined,
    max_km: form.maxKm,
    max_hand: form.maxHand,
    keywords: form.keywords || undefined,
    exclude_keys: form.excludeKeys || undefined,
    seller_filter: form.sellerFilter,
    gear_box: form.gearBox || undefined,
    price_only: form.priceOnly || undefined,
    photo_only: form.photoOnly || undefined,
  };
}

export function formToUpdatePayload(form: Omit<SearchFormData, "name" | "source" | "manufacturer" | "model">) {
  return {
    year_min: form.yearMin,
    year_max: form.yearMax,
    price_min: form.priceMin || undefined,
    price_max: form.priceMax,
    engine_min_cc: form.engineMinCC || undefined,
    max_km: form.maxKm,
    max_hand: form.maxHand,
    keywords: form.keywords || undefined,
    exclude_keys: form.excludeKeys || undefined,
    seller_filter: form.sellerFilter,
    gear_box: form.gearBox || undefined,
    price_only: form.priceOnly,
    photo_only: form.photoOnly,
  };
}

export function isFormValid(form: SearchFormData): boolean {
  const validYear = (y: number) => y === 0 || y >= 1990;
  return (
    validYear(form.yearMin) &&
    validYear(form.yearMax) &&
    (form.yearMin === 0 || form.yearMax === 0 || form.yearMin <= form.yearMax)
  );
}

const SOURCE_OPTIONS = [
  { value: "yad2", label: "יד2" },
];

const HAND_OPTIONS = [0, 1, 2, 3, 4];

const GEARBOX_OPTIONS = [
  { value: "", label: "הכל" },
  { value: "אוטומט", label: "אוטומט" },
  { value: "ידני", label: "ידני" },
];

const SELLER_FILTER_OPTIONS: { value: SearchFormData["sellerFilter"]; label: string }[] = [
  { value: "any", label: "הכל" },
  { value: "private", label: "מוכר פרטי" },
  { value: "commercial", label: "מוסך / סוכנות" },
];

function formatKmLabel(value: number): string {
  if (value === 0) return "ללא הגבלה";
  return `${value.toLocaleString("he-IL")} ק"מ`;
}

function SectionCard({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <Card>
      <CardHeader className="pb-0">
        <CardTitle className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
          {title}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4 pt-4">{children}</CardContent>
    </Card>
  );
}

type Setter = <K extends keyof SearchFormData>(key: K, val: SearchFormData[K]) => void;

export function VehicleFields({
  form,
  set,
  readOnly,
}: {
  form: SearchFormData;
  set: Setter;
  readOnly?: boolean;
}) {
  const { data: manufacturers } = useManufacturers();
  const { data: models } = useModels(form.manufacturer);

  const manufacturerOptions = useMemo<ComboboxOption[]>(
    () => [
      { value: 0, label: "כל היצרנים" },
      ...(manufacturers ?? []).map(catalogOption),
    ],
    [manufacturers],
  );
  const modelOptions = useMemo<ComboboxOption[]>(
    () => [
      {
        value: 0,
        label: form.manufacturer === 0 ? "יש לבחור יצרן קודם" : "כל הדגמים",
      },
      ...(models ?? []).map(catalogOption),
    ],
    [models, form.manufacturer],
  );

  if (readOnly) {
    return null;
  }

  return (
    <SectionCard title="רכב ומקור">
      <div className="space-y-1">
        <span className="text-sm font-medium text-foreground">מקור</span>
        <div className="flex flex-wrap gap-2">
          {SOURCE_OPTIONS.map((src) => (
            <Toggle variant="outline" size="sm"
              key={src.value}
              pressed={form.source === src.value}
              onClick={() => set("source", src.value)}
            >
              {src.label}
            </Toggle>
          ))}
        </div>
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <FormField label="יצרן" htmlFor="mfr">
          <Combobox
            id="mfr"
            options={manufacturerOptions}
            value={form.manufacturer}
            onChange={(v) => {
              set("manufacturer", Number(v));
              set("model", 0);
            }}
            placeholder="כל היצרנים"
            searchPlaceholder="חיפוש יצרן…"
            emptyText="לא נמצא יצרן"
          />
        </FormField>

        <FormField label="דגם" htmlFor="mdl">
          <Combobox
            id="mdl"
            options={modelOptions}
            value={form.model}
            disabled={form.manufacturer === 0}
            onChange={(v) => set("model", Number(v))}
            placeholder={form.manufacturer === 0 ? "יש לבחור יצרן קודם" : "כל הדגמים"}
            searchPlaceholder="חיפוש דגם…"
            emptyText="לא נמצא דגם"
          />
        </FormField>
      </div>

      <FormField
        label="שם החיפוש"
        htmlFor="searchName"
        hint="אופציונלי — ייווצר אוטומטית מהיצרן והדגם"
      >
        <Input
          id="searchName"
          value={form.name}
          onChange={(e) => set("name", e.target.value)}
          placeholder='לדוגמה: "טויוטה קורולה ידנית"'
        />
      </FormField>
    </SectionCard>
  );
}

export function BudgetFields({
  form,
  set,
}: {
  form: SearchFormData;
  set: Setter;
}) {
  return (
    <SectionCard title='תקציב ומפרט'>
      <div className="grid gap-4 sm:grid-cols-2">
        <FormField
          label="שנה מ-"
          htmlFor="yearMin"
          error={
            form.yearMin > form.yearMax
              ? "שנה מינימלית חייבת להיות קטנה מהמקסימלית"
              : undefined
          }
        >
          <Input
            id="yearMin"
            type="number"
            inputMode="numeric"
            value={form.yearMin}
            onChange={(e) => set("yearMin", Number(e.target.value))}
            min={1990}
            max={2030}
            error={form.yearMin > form.yearMax}
            className="tabular-nums"
          />
        </FormField>

        <FormField label="שנה עד" htmlFor="yearMax">
          <Input
            id="yearMax"
            type="number"
            inputMode="numeric"
            value={form.yearMax}
            onChange={(e) => set("yearMax", Number(e.target.value))}
            min={1990}
            max={2030}
            className="tabular-nums"
          />
        </FormField>
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <FormField
          label="מחיר מינימום (₪)"
          htmlFor="priceMin"
          hint={form.priceMin > 0 ? formatPrice(form.priceMin) : undefined}
        >
          <Input
            id="priceMin"
            type="number"
            inputMode="numeric"
            value={form.priceMin || ""}
            onChange={(e) => set("priceMin", Number(e.target.value))}
            placeholder="ללא הגבלה"
            className="tabular-nums"
          />
        </FormField>

        <FormField
          label="מחיר מקסימום (₪)"
          htmlFor="priceMax"
          hint={form.priceMax > 0 ? formatPrice(form.priceMax) : undefined}
        >
          <Input
            id="priceMax"
            type="number"
            inputMode="numeric"
            value={form.priceMax || ""}
            onChange={(e) => set("priceMax", Number(e.target.value))}
            placeholder="ללא הגבלה"
            className="tabular-nums"
          />
        </FormField>
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <FormField label='ק"מ מקסימלי'>
          <RangeSlider
            min={0}
            max={400_000}
            step={10_000}
            value={form.maxKm}
            onChange={(v) => set("maxKm", v)}
            formatLabel={formatKmLabel}
            aria-label='ק"מ מקסימלי'
          />
        </FormField>

        <FormField label="יד מקסימלית">
          <div className="flex flex-wrap gap-2">
            {HAND_OPTIONS.map((h) => (
              <Toggle variant="outline" size="sm"
                key={h}
                pressed={form.maxHand === h}
                onClick={() => set("maxHand", h)}
              >
                {h === 0 ? "כל היידות" : `יד ${h}`}
              </Toggle>
            ))}
          </div>
        </FormField>
      </div>
    </SectionCard>
  );
}

export function AdvancedFields({
  form,
  set,
}: {
  form: SearchFormData;
  set: Setter;
}) {
  return (
    <SectionCard title="פילטרים נוספים">
      <FormField label="תיבת הילוכים">
        <div className="flex flex-wrap gap-2">
          {GEARBOX_OPTIONS.map((opt) => (
            <Toggle variant="outline" size="sm"
              key={opt.value}
              pressed={form.gearBox === opt.value}
              onClick={() => set("gearBox", opt.value)}
            >
              {opt.label}
            </Toggle>
          ))}
        </div>
      </FormField>

      <FormField
        label='נפח מנוע מינימלי (סמ"ק)'
        htmlFor="engineMinCC"
        hint={form.engineMinCC > 0 ? `${(form.engineMinCC / 1000).toFixed(1)}L` : undefined}
      >
        <Input
          id="engineMinCC"
          type="number"
          inputMode="numeric"
          value={form.engineMinCC || ""}
          onChange={(e) => set("engineMinCC", Number(e.target.value))}
          placeholder="ללא הגבלה"
          className="tabular-nums"
        />
      </FormField>

      <div className="space-y-1">
        <span className="text-sm font-medium text-foreground">סוג מוכר</span>
        <div className="flex flex-wrap gap-2">
          {SELLER_FILTER_OPTIONS.map((opt) => (
            <Toggle variant="outline" size="sm"
              key={opt.value}
              pressed={form.sellerFilter === opt.value}
              onClick={() => set("sellerFilter", opt.value)}
            >
              {opt.label}
            </Toggle>
          ))}
        </div>
      </div>

      <div className="space-y-1">
        <span className="text-sm font-medium text-foreground">סינון מודעות</span>
        <div className="flex flex-wrap gap-2">
          <Toggle variant="outline" size="sm"
            pressed={form.priceOnly}
            onClick={() => set("priceOnly", !form.priceOnly)}
          >
            עם מחיר בלבד
          </Toggle>
          <Toggle variant="outline" size="sm"
            pressed={form.photoOnly}
            onClick={() => set("photoOnly", !form.photoOnly)}
          >
            עם תמונה בלבד
          </Toggle>
        </div>
      </div>

      <FormField
        label="כלול מילים"
        htmlFor="keywords"
        hint="הפרד מילים בפסיקים"
      >
        <Input
          id="keywords"
          value={form.keywords}
          onChange={(e) => set("keywords", e.target.value)}
          placeholder='לדוגמה: אוטומט, היברידי, לא פגע...'
        />
      </FormField>

      <FormField
        label="סנן מילים"
        htmlFor="excludeKeys"
        hint="מודעות שמכילות מילים אלה לא יוצגו"
      >
        <Input
          id="excludeKeys"
          value={form.excludeKeys}
          onChange={(e) => set("excludeKeys", e.target.value)}
          placeholder='לדוגמה: חירום, תאונה'
        />
      </FormField>
    </SectionCard>
  );
}
