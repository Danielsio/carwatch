import { useState, type FormEvent, useEffect } from "react";
import { Link, useNavigate, useLocation } from "react-router";
import { Loader2, Eye, EyeOff, Check, X, Car } from "lucide-react";
import { usePageTitle } from "@/hooks/usePageTitle";
import {
  signInWithEmailAndPassword,
  createUserWithEmailAndPassword,
  signInWithPopup,
} from "firebase/auth";
import { auth, firebaseAuthErrorCode, googleProvider } from "@/lib/firebase";
import { useAuth } from "@/contexts/AuthContext";
import { AuroraBackground } from "@/components/ui/AuroraBackground";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent } from "@/components/ui/card";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { cn } from "@/lib/utils";

function mapLoginError(code: string) {
  switch (code) {
    case "auth/invalid-credential":
    case "auth/wrong-password":
    case "auth/user-not-found":
    case "auth/invalid-email":
      return "אימייל או סיסמה לא תקינים.";
    case "auth/too-many-requests":
      return "יותר מדי ניסיונות. נסה שוב מאוחר יותר.";
    case "auth/popup-closed-by-user":
      return "חלון ההתחברות נסגר לפני ההשלמה.";
    case "auth/network-request-failed":
      return "בעיית רשת. בדוק את החיבור.";
    default:
      return "לא הצלחנו להתחבר. נסה שוב.";
  }
}

function mapSignupError(code: string) {
  switch (code) {
    case "auth/email-already-in-use":
      return "כתובת האימייל כבר בשימוש.";
    case "auth/invalid-email":
      return "כתובת אימייל לא תקינה.";
    case "auth/weak-password":
      return "הסיסמה חלשה מדי (לפחות 6 תווים).";
    case "auth/too-many-requests":
      return "יותר מדי ניסיונות. נסה שוב מאוחר יותר.";
    case "auth/popup-closed-by-user":
      return "חלון ההרשמה נסגר לפני ההשלמה.";
    case "auth/network-request-failed":
      return "בעיית רשת. בדוק את החיבור.";
    default:
      return "לא הצלחנו ליצור חשבון. נסה שוב.";
  }
}

function isValidEmail(v: string) {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v);
}

const PASSWORD_RULES = [
  { test: (v: string) => v.length >= 6, label: "לפחות 6 תווים" },
  { test: (v: string) => /[A-Za-z]/.test(v), label: "אות אחת לפחות" },
  { test: (v: string) => /\d/.test(v), label: "ספרה אחת לפחות" },
];

