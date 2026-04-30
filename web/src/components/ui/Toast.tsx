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
import { cn } from "@/lib/utils";

export type ToastType = "success" | "error" | "info";

type ToastRecord = {
  id: string;
  message: string;
  type: ToastType;
  exiting: boolean;
};

const EXIT_MS = 220;
const AUTO_DISMISS_MS = 3000;

type ToastContextValue = {
  toast: (message: string, type?: ToastType) => void;
};

const ToastContext = createContext<ToastContextValue | null>(null);

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext);
  if (!ctx) {
    throw new Error("useToast חייב לעטוף את האפליקציה ב־ToastProvider");
  }
  return ctx;
}

function toastTypeClass(type: ToastType): string {
  switch (type) {
    case "success":
      return "border-score-great/40 bg-card text-foreground ring-1 ring-score-great/20";
    case "error":
      return "border-destructive/40 bg-card text-foreground ring-1 ring-destructive/20";
    case "info":
      return "border-primary/40 bg-card text-foreground ring-1 ring-primary/20";
    default:
      return "border-border bg-card text-foreground";
  }
}

function ToastItem({
  item,
  onExitDone,
}: {
  item: ToastRecord;
  onExitDone: (id: string) => void;
}) {
  useEffect(() => {
    if (!item.exiting) return;
    const t = window.setTimeout(() => {
      onExitDone(item.id);
    }, EXIT_MS);
    return () => window.clearTimeout(t);
  }, [item.exiting, item.id, onExitDone]);

  return (
    <div
      role="status"
      aria-live="polite"
      className={cn(
        "pointer-events-auto w-full max-w-md rounded-lg border px-4 py-3 text-right text-sm shadow-lg dir-rtl",
        toastTypeClass(item.type),
        !item.exiting && "animate-slide-up",
        item.exiting && "opacity-0 transition-opacity duration-200 ease-out",
      )}
    >
      <p className="font-medium leading-snug">{item.message}</p>
    </div>
  );
}

export type ToastProviderProps = {
  children: ReactNode;
};

export function ToastProvider({ children }: ToastProviderProps) {
  const [toasts, setToasts] = useState<ToastRecord[]>([]);
  const timersRef = useRef<Set<number>>(new Set());

  useEffect(() => {
    const timers = timersRef.current;
    return () => {
      timers.forEach((t) => window.clearTimeout(t));
      timers.clear();
    };
  }, []);

  const dismiss = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  const beginExit = useCallback((id: string) => {
    setToasts((prev) =>
      prev.map((t) => (t.id === id ? { ...t, exiting: true } : t)),
    );
  }, []);

  const toast = useCallback(
    (message: string, type: ToastType = "info") => {
      const id =
        typeof crypto !== "undefined" && "randomUUID" in crypto
          ? crypto.randomUUID()
          : `toast-${Date.now()}-${Math.random().toString(16).slice(2)}`;
      setToasts((prev) => [...prev, { id, message, type, exiting: false }]);
      const timer = window.setTimeout(() => {
        timersRef.current.delete(timer);
        beginExit(id);
      }, AUTO_DISMISS_MS);
      timersRef.current.add(timer);
    },
    [beginExit],
  );

  const value = useMemo(() => ({ toast }), [toast]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div
        className="pointer-events-none fixed inset-x-0 bottom-0 z-[100] flex flex-col-reverse items-center gap-2 p-4"
        aria-relevant="additions text"
      >
        {toasts.map((t) => (
          <ToastItem key={t.id} item={t} onExitDone={dismiss} />
        ))}
      </div>
    </ToastContext.Provider>
  );
}
