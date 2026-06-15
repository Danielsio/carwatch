/* eslint-disable react-refresh/only-export-components -- useToast co-located with ToastProvider */
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { CheckCircle2, AlertCircle, Info, X } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";

export type ToastType = "success" | "error" | "info";

export type ToastAction = {
  label: string;
  onClick: () => void;
};

export type ToastOptions = {
  /** Optional inline action, e.g. an "undo". */
  action?: ToastAction;
  /** Override the auto-dismiss duration (ms). */
  duration?: number;
};

type ToastRecord = {
  id: string;
  message: string;
  type: ToastType;
  exiting: boolean;
  action?: ToastAction;
  duration: number;
};

type ToastFn = (message: string, type?: ToastType, options?: ToastOptions) => void;

let globalToastFn: ToastFn | null = null;
const pendingToasts: Array<{
  message: string;
  type: ToastType;
  options?: ToastOptions;
}> = [];

/** Fire a toast from outside React (e.g. QueryCache onError). */
export function showGlobalToast(
  message: string,
  type: ToastType = "info",
  options?: ToastOptions,
) {
  if (globalToastFn) {
    globalToastFn(message, type, options);
  } else {
    pendingToasts.push({ message, type, options });
    console.warn("[toast]", type, message);
  }
}

const EXIT_MS = 220;
const DEFAULT_DISMISS_MS: Record<ToastType, number> = {
  info: 3500,
  success: 4000,
  error: 6000,
};
/** Actions need longer so the undo is actually reachable. */
const ACTION_DISMISS_MS = 7000;

const TYPE_META: Record<
  ToastType,
  { icon: LucideIcon; accent: string; iconColor: string; bar: string }
> = {
  success: {
    icon: CheckCircle2,
    accent: "ring-score-great/25",
    iconColor: "text-score-great",
    bar: "bg-score-great",
  },
  error: {
    icon: AlertCircle,
    accent: "ring-destructive/25",
    iconColor: "text-destructive",
    bar: "bg-destructive",
  },
  info: {
    icon: Info,
    accent: "ring-primary/25",
    iconColor: "text-primary",
    bar: "bg-primary",
  },
};

type ToastContextValue = {
  toast: ToastFn;
};

const ToastContext = createContext<ToastContextValue | null>(null);

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext);
  if (!ctx) {
    throw new Error("useToast חייב לעטוף את האפליקציה ב־ToastProvider");
  }
  return ctx;
}

function ToastItem({
  item,
  onRequestExit,
  onExitDone,
}: {
  item: ToastRecord;
  onRequestExit: (id: string) => void;
  onExitDone: (id: string) => void;
}) {
  const meta = TYPE_META[item.type];
  const Icon = meta.icon;
  const [paused, setPaused] = useState(false);
  const remainingRef = useRef(item.duration);
  const startRef = useRef(0);

  // Auto-dismiss countdown — pauses on hover/focus, resumes on leave.
  useEffect(() => {
    if (item.exiting || paused) return;
    startRef.current = Date.now();
    const t = window.setTimeout(
      () => onRequestExit(item.id),
      remainingRef.current,
    );
    return () => {
      window.clearTimeout(t);
      remainingRef.current = Math.max(
        0,
        remainingRef.current - (Date.now() - startRef.current),
      );
    };
  }, [paused, item.exiting, item.id, onRequestExit]);

  // Remove from the DOM once the exit animation has played.
  useEffect(() => {
    if (!item.exiting) return;
    const t = window.setTimeout(() => onExitDone(item.id), EXIT_MS);
    return () => window.clearTimeout(t);
  }, [item.exiting, item.id, onExitDone]);

  return (
    <div
      role={item.type === "error" ? "alert" : "status"}
      aria-live={item.type === "error" ? "assertive" : "polite"}
      onPointerEnter={() => setPaused(true)}
      onPointerLeave={() => setPaused(false)}
      onFocusCapture={() => setPaused(true)}
      onBlurCapture={() => setPaused(false)}
      className={cn(
        "glass-card pointer-events-auto relative w-full max-w-md overflow-hidden rounded-xl border border-border/60 text-right shadow-2xl ring-1 dir-rtl",
        meta.accent,
        !item.exiting && "animate-slide-up",
        item.exiting && "translate-y-1 opacity-0 transition-all duration-200 ease-out",
      )}
    >
      <div className="flex items-start gap-3 px-4 py-3">
        <Icon className={cn("mt-0.5 h-5 w-5 shrink-0", meta.iconColor)} aria-hidden />
        <p className="flex-1 text-sm font-medium leading-snug text-foreground">
          {item.message}
        </p>
        {item.action ? (
          <button
            type="button"
            onClick={() => {
              item.action?.onClick();
              onRequestExit(item.id);
            }}
            className="shrink-0 rounded-lg px-2.5 py-1 text-xs font-bold text-primary transition-colors hover:bg-primary/10"
          >
            {item.action.label}
          </button>
        ) : null}
        <button
          type="button"
          onClick={() => onRequestExit(item.id)}
          aria-label="סגור הודעה"
          className="shrink-0 rounded-md p-1 text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground"
        >
          <X className="h-3.5 w-3.5" />
        </button>
      </div>
      {/* Countdown bar */}
      {!item.exiting ? (
        <div
          aria-hidden
          className={cn("h-0.5 w-full origin-right opacity-60", meta.bar)}
          style={{
            animation: `toast-progress ${item.duration}ms linear forwards`,
            animationPlayState: paused ? "paused" : "running",
          }}
        />
      ) : null}
    </div>
  );
}

export type ToastProviderProps = {
  children: ReactNode;
};

export function ToastProvider({ children }: ToastProviderProps) {
  const [toasts, setToasts] = useState<ToastRecord[]>([]);

  const dismiss = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  const beginExit = useCallback((id: string) => {
    setToasts((prev) =>
      prev.map((t) => (t.id === id ? { ...t, exiting: true } : t)),
    );
  }, []);

  const toast = useCallback<ToastFn>((message, type = "info", options) => {
    const id =
      typeof crypto !== "undefined" && "randomUUID" in crypto
        ? crypto.randomUUID()
        : `toast-${Date.now()}-${Math.random().toString(16).slice(2)}`;
    const duration =
      options?.duration ??
      (options?.action ? ACTION_DISMISS_MS : DEFAULT_DISMISS_MS[type]);
    setToasts((prev) => [
      ...prev,
      { id, message, type, exiting: false, action: options?.action, duration },
    ]);
  }, []);

  useEffect(() => {
    globalToastFn = toast;
    if (pendingToasts.length > 0) {
      pendingToasts
        .splice(0)
        .forEach(({ message, type, options }) => toast(message, type, options));
    }
    return () => {
      globalToastFn = null;
    };
  }, [toast]);

  const value = useMemo(() => ({ toast }), [toast]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div
        className="pointer-events-none fixed inset-x-0 bottom-0 z-[100] flex flex-col-reverse items-center gap-2 p-4 pb-20 md:pb-4"
        aria-relevant="additions text"
      >
        {toasts.map((t) => (
          <ToastItem
            key={t.id}
            item={t}
            onRequestExit={beginExit}
            onExitDone={dismiss}
          />
        ))}
      </div>
    </ToastContext.Provider>
  );
}
