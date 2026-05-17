import { Link } from "react-router";
import { SearchX } from "lucide-react";
import { Button, EmptyState, PageHeader, PageShell } from "@/components/ui";
import { useAuth } from "@/contexts/AuthContext";

export function NotFoundPage() {
  const { user } = useAuth();
  const homeLink = user ? "/dashboard" : "/";

  return (
    <PageShell>
      <PageHeader title="404" />
      <EmptyState
        icon={SearchX}
        title="הדף לא נמצא"
        description="הדף שחיפשת לא קיים או הוסר"
        action={
          <Button asChild variant="primary" size="sm">
            <Link to={homeLink}>חזרה לדף הבית</Link>
          </Button>
        }
      />
    </PageShell>
  );
}
