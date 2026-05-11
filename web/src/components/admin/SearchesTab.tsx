import { useState } from "react";
import {
  AlertCircle,
  Car,
  FileSearch,
  RefreshCw,
  Trash2,
} from "lucide-react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { AnimatePresence, motion } from "motion/react";
import { adminApi, type AdminSearch } from "@/lib/api";
import { EmptyState, Skeleton, Badge } from "@/components/ui";
import { useToast } from "@/components/ui/Toast";
import { cn, formatPrice, relativeTime } from "@/lib/utils";
import { ConfirmModal } from "./ConfirmModal";
import { DetailModal } from "./DetailModal";

function formatSellerFilter(v: string | undefined): string | null {
  if (v == null || v === "") return null;
  const x = v.toLowerCase();
  if (x === "any") return "הכל";
  if (x === "private") return "פרטי";
  if (x === "commercial") return "מסחרי";
  return v;
}

export function SearchesTab({ onViewListings }: { onViewListings: (searchId: number) => void }) {
  const [detailSearch, setDetailSearch] = useState<AdminSearch | null>(null);
  const [confirmDelete, setConfirmDelete] = useState<AdminSearch | null>(null);
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ["admin", "searches"],
    queryFn: adminApi.searches,
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => adminApi.deleteSearch(id),
    onSuccess: () => {
      toast("החיפוש נמחק בהצלחה", "success");
      setConfirmDelete(null);
      setDetailSearch(null);
      void queryClient.invalidateQueries({ queryKey: ["admin"] });
    },
    onError: () => {
      toast("שגיאה במחיקת החיפוש", "error");
    },
  });

  return (
    <div className="space-y-4">
      <div className="flex gap-3 justify-end">
        <button
          type="button"
          onClick={() => void refetch()}
          className="inline-flex items-center gap-2 px-4 py-2.5 rounded-xl bg-secondary hover:bg-secondary/80 text-sm font-medium text-foreground transition-colors"
        >
          <RefreshCw className="h-3.5 w-3.5" />
          רענן
        </button>
      </div>

      <div className="rounded-2xl border border-border/50 bg-card overflow-hidden">
        <div className="flex items-center justify-between px-6 py-4 border-b border-border/50">
          <h2 className="text-base font-semibold">כל החיפושים</h2>
          {data && (
            <span className="text-sm text-muted-foreground tabular-nums">
              {data.total} סה״כ
            </span>
          )}
        </div>

        <div className="p-3">
          {isLoading ? (
            <div className="space-y-2 p-3">
              {Array.from({ length: 3 }).map((_, i) => (
                <Skeleton key={i} className="h-16 rounded-xl" />
              ))}
            </div>
          ) : isError ? (
            <EmptyState
              icon={AlertCircle}
              title="שגיאה בטעינת החיפושים"
              className="border-0 bg-transparent"
            />
          ) : !data || data.items.length === 0 ? (
            <EmptyState
              icon={FileSearch}
              title="אין חיפושים"
              description="עדיין לא נוצרו חיפושים במערכת"
              className="border-0 bg-transparent"
            />
          ) : (
            <div className="space-y-1.5">
              {data.items.map((search) => (
                <motion.div
                  key={search.id}
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  className="flex items-center gap-3 rounded-xl bg-secondary/40 px-4 py-3 transition-colors hover:bg-secondary cursor-pointer group focus-visible:ring-2 focus-visible:ring-ring outline-none"
                  tabIndex={0}
                  role="button"
                  onClick={() => setDetailSearch(search)}
                  onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); setDetailSearch(search); } }}
                >
                  <div
                    className={cn(
                      "h-2.5 w-2.5 rounded-full flex-shrink-0",
                      search.active
                        ? "bg-success animate-pulse-soft"
                        : "bg-muted-foreground/40",
                    )}
                  />
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium truncate">
                      {search.name || `חיפוש #${search.id}`}
                    </p>
                    <div className="flex items-center gap-2 mt-0.5 flex-wrap">
                      <span className="text-xs text-muted-foreground">
                        מקור: {search.source}
                      </span>
                      {search.price_max > 0 && (
                        <span className="text-xs text-muted-foreground">
                          · עד {formatPrice(search.price_max)}
                        </span>
                      )}
                      <span className="text-xs text-muted-foreground">
                        · {relativeTime(search.created_at)}
                      </span>
                    </div>
                  </div>
                  <Badge variant={search.active ? "success" : "default"}>
                    {search.active ? "פעיל" : "מושהה"}
                  </Badge>
                  <div className="flex items-center gap-1 flex-shrink-0 opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 transition-opacity">
                    <button
                      type="button"
                      onClick={(e) => { e.stopPropagation(); onViewListings(search.id); }}
                      aria-label="צפה במודעות"
                      title="צפה במודעות"
                      className="rounded-lg p-1.5 text-muted-foreground/50 transition-colors hover:text-primary hover:bg-primary/10"
                    >
                      <Car className="h-3.5 w-3.5" />
                    </button>
                    <button
                      type="button"
                      onClick={(e) => { e.stopPropagation(); setConfirmDelete(search); }}
                      aria-label={`מחק חיפוש ${search.name || search.id}`}
                      title="מחק חיפוש"
                      className="rounded-lg p-1.5 text-muted-foreground/50 transition-colors hover:text-destructive hover:bg-destructive/10"
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </button>
                  </div>
                  <span className="text-xs text-muted-foreground tabular-nums flex-shrink-0">
                    #{search.chat_id}
                  </span>
                </motion.div>
              ))}
            </div>
          )}
        </div>
      </div>

      <AnimatePresence>
        {detailSearch && (
          <DetailModal
            title={detailSearch.name || `חיפוש #${detailSearch.id}`}
            fields={[
              { label: "מזהה", value: detailSearch.id },
              { label: "שם", value: detailSearch.name },
              { label: "מקור", value: detailSearch.source },
              { label: "יצרן", value: detailSearch.manufacturer || null },
              { label: "דגם", value: detailSearch.model || null },
              { label: "מילות חיפוש", value: detailSearch.keywords },
              { label: "מילות לא כלולות", value: detailSearch.exclude_keys },
              {
                label: "סינון מוכר",
                value: formatSellerFilter(detailSearch.seller_filter),
              },
              {
                label: "נפח מנוע מינימום",
                value:
                  detailSearch.engine_min_cc > 0
                    ? `${detailSearch.engine_min_cc.toLocaleString("he-IL")} סמ״ק`
                    : null,
              },
              { label: "שנה מ-", value: detailSearch.year_min || null },
              { label: "שנה עד", value: detailSearch.year_max || null },
              {
                label: "מחיר מקס׳",
                value: detailSearch.price_max
                  ? formatPrice(detailSearch.price_max)
                  : null,
              },
              {
                label: "ק״מ מקס׳",
                value: detailSearch.max_km
                  ? detailSearch.max_km.toLocaleString("he-IL")
                  : null,
              },
              { label: "יד מקס׳", value: detailSearch.max_hand || null },
              {
                label: "סטטוס",
                value: detailSearch.active ? "פעיל" : "מושהה",
              },
              { label: "Chat ID", value: detailSearch.chat_id },
              {
                label: "נוצר",
                value: relativeTime(detailSearch.created_at),
              },
            ]}
            onClose={() => setDetailSearch(null)}
            actions={
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={() => { setDetailSearch(null); onViewListings(detailSearch.id); }}
                  className="inline-flex items-center gap-1.5 rounded-xl bg-primary/10 px-3 py-2 text-xs font-medium text-primary transition-colors hover:bg-primary/20"
                >
                  <Car className="h-3 w-3" />
                  צפה במודעות
                </button>
                <button
                  type="button"
                  onClick={() => { setDetailSearch(null); setConfirmDelete(detailSearch); }}
                  className="inline-flex items-center gap-1.5 rounded-xl bg-destructive/10 px-3 py-2 text-xs font-medium text-destructive transition-colors hover:bg-destructive/20"
                >
                  <Trash2 className="h-3 w-3" />
                  מחק חיפוש
                </button>
              </div>
            }
          />
        )}
      </AnimatePresence>

      <AnimatePresence>
        {confirmDelete && (
          <ConfirmModal
            message={`למחוק את החיפוש "${confirmDelete.name || `#${confirmDelete.id}`}" של משתמש #${confirmDelete.chat_id}? פעולה זו תמחק גם את כל המודעות וההיסטוריה המשויכים.`}
            onConfirm={() => deleteMutation.mutate(confirmDelete.id)}
            onCancel={() => setConfirmDelete(null)}
            loading={deleteMutation.isPending}
          />
        )}
      </AnimatePresence>
    </div>
  );
}
