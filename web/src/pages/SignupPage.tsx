import { useState, FormEvent, useEffect } from "react";
import { Link, useNavigate } from "react-router";
import { Loader2, Eye, EyeOff, Check, X } from "lucide-react";
import {
  createUserWithEmailAndPassword,
  signInWithPopup,
} from "firebase/auth";
import { auth, firebaseAuthErrorCode, googleProvider } from "@/lib/firebase";
import { useAuth } from "@/contexts/AuthContext";
import { cn } from "@/lib/utils";

function mapAuthError(code: string) {
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

export function SignupPage() {
  const navigate = useNavigate();
  const { user } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<"email" | "google" | null>(null);
  const [touched, setTouched] = useState({
    email: false,
    password: false,
    confirm: false,
  });

  useEffect(() => {
    if (user) navigate("/", { replace: true });
  }, [user, navigate]);

  const emailErr =
    touched.email && email.length > 0 && !isValidEmail(email)
      ? "כתובת אימייל לא תקינה"
      : null;

  const allRulesPass = PASSWORD_RULES.every((r) => r.test(password));
  const confirmErr =
    touched.confirm && confirm.length > 0 && confirm !== password
      ? "הסיסמאות אינן תואמות"
      : null;

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setTouched({ email: true, password: true, confirm: true });
    if (!isValidEmail(email)) return;
    if (!allRulesPass) {
      setError("הסיסמה לא עומדת בדרישות.");
      return;
    }
    if (password !== confirm) {
      setError("הסיסמאות אינן תואמות.");
      return;
    }
    setBusy("email");
    try {
      await createUserWithEmailAndPassword(auth, email.trim(), password);
      navigate("/", { replace: true });
    } catch (err) {
      setError(mapAuthError(firebaseAuthErrorCode(err)));
    } finally {
      setBusy(null);
    }
  }

  async function onGoogle() {
    setError(null);
    setBusy("google");
    try {
      await signInWithPopup(auth, googleProvider);
      navigate("/", { replace: true });
    } catch (err) {
      setError(mapAuthError(firebaseAuthErrorCode(err)));
    } finally {
      setBusy(null);
    }
  }

  return (
    <div dir="rtl" className="relative min-h-screen overflow-hidden bg-background">
      <div
        className="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_80%_50%_at_50%_-20%,rgba(59,130,246,0.18),transparent)]"
        aria-hidden
      />
      <div
        className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_20%_70%,rgba(16,185,129,0.06),transparent_45%)]"
        aria-hidden
      />

      <div className="relative flex min-h-screen items-center justify-center px-4 py-12">
        <div className="w-full max-w-[420px] animate-fade-in">
          <div className="mb-8 flex flex-col items-center gap-2 text-center">
            <img
              src="/logo-login.png"
              alt="CarWatch"
              className="h-24 w-24 object-contain drop-shadow-[0_4px_24px_rgba(59,130,246,0.25)]"
              width={96}
              height={96}
            />
            <div>
              <h1 className="text-2xl font-semibold tracking-tight text-foreground">
                CarWatch
              </h1>
              <p className="mt-1 text-sm text-muted-foreground">
                יצירת חשבון חדש
              </p>
            </div>
          </div>

          <div className="rounded-2xl border border-border/60 bg-card/80 p-6 shadow-[0_24px_64px_-16px_rgba(0,0,0,0.55)] backdrop-blur-xl sm:p-8">
            {error && (
              <div
                className="mb-5 rounded-xl border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive-foreground"
                role="alert"
              >
                {error}
              </div>
            )}

            <form onSubmit={onSubmit} className="space-y-4">
              <div>
                <label
                  htmlFor="signup-email"
                  className="mb-1.5 block text-sm font-medium text-foreground"
                >
                  אימייל
                </label>
                <input
                  id="signup-email"
                  name="email"
                  type="email"
                  autoComplete="email"
                  dir="ltr"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  onBlur={() => setTouched((p) => ({ ...p, email: true }))}
                  required
                  aria-invalid={!!emailErr}
                  className={cn(
                    "w-full rounded-xl border bg-secondary px-4 py-3 text-sm text-foreground outline-none transition-[box-shadow,border-color]",
                    "placeholder:text-muted-foreground/60",
                    "focus:border-primary/50 focus:ring-2 focus:ring-ring/40",
                    emailErr ? "border-destructive/60" : "border-input",
                  )}
                  placeholder="you@example.com"
                />
                {emailErr && (
                  <p className="mt-1.5 text-xs text-destructive">{emailErr}</p>
                )}
              </div>

              <div>
                <label
                  htmlFor="signup-password"
                  className="mb-1.5 block text-sm font-medium text-foreground"
                >
                  סיסמה
                </label>
                <div className="relative">
                  <input
                    id="signup-password"
                    name="password"
                    type={showPassword ? "text" : "password"}
                    autoComplete="new-password"
                    dir="ltr"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    onBlur={() =>
                      setTouched((p) => ({ ...p, password: true }))
                    }
                    required
                    minLength={6}
                    className={cn(
                      "w-full rounded-xl border border-input bg-secondary py-3 pl-11 pr-4 text-sm text-foreground outline-none transition-[box-shadow,border-color]",
                      "placeholder:text-muted-foreground/60",
                      "focus:border-primary/50 focus:ring-2 focus:ring-ring/40",
                    )}
                    placeholder="••••••••"
                  />
                  <button
                    type="button"
                    tabIndex={-1}
                    onClick={() => setShowPassword((v) => !v)}
                    className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground transition-colors hover:text-foreground"
                    aria-label={showPassword ? "הסתר סיסמה" : "הצג סיסמה"}
                  >
                    {showPassword ? (
                      <EyeOff className="h-4 w-4" />
                    ) : (
                      <Eye className="h-4 w-4" />
                    )}
                  </button>
                </div>
                {touched.password && password.length > 0 && (
                  <ul className="mt-2 space-y-1">
                    {PASSWORD_RULES.map((rule) => {
                      const pass = rule.test(password);
                      return (
                        <li
                          key={rule.label}
                          className={cn(
                            "flex items-center gap-1.5 text-xs transition-colors",
                            pass
                              ? "text-score-great"
                              : "text-muted-foreground",
                          )}
                        >
                          {pass ? (
                            <Check className="h-3 w-3" />
                          ) : (
                            <X className="h-3 w-3" />
                          )}
                          {rule.label}
                        </li>
                      );
                    })}
                  </ul>
                )}
              </div>

              <div>
                <label
                  htmlFor="signup-confirm"
                  className="mb-1.5 block text-sm font-medium text-foreground"
                >
                  אימות סיסמה
                </label>
                <div className="relative">
                  <input
                    id="signup-confirm"
                    name="confirm"
                    type={showConfirm ? "text" : "password"}
                    autoComplete="new-password"
                    dir="ltr"
                    value={confirm}
                    onChange={(e) => setConfirm(e.target.value)}
                    onBlur={() =>
                      setTouched((p) => ({ ...p, confirm: true }))
                    }
                    required
                    className={cn(
                      "w-full rounded-xl border bg-secondary py-3 pl-11 pr-4 text-sm text-foreground outline-none transition-[box-shadow,border-color]",
                      "placeholder:text-muted-foreground/60",
                      "focus:border-primary/50 focus:ring-2 focus:ring-ring/40",
                      confirmErr ? "border-destructive/60" : "border-input",
                    )}
                    placeholder="••••••••"
                  />
                  <button
                    type="button"
                    tabIndex={-1}
                    onClick={() => setShowConfirm((v) => !v)}
                    className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground transition-colors hover:text-foreground"
                    aria-label={showConfirm ? "הסתר סיסמה" : "הצג סיסמה"}
                  >
                    {showConfirm ? (
                      <EyeOff className="h-4 w-4" />
                    ) : (
                      <Eye className="h-4 w-4" />
                    )}
                  </button>
                </div>
                {confirmErr && (
                  <p className="mt-1.5 text-xs text-destructive">
                    {confirmErr}
                  </p>
                )}
                {touched.confirm &&
                  confirm.length > 0 &&
                  confirm === password && (
                    <p className="mt-1.5 flex items-center gap-1 text-xs text-score-great">
                      <Check className="h-3 w-3" />
                      סיסמאות תואמות
                    </p>
                  )}
              </div>

              <button
                type="submit"
                disabled={busy !== null}
                className={cn(
                  "flex w-full items-center justify-center gap-2 rounded-xl bg-primary px-4 py-3 text-sm font-semibold text-primary-foreground transition-opacity",
                  "hover:opacity-95 active:opacity-90",
                  "disabled:pointer-events-none disabled:opacity-50",
                )}
              >
                {busy === "email" ? (
                  <Loader2 className="h-4 w-4 animate-spin" aria-hidden />
                ) : null}
                הירשם
              </button>
            </form>

            <div className="relative my-6">
              <div className="absolute inset-0 flex items-center">
                <div className="w-full border-t border-border/80" />
              </div>
              <div className="relative flex justify-center text-xs uppercase tracking-wider">
                <span className="bg-card px-3 text-muted-foreground">
                  או המשך עם
                </span>
              </div>
            </div>

            <button
              type="button"
              onClick={onGoogle}
              disabled={busy !== null}
              className={cn(
                "flex w-full items-center justify-center gap-3 rounded-xl border border-border bg-secondary px-4 py-3 text-sm font-medium text-foreground transition-colors",
                "hover:bg-accent hover:border-border",
                "disabled:pointer-events-none disabled:opacity-50",
              )}
            >
              {busy === "google" ? (
                <Loader2 className="h-4 w-4 animate-spin" aria-hidden />
              ) : (
                <svg className="h-5 w-5" viewBox="0 0 24 24" aria-hidden>
                  <path
                    fill="#4285F4"
                    d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"
                  />
                  <path
                    fill="#34A853"
                    d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
                  />
                  <path
                    fill="#FBBC05"
                    d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"
                  />
                  <path
                    fill="#EA4335"
                    d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"
                  />
                </svg>
              )}
              הרשמה עם Google
            </button>

            <p className="mt-6 text-center text-sm text-muted-foreground">
              כבר יש לך חשבון?{" "}
              <Link
                to="/login"
                className="font-medium text-primary underline-offset-4 hover:underline"
              >
                התחבר
              </Link>
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
