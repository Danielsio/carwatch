import { useMemo, useState } from "react";
import { AlertCircle, FileSearch, MessageCircle, RefreshCcw, RefreshCw, ToggleLeft, ToggleRight, Trash2, Users } from "lucide-react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { AnimatePresence, motion } from "motion/react";
import { adminApi, type AdminUser } from "@/lib/api";
import { EmptyState, Skeleton, Badge } from "@/components/ui";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useToast } from "@/components/ui/Toast";
import { cn, formatPrice, relativeTime } from "@/lib/utils";
import { ConfirmModal } from "./ConfirmModal";
import { DetailModal } from "./DetailModal";

export function UsersTab() {
  const [detailUser, setDetailUser] = useState<AdminUser | null>(null);
  const [confirmDelete, setConfirmDelete] = useState<AdminUser | null>(null);
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ["admin", "users"],
    queryFn: adminApi.users,
  });

  const deleteUserMutation = useMutation({
    mutationFn: (chatId: number) => adminApi.deleteUser(chatId),
    meta: { suppressToast: true },
    onSuccess: () => {
      toast("המשתמש נמחק בהצלחה", "success");
      setConfirmDelete(null);
      void queryClient.invalidateQueries({ queryKey: ["admin", "users"] });
      void queryClient.invalidateQueries({ queryKey: ["admin", "stats"] });
    },
    onError: () => {
      toast("שגיאה במחיקת המשתמש", "error");
    },
  });

  const toggleActiveMutation = useMutation({
    mutationFn: ({ chatId, active }: { chatId: number; active: boolean }) =>
      adminApi.setUserActive(chatId, active),
    meta: { suppressToast: true },
    onSuccess: (_data, variables) => {
      toast(variables.active ? "המשתמש הופעל" : "המשתמש הושבת", "success");
      void queryClient.invalidateQueries({ queryKey: ["admin", "users"] });
    },
    onError: () => {
      toast("שגיאה בעדכון סטטוס המשתמש", "error");
    },
  });

  const { data: searchesData, isLoading: searchesLoading, isError: searchesError } = useQuery({
    queryKey: ["admin", "searches"],
    queryFn: adminApi.searches,
  });

  const userSearches = useMemo(() => {
    if (!detailUser || !searchesData) return [];
    const searchChatId = detailUser.linked_telegram?.chat_id ?? detailUser.chat_id;
    return searchesData.items.filter(
      (s) => s.chat_id === searchChatId && s.active,
    );
  }, [detailUser, searchesData]);

  const syncUserStatusMutation = useMutation({
    mutationFn: () => adminApi.syncUserStatus(),
    meta: { suppressToast: true },
    onSuccess: (res) => {
      toast(
        `סנכרון הושלם: ${res.activated.toLocaleString("he-IL")} הופעלו, ${res.deactivated.toLocaleString("he-IL")} הושבתו`,
        "success",
      );
      void queryClient.invalidateQueries({ queryKey: ["admin", "users"] });
    },
    onError: () => {
      toast("שגיאה בסנכרון סטטוס משתמשים", "error");
    },
  });

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap gap-3 justify-end">
        <Button
          variant="outline"
          size="sm"
          onClick={() => void syncUserStatusMutation.mutate()}
          loading={syncUserStatusMutation.isPending}
        >
          <RefreshCcw className="h-3.5 w-3.5" />
          סנכרן סטטוס
        </Button>
        <Button variant="outline" size="sm" onClick={() => void refetch()}>
          <RefreshCw className="h-3.5 w-3.5" />
          רענן
        </Button>
      </div>

      <Card>
        <CardHeader className="flex-row items-center justify-between">
          <CardTitle className="text-base">כל המשתמשים</CardTitle>
          {data && (
            <span className="text-sm text-muted-foreground tabular-nums">
              {data.total} סה״כ
            </span>
          )}
        </CardHeader>

        <CardContent className="p-3">
          {isLoading ? (
            <div className="space-y-2 p-3">
              {Array.from({ length: 3 }).map((_, i) => (
                <Skeleton key={i} className="h-16 rounded-xl" />
              ))}
            </div>
          ) : isError ? (
            <EmptyState
              icon={AlertCircle}
              title="שגיאה בטעינת המשתמשים"
              className="border-0 bg-transparent"
            />
          ) : !data || data.items.length === 0 ? (
            <EmptyState
              icon={Users}
              title="אין משתמשים"
              description="עדיין לא נרשמו משתמשים למערכת"
              className="border-0 bg-transparent"
            />
          ) : (
            <div className="space-y-1.5">
              {data.items.map((user) => (
                <motion.div
                  key={user.chat_id}
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  className="group flex items-center gap-3 rounded-xl bg-secondary/40 px-4 py-3 transition-colors hover:bg-secondary cursor-pointer focus-visible:ring-2 focus-visible:ring-ring outline-none"
                  tabIndex={0}
                  role="button"
                  onClick={() => setDetailUser(user)}
                  onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); setDetailUser(user); } }}
                >
                  <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-primary/10 text-primary font-bold text-sm flex-shrink-0">
                    {(user.username || user.channel || "?")[0].toUpperCase()}
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium truncate">
                      {user.username || `User #${user.chat_id}`}
                    </p>
                    <div className="flex items-center gap-2 mt-0.5 flex-wrap">
                      {user.channel && (
                        <span className="text-xs text-muted-foreground">
                          {user.channel}
                        </span>
                      )}
                      {user.linked_telegram && (
                        <span className="text-xs text-muted-foreground">
                          · טלגרם מקושר
                        </span>
                      )}
                      <span className="text-xs text-muted-foreground">
                        · {relativeTime(user.created_at)}
                      </span>
                    </div>
                  </div>
                  <div className="flex items-center gap-2 flex-shrink-0">
                    {user.tier && user.tier !== "free" && (
                      <Badge variant="success">{user.tier}</Badge>
                    )}
                    <Badge variant={user.active ? "success" : "default"}>
                      {user.active ? "פעיל" : "לא פעיל"}
                    </Badge>
                    <button
                      type="button"
                      disabled={toggleActiveMutation.isPending}
                      onClick={(e) => {
                        e.stopPropagation();
                        toggleActiveMutation.mutate({ chatId: user.chat_id, active: !user.active });
                      }}
                      aria-label={user.active ? "השבת משתמש" : "הפעל משתמש"}
                      title={user.active ? "השבת משתמש" : "הפעל משתמש"}
                      className={cn(
                        "rounded-lg p-1.5 transition-colors opacity-0 group-hover:opacity-100 focus-visible:opacity-100 disabled:opacity-50",
                        user.active
                          ? "text-emerald-500 hover:text-amber-600 hover:bg-amber-100/50"
                          : "text-muted-foreground/50 hover:text-emerald-600 hover:bg-emerald-100/50"
                      )}
                    >
                      {user.active ? <ToggleRight className="h-4 w-4" /> : <ToggleLeft className="h-4 w-4" />}
                    </button>
                    <button
                      type="button"
                      onClick={(e) => { e.stopPropagation(); setConfirmDelete(user); }}
                      aria-label={`מחק משתמש ${user.username || user.chat_id}`}
                      title="מחק משתמש"
                      className="rounded-lg p-1.5 text-muted-foreground/50 transition-colors hover:text-destructive hover:bg-destructive/10 opacity-0 group-hover:opacity-100 focus-visible:opacity-100"
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </button>
                  </div>
                </motion.div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <AnimatePresence>
        {detailUser && (
          <DetailModal
            title={detailUser.username || `User #${detailUser.chat_id}`}
            fields={[
              { label: "Chat ID", value: detailUser.chat_id },
              { label: "שם משתמש", value: detailUser.username },
              { label: "ערוץ", value: detailUser.channel },
              { label: "מזהה ערוץ", value: detailUser.channel_id },
              { label: "שפה", value: detailUser.language },
              { label: "דרגה", value: detailUser.tier },
              {
                label: "סטטוס",
                value: detailUser.active ? "פעיל" : "לא פעיל",
              },
              { label: "נוצר", value: relativeTime(detailUser.created_at) },
            ]}
            onClose={() => setDetailUser(null)}
            actions={
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  disabled={toggleActiveMutation.isPending}
                  onClick={() => {
                    toggleActiveMutation.mutate(
                      { chatId: detailUser.chat_id, active: !detailUser.active },
                      { onSuccess: () => setDetailUser(null) }
                    );
                  }}
                  className={cn(
                    "inline-flex items-center gap-2 px-4 py-2 rounded-xl text-sm font-medium transition-colors disabled:opacity-50",
                    detailUser.active
                      ? "text-amber-700 bg-amber-100 hover:bg-amber-200"
                      : "text-emerald-700 bg-emerald-100 hover:bg-emerald-200"
                  )}
                >
                  {detailUser.active ? <ToggleRight className="h-3.5 w-3.5" /> : <ToggleLeft className="h-3.5 w-3.5" />}
                  {detailUser.active ? "השבת" : "הפעל"}
                </button>
                <button
                  type="button"
                  onClick={() => { setDetailUser(null); setConfirmDelete(detailUser); }}
                  className="inline-flex items-center gap-2 px-4 py-2 rounded-xl text-sm font-medium text-destructive bg-destructive/10 hover:bg-destructive/20 transition-colors"
                >
                  <Trash2 className="h-3.5 w-3.5" />
                  מחק משתמש
                </button>
              </div>
            }
          >
            {detailUser.linked_telegram && (
              <div className="mt-4 pt-4 border-t border-border/50">
                <div className="flex items-center gap-2 mb-3">
                  <MessageCircle className="h-4 w-4 text-muted-foreground" />
                  <h3 className="text-sm font-semibold">טלגרם מקושר</h3>
                </div>
                <div className="space-y-0.5">
                  <div className="flex gap-3 py-2 border-b border-border/50">
                    <span className="text-xs text-muted-foreground w-28 flex-shrink-0 mt-0.5">שם משתמש</span>
                    <span className="text-sm text-foreground font-medium break-all">@{detailUser.linked_telegram.username}</span>
                  </div>
                  <div className="flex gap-3 py-2">
                    <span className="text-xs text-muted-foreground w-28 flex-shrink-0 mt-0.5">Chat ID</span>
                    <span className="text-sm text-foreground font-medium break-all font-mono tabular-nums">{detailUser.linked_telegram.chat_id}</span>
                  </div>
                </div>
              </div>
            )}
            <div className="mt-4 pt-4 border-t border-border/50">
              <div className="flex items-center gap-2 mb-3">
                <FileSearch className="h-4 w-4 text-muted-foreground" />
                <h3 className="text-sm font-semibold">חיפושים פעילים</h3>
                <span className="text-xs text-muted-foreground tabular-nums">
                  ({userSearches.length})
                </span>
              </div>
              {searchesLoading ? (
                <p className="text-xs text-muted-foreground">טוען חיפושים…</p>
              ) : searchesError ? (
                <p className="text-xs text-destructive">שגיאה בטעינת חיפושים</p>
              ) : userSearches.length === 0 ? (
                <p className="text-xs text-muted-foreground">אין חיפושים פעילים</p>
              ) : (
                <div className="space-y-1.5">
                  {userSearches.map((s) => (
                    <div
                      key={s.id}
                      className="flex items-center gap-2 rounded-lg bg-secondary/40 px-3 py-2"
                    >
                      <div className="h-2 w-2 rounded-full bg-success animate-pulse-soft flex-shrink-0" />
                      <div className="min-w-0 flex-1">
                        <p className="text-xs font-medium truncate">
                          {s.name || `חיפוש #${s.id}`}
                        </p>
                        <div className="flex items-center gap-1.5 mt-0.5">
                          <span className="text-[11px] text-muted-foreground">{s.source}</span>
                          {s.price_max > 0 && (
                            <span className="text-[11px] text-muted-foreground">
                              · עד {formatPrice(s.price_max)}
                            </span>
                          )}
                          <span className="text-[11px] text-muted-foreground">
                            · {relativeTime(s.created_at)}
                          </span>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </DetailModal>
        )}
      </AnimatePresence>

      <AnimatePresence>
        {confirmDelete && (
          <ConfirmModal
            message={`האם למחוק את המשתמש "${confirmDelete.username || confirmDelete.chat_id}" וכל הנתונים שלו?`}
            onConfirm={() => deleteUserMutation.mutate(confirmDelete.chat_id)}
            onCancel={() => setConfirmDelete(null)}
            loading={deleteUserMutation.isPending}
          />
        )}
      </AnimatePresence>
    </div>
  );
}
