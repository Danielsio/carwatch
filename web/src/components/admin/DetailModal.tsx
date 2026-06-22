import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

export function DetailModal({
  title,
  fields,
  onClose,
  actions,
  children,
}: {
  title: string;
  fields: { label: string; value: string | number | null | undefined }[];
  onClose: () => void;
  actions?: React.ReactNode;
  children?: React.ReactNode;
}) {
  return (
    <Dialog open onOpenChange={(v) => !v && onClose()}>
      <DialogContent dir="rtl" className="max-w-lg max-h-[80vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
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
        {children}
        {actions && (
          <div className="flex justify-end mt-4 pt-4 border-t border-border/50">
            {actions}
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
