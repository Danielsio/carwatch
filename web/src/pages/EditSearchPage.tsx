import { useState, useEffect } from "react";
import { useNavigate, useParams, Link } from "react-router";
import { Save, Loader2 } from "lucide-react";
import { useSearch, useUpdateSearch } from "@/hooks/useSearches";
import { formatPrice, cn } from "@/lib/utils";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { FormField } from "@/components/ui/FormField";
import { PageHeader } from "@/components/ui/PageHeader";
import { Skeleton } from "@/components/ui/Skeleton";
import { EmptyState } from "@/components/ui/EmptyState";
import { useToast } from "@/components/ui/Toast";

const KM_OPTIONS = [0, 50_000, 100_000, 150_000, 200_000];
const HAND_OPTIONS = [0, 1, 2, 3, 4];

export function EditSearchPage() {
  const navigate = useNavigate();
  const { id } = useParams();
  const searchId = Number(id);
  const { data: search, isLoading, isError } = useSearch(searchId);
  const updateSearch = useUpdateSearch();
  const { toast } = useToast();
  const [error, setError] = useState<string | null>(null);

  const [form, setForm] = useState({
    yearMin: 0,
    yearMax: 0,
    priceMax: 0,
    engineMinCC: 0,
    maxKm: 0,
    maxHand: 0,
    keywords: "",
    excludeKeys: "",
  });

  useEffect(() => {
    if (search) {
      setForm({
        yearMin: search.year_min,
        yearMax: search.year_max,
        priceMax: search.price_max,
        engineMinCC: search.engine_min_cc,
        maxKm: search.max_km,
        maxHand: search.max_hand,
        keywords: search.keywords,
        excludeKeys: search.exclude_keys,
      });
    }
  }, [search]);

  const set = <K extends keyof typeof form>(key: K, val: (typeof form)[K]) =>
    setForm((prev) => ({ ...prev, [key]: val }));

  const canSubmit =
    form.yearMin >= 2000 &&
    form.yearMax >= 2000 &&
    form.yearMin <= form.yearMax &&
    !updateSearch.isPending;

  function handleSubmit() {
    setError(null);
    updateSearch.mutate(
      {
        id: searchId,
        data: {
          year_min: form.yearMin,
          year_max: form.yearMax,
          price_max: form.priceMax,
          engine_min_cc: form.engineMinCC || undefined,
          max_km: form.maxKm,
          max_hand: form.maxHand,
          keywords: form.keywords || undefined,
          exclude_keys: form.excludeKeys || undefined,
        },
      },
      {
        onSuccess: () => {
          toast("החיפוש עודכן בהצלחה!", "success");
          navigate("/");
        },
        onError: () => setError("שגיאה בעדכון החיפוש, נסה שוב"),
      },
    );
  }

  if (isLoading) {
    return (
      <div className="space-y-6 pb-24 md:pb-8">
        <Skeleton className="h-8 w-48 rounded-lg" />
        <Skeleton className="h-64 rounded-2xl" />
        <Skeleton className="h-48 rounded-2xl" />
      </div>
    );
  }

  if (isError || !search) {
    return (
      <EmptyState
        icon={Save}
        title="החיפוש לא נמצא"
        description="ניתן לחזור לדף הראשי"
        action={
          <Button asChild>
            <Link to="/">חזרה לחיפושים</Link>
          </Button>
        }
      />
    );
  }

  return (
    <div className="space-y-6 pb-24 md:pb-8">
      <PageHeader
        title={`עריכת ${search.manufacturer_name} ${search.model_name}`}
        subtitle={`מקור: ${search.source}`}
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

      <section className="rounded-2xl border border-border/50 bg-card p-5 space-y-5">
        <h2 className="text-sm font-semibold text-foreground">טווח שנים</h2>
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

      <div className="flex items-center gap-3">
        <Button onClick={handleSubmit} disabled={!canSubmit} size="lg">
          {updateSearch.isPending ? (
            <Loader2 className="h-4 w-4 animate-spin" />
          ) : (
            <Save className="h-4 w-4" />
          )}
          שמור שינויים
        </Button>
        <Button variant="secondary" size="lg" asChild>
          <Link to="/">ביטול</Link>
        </Button>
      </div>
    </div>
  );
}

function ChipButton({
  selected,
  onClick,
  children,
}: {
  selected: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={selected}
      className={cn(
        "rounded-xl border px-3.5 py-2 text-sm transition-all duration-200 active:scale-[0.97]",
        selected
          ? "border-primary bg-primary/10 text-primary ring-1 ring-primary/20"
          : "border-border/50 bg-card hover:border-border hover:bg-surface-hover text-secondary-foreground",
      )}
    >
      {children}
    </button>
  );
}
