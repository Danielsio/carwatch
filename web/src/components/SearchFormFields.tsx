import { formatPrice } from "@/lib/utils";
import { useManufacturers, useModels } from "@/hooks/useCatalog";
import { ChipButton } from "@/components/ui/ChipButton";
import { Input } from "@/components/ui/Input";
import { RangeSlider } from "@/components/ui/RangeSlider";
import { Select } from "@/components/ui/Select";
import { FormField } from "@/components/ui/FormField";

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
    <div className="bg-card border border-border rounded-2xl overflow-hidden">
      <div className="px-5 py-3 border-b border-border bg-secondary/30">
        <h2 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">{title}</h2>
      </div>
      <div className="p-5 space-y-4">{children}</div>
    </div>
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

  if (readOnly) {
    return null;
  }

  return (
    <SectionCard title="רכב ומקור">
      <div className="space-y-1">
        <span className="text-sm font-medium text-foreground">מקור</span>
        <div className="flex flex-wrap gap-2">
          {SOURCE_OPTIONS.map((src) => (
            <ChipButton
              key={src.value}
              selected={form.source === src.value}
              onClick={() => set("source", src.value)}
            >
              {src.label}
            </ChipButton>
          ))}
        </div>
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <FormField label="יצרן" htmlFor="mfr">
          <Select
            id="mfr"
            value={form.manufacturer}
            onChange={(e) => {
              set("manufacturer", Number(e.target.value));
              set("model", 0);
            }}
          >
            <option value={0}>כל היצרנים</option>
            {manufacturers?.map((m) => (
              <option key={m.id} value={m.id}>
                {m.name_he && m.name_he !== m.name ? `${m.name_he} (${m.name})` : m.name}
              </option>
            ))}
          </Select>
        </FormField>

        <FormField label="דגם" htmlFor="mdl">
          <Select
            id="mdl"
            value={form.model}
            disabled={form.manufacturer === 0}
            onChange={(e) => set("model", Number(e.target.value))}
          >
            <option value={0}>
              {form.manufacturer === 0 ? "יש לבחור יצרן קודם" : "כל הדגמים"}
            </option>
            {models?.map((m) => (
              <option key={m.id} value={m.id}>
                {m.name_he && m.name_he !== m.name ? `${m.name_he} (${m.name})` : m.name}
              </option>
            ))}
          </Select>
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
              <ChipButton
                key={h}
                selected={form.maxHand === h}
                onClick={() => set("maxHand", h)}
              >
                {h === 0 ? "כל היידות" : `יד ${h}`}
              </ChipButton>
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
            <ChipButton
              key={opt.value}
              selected={form.gearBox === opt.value}
              onClick={() => set("gearBox", opt.value)}
            >
              {opt.label}
            </ChipButton>
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
            <ChipButton
              key={opt.value}
              selected={form.sellerFilter === opt.value}
              onClick={() => set("sellerFilter", opt.value)}
            >
              {opt.label}
            </ChipButton>
          ))}
        </div>
      </div>

      <div className="space-y-1">
        <span className="text-sm font-medium text-foreground">סינון מודעות</span>
        <div className="flex flex-wrap gap-2">
          <ChipButton
            selected={form.priceOnly}
            onClick={() => set("priceOnly", !form.priceOnly)}
          >
            עם מחיר בלבד
          </ChipButton>
          <ChipButton
            selected={form.photoOnly}
            onClick={() => set("photoOnly", !form.photoOnly)}
          >
            עם תמונה בלבד
          </ChipButton>
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
