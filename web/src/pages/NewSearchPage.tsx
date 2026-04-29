import { useState } from "react";
import { useNavigate } from "react-router";
import { ChevronLeft, ChevronRight, Check, Loader2 } from "lucide-react";
import { useManufacturers, useModels } from "@/hooks/useCatalog";
import { useCreateSearch } from "@/hooks/useSearches";
import { formatPrice, cn } from "@/lib/utils";

type WizardStep =
  | "manufacturer"
  | "model"
  | "year"
  | "price"
  | "filters"
  | "confirm";

const STEPS: WizardStep[] = [
  "manufacturer",
  "model",
  "year",
  "price",
  "filters",
  "confirm",
];

const STEP_LABELS: Record<WizardStep, string> = {
  manufacturer: "יצרן",
  model: "דגם",
  year: "שנה",
  price: "מחיר",
  filters: "סינונים",
  confirm: "אישור",
};

interface FormData {
  manufacturer: number;
  manufacturerName: string;
  model: number;
  modelName: string;
  yearMin: number;
  yearMax: number;
  priceMax: number;
  maxKm: number;
  maxHand: number;
  keywords: string;
  excludeKeys: string;
}

export function NewSearchPage() {
  const navigate = useNavigate();
  const createSearch = useCreateSearch();
  const [step, setStep] = useState<WizardStep>("manufacturer");
  const [mfrSearch, setMfrSearch] = useState("");
  const [modelSearch, setModelSearch] = useState("");
  const [form, setForm] = useState<FormData>({
    manufacturer: 0,
    manufacturerName: "",
    model: 0,
    modelName: "",
    yearMin: 2018,
    yearMax: new Date().getFullYear(),
    priceMax: 0,
    maxKm: 0,
    maxHand: 0,
    keywords: "",
    excludeKeys: "",
  });

  const { data: manufacturers } = useManufacturers(
    mfrSearch.length >= 1 ? mfrSearch : undefined,
  );
  const { data: models } = useModels(
    form.manufacturer,
    modelSearch.length >= 1 ? modelSearch : undefined,
  );

  const currentIndex = STEPS.indexOf(step);
  const canGoBack = currentIndex > 0;

  function goNext() {
    if (currentIndex < STEPS.length - 1) {
      setStep(STEPS[currentIndex + 1]);
    }
  }

  function goBack() {
    if (canGoBack) {
      setStep(STEPS[currentIndex - 1]);
    }
  }

  const [error, setError] = useState<string | null>(null);

  function handleSubmit() {
    setError(null);
    createSearch.mutate(
      {
        source: "yad2",
        manufacturer: form.manufacturer,
        model: form.model,
        year_min: form.yearMin,
        year_max: form.yearMax,
        price_max: form.priceMax,
        max_km: form.maxKm,
        max_hand: form.maxHand,
        keywords: form.keywords || undefined,
        exclude_keys: form.excludeKeys || undefined,
      },
      {
        onSuccess: () => navigate("/"),
        onError: () => setError("שגיאה ביצירת החיפוש, נסה שוב"),
      },
    );
  }

  return (
    <div className="space-y-8">
      <h1 className="text-2xl font-semibold tracking-tight">חיפוש חדש</h1>

      {/* Progress bar */}
      <div className="space-y-2">
        <div className="flex items-center gap-1.5">
          {STEPS.map((_, i) => (
            <div key={i} className="flex-1">
              <div
                className={cn(
                  "h-1.5 rounded-full transition-all duration-500",
                  i <= currentIndex
                    ? "bg-primary shadow-[0_0_8px_rgba(59,130,246,0.4)]"
                    : "bg-secondary",
                )}
              />
            </div>
          ))}
        </div>
        <p className="text-sm text-muted-foreground">
          שלב {currentIndex + 1} מתוך {STEPS.length} — {STEP_LABELS[step]}
        </p>
      </div>

      {/* Step content */}
      <div className="min-h-[300px] animate-fade-in" key={step}>
        {step === "manufacturer" && (
          <div className="space-y-3">
            <input
              type="text"
              placeholder="חפש יצרן..."
              value={mfrSearch}
              onChange={(e) => setMfrSearch(e.target.value)}
              className="w-full rounded-xl border border-border bg-card px-4 py-3 text-sm outline-none transition-all duration-200 focus:border-primary focus:ring-2 focus:ring-primary/20 placeholder:text-muted-foreground"
            />
            <div className="grid grid-cols-2 gap-2 max-h-80 overflow-y-auto sm:grid-cols-3">
              {manufacturers?.map((mfr) => (
                <button
                  key={mfr.id}
                  onClick={() => {
                    setForm({
                      ...form,
                      manufacturer: mfr.id,
                      manufacturerName: mfr.name,
                      model: 0,
                      modelName: "",
                    });
                    setModelSearch("");
                    goNext();
                  }}
                  className={cn(
                    "rounded-xl border px-3 py-2.5 text-sm text-right transition-all duration-200 active:scale-[0.97]",
                    form.manufacturer === mfr.id
                      ? "border-primary bg-primary/10 text-primary shadow-[0_0_12px_rgba(59,130,246,0.15)]"
                      : "border-border/50 bg-card hover:border-border hover:bg-surface-hover",
                  )}
                >
                  {mfr.name}
                </button>
              ))}
            </div>
          </div>
        )}

        {step === "model" && (
          <div className="space-y-3">
            <p className="text-sm font-medium text-muted-foreground">
              נבחר:{" "}
              <span className="text-foreground">{form.manufacturerName}</span>
            </p>
            <input
              type="text"
              placeholder="חפש דגם..."
              value={modelSearch}
              onChange={(e) => setModelSearch(e.target.value)}
              className="w-full rounded-xl border border-border bg-card px-4 py-3 text-sm outline-none transition-all duration-200 focus:border-primary focus:ring-2 focus:ring-primary/20 placeholder:text-muted-foreground"
            />
            <div className="grid grid-cols-2 gap-2 max-h-80 overflow-y-auto sm:grid-cols-3">
              {models?.map((mdl) => (
                <button
                  key={mdl.id}
                  onClick={() => {
                    setForm({ ...form, model: mdl.id, modelName: mdl.name });
                    goNext();
                  }}
                  className={cn(
                    "rounded-xl border px-3 py-2.5 text-sm text-right transition-all duration-200 active:scale-[0.97]",
                    form.model === mdl.id
                      ? "border-primary bg-primary/10 text-primary shadow-[0_0_12px_rgba(59,130,246,0.15)]"
                      : "border-border/50 bg-card hover:border-border hover:bg-surface-hover",
                  )}
                >
                  {mdl.name}
                </button>
              ))}
            </div>
          </div>
        )}

        {step === "year" && (
          <div className="space-y-5">
            <div>
              <label className="block text-sm font-medium mb-2">
                שנה מינימלית
              </label>
              <input
                type="number"
                value={form.yearMin}
                onChange={(e) =>
                  setForm({ ...form, yearMin: Number(e.target.value) })
                }
                min={2000}
                max={2030}
                className="w-full rounded-xl border border-border bg-card px-4 py-3 text-sm outline-none transition-all duration-200 focus:border-primary focus:ring-2 focus:ring-primary/20 tabular-nums"
              />
            </div>
            <div>
              <label className="block text-sm font-medium mb-2">
                שנה מקסימלית
              </label>
              <input
                type="number"
                value={form.yearMax}
                onChange={(e) =>
                  setForm({ ...form, yearMax: Number(e.target.value) })
                }
                min={2000}
                max={2030}
                className="w-full rounded-xl border border-border bg-card px-4 py-3 text-sm outline-none transition-all duration-200 focus:border-primary focus:ring-2 focus:ring-primary/20 tabular-nums"
              />
            </div>
            {form.yearMin > form.yearMax && (
              <p className="text-sm text-destructive">
                שנה מינימלית חייבת להיות קטנה או שווה לשנה מקסימלית
              </p>
            )}
          </div>
        )}

        {step === "price" && (
          <div className="space-y-5">
            <div>
              <label className="block text-sm font-medium mb-2">
                מחיר מקסימלי (₪)
              </label>
              <input
                type="number"
                value={form.priceMax || ""}
                onChange={(e) =>
                  setForm({ ...form, priceMax: Number(e.target.value) })
                }
                placeholder="ללא הגבלה"
                className="w-full rounded-xl border border-border bg-card px-4 py-3 text-sm outline-none transition-all duration-200 focus:border-primary focus:ring-2 focus:ring-primary/20 tabular-nums placeholder:text-muted-foreground"
              />
              {form.priceMax > 0 && (
                <p className="mt-1.5 text-xs text-muted-foreground tabular-nums">
                  {formatPrice(form.priceMax)}
                </p>
              )}
            </div>
            <div className="flex flex-wrap gap-2">
              {[100000, 150000, 200000, 250000, 300000, 400000].map((p) => (
                <button
                  key={p}
                  onClick={() => setForm({ ...form, priceMax: p })}
                  className={cn(
                    "rounded-xl border px-3.5 py-2 text-sm tabular-nums transition-all duration-200 active:scale-[0.97]",
                    form.priceMax === p
                      ? "border-primary bg-primary/10 text-primary shadow-[0_0_12px_rgba(59,130,246,0.15)]"
                      : "border-border/50 bg-card hover:border-border hover:bg-surface-hover",
                  )}
                >
                  {formatPrice(p)}
                </button>
              ))}
            </div>
          </div>
        )}

        {step === "filters" && (
          <div className="space-y-6">
            <div>
              <label className="block text-sm font-medium mb-2">
                ק&quot;מ מקסימלי
              </label>
              <div className="flex flex-wrap gap-2">
                {[0, 50000, 100000, 150000, 200000].map((km) => (
                  <button
                    key={km}
                    onClick={() => setForm({ ...form, maxKm: km })}
                    className={cn(
                      "rounded-xl border px-3.5 py-2 text-sm transition-all duration-200 active:scale-[0.97]",
                      form.maxKm === km
                        ? "border-primary bg-primary/10 text-primary shadow-[0_0_12px_rgba(59,130,246,0.15)]"
                        : "border-border/50 bg-card hover:border-border hover:bg-surface-hover",
                    )}
                  >
                    {km === 0
                      ? "ללא הגבלה"
                      : km.toLocaleString("he-IL")}
                  </button>
                ))}
              </div>
            </div>
            <div>
              <label className="block text-sm font-medium mb-2">
                יד מקסימלית
              </label>
              <div className="flex flex-wrap gap-2">
                {[0, 1, 2, 3, 4].map((h) => (
                  <button
                    key={h}
                    onClick={() => setForm({ ...form, maxHand: h })}
                    className={cn(
                      "rounded-xl border px-3.5 py-2 text-sm transition-all duration-200 active:scale-[0.97]",
                      form.maxHand === h
                        ? "border-primary bg-primary/10 text-primary shadow-[0_0_12px_rgba(59,130,246,0.15)]"
                        : "border-border/50 bg-card hover:border-border hover:bg-surface-hover",
                    )}
                  >
                    {h === 0 ? "ללא הגבלה" : `יד ${h}`}
                  </button>
                ))}
              </div>
            </div>
            <div>
              <label className="block text-sm font-medium mb-2">
                מילות מפתח (אופציונלי)
              </label>
              <input
                type="text"
                value={form.keywords}
                onChange={(e) =>
                  setForm({ ...form, keywords: e.target.value })
                }
                placeholder='לדוגמה: היברידי, אוטומט'
                className="w-full rounded-xl border border-border bg-card px-4 py-3 text-sm outline-none transition-all duration-200 focus:border-primary focus:ring-2 focus:ring-primary/20 placeholder:text-muted-foreground"
              />
            </div>
            <div>
              <label className="block text-sm font-medium mb-2">
                מילות מפתח לסינון (אופציונלי)
              </label>
              <input
                type="text"
                value={form.excludeKeys}
                onChange={(e) =>
                  setForm({ ...form, excludeKeys: e.target.value })
                }
                placeholder='לדוגמה: חירום, תאונה'
                className="w-full rounded-xl border border-border bg-card px-4 py-3 text-sm outline-none transition-all duration-200 focus:border-primary focus:ring-2 focus:ring-primary/20 placeholder:text-muted-foreground"
              />
              <p className="mt-1.5 text-xs text-muted-foreground">
                מודעות שמכילות מילים אלה לא יוצגו
              </p>
            </div>
          </div>
        )}

        {step === "confirm" && (
          <div className="space-y-5">
            {error && (
              <div className="rounded-xl border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
                {error}
              </div>
            )}
            <div className="rounded-2xl border border-border/50 bg-card p-6">
              <h3 className="text-lg font-semibold mb-4">סיכום חיפוש</h3>
              <dl className="grid grid-cols-2 gap-3 text-sm">
                <dt className="text-muted-foreground">יצרן</dt>
                <dd className="font-medium">{form.manufacturerName}</dd>
                <dt className="text-muted-foreground">דגם</dt>
                <dd className="font-medium">{form.modelName}</dd>
                <dt className="text-muted-foreground">שנים</dt>
                <dd className="font-medium tabular-nums">
                  {form.yearMin}–{form.yearMax}
                </dd>
                {form.priceMax > 0 && (
                  <>
                    <dt className="text-muted-foreground">מחיר מקסימלי</dt>
                    <dd className="font-medium tabular-nums">
                      {formatPrice(form.priceMax)}
                    </dd>
                  </>
                )}
                {form.maxKm > 0 && (
                  <>
                    <dt className="text-muted-foreground">
                      ק&quot;מ מקסימלי
                    </dt>
                    <dd className="font-medium tabular-nums">
                      {form.maxKm.toLocaleString("he-IL")}
                    </dd>
                  </>
                )}
                {form.maxHand > 0 && (
                  <>
                    <dt className="text-muted-foreground">יד מקסימלית</dt>
                    <dd className="font-medium">יד {form.maxHand}</dd>
                  </>
                )}
                {form.keywords && (
                  <>
                    <dt className="text-muted-foreground">מילות מפתח</dt>
                    <dd className="font-medium">{form.keywords}</dd>
                  </>
                )}
              </dl>
            </div>
          </div>
        )}
      </div>

      {/* Navigation */}
      <div className="flex items-center gap-3 border-t border-border/50 pt-5">
        {canGoBack && (
          <button
            onClick={goBack}
            className="inline-flex items-center gap-1.5 rounded-xl bg-secondary px-4 py-2.5 text-sm font-medium text-secondary-foreground transition-all duration-200 hover:bg-accent active:scale-[0.97]"
          >
            <ChevronRight className="h-4 w-4" />
            הקודם
          </button>
        )}

        <div className="mr-auto" />

        {step === "confirm" ? (
          <button
            onClick={handleSubmit}
            disabled={
              createSearch.isPending ||
              form.manufacturer === 0 ||
              form.model === 0
            }
            className="inline-flex items-center gap-1.5 rounded-xl bg-primary px-6 py-2.5 text-sm font-medium text-primary-foreground shadow-[0_0_20px_rgba(59,130,246,0.25)] transition-all duration-200 hover:bg-primary/90 hover:shadow-[0_0_30px_rgba(59,130,246,0.35)] disabled:opacity-50 disabled:shadow-none active:scale-[0.98]"
          >
            {createSearch.isPending ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <Check className="h-4 w-4" />
            )}
            צור חיפוש
          </button>
        ) : (
          step !== "manufacturer" &&
          step !== "model" && (
            <button
              onClick={goNext}
              className="inline-flex items-center gap-1.5 rounded-xl bg-primary px-5 py-2.5 text-sm font-medium text-primary-foreground shadow-[0_0_20px_rgba(59,130,246,0.25)] transition-all duration-200 hover:bg-primary/90 hover:shadow-[0_0_30px_rgba(59,130,246,0.35)] active:scale-[0.98]"
            >
              הבא
              <ChevronLeft className="h-4 w-4" />
            </button>
          )
        )}
      </div>
    </div>
  );
}
