import { Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";

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
  return (
    <AlertDialog open onOpenChange={(v) => !v && onCancel()}>
      <AlertDialogContent dir="rtl" className="max-w-sm">
        <AlertDialogHeader className="flex-row items-center gap-3">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-destructive/15">
            <Trash2 className="h-[18px] w-[18px] text-destructive" />
          </div>
          <AlertDialogTitle>אישור פעולה</AlertDialogTitle>
        </AlertDialogHeader>
        <AlertDialogDescription className="leading-relaxed">
          {message}
        </AlertDialogDescription>
        <AlertDialogFooter className="flex-row gap-3">
          <Button variant="secondary" onClick={onCancel} className="flex-1">
            ביטול
          </Button>
          <Button
            variant="destructive"
            onClick={onConfirm}
            loading={loading}
            className="flex-1"
          >
            אישור
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
