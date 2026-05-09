import { useEffect, useRef } from "react";
import { Loader2, Trash2 } from "lucide-react";
import { motion } from "motion/react";

export function ConfirmModal({
  message,
  onConfirm,
  onCancel,
  loading,
}: {
  message: string;
  onConfirm: () => void;
  onCancel: () => void;
  loading?: boolean;
}) {
  const modalRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    modalRef.current?.focus();
  }, []);

  useEffect(() => {
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = "";
    };
  }, []);

  function handleKeyDown(e: React.KeyboardEvent<HTMLDivElement>) {
    if (e.key === "Escape") {
      onCancel();
      return;
    }
    if (e.key !== "Tab" || !modalRef.current) return;
    const root = modalRef.current;
    const selector =
      'button:not([disabled]), a[href], input:not([disabled]):not([type="hidden"]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';
    const focusable = [...root.querySelectorAll<HTMLElement>(selector)].filter(
      (el) => !el.hasAttribute("disabled") && !el.closest("[inert]"),
    );
    if (focusable.length === 0) return;
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    const activeEl = document.activeElement;
    const activeIndex =
      activeEl instanceof HTMLElement && root.contains(activeEl)
        ? focusable.indexOf(activeEl)
        : -1;
    if (e.shiftKey) {
      if (activeIndex <= 0) {
        e.preventDefault();
        last.focus();
      }
    } else if (activeIndex === -1 || activeIndex >= focusable.length - 1) {
      e.preventDefault();
      first.focus();
    }
  }

  return (
    <div
      ref={modalRef}
      tabIndex={-1}
      className="fixed inset-0 z-50 flex items-center justify-center p-4 outline-none"
      role="dialog"
      aria-modal="true"
      aria-labelledby="confirm-title"
      aria-describedby="confirm-desc"
      onKeyDown={handleKeyDown}
    >
      <motion.div
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        exit={{ opacity: 0 }}
        className="absolute inset-0 bg-black/60 backdrop-blur-sm"
        onClick={onCancel}
      />
      <motion.div
        initial={{ opacity: 0, scale: 0.95, y: 8 }}
        animate={{ opacity: 1, scale: 1, y: 0 }}
        exit={{ opacity: 0, scale: 0.95, y: 8 }}
        transition={{ type: "spring", duration: 0.3, bounce: 0.15 }}
        className="relative bg-card border border-border rounded-2xl p-6 max-w-sm w-full shadow-2xl"
      >
        <div className="flex items-center gap-3 mb-4">
          <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-destructive/15">
            <Trash2 className="h-[18px] w-[18px] text-destructive" />
          </div>
          <h3 id="confirm-title" className="font-bold text-foreground">
            אישור פעולה
          </h3>
        </div>
        <p
          id="confirm-desc"
          className="text-sm text-muted-foreground mb-6 leading-relaxed"
        >
          {message}
        </p>
        <div className="flex gap-3">
          <button
            type="button"
            onClick={onCancel}
            className="flex-1 py-2.5 rounded-xl border border-border text-sm font-medium text-muted-foreground hover:bg-secondary transition-colors"
          >
            ביטול
          </button>
          <button
            type="button"
            onClick={onConfirm}
            disabled={loading}
            className="flex-1 py-2.5 rounded-xl bg-destructive hover:bg-destructive/90 text-white text-sm font-semibold transition-colors disabled:opacity-50"
          >
            {loading ? (
              <Loader2 className="mx-auto h-4 w-4 animate-spin" />
            ) : (
              "אישור"
            )}
          </button>
        </div>
      </motion.div>
    </div>
  );
}
