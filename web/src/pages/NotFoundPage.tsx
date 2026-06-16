import { Link } from "react-router";
import { SearchX } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { useAuth } from "@/contexts/AuthContext";

export function NotFoundPage() {
  const { user } = useAuth();
  const homeLink = user ? "/dashboard" : "/";

  return (
    <div className="flex min-h-[60vh] items-center justify-center px-4">
      <Card className="w-full max-w-md text-center">
        <CardContent className="space-y-6 p-8">
          <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-2xl bg-muted">
            <SearchX className="h-8 w-8 text-muted-foreground" />
          </div>
          <div className="space-y-2">
            <h1 className="text-4xl font-extrabold tracking-tight text-foreground">
              404
            </h1>
            <p className="text-sm text-muted-foreground">
              הדף שחיפשת לא קיים או הוסר
            </p>
          </div>
          <Button asChild>
            <Link to={homeLink}>חזרה לדף הבית</Link>
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
