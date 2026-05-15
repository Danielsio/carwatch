import { useState, useEffect, useRef } from "react";
import { useNavigate, useParams, Link } from "react-router";
import { Save, Loader2 } from "lucide-react";
import { useSearch, useUpdateSearch } from "@/hooks/useSearches";
import { formatPrice } from "@/lib/utils";
import { Button } from "@/components/ui/Button";
import { ChipButton } from "@/components/ui/ChipButton";
import { Input } from "@/components/ui/Input";
import { RangeSlider } from "@/components/ui/RangeSlider";
import { FormField } from "@/components/ui/FormField";
import { PageHeader } from "@/components/ui/PageHeader";
import { Skeleton } from "@/components/ui/Skeleton";
import { EmptyState } from "@/components/ui/EmptyState";
import { useToast } from "@/components/ui/Toast";

const HAND_OPTIONS = [0, 1, 2, 3, 4];

const SELLER_FILTER_OPTIONS: {
  value: "any" | "private" | "commercial";
  label: string;
}[] = [
  { value: "any", label: "הכל" },
  { value: "private", label: "מוכר פרטי" },
  { value: "commercial", label: "מוסך / סוכנות" },
];

const GEARBOX_OPTIONS = [
  { value: "", label: "הכל" },
  { value: "אוטומט", label: "אוטומט" },
  { value: "ידני", label: "ידני" },
];

function normalizeSellerFilter(v: string | undefined): "any" | "private" | "commercial" {
  const s = (v ?? "any").toLowerCase().trim();
  if (s === "private") return "private";
  if (s === "commercial" || s === "dealer" || s === "dealership") {
    return "commercial";
  }
  return "any";
}

function formatKmLabel(value: number): string {
  if (value === 0) return "ללא הגבלה";
  return `${value.toLocaleString("he-IL")} ק"מ`;
}

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
    priceMin: 0,
    priceMax: 0,
    engineMinCC: 0,
    maxKm: 0,
    maxHand: 0,
    keywords: "",
    excludeKeys: "",
    sellerFilter: "any" as "any" | "private" | "commercial",
    gearBox: "",
    priceOnly: false,
    photoOnly: false,
  });

  const initializedSearchIdRef = useRef<number | null>(null);

  useEffect(() => {
    if (search && initializedSearchIdRef.current !== search.id) {
      setForm({
        yearMin: search.year_min,
        yearMax: search.year_max,
        priceMin: search.price_min ?? 0,
        priceMax: search.price_max,
        engineMinCC: search.engine_min_cc,
        maxKm: search.max_km,
        maxHand: search.max_hand,
        keywords: search.keywords,
        excludeKeys: search.exclude_keys,
        sellerFilter: normalizeSellerFilter(search.seller_filter),
        gearBox: search.gear_box ?? "",
        priceOnly: search.price_only ?? false,
        photoOnly: search.photo_only ?? false,
      });
      initializedSearchIdRef.current = search.id;
    }
  }, [search]);

  const set = <K extends keyof typeof form>(key: K, val: (typeof form)[K]) =>
    setForm((prev) => ({ ...prev, [key]: val }));

  const validYear = (y: number) => y === 0 || y >= 1990;
  const canSubmit =
    validYear(form.yearMin) &&
    validYear(form.yearMax) &&
    (form.yearMin === 0 || form.yearMax === 0 || form.yearMin <= form.yearMax) &&
    !updateSearch.isPending;

  function handleFormSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError(null);
    updateSearch.mutate(
      {
        id: searchId,
        data: {
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
        },
      },
      {
        onSuccess: () => {
          toast("החיפוש עודכן בהצלחה!", "success");
          navigate("/dashboard");
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
            <Link to="/dashboard">חזרה לחיפושים</Link>
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
        backTo="/dashboard"
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

      <form onSubmit={handleFormSubmit} className="contents">
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
      </section>

      <section className="rounded-2xl border border-border/50 bg-card p-5 space-y-5">
        <h2 className="text-sm font-semibold text-foreground">מחיר וק&quot;מ</h2>

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

          <FormField label="תיבת הילוכים" htmlFor="gearBox">
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
        </div>

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

        <div className="space-y-3">
          <h3 className="text-xs font-medium text-muted-foreground">סוג מוכר</h3>
          <p className="text-xs text-muted-foreground">
            מסנן לפי מודעות ממוכר פרטי או ממוסך/סוכנות.
          </p>
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

        <div className="space-y-3">
          <h3 className="text-xs font-medium text-muted-foreground">סינון מודעות</h3>
          <p className="text-xs text-muted-foreground">
            הצג רק מודעות שעומדות בתנאים הבאים.
          </p>
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

      <div className="sticky bottom-[calc(4rem+env(safe-area-inset-bottom,0px))] landscape:bottom-14 md:bottom-0 z-40 -mx-4 px-4 py-3 bg-background/90 backdrop-blur-xl border-t border-border/30 md:static md:mx-0 md:px-0 md:py-0 md:bg-transparent md:backdrop-blur-none md:border-0">
        <div className="flex items-center gap-3">
          <Button type="submit" disabled={!canSubmit} size="lg" className="flex-1 md:flex-none">
            {updateSearch.isPending ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <Save className="h-4 w-4" />
            )}
            שמור שינויים
          </Button>
          <Button variant="secondary" size="lg" asChild className="md:flex-none">
            <Link to="/dashboard">ביטול</Link>
          </Button>
        </div>
      </div>
      </form>
    </div>
  );
}

