import { Navigate, Outlet } from "react-router";
import { Loader2 } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";

export function ProtectedRoute() {
  const { user, loading } = useAuth();

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background">
        <span className="sr-only" role="status">טוען...</span>
        <Loader2
          className="h-10 w-10 animate-spin text-primary"
          aria-hidden
        />
      </div>
    );
  }

  if (!user) {
    return <Navigate to="/login" replace />;
  }

  return <Outlet />;
}
