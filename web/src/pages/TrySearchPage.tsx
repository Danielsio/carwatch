import { useState, useEffect } from "react";
import { Link } from "react-router";
import { Search, Loader2, SearchX, ShieldAlert } from "lucide-react";
import { useQuery, useMutation } from "@tanstack/react-query";
import {
  guestApi,
  ApiError,
  type Listing,
  type Manufacturer,
  type Model,
} from "@/lib/api";
import { formatPrice } from "@/lib/utils";
import { ChipButton } from "@/components/ui/ChipButton";
import { Input } from "@/components/ui/Input";
import { RangeSlider } from "@/components/ui/RangeSlider";
import { Select } from "@/components/ui/Select";
import { FormField } from "@/components/ui/FormField";
import { EmptyState } from "@/components/ui/EmptyState";
import { ListingCardBody } from "@/components/ListingCardBody";
import { ListingCardSkeleton } from "@/components/ListingCardSkeleton";

/* ------------------------------------------------------------------ */
/*  SectionCard — mirrors the pattern from NewSearchPage               */
/* ------------------------------------------------------------------ */

function SectionCard({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div className="bg-card border border-border rounded-2xl overflow-hidden">
      <div className="px-6 py-3.5 border-b border-border bg-secondary/30">
        <h2 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
          {title}
        </h2>
      </div>
      <div className="p-6 space-y-4">{children}</div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Constants                                                          */
/* ------------------------------------------------------------------ */

const HAND_OPTIONS = [0, 1, 2, 3, 4];

function formatKmLabel(value: number): string {
  if (value === 0) return "ללא הגבלה";
  return `${value.toLocaleString("he-IL")} ק"מ`;
}

/* ------------------------------------------------------------------ */
/*  Form state                                                         */
/* ------------------------------------------------------------------ */

interface GuestFormData {
  manufacturer: number;
  model: number;
  yearMin: number;
  yearMax: number;
  priceMin: number;
  priceMax: number;
  maxKm: number;
  maxHand: number;
}

const INITIAL_FORM: GuestFormData = {
  manufacturer: 0,
  model: 0,
  yearMin: 2018,
  yearMax: new Date().getFullYear(),
  priceMin: 0,
  priceMax: 0,
  maxKm: 0,
  maxHand: 0,
};

const STORAGE_KEY = "carwatch_try_search_form";

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export default function TrySearchPage() {
  const [form, setForm] = useState<GuestFormData>(() => {
    try {
      const saved = sessionStorage.getItem(STORAGE_KEY);
      return saved ? { ...INITIAL_FORM, ...JSON.parse(saved) } : INITIAL_FORM;
    } catch {
      return INITIAL_FORM;
    }
  });
  const [rateLimited, setRateLimited] = useState(false);

  useEffect(() => {
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify(form));
  }, [form]);

  const set = <K extends keyof GuestFormData>(key: K, val: GuestFormData[K]) =>
    setForm((prev) => ({ ...prev, [key]: val }));

  /* -- Guest catalog queries -- */
  const { data: manufacturers } = useQuery<Manufacturer[]>({
    queryKey: ["guest-manufacturers"],
    queryFn: () => guestApi.catalog.manufacturers(),
    staleTime: 5 * 60_000,
  });

  const { data: models } = useQuery<Model[]>({
    queryKey: ["guest-models", form.manufacturer],
    queryFn: () => guestApi.catalog.models(form.manufacturer),
    enabled: form.manufacturer > 0,
    staleTime: 5 * 60_000,
  });

  /* -- Instant search mutation -- */
  const search = useMutation({
    mutationFn: () => {
      setRateLimited(false);
      return guestApi.instantSearch({
        source: "yad2",
        manufacturer: form.manufacturer,
        model: form.model,
        year_min: form.yearMin || undefined,
        year_max: form.yearMax || undefined,
        price_min: form.priceMin || undefined,
        price_max: form.priceMax || undefined,
        max_km: form.maxKm || undefined,
        max_hand: form.maxHand || undefined,
      });
    },
    onError: (err) => {
      if (err instanceof ApiError && err.status === 429) {
        setRateLimited(true);
      }
    },
  });

  const results: Listing[] | undefined = search.data?.items;
  const total = search.data?.total ?? 0;

  const validYear = (y: number) => y === 0 || y >= 1990;
  const canSubmit =
    form.manufacturer > 0 &&
    validYear(form.yearMin) &&
    validYear(form.yearMax) &&
    (form.yearMin === 0 ||
      form.yearMax === 0 ||
      form.yearMin <= form.yearMax) &&
    !search.isPending;

  function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    search.mutate();
  }

  return (
    <div className="mx-auto max-w-3xl px-4 py-10 sm:px-6 dir-rtl">
      {/* Header */}
      <header className="mb-8 text-center">
        <h1 className="text-3xl font-bold tracking-tight text-foreground">
          נסה חיפוש חינם
        </h1>
        <p className="mt-2 text-sm text-muted-foreground">
          מצא רכב במחיר הטוב ביותר — ללא הרשמה
        </p>
      </header>

      {/* Rate limit banner */}
      {rateLimited && (
        <div
          className="mb-6 rounded-2xl border border-amber-500/30 bg-amber-500/10 p-4 text-center text-sm text-amber-700 dark:text-amber-400"
          role="alert"
        >
          <ShieldAlert className="mx-auto mb-1 h-5 w-5" aria-hidden />
          הגעת למגבלת חיפושים. הירשם לחיפושים ללא הגבלה
        </div>
      )}

      <form onSubmit={handleSubmit} className="space-y-5">
        {/* Vehicle filter */}
        <SectionCard title="סינון לפי רכב">
          <div className="grid gap-4 sm:grid-cols-2">
            <FormField label="יצרן" htmlFor="guest-mfr">
              <Select
                id="guest-mfr"
                value={form.manufacturer}
                onChange={(e) => {
                  set("manufacturer", Number(e.target.value));
                  set("model", 0);
                }}
              >
                <option value={0}>בחר יצרן...</option>
                {manufacturers?.map((m) => (
                  <option key={m.id} value={m.id}>
                    {m.name_he && m.name_he !== m.name
                      ? `${m.name_he} (${m.name})`
                      : m.name}
                  </option>
                ))}
              </Select>
            </FormField>

            <FormField label="דגם" htmlFor="guest-mdl">
              <Select
                id="guest-mdl"
                value={form.model}
                disabled={form.manufacturer === 0}
                onChange={(e) => set("model", Number(e.target.value))}
              >
                <option value={0}>
                  {form.manufacturer === 0 ? "יש לבחור יצרן קודם" : "כל הדגמים"}
                </option>
                {models?.map((m) => (
                  <option key={m.id} value={m.id}>
                    {m.name_he && m.name_he !== m.name
                      ? `${m.name_he} (${m.name})`
                      : m.name}
                  </option>
                ))}
              </Select>
            </FormField>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <FormField
              label="שנה מ-"
              htmlFor="guest-yearMin"
              error={
                form.yearMin > form.yearMax
                  ? "שנה מינימלית חייבת להיות קטנה מהמקסימלית"
                  : undefined
              }
            >
              <Input
                id="guest-yearMin"
                type="number"
                value={form.yearMin}
                onChange={(e) => set("yearMin", Number(e.target.value))}
                min={1990}
                max={2030}
                error={form.yearMin > form.yearMax}
                className="tabular-nums"
              />
            </FormField>

            <FormField label="שנה עד" htmlFor="guest-yearMax">
              <Input
                id="guest-yearMax"
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

        {/* Price & km */}
        <SectionCard title='מחיר וק"מ'>
          <div className="grid gap-4 sm:grid-cols-2">
            <FormField
              label="מחיר מינימום (₪)"
              htmlFor="guest-priceMin"
              hint={form.priceMin > 0 ? formatPrice(form.priceMin) : undefined}
            >
              <Input
                id="guest-priceMin"
                type="number"
                value={form.priceMin || ""}
                onChange={(e) => set("priceMin", Number(e.target.value))}
                placeholder="ללא הגבלה"
                className="tabular-nums"
              />
            </FormField>

            <FormField
              label="מחיר מקסימום (₪)"
              htmlFor="guest-priceMax"
              hint={form.priceMax > 0 ? formatPrice(form.priceMax) : undefined}
            >
              <Input
                id="guest-priceMax"
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

        {/* Submit */}
        <button
          type="submit"
          disabled={!canSubmit}
          className="w-full inline-flex items-center justify-center gap-2 bg-primary rounded-2xl px-6 py-3.5 text-sm font-semibold text-white shadow-lg shadow-primary/20 transition-all hover:-translate-y-px hover:shadow-xl hover:shadow-primary/30 disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:translate-y-0 disabled:hover:shadow-lg"
        >
          {search.isPending ? (
            <Loader2 className="h-4 w-4 animate-spin" />
          ) : (
            <Search className="h-4 w-4" />
          )}
          חפש עכשיו
        </button>
      </form>

      {/* ------------------------------------------------------------ */}
      {/*  Results                                                       */}
      {/* ------------------------------------------------------------ */}

      {/* Loading skeletons */}
      {search.isPending && (
        <div className="mt-10">
          <div className="mb-4 h-5 w-32 rounded shimmer-skeleton" />
          <div className="grid gap-4 sm:grid-cols-2">
            {Array.from({ length: 6 }).map((_, i) => (
              <ListingCardSkeleton key={i} />
            ))}
          </div>
        </div>
      )}

      {/* Error (non-rate-limit) */}
      {search.isError && !rateLimited && (
        <div
          className="mt-10 rounded-2xl border border-destructive/20 bg-destructive/5 p-4 text-center text-sm text-destructive"
          role="alert"
        >
          שגיאה בביצוע החיפוש, נסה שוב
        </div>
      )}

      {/* No results */}
      {search.isSuccess && results?.length === 0 && (
        <div className="mt-10">
          <EmptyState
            icon={SearchX}
            title="לא נמצאו תוצאות"
            description="נסה לשנות את הפילטרים"
          />
        </div>
      )}

      {/* Results grid */}
      {search.isSuccess && results && results.length > 0 && (
        <div className="mt-10">
          <h2 className="mb-4 text-sm font-semibold text-muted-foreground">
            נמצאו {total.toLocaleString("he-IL")} תוצאות
          </h2>

          <div className="grid gap-4 sm:grid-cols-2">
            {results.map((listing) => (
              <a
                key={listing.token}
                href={listing.page_link}
                target="_blank"
                rel="noopener noreferrer"
                className="group rounded-2xl border border-border/50 bg-card overflow-hidden transition-all hover:border-primary/30 hover:shadow-lg"
              >
                <ListingCardBody listing={listing} hoverScale />
              </a>
            ))}
          </div>

          {/* CTA banner */}
          <div className="mt-8 rounded-2xl border border-primary/20 bg-primary/5 p-6 text-center">
            <h3 className="text-lg font-bold mb-2">אהבת את התוצאות?</h3>
            <p className="text-sm text-muted-foreground mb-4">
              הירשם בחינם וקבל: התראות בזמן אמת &bull; מעקב ירידות מחיר &bull;
              שמירת מודעות &bull; ניקוד עסקאות
            </p>
            <div className="flex gap-3 justify-center">
              <Link
                to="/signup"
                state={{ from: "/searches/new", searchData: form }}
                className="rounded-xl bg-primary px-6 py-2.5 text-sm font-semibold text-white shadow-lg"
              >
                הירשם בחינם
              </Link>
              <Link
                to="/login"
                state={{ from: "/searches/new", searchData: form }}
                className="rounded-xl border border-border px-6 py-2.5 text-sm font-medium"
              >
                התחבר
              </Link>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
