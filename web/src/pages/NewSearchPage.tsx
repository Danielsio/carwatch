import { useState, useMemo } from "react";
import { useNavigate, useLocation, Link, Navigate } from "react-router";
import { Search, Loader2, Car, Zap, Trees, UserRound, ArrowRight, ArrowLeft, Check } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { useCreateSearch } from "@/hooks/useSearches";
import { useToast } from "@/components/ui/Toast";
import { cn } from "@/lib/utils";
import { errorToHebrew } from "@/lib/error-messages";
import {
  type SearchFormData,
  defaultFormData,
  formToPayload,
  isFormValid,
  VehicleFields,
  BudgetFields,
  AdvancedFields,
} from "@/components/SearchFormFields";

interface SearchPreset {
  id: string;
  title: string;
  description: string;
  icon: typeof Car;
  values: Partial<SearchFormData>;
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

const STEPS = [
  { key: "vehicle", label: "רכב", shortLabel: "1" },
  { key: "budget", label: "תקציב", shortLabel: "2" },
  { key: "filters", label: "פילטרים", shortLabel: "3" },
] as const;

export function NewSearchPage() {
  const { user, loading } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const createSearch = useCreateSearch();
  const { toast } = useToast();
  const [error, setError] = useState<string | null>(null);
  const [step, setStep] = useState(0);

  // Pre-fill form from TrySearchPage search data passed via location state
  const [form, setForm] = useState<SearchFormData>(() => {
    const locState = typeof location.state === "object" && location.state !== null
      ? (location.state as Record<string, unknown>)
      : null;
    if (locState && "searchData" in locState && typeof locState.searchData === "object" && locState.searchData !== null) {
      const sd = locState.searchData as Record<string, unknown>;
      const base = defaultFormData();
      return {
        ...base,
        manufacturer: typeof sd.manufacturer === "number" ? sd.manufacturer : base.manufacturer,
        model: typeof sd.model === "number" ? sd.model : base.model,
        yearMin: typeof sd.yearMin === "number" ? sd.yearMin : base.yearMin,
        yearMax: typeof sd.yearMax === "number" ? sd.yearMax : base.yearMax,
        priceMin: typeof sd.priceMin === "number" ? sd.priceMin : base.priceMin,
        priceMax: typeof sd.priceMax === "number" ? sd.priceMax : base.priceMax,
        maxKm: typeof sd.maxKm === "number" ? sd.maxKm : base.maxKm,
        maxHand: typeof sd.maxHand === "number" ? sd.maxHand : base.maxHand,
      };
    }
    return defaultFormData();
  });

  const [presetsUsed, setPresetsUsed] = useState(false);
  const presets = useMemo(() => getPresets(), []);
  const showPresets = !presetsUsed && step === 0 && form.manufacturer === 0;

  if (!loading && !user) {
    return <Navigate to="/try" replace />;
  }

  const set = <K extends keyof SearchFormData>(key: K, val: SearchFormData[K]) =>
    setForm((prev) => ({ ...prev, [key]: val }));

  function applyPreset(preset: SearchPreset) {
    setForm((prev) => ({ ...prev, ...preset.values }));
    setPresetsUsed(true);
  }

  const canSubmit = isFormValid(form) && !createSearch.isPending;

  function handleSubmit() {
    setError(null);
    if (form.model > 0 && form.manufacturer === 0) {
      setError("יש לבחור יצרן כדי לבחור דגם");
      return;
    }
    createSearch.mutate(formToPayload(form), {
      onSuccess: () => {
        toast("החיפוש נוצר בהצלחה!", "success");
        navigate("/dashboard");
      },
      onError: (err) => setError(errorToHebrew(err)),
    });
  }

  const isLastStep = step === STEPS.length - 1;

  return (
    <div className="space-y-5 pb-24 md:pb-8 dir-rtl">
      {/* Header */}
      <header className="flex flex-col gap-1">
        <Link
          to="/dashboard"
          className="mb-2 inline-flex items-center gap-1.5 rounded-lg bg-secondary/60 px-3 py-1.5 text-sm text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground w-fit"
        >
          <span>חזרה</span>
          <ArrowRight className="h-4 w-4 shrink-0" aria-hidden />
        </Link>
        <h1 className="text-xl font-bold tracking-tight text-foreground sm:text-2xl">חיפוש חדש</h1>
      </header>

      {/* Step indicator */}
      <div className="flex items-center gap-2">
        {STEPS.map((s, i) => (
          <button
            key={s.key}
            type="button"
            onClick={() => setStep(i)}
            className={cn(
              "flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium transition-all duration-150",
              i === step
                ? "bg-primary/10 text-primary"
                : i < step
                  ? "bg-success/10 text-success"
                  : "bg-secondary text-muted-foreground",
            )}
          >
            <span className={cn(
              "flex h-6 w-6 items-center justify-center rounded-full text-xs font-bold",
              i === step
                ? "bg-primary text-white"
                : i < step
                  ? "bg-success text-white"
                  : "bg-muted text-muted-foreground",
            )}>
              {i < step ? <Check className="h-3.5 w-3.5" /> : s.shortLabel}
            </span>
            <span className="hidden sm:inline">{s.label}</span>
          </button>
        ))}
      </div>

      {error && (
        <div
          className="rounded-xl border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive"
          role="alert"
        >
          {error}
        </div>
      )}

      {/* Presets — only on step 0 when no manufacturer selected */}
      {showPresets && (
        <section aria-label="תבניות חיפוש מוכנות">
          <h2 className="text-sm font-semibold text-foreground mb-3">או התחל מתבנית</h2>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            {presets.map((preset) => {
              const Icon = preset.icon;
              return (
                <button
                  key={preset.id}
                  type="button"
                  onClick={() => applyPreset(preset)}
                  className="rounded-xl border border-border/50 bg-card p-4 text-start transition-all duration-150 hover:border-primary/40 hover:bg-primary/5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
                >
                  <Icon className="h-5 w-5 text-primary mb-2" />
                  <p className="text-sm font-semibold text-foreground">{preset.title}</p>
                  <p className="text-xs text-muted-foreground mt-0.5">{preset.description}</p>
                </button>
              );
            })}
          </div>
        </section>
      )}

      {/* Step content + Navigation wrapped in form for Enter key submission */}
      <form
        onSubmit={(e) => {
          e.preventDefault();
          if (isLastStep) {
            handleSubmit();
          } else {
            setStep((s) => s + 1);
          }
        }}
      >
        <div className="min-h-[300px]">
          {step === 0 && <VehicleFields form={form} set={set} />}
          {step === 1 && <BudgetFields form={form} set={set} />}
          {step === 2 && <AdvancedFields form={form} set={set} />}
        </div>

        {/* Navigation */}
        <div className="sticky bottom-[var(--bottom-nav-h)] landscape:bottom-14 md:bottom-0 z-40 -mx-4 px-4 py-3 bg-background/90 backdrop-blur-xl border-t border-border/30 md:static md:mx-0 md:px-0 md:py-0 md:bg-transparent md:backdrop-blur-none md:border-0 mt-5">
          <div className="flex items-center gap-3">
            {step > 0 && (
              <button
                type="button"
                onClick={() => setStep((s) => s - 1)}
                className="inline-flex items-center justify-center gap-2 rounded-xl border border-border px-5 py-3 text-sm font-medium text-muted-foreground transition-colors hover:bg-secondary"
              >
                <ArrowRight className="h-4 w-4" />
                הקודם
              </button>
            )}

            {isLastStep ? (
              <button
                type="submit"
                disabled={!canSubmit}
                className="flex-1 md:flex-none inline-flex items-center justify-center gap-2 bg-primary rounded-xl px-6 py-3 text-sm font-semibold text-white shadow-[0_4px_14px_-4px_var(--color-glow-primary)] transition-all duration-150 hover:opacity-95 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {createSearch.isPending ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <Search className="h-4 w-4" />
                )}
                צור חיפוש
              </button>
            ) : (
              <button
                type="submit"
                className="flex-1 md:flex-none inline-flex items-center justify-center gap-2 bg-primary rounded-xl px-6 py-3 text-sm font-semibold text-white shadow-[0_4px_14px_-4px_var(--color-glow-primary)] transition-all duration-150 hover:opacity-95"
              >
                הבא
                <ArrowLeft className="h-4 w-4" />
              </button>
            )}

            {step === 0 && (
              <Link
                to="/dashboard"
                className="inline-flex items-center justify-center rounded-xl border border-border px-5 py-3 text-sm font-medium text-muted-foreground transition-colors hover:bg-secondary"
              >
                ביטול
              </Link>
            )}
          </div>
        </div>
      </form>
    </div>
  );
}
