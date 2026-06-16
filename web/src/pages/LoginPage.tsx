import { AuthPage } from "./AuthPage";
import { useDocumentTitle } from "@/hooks/useDocumentTitle";

export function LoginPage() {
  useDocumentTitle("התחברות");
  return <AuthPage defaultTab="login" />;
}
