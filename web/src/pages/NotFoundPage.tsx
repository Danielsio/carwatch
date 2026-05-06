import { Link } from "react-router";
import { SearchX } from "lucide-react";
import { Button, EmptyState, PageHeader, PageShell } from "@/components/ui";

export function NotFoundPage() {
  return (
    <PageShell>
      <PageHeader title="404" />
      <EmptyState
        icon={SearchX}
        title="הדף לא נמצא"
        description="הדף שחיפשת לא קיים או הוסר"
        action={
          <Button asChild variant="primary" size="sm">
            <Link to="/dashboard">חזרה לדף הבית</Link>
          </Button>
        }
      />
    </PageShell>
  );
}
