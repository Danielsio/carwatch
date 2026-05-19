import { useState, useEffect, useCallback } from "react";
import { usePageTitle } from "@/hooks/usePageTitle";
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
import { telegramApi, type TelegramStatus } from "@/lib/api";
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

  const [tgStatus, setTgStatus] = useState<TelegramStatus | null>(null);
  const [tgLoading, setTgLoading] = useState(true);
  const [linkLoading, setLinkLoading] = useState(false);

  const fetchTgStatus = useCallback(async () => {
    try {
      setTgLoading(true);
      const status = await telegramApi.status();
      setTgStatus(status);
    } catch {
      setTgStatus(null);
    } finally {
      setTgLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchTgStatus();
  }, [fetchTgStatus]);

  async function handleTelegramLink() {
    try {
      setLinkLoading(true);
      const result = await telegramApi.createLink();
      window.open(result.link, "_blank", "noopener");
      toast("נפתח קישור לטלגרם — לחץ Start בבוט", "success");
      setTimeout(fetchTgStatus, 5000);
    } catch {
      toast("לא ניתן ליצור קישור. נסה שוב.", "error");
    } finally {
      setLinkLoading(false);
    }
  }

  return (
    <div className="space-y-6 pb-24 md:pb-8">
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
              onClick={fetchTgStatus}
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
        ) : (
          <div className="space-y-3">
            <p className="text-xs text-muted-foreground leading-relaxed">
              חבר את חשבונך כדי לקבל התראות על מודעות חדשות ישירות בטלגרם. הקישור תקף ל-15 דקות.
            </p>
            <Button
              onClick={handleTelegramLink}
              disabled={linkLoading}
              variant="secondary"
              className="w-full sm:w-auto"
            >
              {linkLoading ? (
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
          onClick={() => void signOut()}
          className="w-full text-destructive hover:bg-destructive/5 hover:text-destructive"
        >
          <LogOut className="h-4 w-4" />
          התנתק מהחשבון
        </Button>
      </section>
    </div>
  );
}
