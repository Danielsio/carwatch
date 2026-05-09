import { useEffect, useRef } from "react";
import { X } from "lucide-react";
import { motion } from "motion/react";

export function DetailModal({
  title,
  fields,
  onClose,
  actions,
}: {
  title: string;
  fields: { label: string; value: string | number | null | undefined }[];
  onClose: () => void;
  actions?: React.ReactNode;
}) {
  const closeRef = useRef<HTMLButtonElement>(null);
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    closeRef.current?.focus();
  }, []);

  useEffect(() => {
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = "";
    };
  }, []);

  function handleKeyDown(e: React.KeyboardEvent<HTMLDivElement>) {
    if (e.key === "Escape") {
      onClose();
      return;
    }
    if (e.key !== "Tab" || !rootRef.current) return;
    const root = rootRef.current;
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
      ref={rootRef}
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      role="dialog"
      aria-modal="true"
      aria-labelledby="detail-title"
      onKeyDown={handleKeyDown}
    >
      <motion.div
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        exit={{ opacity: 0 }}
        className="absolute inset-0 bg-black/60 backdrop-blur-sm"
        onClick={onClose}
      />
      <motion.div
        initial={{ opacity: 0, y: 24 }}
        animate={{ opacity: 1, y: 0 }}
        exit={{ opacity: 0, y: 24 }}
        transition={{ type: "spring", duration: 0.35, bounce: 0.1 }}
        className="relative bg-card border border-border rounded-2xl p-6 max-w-lg w-full shadow-2xl max-h-[80vh] overflow-y-auto"
      >
        <div className="flex items-center justify-between mb-5">
          <h3 id="detail-title" className="font-bold text-foreground">
            {title}
          </h3>
          <button
            ref={closeRef}
            type="button"
            onClick={onClose}
            className="flex h-8 w-8 items-center justify-center rounded-lg bg-secondary text-muted-foreground hover:text-foreground transition-colors"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
        <div className="space-y-0.5">
          {fields.map((f) => {
            if (f.value === undefined || f.value === null || f.value === "")
              return null;
            return (
              <div
                key={f.label}
                className="flex gap-3 py-2.5 border-b border-border/50 last:border-0"
              >
                <span className="text-xs text-muted-foreground w-28 flex-shrink-0 mt-0.5">
                  {f.label}
                </span>
                <span className="text-sm text-foreground font-medium break-all">
                  {String(f.value)}
                </span>
              </div>
            );
          })}
        </div>
        {actions && (
          <div className="flex justify-end mt-4 pt-4 border-t border-border/50">
            {actions}
          </div>
        )}
      </motion.div>
    </div>
  );
}
