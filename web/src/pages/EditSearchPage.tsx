import { useState, useEffect, useRef } from "react";
import { useNavigate, useParams, Link } from "react-router";
import { Save, Loader2, ArrowRight } from "lucide-react";
import { useSearch, useUpdateSearch } from "@/hooks/useSearches";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/EmptyState";
import { useToast } from "@/components/ui/Toast";
import {
  type SearchFormData,
  defaultFormData,
  normalizeSellerFilter,
  formToUpdatePayload,
  isFormValid,
  BudgetFields,
  AdvancedFields,
} from "@/components/SearchFormFields";

export function EditSearchPage() {
  const navigate = useNavigate();
  const { id } = useParams();
  const searchId = Number(id);
  const { data: search, isLoading, isError } = useSearch(searchId);
  const updateSearch = useUpdateSearch();
  const { toast } = useToast();
  const [error, setError] = useState<string | null>(null);
  const [form, setForm] = useState<SearchFormData>(defaultFormData);

  const initializedSearchIdRef = useRef<number | null>(null);

  useEffect(() => {
    if (search && initializedSearchIdRef.current !== search.id) {
      setForm((prev) => ({
        ...prev,
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
      }));
      initializedSearchIdRef.current = search.id;
    }
  }, [search]);

  const set = <K extends keyof SearchFormData>(key: K, val: SearchFormData[K]) =>
    setForm((prev) => ({ ...prev, [key]: val }));

  const canSubmit = isFormValid(form) && !updateSearch.isPending;

  function handleFormSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError(null);
    updateSearch.mutate(
      { id: searchId, data: formToUpdatePayload(form) },
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
      <div className="space-y-5 pb-24 md:pb-8 dir-rtl">
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
    <div className="space-y-5 pb-24 md:pb-8 dir-rtl">
      <header className="flex flex-col gap-1">
        <Link
          to="/dashboard"
          className="mb-2 inline-flex items-center gap-1.5 rounded-lg bg-secondary/60 px-3 py-1.5 text-sm text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground w-fit"
        >
          <span>חזרה</span>
          <ArrowRight className="h-4 w-4 shrink-0" aria-hidden />
        </Link>
        <h1 className="text-xl font-bold tracking-tight text-foreground sm:text-2xl">
          עריכת {search.manufacturer_name} {search.model_name}
        </h1>
        <p className="text-sm text-muted-foreground">מקור: {search.source}</p>
      </header>

      {error && (
        <div
          className="rounded-xl border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive"
          role="alert"
        >
          {error}
        </div>
      )}

      <form onSubmit={handleFormSubmit} className="space-y-5">
        <BudgetFields form={form} set={set} />
        <AdvancedFields form={form} set={set} />

        <div className="sticky bottom-[calc(4.5rem+env(safe-area-inset-bottom,0px))] landscape:bottom-14 md:bottom-0 z-40 -mx-4 px-4 py-3 bg-background/90 border-t border-border/30 md:static md:mx-0 md:px-0 md:py-0 md:bg-transparent md: md:border-0">
          <div className="flex items-center gap-3">
            <button
              type="submit"
              disabled={!canSubmit}
              className="flex-1 md:flex-none inline-flex items-center justify-center gap-2 bg-primary rounded-xl px-6 py-3 text-sm font-semibold text-white transition-all duration-150 hover:opacity-95 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {updateSearch.isPending ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <Save className="h-4 w-4" />
              )}
              שמור שינויים
            </button>
            <Link
              to="/dashboard"
              className="inline-flex items-center justify-center rounded-xl border border-border px-6 py-3 text-sm font-medium text-muted-foreground transition-colors hover:bg-secondary md:flex-none"
            >
              ביטול
            </Link>
          </div>
        </div>
      </form>
    </div>
  );
}
