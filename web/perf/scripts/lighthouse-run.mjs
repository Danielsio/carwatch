// Run Lighthouse audits against a local or remote server and write JSON reports
// to web/perf/reports/. Supports both mobile and desktop configs.
//
// Usage:
//   BASE=http://localhost:4173 node scripts/lighthouse-run.mjs
//
// Requires Chrome/Chromium installed on the system.

import { writeFileSync, mkdirSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const reportsDir = resolve(__dirname, "../reports");
mkdirSync(reportsDir, { recursive: true });

const BASE = process.env.BASE || "http://localhost:4173";

const ROUTES = [
  { path: "/", name: "landing" },
];

const CONFIGS = [
  {
    name: "mobile",
    settings: {
      formFactor: "mobile",
      screenEmulation: {
        mobile: true,
        width: 412,
        height: 823,
        deviceScaleFactor: 1.75,
      },
      throttling: {
        rttMs: 150,
        throughputKbps: 1638.4,
        cpuSlowdownMultiplier: 4,
      },
    },
  },
  {
    name: "desktop",
    settings: {
      formFactor: "desktop",
      screenEmulation: {
        mobile: false,
        width: 1350,
        height: 940,
        deviceScaleFactor: 1,
      },
      throttling: {
        rttMs: 40,
        throughputKbps: 10240,
        cpuSlowdownMultiplier: 1,
      },
    },
  },
];

async function run() {
  const lighthouse = (await import("lighthouse")).default;
  const chromeLauncher = await import("chrome-launcher");

  const chrome = await chromeLauncher.launch({ chromeFlags: ["--headless", "--no-sandbox"] });

  try {
    const summary = {};

    for (const route of ROUTES) {
      summary[route.name] = {};
      for (const config of CONFIGS) {
        const url = `${BASE}${route.path}`;
        console.log(`[lighthouse] ${config.name} ${url}`);

        const result = await lighthouse(url, {
          port: chrome.port,
          output: "json",
          onlyCategories: ["performance", "accessibility"],
          ...config.settings,
        });

        const report = JSON.parse(result.report);
        const perf = report.categories.performance.score * 100;
        const a11y = report.categories.accessibility.score * 100;
        const fcp = report.audits["first-contentful-paint"].numericValue;
        const lcp = report.audits["largest-contentful-paint"].numericValue;
        const tbt = report.audits["total-blocking-time"].numericValue;

        summary[route.name][config.name] = { perf, a11y, fcp, lcp, tbt };
        console.log(`  perf=${perf} a11y=${a11y} FCP=${Math.round(fcp)}ms LCP=${Math.round(lcp)}ms TBT=${Math.round(tbt)}ms`);

        const reportPath = resolve(reportsDir, `${route.name}-${config.name}.json`);
        writeFileSync(reportPath, result.report);
      }
    }

    const summaryPath = resolve(reportsDir, "lighthouse-summary.json");
    writeFileSync(summaryPath, JSON.stringify(summary, null, 2));
    console.log(`[lighthouse] summary → ${summaryPath}`);
  } finally {
    await chrome.kill();
  }
}

run().catch((err) => {
  console.error(err);
  process.exit(1);
});
