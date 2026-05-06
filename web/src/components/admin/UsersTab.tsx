import { useState } from "react";
import { AlertCircle, RefreshCw, Trash2, Users } from "lucide-react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { AnimatePresence, motion } from "motion/react";
import { adminApi, type AdminUser } from "@/lib/api";
import { EmptyState, Skeleton, Badge } from "@/components/ui";
import { useToast } from "@/components/ui/Toast";
import { relativeTime } from "@/lib/utils";
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
          <h2 className="text-base font-semibold">כל המשתמשים</h2>
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
        </div>
      </div>

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
              <button
                type="button"
                onClick={() => { setDetailUser(null); setConfirmDelete(detailUser); }}
                className="inline-flex items-center gap-2 px-4 py-2 rounded-xl text-sm font-medium text-destructive bg-destructive/10 hover:bg-destructive/20 transition-colors"
              >
                <Trash2 className="h-3.5 w-3.5" />
                מחק משתמש
              </button>
            }
          />
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
