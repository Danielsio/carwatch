import { useNavigate } from "react-router";
import { SearchX } from "lucide-react";
import { EmptyState, PageHeader } from "@/components/ui";

export function NotFoundPage() {
  const navigate = useNavigate();

  return (
    <div className="space-y-6 pb-20 md:pb-4">
      <PageHeader title="404" />
      <EmptyState
        icon={SearchX}
        title="הדף לא נמצא"
        description="הדף שחיפשת לא קיים או הוסר"
        action={
          <button
            type="button"
            onClick={() => void navigate("/")}
            className="mt-4 rounded-xl bg-primary px-5 py-2.5 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90"
          >
            חזרה לדף הבית
          </button>
        }
      />
    </div>
  );
}
