import { useEffect, useState } from "react";
import {
  AlertCircle,
  Car,
  ChevronLeft,
  ChevronRight,
  ExternalLink,
  RefreshCw,
  Search,
  Trash2,
  X,
} from "lucide-react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { AnimatePresence, motion } from "motion/react";
import { adminApi, type AdminListing } from "@/lib/api";
import { EmptyState, Skeleton, Badge } from "@/components/ui";
import { useToast } from "@/components/ui/Toast";
import { formatKm, formatPrice, relativeTime, safeHref } from "@/lib/utils";
import { ConfirmModal } from "./ConfirmModal";
import { DetailModal } from "./DetailModal";

export function ListingsTab({
  searchId,
  onClearFilter,
}: {
  searchId: number | null;
  onClearFilter: () => void;
}) {
  const [page, setPage] = useState(0);
  const [searchQuery, setSearchQuery] = useState("");
  const [detailListing, setDetailListing] = useState<AdminListing | null>(null);
  const [confirmDelete, setConfirmDelete] = useState<AdminListing | null>(null);
  const pageSize = 20;
  const { toast } = useToast();
  const queryClient = useQueryClient();

  useEffect(() => {
    setPage(0);
  }, [searchId]);

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ["admin", "listings", page, searchId],
    queryFn: () =>
      adminApi.listings({
        limit: pageSize,
        offset: page * pageSize,
        ...(searchId ? { search_id: searchId } : {}),
      }),
  });

  const deleteMutation = useMutation({
    mutationFn: ({ token, chatId }: { token: string; chatId: number }) =>
      adminApi.deleteListing(token, chatId),
    meta: { suppressToast: true },
    onSuccess: () => {
      toast("המודעה נמחקה", "success");
      setConfirmDelete(null);
      void queryClient.invalidateQueries({ queryKey: ["admin"] });
    },
    onError: () => {
      toast("שגיאה במחיקת המודעה", "error");
    },
  });

  const totalPages = data ? Math.ceil(data.total / pageSize) : 0;

  const filtered =
    searchQuery && data
      ? data.items.filter(
          (l) =>
            l.manufacturer?.toLowerCase().includes(searchQuery.toLowerCase()) ||
            l.model?.toLowerCase().includes(searchQuery.toLowerCase()) ||
            l.city?.toLowerCase().includes(searchQuery.toLowerCase()),
        )
      : data?.items ?? [];

  return (
    <div className="space-y-4">
      {/* Filter banner */}
      {searchId && (
        <div className="flex items-center justify-between rounded-xl bg-primary/5 border border-primary/20 px-4 py-3">
          <span className="text-sm text-foreground">
            מסנן לפי חיפוש #{searchId}
            {data && (
              <span className="text-muted-foreground mr-2">
                · {data.total} מודעות
              </span>
            )}
          </span>
          <button
            type="button"
            onClick={onClearFilter}
            className="inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-xs font-medium text-primary hover:bg-primary/10 transition-colors"
          >
            <X className="h-3 w-3" />
            הסר סינון
          </button>
        </div>
      )}

      {/* Header with search */}
      <div className="flex flex-col sm:flex-row gap-3">
        <div className="relative flex-1">
          <Search className="absolute right-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="סנן עמוד נוכחי לפי יצרן, דגם, עיר..."
            className="w-full bg-secondary/50 border border-border rounded-xl pe-10 ps-4 py-2.5 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring transition-colors"
          />
        </div>
        <button
          type="button"
          onClick={() => void refetch()}
          className="inline-flex items-center gap-2 px-4 py-2.5 rounded-xl bg-secondary hover:bg-secondary/80 text-sm font-medium text-foreground transition-colors"
        >
          <RefreshCw className="h-3.5 w-3.5" />
          רענן
        </button>
      </div>

      {/* Listings */}
      <div className="rounded-2xl border border-border/50 bg-card overflow-hidden">
        <div className="flex items-center justify-between px-6 py-4 border-b border-border/50">
          <h2 className="text-base font-semibold">
            {searchId ? "מודעות לחיפוש" : "כל המודעות"}
          </h2>
          {data && (
            <span className="text-sm text-muted-foreground tabular-nums">
              {data.total.toLocaleString("he-IL")} סה״כ
            </span>
          )}
        </div>

        <div className="p-3">
          {isLoading ? (
            <div className="space-y-2 p-3">
              {Array.from({ length: 5 }).map((_, i) => (
                <Skeleton key={i} className="h-[72px] rounded-xl" />
              ))}
            </div>
          ) : isError ? (
            <EmptyState
              icon={AlertCircle}
              title="שגיאה בטעינת המודעות"
              className="border-0 bg-transparent"
            />
          ) : filtered.length === 0 ? (
            <EmptyState
              icon={Car}
              title="אין מודעות"
              description={
                searchQuery
                  ? "לא נמצאו מודעות התואמות לחיפוש"
                  : "עדיין לא נמצאו מודעות במערכת"
              }
              className="border-0 bg-transparent"
            />
          ) : (
            <div className="space-y-1.5">
              <AnimatePresence mode="popLayout">
                {filtered.map((listing) => (
                  <motion.div
                    key={`${listing.token}-${listing.chat_id}`}
                    layout
                    initial={{ opacity: 0, y: 4 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, scale: 0.95 }}
                    className="flex items-center gap-3 rounded-xl bg-secondary/40 px-4 py-3 transition-colors hover:bg-secondary group"
                  >
                    {listing.image_url ? (
                      <img
                        src={listing.image_url}
                        alt=""
                        className="h-12 w-16 rounded-lg object-cover bg-muted flex-shrink-0"
                        referrerPolicy="no-referrer"
                      />
                    ) : (
                      <div className="h-12 w-16 rounded-lg bg-muted flex items-center justify-center flex-shrink-0">
                        <Car className="h-5 w-5 text-muted-foreground/30" />
                      </div>
                    )}
                    <div className="min-w-0 flex-1">
                      <p className="text-sm font-medium truncate">
                        {listing.manufacturer} {listing.model} {listing.year}
                      </p>
                      <div className="flex items-center gap-2 mt-0.5 flex-wrap">
                        <span className="text-xs text-muted-foreground">
                          {formatPrice(listing.price)}
                        </span>
                        <span className="text-xs text-muted-foreground">
                          · {formatKm(listing.km)}
                        </span>
                        {listing.city && (
                          <span className="text-xs text-muted-foreground">
                            · {listing.city}
                          </span>
                        )}
                        {listing.fitness_score != null && (
                          <Badge
                            variant={
                              listing.fitness_score >= 7.0
                                ? "success"
                                : listing.fitness_score >= 4.0
                                  ? "warning"
                                  : "destructive"
                            }
                            className="text-[10px] px-1.5 py-0"
                          >
                            {listing.fitness_score.toFixed(1)}
                          </Badge>
                        )}
                      </div>
                    </div>
                    <div className="flex items-center gap-1.5 flex-shrink-0 opacity-100 sm:opacity-0 sm:group-hover:opacity-100 sm:group-focus-within:opacity-100 transition-opacity">
                      <button
                        type="button"
                        onClick={() => setDetailListing(listing)}
                        className="flex h-8 w-8 items-center justify-center rounded-lg bg-card border border-border text-muted-foreground hover:text-foreground transition-colors"
                        title="פרטים"
                      >
                        <Search className="h-3.5 w-3.5" />
                      </button>
                      {safeHref(listing.page_link) && (
                        <a
                          href={safeHref(listing.page_link)!}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="flex h-8 w-8 items-center justify-center rounded-lg bg-card border border-border text-muted-foreground hover:text-primary transition-colors"
                          title="צפה במודעה"
                        >
                          <ExternalLink className="h-3.5 w-3.5" />
                        </a>
                      )}
                      <button
                        type="button"
                        onClick={() => setConfirmDelete(listing)}
                        className="flex h-8 w-8 items-center justify-center rounded-lg bg-destructive/10 border border-destructive/20 text-destructive hover:bg-destructive/20 transition-colors"
                        title="מחק"
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </button>
                    </div>
                  </motion.div>
                ))}
              </AnimatePresence>
            </div>
          )}
        </div>

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="flex items-center justify-center gap-4 px-6 py-4 border-t border-border/50">
            <button
              type="button"
              onClick={() => setPage((p) => Math.max(0, p - 1))}
              disabled={page === 0}
              aria-label="עמוד הקודם"
              className="flex h-9 w-9 items-center justify-center rounded-lg border border-border transition-colors hover:bg-muted disabled:opacity-30"
            >
              <ChevronRight className="h-4 w-4" />
            </button>
            <span className="text-sm tabular-nums text-muted-foreground">
              עמוד {page + 1} מתוך {totalPages}
            </span>
            <button
              type="button"
              onClick={() =>
                setPage((p) => Math.min(totalPages - 1, p + 1))
              }
              disabled={page >= totalPages - 1}
              aria-label="עמוד הבא"
              className="flex h-9 w-9 items-center justify-center rounded-lg border border-border transition-colors hover:bg-muted disabled:opacity-30"
            >
              <ChevronLeft className="h-4 w-4" />
            </button>
          </div>
        )}
      </div>

      <AnimatePresence>
        {confirmDelete && (
          <ConfirmModal
            message={`האם למחוק את "${confirmDelete.manufacturer} ${confirmDelete.model} ${confirmDelete.year}"?`}
            onConfirm={() =>
              deleteMutation.mutate({
                token: confirmDelete.token,
                chatId: confirmDelete.chat_id,
              })
            }
            onCancel={() => setConfirmDelete(null)}
            loading={deleteMutation.isPending}
          />
        )}
        {detailListing && (
          <DetailModal
            title={`${detailListing.manufacturer} ${detailListing.model} ${detailListing.year}`}
            fields={[
              { label: "מחיר", value: formatPrice(detailListing.price) },
              { label: "קילומטרז'", value: formatKm(detailListing.km) },
              { label: "יד", value: detailListing.hand || null },
              { label: "עיר", value: detailListing.city },
              { label: "תת-דגם", value: detailListing.sub_model },
              {
                label: "נפח מנוע (סמ״ק)",
                value:
                  detailListing.engine_volume != null
                    ? String(detailListing.engine_volume)
                    : null,
              },
              {
                label: "כוח סוס",
                value:
                  detailListing.horse_power != null
                    ? String(detailListing.horse_power)
                    : null,
              },
              { label: "סוג מנוע", value: detailListing.engine_type },
              { label: "תיבת הילוכים", value: detailListing.gear_box },
              { label: "תיאור", value: detailListing.description },
              {
                label: "סוג מוכר",
                value:
                  detailListing.is_commercial === true
                    ? "מסחרי"
                    : detailListing.is_commercial === false
                      ? "פרטי"
                      : "לא ידוע",
              },
              {
                label: "ציון התאמה",
                value: detailListing.fitness_score != null
                  ? `${detailListing.fitness_score.toFixed(1)} / 10`
                  : null,
              },
              {
                label: "תאריך",
                value: detailListing.first_seen_at
                  ? relativeTime(detailListing.first_seen_at)
                  : null,
              },
              { label: "חיפוש", value: detailListing.search_name },
              { label: "Token", value: detailListing.token },
              { label: "Chat ID", value: detailListing.chat_id },
            ]}
            onClose={() => setDetailListing(null)}
          />
        )}
      </AnimatePresence>
    </div>
  );
}
