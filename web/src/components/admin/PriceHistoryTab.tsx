import { useState, useEffect } from "react";
import { adminApi, type AdminPriceRecord } from "@/lib/api";
import { formatPrice, relativeTime } from "@/lib/utils";
import { Skeleton } from "@/components/ui";

export function PriceHistoryTab() {
  const [items, setItems] = useState<AdminPriceRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(0);
  const [loading, setLoading] = useState(true);
  const limit = 50;

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    adminApi.priceHistory({ limit, offset: page * limit }).then((res) => {
      if (!cancelled) {
        setItems(res.items);
        setTotal(res.total);
        setLoading(false);
      }
    }).catch(() => {
      if (!cancelled) setLoading(false);
    });
    return () => { cancelled = true; };
  }, [page]);

  if (loading) {
    return (
      <div className="space-y-3">
        {Array.from({ length: 8 }).map((_, i) => (
          <Skeleton key={i} className="h-12 w-full rounded-xl" />
        ))}
      </div>
    );
  }

  const totalPages = Math.ceil(total / limit);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium text-muted-foreground">
          {total.toLocaleString()} רשומות מחיר
        </h3>
      </div>

      <div className="overflow-hidden rounded-xl border border-border/50">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border/50 bg-secondary/30">
              <th className="px-4 py-2.5 text-right font-medium text-muted-foreground">רכב</th>
              <th className="px-4 py-2.5 text-right font-medium text-muted-foreground">מחיר</th>
              <th className="px-4 py-2.5 text-right font-medium text-muted-foreground">תאריך</th>
              <th className="px-4 py-2.5 text-right font-medium text-muted-foreground">טוקן</th>
            </tr>
          </thead>
          <tbody>
            {items.map((item, i) => (
              <tr key={`${item.token}-${i}`} className="border-b border-border/30 last:border-0 hover:bg-secondary/20">
                <td className="px-4 py-2.5">
                  {item.manufacturer && item.model
                    ? `${item.manufacturer} ${item.model} ${item.year || ""}`
                    : "—"}
                </td>
                <td className="px-4 py-2.5 font-medium">{formatPrice(item.price)}</td>
                <td className="px-4 py-2.5 text-muted-foreground">{relativeTime(item.observed_at)}</td>
                <td className="px-4 py-2.5 font-mono text-xs text-muted-foreground">{item.token}</td>
              </tr>
            ))}
            {items.length === 0 && (
              <tr>
                <td colSpan={4} className="px-4 py-8 text-center text-muted-foreground">
                  אין היסטוריית מחירים
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {totalPages > 1 && (
        <div className="flex items-center justify-center gap-2">
          <button
            type="button"
            onClick={() => setPage((p) => Math.max(0, p - 1))}
            disabled={page === 0}
            className="rounded-lg px-3 py-1.5 text-sm hover:bg-secondary disabled:opacity-40"
          >
            הקודם
          </button>
          <span className="text-sm text-muted-foreground">
            {page + 1} / {totalPages}
          </span>
          <button
            type="button"
            onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
            disabled={page >= totalPages - 1}
            className="rounded-lg px-3 py-1.5 text-sm hover:bg-secondary disabled:opacity-40"
          >
            הבא
          </button>
        </div>
      )}
    </div>
  );
}