export function AuthPage({ defaultTab }: { defaultTab?: "login" | "signup" }) {
  const [tab, setTab] = useState<"login" | "signup">(defaultTab ?? "login");
  usePageTitle(tab === "login" ? "התחברות" : "הרשמה");
  const navigate = useNavigate();
  const location = useLocation();
  const { user } = useAuth();

  const stateObj =
    typeof location.state === "object" && location.state !== null
      ? (location.state as Record<string, unknown>)
      : {};

  const redirectTo =
    "from" in stateObj && typeof stateObj.from === "string"
      ? stateObj.from
      : undefined;

  const searchData = stateObj.searchData ?? undefined;

  const isSafePath =
    !!redirectTo &&
    redirectTo.startsWith("/") &&
    !redirectTo.startsWith("//") &&
    redirectTo !== "/login" &&
    redirectTo !== "/signup";

  const from = isSafePath && redirectTo ? redirectTo : "/dashboard";

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<"email" | "google" | null>(null);
  const [touched, setTouched] = useState({ email: false, password: false, confirm: false });

  useEffect(() => {
    if (user) navigate(from, { replace: true, state: searchData ? { searchData } : undefined });
  }, [user, navigate, from, searchData]);

  useEffect(() => {
    setError(null);
    setTouched({ email: false, password: false, confirm: false });
  }, [tab]);

  const emailErr =
    touched.email && email.length > 0 && !isValidEmail(email)
      ? "כתובת אימייל לא תקינה"
      : null;
  const passwordErr =
    touched.password && password.length > 0 && password.length < 6
      ? "סיסמה חייבת להכיל 6 תווים לפחות"
      : null;
  const allRulesPass = PASSWORD_RULES.every((r) => r.test(password));
  const confirmErr =
    tab === "signup" && touched.confirm && confirm.length > 0 && confirm !== password
      ? "הסיסמאות אינן תואמות"
      : null;

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setTouched({ email: true, password: true, confirm: true });
    if (!isValidEmail(email)) return;

    if (tab === "signup") {
      if (!allRulesPass) {
        setError("הסיסמה לא עומדת בדרישות.");
        return;
      }
      if (password !== confirm) {
        setError("הסיסמאות אינן תואמות.");
        return;
      }
    }

    setBusy("email");
    try {
      if (tab === "login") {
        await signInWithEmailAndPassword(auth, email.trim(), password);
      } else {
        await createUserWithEmailAndPassword(auth, email.trim(), password);
      }
      navigate(from, { replace: true, state: searchData ? { searchData } : undefined });
    } catch (err) {
      const mapFn = tab === "login" ? mapLoginError : mapSignupError;
      setError(mapFn(firebaseAuthErrorCode(err)));
    } finally {
      setBusy(null);
    }
  }

  async function onGoogle() {
    setError(null);
    setBusy("google");
    try {
      await signInWithPopup(auth, googleProvider);
      navigate(from, { replace: true, state: searchData ? { searchData } : undefined });
    } catch (err) {
      const mapFn = tab === "login" ? mapLoginError : mapSignupError;
      setError(mapFn(firebaseAuthErrorCode(err)));
    } finally {
      setBusy(null);
    }
  }

  return (
    <div dir="rtl" className="relative flex min-h-screen items-center justify-center px-4 py-12">
      <AuroraBackground variant="hero" />
      <div className="w-full max-w-[420px] animate-fade-in">
        {/* Brand */}
        <div className="mb-8 flex flex-col items-center gap-3 text-center">
          <div className="shine flex h-14 w-14 items-center justify-center rounded-2xl bg-gradient-to-br from-primary to-primary/80 text-white shadow-[0_8px_28px_-6px_var(--color-glow-primary)]">
            <Car className="h-7 w-7" />
          </div>
          <h1 className="text-aurora text-2xl font-extrabold tracking-tight">
            CarWatch
          </h1>
        </div>

        {/* Card */}
        <Card className="glow-border relative shadow-2xl">
          <CardContent className="p-6 sm:p-8">
          {/* Tab toggle */}
          <Tabs value={tab} onValueChange={(v) => setTab(v as "login" | "signup")} className="mb-6">
            <TabsList className="w-full">
              <TabsTrigger value="login" className="flex-1">התחברות</TabsTrigger>
              <TabsTrigger value="signup" className="flex-1">הרשמה</TabsTrigger>
            </TabsList>
          </Tabs>

          {/* Google OAuth — primary CTA */}
          <Button
            variant="outline"
            size="lg"
            className="w-full gap-3"
            onClick={onGoogle}
            disabled={busy !== null}
          >
            {busy === "google" ? (
              <Loader2 className="h-4 w-4 animate-spin" aria-hidden />
            ) : (
              <svg className="h-5 w-5" viewBox="0 0 24 24" aria-hidden>
                <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" />
                <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" />
                <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" />
                <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" />
              </svg>
            )}
            {tab === "login" ? "המשך עם Google" : "הירשם עם Google"}
          </Button>

          {/* Divider */}
          <div className="relative my-5">
            <div className="absolute inset-0 flex items-center">
              <div className="w-full border-t border-border/60" />
            </div>
            <div className="relative flex justify-center text-[11px] uppercase tracking-wider">
              <span className="bg-card px-3 text-muted-foreground">
                או עם אימייל
              </span>
            </div>
          </div>

          {error && (
            <div
              className="mb-4 rounded-lg border border-destructive/40 bg-destructive/10 px-4 py-2.5 text-sm text-destructive"
              role="alert"
            >
              {error}
            </div>
          )}

          <form onSubmit={onSubmit} className="space-y-3.5">
            <div>
              <Label htmlFor="auth-email">
                אימייל
              </Label>
              <Input
                id="auth-email"
                name="email"
                type="email"
                autoComplete="email"
                dir="ltr"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                onBlur={() => setTouched((p) => ({ ...p, email: true }))}
                required
                error={!!emailErr}
                aria-describedby={emailErr ? "auth-email-error" : undefined}
                placeholder="you@example.com"
              />
              {emailErr && <p id="auth-email-error" className="mt-1 text-xs text-destructive">{emailErr}</p>}
            </div>

            <div>
              <Label htmlFor="auth-password">סיסמה</Label>
              <div className="relative">
                <Input
                  id="auth-password"
                  name="password"
                  type={showPassword ? "text" : "password"}
                  autoComplete={tab === "login" ? "current-password" : "new-password"}
                  dir="ltr"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  onBlur={() => setTouched((p) => ({ ...p, password: true }))}
                  required
                  error={!!passwordErr}
                  aria-describedby={passwordErr ? "auth-password-error" : undefined}
                  className="ps-10"
                  placeholder="••••••••"
                />
                <button
                  type="button"
                  tabIndex={-1}
                  onClick={() => setShowPassword((v) => !v)}
                  className="absolute start-3 top-1/2 -translate-y-1/2 text-muted-foreground transition-colors hover:text-foreground"
                  aria-label={showPassword ? "הסתר סיסמה" : "הצג סיסמה"}
                >
                  {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </button>
              </div>
              {tab === "login" && passwordErr && (
                <p id="auth-password-error" className="mt-1 text-xs text-destructive">{passwordErr}</p>
              )}
              {tab === "signup" && touched.password && password.length > 0 && (
                <ul className="mt-2 space-y-0.5">
                  {PASSWORD_RULES.map((rule) => {
                    const pass = rule.test(password);
                    return (
                      <li
                        key={rule.label}
                        className={cn(
                          "flex items-center gap-1.5 text-xs transition-colors",
                          pass ? "text-success" : "text-muted-foreground",
                        )}
                      >
                        {pass ? <Check className="h-3 w-3" /> : <X className="h-3 w-3" />}
                        {rule.label}
                      </li>
                    );
                  })}
                </ul>
              )}
            </div>

            {tab === "signup" && (
              <div>
                <Label htmlFor="auth-confirm">אימות סיסמה</Label>
                <Input
                  id="auth-confirm"
                  name="confirm"
                  type="password"
                  autoComplete="new-password"
                  dir="ltr"
                  value={confirm}
                  onChange={(e) => setConfirm(e.target.value)}
                  onBlur={() => setTouched((p) => ({ ...p, confirm: true }))}
                  required
                  error={!!confirmErr}
                  aria-describedby={confirmErr ? "auth-confirm-error" : undefined}
                  placeholder="••••••••"
                />
                {confirmErr && <p id="auth-confirm-error" className="mt-1 text-xs text-destructive">{confirmErr}</p>}
                {touched.confirm && confirm.length > 0 && confirm === password && (
                  <p className="mt-1 flex items-center gap-1 text-xs text-success">
                    <Check className="h-3 w-3" />
                    סיסמאות תואמות
                  </p>
                )}
              </div>
            )}

            <Button
              type="submit"
              size="lg"
              className="w-full"
              loading={busy === "email"}
              disabled={busy !== null}
            >
              {tab === "login" ? "התחבר" : "צור חשבון"}
            </Button>
          </form>

          {/* Guest entry */}
          <div className="mt-5 flex flex-col items-center gap-2">
            <Button asChild variant="ghost" size="lg" className="w-full">
              <Link to="/try">המשך כאורח</Link>
            </Button>
            <p className="text-center text-xs text-muted-foreground/70">
              <Link to="/" className="underline-offset-4 hover:underline hover:text-muted-foreground">
                מה זה CarWatch?
              </Link>
            </p>
          </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
