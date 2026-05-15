import { useState, useMemo } from "react";
import { useNavigate, Link } from "react-router";
import { Search, Loader2, Car, Zap, Trees, UserRound, ArrowRight } from "lucide-react";
import { useManufacturers, useModels } from "@/hooks/useCatalog";
import { useCreateSearch } from "@/hooks/useSearches";
import { formatPrice } from "@/lib/utils";
import { ChipButton } from "@/components/ui/ChipButton";
import { Input } from "@/components/ui/Input";
import { RangeSlider } from "@/components/ui/RangeSlider";
import { Select } from "@/components/ui/Select";
import { FormField } from "@/components/ui/FormField";
import { useToast } from "@/components/ui/Toast";

interface FormData {
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

const SELLER_FILTER_OPTIONS: { value: FormData["sellerFilter"]; label: string }[] = [
  { value: "any", label: "הכל" },
  { value: "private", label: "מוכר פרטי" },
  { value: "commercial", label: "מוסך / סוכנות" },
];

const GEARBOX_OPTIONS = [
  { value: "", label: "הכל" },
  { value: "אוטומט", label: "אוטומט" },
  { value: "ידני", label: "ידני" },
];

const SOURCE_OPTIONS = [
  { value: "yad2", label: "יד2" },
  { value: "winwin", label: "WinWin" },
];

const HAND_OPTIONS = [0, 1, 2, 3, 4];

interface SearchPreset {
  id: string;
  title: string;
  description: string;
  icon: typeof Car;
  values: Partial<FormData>;
}

function getPresets(): SearchPreset[] {
  const currentYear = new Date().getFullYear();
  return [
    {
      id: "family",
      title: "רכב משפחתי",
      description: "עד 6 שנים, עד 150,000 ₪",
      icon: Car,
      values: { yearMin: currentYear - 6, priceMax: 150_000 },
    },
    {
      id: "first",
      title: "רכב ראשון",
      description: "עד 80,000 ₪",
      icon: UserRound,
      values: { priceMax: 80_000 },
    },
    {
      id: "suv",
      title: "SUV",
      description: "עד 5 שנים, עד 200,000 ₪",
      icon: Trees,
      values: { yearMin: currentYear - 5, priceMax: 200_000 },
    },
    {
      id: "hybrid",
      title: "היברידי / חשמלי",
      description: "עד 4 שנים",
      icon: Zap,
      values: { yearMin: currentYear - 4 },
    },
  ];
}

function formatKmLabel(value: number): string {
  if (value === 0) return "ללא הגבלה";
  return `${value.toLocaleString("he-IL")} ק"מ`;
}

function SectionCard({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="bg-card border border-border rounded-2xl overflow-hidden">
      <div className="px-6 py-3.5 border-b border-border bg-secondary/30">
        <h2 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">{title}</h2>
      </div>
      <div className="p-6 space-y-4">{children}</div>
    </div>
  );
}

export function NewSearchPage() {
  const navigate = useNavigate();
  const createSearch = useCreateSearch();
  const { toast } = useToast();
  const [error, setError] = useState<string | null>(null);
  const [form, setForm] = useState<FormData>({
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
  });

  const [presetsHidden, setPresetsHidden] = useState(false);
  const presets = useMemo(() => getPresets(), []);
  const showPresets = !presetsHidden && form.manufacturer === 0;

  const { data: manufacturers } = useManufacturers();
  const { data: models } = useModels(form.manufacturer);

  const set = <K extends keyof FormData>(key: K, val: FormData[K]) =>
    setForm((prev) => ({ ...prev, [key]: val }));

  function applyPreset(preset: SearchPreset) {
    setForm((prev) => ({ ...prev, ...preset.values }));
    setPresetsHidden(true);
  }

  const validYear = (y: number) => y === 0 || y >= 1990;
  const canSubmit =
    validYear(form.yearMin) &&
    validYear(form.yearMax) &&
    (form.yearMin === 0 || form.yearMax === 0 || form.yearMin <= form.yearMax) &&
    !createSearch.isPending;

  function handleFormSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError(null);
    if (form.model > 0 && form.manufacturer === 0) {
      setError("יש לבחור יצרן כדי לבחור דגם");
      return;
    }
    createSearch.mutate(
      {
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
      },
      {
        onSuccess: () => {
          toast("החיפוש נוצר בהצלחה!", "success");
          navigate("/dashboard");
        },
        onError: () => setError("שגיאה ביצירת החיפוש, נסה שוב"),
      },
    );
  }

