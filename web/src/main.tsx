import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router";
import { MutationCache, QueryClient, QueryClientProvider } from "@tanstack/react-query";
import App from "./App";
import { ErrorBoundary } from "./components/ErrorBoundary";
import { AuthProvider } from "./contexts/AuthContext";
import { ThemeProvider } from "./contexts/ThemeContext";
import { ToastProvider, showGlobalToast } from "./components/ui/Toast";
import { errorToHebrew } from "./lib/error-messages";
import "./index.css";

if ('serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    // Register once the main thread is idle so the sw.js fetch never competes
    // with lazy chunk downloads / hydration during the initial load. Falls back
    // to a short timer where requestIdleCallback is unavailable (Safari).
    const register = () => {
      navigator.serviceWorker.register('/sw.js').catch(() => {});
    };
    if ('requestIdleCallback' in window) {
      requestIdleCallback(register, { timeout: 5000 });
    } else {
      setTimeout(register, 3000);
    }
  });
}

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5 * 60_000,
      retry: 1,
    },
  },
  mutationCache: new MutationCache({
    onError: (error, _variables, _context, mutation) => {
      if (mutation.options.meta?.suppressToast) return;
      showGlobalToast(errorToHebrew(error), "error");
    },
  }),
});

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ErrorBoundary>
      <ThemeProvider>
        <QueryClientProvider client={queryClient}>
          <BrowserRouter>
            <AuthProvider>
              <ToastProvider>
                <App />
              </ToastProvider>
            </AuthProvider>
          </BrowserRouter>
        </QueryClientProvider>
      </ThemeProvider>
    </ErrorBoundary>
  </StrictMode>,
);
