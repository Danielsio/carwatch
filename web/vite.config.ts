/// <reference types="vitest/config" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { visualizer } from "rollup-plugin-visualizer";
import path from "path";

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    // `ANALYZE=1 npm run build` writes dist/stats.html (gzip-sized treemap)
    // for auditing chunk composition. No effect on normal builds.
    ...(process.env.ANALYZE
      ? [visualizer({ filename: "dist/stats.html", gzipSize: true, brotliSize: true })]
      : []),
  ],
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/test/setup.ts"],
    include: ["src/**/*.test.{ts,tsx}"],
    css: false,
    coverage: {
      provider: "v8",
      reporter: ["text", "text-summary"],
      include: ["src/**/*.{ts,tsx}"],
      exclude: ["src/**/*.test.{ts,tsx}", "src/test/**", "src/vite-env.d.ts"],
    },
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
      "/healthz": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
  build: {
    chunkSizeWarningLimit: 600,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes("node_modules")) return;
          // recharts + its whole transitive cluster (d3, victory-vendor,
          // es-toolkit, decimal.js, …). These are only used by lazy chart
          // pages, so keeping them together stops them leaking into the
          // first-load vendor chunk.
          if (
            id.includes("recharts") ||
            id.includes("d3-") ||
            id.includes("internmap") ||
            id.includes("victory-vendor") ||
            id.includes("decimal.js") ||
            id.includes("es-toolkit") ||
            id.includes("react-smooth") ||
            id.includes("eventemitter3") ||
            id.includes("react-is") ||
            id.includes("tiny-invariant")
          )
            return "recharts";
          if (id.includes("motion")) return "motion";
          if (id.includes("firebase")) return "firebase";
          if (id.includes("@tanstack/react-query")) return "rq";
          if (id.includes("lucide-react")) return "icons";
          // base-ui + cmdk power the shadcn primitives used only inside the
          // authenticated app (Shell) and a few lazy routes — never the public
          // landing. Keep them out of the always-preloaded vendor chunk.
          if (id.includes("@base-ui") || id.includes("@floating-ui"))
            return "baseui";
          if (id.includes("cmdk")) return "cmdk";
          return "vendor";
        },
      },
    },
  },
});