  return (
    <div className="space-y-5 pb-24 md:pb-8 dir-rtl">
      {/* Header */}
      <header className="flex flex-col gap-1 pb-2">
        <Link
          to="/dashboard"
          className="mb-2 inline-flex items-center gap-1.5 rounded-xl bg-secondary/60 px-3 py-1.5 text-sm text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground w-fit"
        >
          <span>חזרה</span>
          <ArrowRight className="h-4 w-4 shrink-0" aria-hidden />
        </Link>
        <h1 className="text-2xl font-semibold tracking-tight text-foreground">חיפוש חדש</h1>
        <p className="text-sm text-muted-foreground">הגדר פילטרים למעקב מודעות</p>
      </header>

      {error && (
        <div
          className="rounded-2xl border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive"
          role="alert"
        >
          {error}
        </div>
      )}

      {showPresets && (
        <section aria-label="תבניות חיפוש מוכנות">
          <h2 className="text-sm font-semibold text-foreground mb-3">התחל מתבנית</h2>
          <div className="flex gap-3 overflow-x-auto pb-2 sm:grid sm:grid-cols-2 sm:overflow-visible sm:pb-0 snap-x snap-mandatory">
            {presets.map((preset) => {
              const Icon = preset.icon;
              return (
                <button
                  key={preset.id}
                  type="button"
                  onClick={() => applyPreset(preset)}
                  className="flex-shrink-0 w-40 sm:w-auto snap-start rounded-2xl border border-border/50 bg-card p-4 text-start transition-all hover:border-primary/40 hover:bg-primary/5 hover:shadow-md hover:-translate-y-px focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
                >
                  <Icon className="h-5 w-5 text-primary mb-2" />
                  <p className="text-sm font-semibold text-foreground">{preset.title}</p>
                  <p className="text-xs text-muted-foreground mt-1">{preset.description}</p>
                </button>
              );
            })}
          </div>
        </section>
      )}

      <form onSubmit={handleFormSubmit} className="space-y-5">
        {/* Search Name */}
        <SectionCard title="שם החיפוש">
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

        {/* Vehicle Filter */}
        <SectionCard title="סינון לפי רכב">
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
        </SectionCard>

        {/* Price & Mileage */}
        <SectionCard title='מחיר וק"מ'>
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

        {/* Advanced Filters */}
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

        {/* Action buttons */}
        <div className="sticky bottom-[calc(4rem+env(safe-area-inset-bottom,0px))] landscape:bottom-14 md:bottom-0 z-40 -mx-4 px-4 py-3 bg-background/90 backdrop-blur-xl border-t border-border/30 md:static md:mx-0 md:px-0 md:py-0 md:bg-transparent md:backdrop-blur-none md:border-0">
          <div className="flex items-center gap-3">
            <button
              type="submit"
              disabled={!canSubmit}
              className="flex-1 md:flex-none inline-flex items-center justify-center gap-2 bg-primary rounded-2xl px-6 py-3 text-sm font-semibold text-white shadow-lg shadow-primary/20 transition-all hover:-translate-y-px hover:shadow-xl hover:shadow-primary/30 disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:translate-y-0 disabled:hover:shadow-lg"
            >
              {createSearch.isPending ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <Search className="h-4 w-4" />
              )}
              צור חיפוש
            </button>
            <Link
              to="/dashboard"
              className="inline-flex items-center justify-center rounded-2xl border border-border px-6 py-3 text-sm font-medium text-muted-foreground transition-colors hover:bg-secondary md:flex-none"
            >
              ביטול
            </Link>
          </div>
        </div>
      </form>
    </div>
  );
}
