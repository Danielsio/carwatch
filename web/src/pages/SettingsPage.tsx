import { usePageTitle } from "@/hooks/usePageTitle";
import { useQuery, useMutation } from "@tanstack/react-query";
import {
  Loader2,
  MessageCircle,
  ExternalLink,
  CheckCircle2,
  RefreshCw,
  Sun,
  Moon,
  User,
  LogOut,
  Bell,
  BellOff,
  BellRing,
} from "lucide-react";
import { Button } from "@/components/ui/Button";
import { PageHeader } from "@/components/ui/PageHeader";
import { useToast } from "@/components/ui/Toast";
import { telegramApi } from "@/lib/api";
import { useAuth } from "@/contexts/AuthContext";
import { useTheme } from "@/contexts/ThemeContext";
import { usePushSubscription } from "@/hooks/usePushSubscription";

export function SettingsPage() {
  usePageTitle("הגדרות");
  const { toast } = useToast();
  const { user, signOut } = useAuth();
  const { theme, toggle: toggleTheme } = useTheme();

  const { pushState, subscribe: pushSubscribe, unsubscribe: pushUnsubscribe } =
    usePushSubscription(!!user);

  const { data: tgStatus, isLoading: tgLoading, refetch: refetchTg } = useQuery({
    queryKey: ["telegram-status"],
    queryFn: () => telegramApi.status(),
  });

  const linkMutation = useMutation({
    mutationFn: () => telegramApi.createLink(),
    onSuccess: (result) => {
      window.open(result.link, "_blank", "noopener");
      toast("נפתח קישור לטלגרם — לחץ Start בבוט וחזור לכאן", "success");
    },
    onError: () => {
      toast("לא ניתן ליצור קישור. נסה שוב.", "error");
    },
  });

  return (
    <div className="max-w-xl space-y-6 pb-24 md:pb-8">
      <PageHeader title="הגדרות" subtitle="ניהול חשבון וחיבורים" />

      {/* Account */}
      <section className="rounded-2xl border border-border/50 bg-card p-5 space-y-4">
        <div className="flex items-center gap-3">
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary/10">
            <User className="h-4 w-4 text-primary" />
          </div>
          <div className="flex-1 min-w-0">
            <h2 className="text-sm font-semibold text-foreground">חשבון</h2>
            <p className="text-xs text-muted-foreground truncate">
              {user?.email ?? "—"}
            </p>
          </div>
        </div>
      </section>

      {/* Theme */}
      <section className="rounded-2xl border border-border/50 bg-card p-5">
        <div className="flex items-center gap-3">
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary/10">
            {theme === "dark" ? (
              <Moon className="h-4 w-4 text-primary" />
            ) : (
              <Sun className="h-4 w-4 text-primary" />
            )}
          </div>
          <div className="flex-1 min-w-0">
            <h2 className="text-sm font-semibold text-foreground">מראה</h2>
            <p className="text-xs text-muted-foreground">
              {theme === "dark" ? "מצב כהה" : "מצב בהיר"}
            </p>
          </div>
          <button
            type="button"
            role="switch"
            aria-checked={theme === "dark"}
            aria-label="מצב כהה"
            onClick={toggleTheme}
            dir="ltr"
            className={`relative inline-flex h-6 w-11 shrink-0 cursor-pointer items-center rounded-full border-2 border-transparent transition-colors duration-150 ${
              theme === "dark" ? "bg-primary" : "bg-muted"
            }`}
          >
            <span
              className={`pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow-sm ring-0 transition-transform duration-150 ${
                theme === "dark" ? "translate-x-5" : "translate-x-0.5"
              }`}
            />
          </button>
        </div>
      </section>

      {/* Telegram Connection */}
      <section className="rounded-2xl border border-border/50 bg-card p-5 space-y-4">
        <div className="flex items-center gap-3">
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-[#0088cc]/10">
            <MessageCircle className="h-4 w-4 text-[#0088cc]" />
          </div>
          <div className="flex-1 min-w-0">
            <h2 className="text-sm font-semibold text-foreground">
              התראות Telegram
            </h2>
            <p className="text-xs text-muted-foreground">
              קבל עדכונים על מודעות חדשות בטלגרם
            </p>
          </div>
          {!tgLoading && tgStatus?.connected && (
            <button
              type="button"
              onClick={() => void refetchTg()}
              className="p-1.5 rounded-lg hover:bg-muted/50 text-muted-foreground transition-colors"
              aria-label="רענון סטטוס"
            >
              <RefreshCw className="h-3.5 w-3.5" />
            </button>
          )}
        </div>

        {tgLoading ? (
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Loader2 className="h-4 w-4 animate-spin" />
            בודק חיבור...
          </div>
        ) : tgStatus?.connected ? (
          <div className="flex items-center gap-3 rounded-xl bg-success/5 border border-success/20 p-3">
            <CheckCircle2 className="h-5 w-5 text-success shrink-0" />
            <div className="flex-1 min-w-0">
              <p className="text-sm font-medium text-foreground">מחובר</p>
              {tgStatus.telegram_username && (
                <p className="text-xs text-muted-foreground truncate">
                  @{tgStatus.telegram_username}
                </p>
              )}
            </div>
          </div>
        ) : linkMutation.isSuccess ? (
          <div className="space-y-3">
            <div className="rounded-xl bg-primary/5 border border-primary/20 p-4 space-y-2">
              <p className="text-sm font-medium text-foreground">
                כמעט שם! עקוב אחרי הצעדים:
              </p>
              <ol className="text-xs text-muted-foreground space-y-1.5 list-decimal list-inside leading-relaxed">
                <li>עבור לטלגרם (נפתח בחלון חדש)</li>
                <li>לחץ <span className="font-medium text-foreground">Start</span> בבוט</li>
                <li>חזור לכאן ולחץ &quot;בדוק חיבור&quot;</li>
              </ol>
            </div>
            <div className="flex gap-2">
              <Button
                onClick={() => void refetchTg()}
                variant="secondary"
                className="flex-1 sm:flex-none"
              >
                <RefreshCw className="h-4 w-4" />
                בדוק חיבור
              </Button>
              <Button
                onClick={() => linkMutation.mutate()}
                variant="ghost"
                disabled={linkMutation.isPending}
                className="flex-1 sm:flex-none"
              >
                שלח קישור חדש
              </Button>
            </div>
          </div>
        ) : (
          <div className="space-y-3">
            <div className="text-xs text-muted-foreground leading-relaxed space-y-1.5">
              <p>חבר את חשבון הטלגרם שלך כדי:</p>
              <ul className="list-disc list-inside space-y-0.5">
                <li>לקבל התראות על מודעות חדשות ישירות בטלגרם</li>
                <li>לסנכרן חיפושים ומודעות שמורות בין האתר לבוט</li>
                <li>לנהל הכל ממקום אחד</li>
              </ul>
            </div>
            <Button
              onClick={() => linkMutation.mutate()}
              disabled={linkMutation.isPending}
              variant="secondary"
              className="w-full sm:w-auto"
            >
              {linkMutation.isPending ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <ExternalLink className="h-4 w-4" />
              )}
              חבר חשבון Telegram
            </Button>
          </div>
        )}
      </section>

      {/* Push Notifications */}
      <section className="rounded-2xl border border-border/50 bg-card p-5 space-y-4">
        <div className="flex items-center gap-3">
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary/10">
            {pushState.subscribed ? (
              <BellRing className="h-4 w-4 text-primary" />
            ) : (
              <Bell className="h-4 w-4 text-primary" />
            )}
          </div>
          <div className="flex-1 min-w-0">
            <h2 className="text-sm font-semibold text-foreground">
              התראות Push
            </h2>
            <p className="text-xs text-muted-foreground">
              קבל התראות ישירות בדפדפן על מודעות חדשות
            </p>
          </div>
        </div>

        {!pushState.supported ? (
          <div className="flex items-center gap-3 rounded-xl bg-muted/50 border border-border/30 p-3">
            <BellOff className="h-5 w-5 text-muted-foreground shrink-0" />
            <p className="text-sm text-muted-foreground">
              הדפדפן שלך לא תומך בהתראות
            </p>
          </div>
        ) : pushState.permission === "denied" ? (
          <div className="flex items-center gap-3 rounded-xl bg-destructive/5 border border-destructive/20 p-3">
            <BellOff className="h-5 w-5 text-destructive shrink-0" />
            <p className="text-sm text-muted-foreground">
              ההתראות חסומות בדפדפן. שנה את ההגדרות בדפדפן.
            </p>
          </div>
        ) : pushState.subscribed ? (
          <div className="space-y-3">
            <div className="flex items-center gap-3 rounded-xl bg-success/5 border border-success/20 p-3">
              <CheckCircle2 className="h-5 w-5 text-success shrink-0" />
              <p className="text-sm font-medium text-foreground">
                התראות Push מופעלות
              </p>
            </div>
            <Button
              onClick={() => pushUnsubscribe.mutate()}
              disabled={pushUnsubscribe.isPending}
              variant="secondary"
              className="w-full sm:w-auto"
            >
              {pushUnsubscribe.isPending ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <BellOff className="h-4 w-4" />
              )}
              כבה התראות
            </Button>
          </div>
        ) : (
          <div className="space-y-3">
            <p className="text-xs text-muted-foreground leading-relaxed">
              הפעל התראות Push כדי לקבל עדכונים מיידיים על מודעות חדשות ישירות בדפדפן.
            </p>
            <Button
              onClick={() => pushSubscribe.mutate()}
              disabled={pushSubscribe.isPending}
              variant="secondary"
              className="w-full sm:w-auto"
            >
              {pushSubscribe.isPending ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <Bell className="h-4 w-4" />
              )}
              הפעל התראות
            </Button>
          </div>
        )}
      </section>

      {/* Sign Out */}
      <section className="rounded-2xl border border-border/50 bg-card p-5">
        <Button
          variant="ghost"
          onClick={() => {
            if (window.confirm("האם לצאת מהחשבון?")) void signOut();
          }}
          className="w-full text-destructive hover:bg-destructive/5 hover:text-destructive"
        >
          <LogOut className="h-4 w-4" />
          התנתק מהחשבון
        </Button>
      </section>
    </div>
  );
}
