import { Component } from "react";
import type { ErrorInfo, ReactNode } from "react";
import { AlertTriangle, RotateCcw, Home } from "lucide-react";

interface Props {
  children: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error(
      "[ErrorBoundary]",
      error.name,
      error.message,
      "\nComponent stack:",
      info.componentStack,
    );
  }

  private handleRetry = () => {
    this.setState({ hasError: false, error: null });
  };

  render() {
    if (this.state.hasError) {
      const isDev = import.meta.env.DEV;

      return (
        <div className="flex min-h-screen flex-col items-center justify-center gap-4 bg-background p-8 text-center">
          <div className="flex h-14 w-14 items-center justify-center rounded-full bg-destructive/10">
            <AlertTriangle className="h-7 w-7 text-destructive" />
          </div>
          <h1 className="text-xl font-semibold text-foreground">
            משהו השתבש
          </h1>
          <p className="max-w-md text-sm text-muted-foreground">
            אירעה שגיאה בלתי צפויה. נסה ללחוץ &quot;נסה שוב&quot; או לחזור
            לדף הבית.
          </p>

          {isDev && this.state.error && (
            <pre className="mt-2 max-h-40 max-w-lg overflow-auto rounded-lg bg-muted p-3 text-left text-xs text-muted-foreground dir-ltr">
              {this.state.error.name}: {this.state.error.message}
            </pre>
          )}

          <div className="mt-2 flex gap-3">
            <button
              type="button"
              onClick={this.handleRetry}
              className="inline-flex items-center gap-2 rounded-xl bg-primary px-5 py-2.5 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90"
            >
              <RotateCcw className="h-4 w-4" />
              נסה שוב
            </button>
            <a
              href="/"
              className="inline-flex items-center gap-2 rounded-xl border border-border bg-card px-5 py-2.5 text-sm font-medium text-foreground transition-colors hover:bg-muted"
            >
              <Home className="h-4 w-4" />
              דף הבית
            </a>
          </div>
        </div>
      );
    }

    return this.props.children;
  }
}
