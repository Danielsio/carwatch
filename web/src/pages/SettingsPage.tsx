import { useState } from "react";
import { Save, Bell, Mail, Clock, Hash, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { ChipButton } from "@/components/ui/ChipButton";
import { PageHeader } from "@/components/ui/PageHeader";
import { useToast } from "@/components/ui/Toast";

const SCAN_FREQ_OPTIONS = [
  { value: 15, label: "כל 15 דקות" },
  { value: 30, label: "כל 30 דקות" },
  { value: 60, label: "כל שעה" },
  { value: 120, label: "כל שעתיים" },
];

const ALERT_COUNT_OPTIONS = [1, 3, 5, 10, 20];

export function SettingsPage() {
  const { toast } = useToast();
  const [saving, setSaving] = useState(false);

  const [telegramEnabled, setTelegramEnabled] = useState(true);
  const [emailEnabled, setEmailEnabled] = useState(false);
  const [scanFrequency, setScanFrequency] = useState(30);
  const [alertCount, setAlertCount] = useState(5);

  async function handleSave() {
    setSaving(true);
    await new Promise((r) => setTimeout(r, 600));
    setSaving(false);
    toast("ההגדרות נשמרו בהצלחה", "success");
  }

  return (
    <div className="space-y-6 pb-24 md:pb-8">
      <PageHeader title="הגדרות" subtitle="התאם את חוויית השימוש שלך" />

      {/* Notifications */}
      <section className="rounded-2xl border border-border/50 bg-card p-5 space-y-5">
        <h2 className="text-sm font-semibold text-foreground">התראות</h2>

        <ToggleRow
          icon={Bell}
          label="התראות Telegram"
          description="קבל עדכונים על מודעות חדשות בטלגרם"
          enabled={telegramEnabled}
          onToggle={() => setTelegramEnabled((v) => !v)}
        />

        <div className="border-t border-border/30" />

        <ToggleRow
          icon={Mail}
          label="התראות אימייל"
          description="קבל עדכונים על מודעות חדשות באימייל"
          enabled={emailEnabled}
          onToggle={() => setEmailEnabled((v) => !v)}
        />
      </section>

      {/* Scan Frequency */}
      <section className="rounded-2xl border border-border/50 bg-card p-5 space-y-4">
        <div className="flex items-center gap-3">
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary/10">
            <Clock className="h-4 w-4 text-primary" />
          </div>
          <div>
            <h2 className="text-sm font-semibold text-foreground">תדירות סריקה</h2>
            <p className="text-xs text-muted-foreground">כמה פעמים לסרוק מודעות חדשות</p>
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          {SCAN_FREQ_OPTIONS.map((opt) => (
            <ChipButton
              key={opt.value}
              selected={scanFrequency === opt.value}
              onClick={() => setScanFrequency(opt.value)}
            >
              {opt.label}
            </ChipButton>
          ))}
        </div>
      </section>

      {/* Alert Count */}
      <section className="rounded-2xl border border-border/50 bg-card p-5 space-y-4">
        <div className="flex items-center gap-3">
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-score-good/10">
            <Hash className="h-4 w-4 text-score-good" />
          </div>
          <div>
            <h2 className="text-sm font-semibold text-foreground">מודעות בהתראה</h2>
            <p className="text-xs text-muted-foreground">כמה מודעות להציג בכל התראה</p>
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          {ALERT_COUNT_OPTIONS.map((count) => (
            <ChipButton
              key={count}
              selected={alertCount === count}
              onClick={() => setAlertCount(count)}
            >
              {count}
            </ChipButton>
          ))}
        </div>
      </section>

      {/* Save */}
      <div className="sticky bottom-[5.5rem] landscape:bottom-14 md:bottom-0 z-40 -mx-4 px-4 py-3 bg-background/90 backdrop-blur-xl border-t border-border/30 md:static md:mx-0 md:px-0 md:py-0 md:bg-transparent md:backdrop-blur-none md:border-0">
        <Button onClick={handleSave} disabled={saving} size="lg" className="w-full md:w-auto">
          {saving ? (
            <Loader2 className="h-4 w-4 animate-spin" />
          ) : (
            <Save className="h-4 w-4" />
          )}
          שמור הגדרות
        </Button>
      </div>
    </div>
  );
}

function ToggleRow({
  icon: Icon,
  label,
  description,
  enabled,
  onToggle,
}: {
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  description: string;
  enabled: boolean;
  onToggle: () => void;
}) {
  return (
    <div className="flex items-center gap-4">
      <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary/10">
        <Icon className="h-4 w-4 text-primary" />
      </div>
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium text-foreground">{label}</p>
        <p className="text-xs text-muted-foreground">{description}</p>
      </div>
      <button
        type="button"
        role="switch"
        aria-checked={enabled}
        aria-label={label}
        onClick={onToggle}
        dir="ltr"
        className={`relative inline-flex h-6 w-11 shrink-0 cursor-pointer items-center rounded-full border-2 border-transparent transition-colors duration-200 ${
          enabled ? "bg-primary" : "bg-muted"
        }`}
      >
        <span
          className={`pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow-sm ring-0 transition-transform duration-200 ${
            enabled ? "translate-x-5" : "translate-x-0.5"
          }`}
        />
      </button>
    </div>
  );
}
