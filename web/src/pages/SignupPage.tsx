import { AuthPage } from "./AuthPage";
import { useDocumentTitle } from "@/hooks/useDocumentTitle";

export function SignupPage() {
  useDocumentTitle("הרשמה");
  return <AuthPage defaultTab="signup" />;
}
