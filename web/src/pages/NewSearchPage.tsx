import { useState } from "react";
import { useNavigate, Link } from "react-router";
import { Search, Loader2 } from "lucide-react";
import { useManufacturers, useModels } from "@/hooks/useCatalog";
import { useCreateSearch } from "@/hooks/useSearches";
import { formatPrice } from "@/lib/utils";
import { Button } from "@/components/ui/Button";
import { ChipButton } from "@/components/ui/ChipButton";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { FormField } from "@/components/ui/FormField";
import { PageHeader } from "@/components/ui/PageHeader";
import { useToast } from "@/components/ui/Toast";

interface FormData {
  source: string;
  manufacturer: number;
  model: number;
  yearMin: number;
  yearMax: number;
  priceMax: number;
  engineMinCC: number;
  maxKm: number;
  maxHand: number;
  keywords: string;
  excludeKeys: string;
}

const SOURCE_OPTIONS = [
  { value: "yad2", label: "יד2" },
  { value: "winwin", label: "WinWin" },
];

const KM_OPTIONS = [0, 50_000, 100_000, 150_000, 200_000];
const HAND_OPTIONS = [0, 1, 2, 3, 4];

export function NewSearchPage() {
  const navigate = useNavigate();
  const createSearch = useCreateSearch();
  const { toast } = useToast();
  const [error, setError] = useState<string | null>(null);
  const [form, setForm] = useState<FormData>({
    source: "yad2",
    manufacturer: 0,
    model: 0,
    yearMin: 2018,
    yearMax: new Date().getFullYear(),
    priceMax: 0,
    engineMinCC: 0,
    maxKm: 0,
    maxHand: 0,
    keywords: "",
    excludeKeys: "",
  });

  const { data: manufacturers } = useManufacturers();
  const { data: models } = useModels(form.manufacturer);

  const set = <K extends keyof FormData>(key: K, val: FormData[K]) =>
    setForm((prev) => ({ ...prev, [key]: val }));

  const canSubmit =
    form.manufacturer > 0 &&
    form.model > 0 &&
    form.yearMin >= 2000 &&
    form.yearMax >= 2000 &&
    form.yearMin <= form.yearMax &&
    !createSearch.isPending;

  function handleSubmit() {
    setError(null);
    createSearch.mutate(
      {
        source: form.source,
        manufacturer: form.manufacturer,
        model: form.model,
        year_min: form.yearMin,
        year_max: form.yearMax,
        price_max: form.priceMax,
        engine_min_cc: form.engineMinCC || undefined,
        max_km: form.maxKm,
        max_hand: form.maxHand,
        keywords: form.keywords || undefined,
        exclude_keys: form.excludeKeys || undefined,
      },
      {
        onSuccess: () => {
          toast("החיפוש נוצר בהצלחה!", "success");
          navigate("/");
        },
        onError: () => setError("שגיאה ביצירת החיפוש, נסה שוב"),
      },
    );
  }

  return (
    <div className="space-y-6 pb-24 md:pb-8">
      <PageHeader
        title="חיפוש חדש"
        subtitle="הגדר פילטרים למעקב אחר מודעות"
        backTo="/"
        backLabel="חזרה"
      />

      {error && (
        <div
          className="rounded-xl border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive"
          role="alert"
        >
          {error}
        </div>
      )}

      {/* Section: Source */}
      <section className="rounded-2xl border border-border/50 bg-card p-5 space-y-5">
        <h2 className="text-sm font-semibold text-foreground">מקור</h2>
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
      </section>

      {/* Section: Vehicle filter */}
      <section className="rounded-2xl border border-border/50 bg-card p-5 space-y-5">
        <h2 className="text-sm font-semibold text-foreground">סינון לפי רכב</h2>

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
                  {m.name}
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
              <option value={0}>כל הדגמים</option>
              {models?.map((m) => (
                <option key={m.id} value={m.id}>
                  {m.name}
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
              min={2000}
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
              min={2000}
              max={2030}
              className="tabular-nums"
            />
          </FormField>
        </div>
      </section>

      {/* Section: Price & KM */}
      <section className="rounded-2xl border border-border/50 bg-card p-5 space-y-5">
        <h2 className="text-sm font-semibold text-foreground">מחיר וק&quot;מ</h2>

        <div className="grid gap-4 sm:grid-cols-2">
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
        </div>

        <FormField label='ק"מ מקסימלי'>
          <div className="flex flex-wrap gap-2">
            {KM_OPTIONS.map((km) => (
              <ChipButton
                key={km}
                selected={form.maxKm === km}
                onClick={() => set("maxKm", km)}
              >
                {km === 0 ? "ללא הגבלה" : km.toLocaleString("he-IL")}
              </ChipButton>
            ))}
          </div>
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
      </section>

      {/* Section: Keywords */}
      <section className="rounded-2xl border border-border/50 bg-card p-5 space-y-5">
        <h2 className="text-sm font-semibold text-foreground">מילות מפתח</h2>

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
      </section>

      {/* Actions */}
      <div className="flex items-center gap-3">
        <Button onClick={handleSubmit} disabled={!canSubmit} size="lg">
          {createSearch.isPending ? (
            <Loader2 className="h-4 w-4 animate-spin" />
          ) : (
            <Search className="h-4 w-4" />
          )}
          צור חיפוש
        </Button>
        <Button variant="secondary" size="lg" asChild>
          <Link to="/">ביטול</Link>
        </Button>
      </div>
    </div>
  );
}

