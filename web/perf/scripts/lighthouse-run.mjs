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

    // Single-run mobile Lighthouse in CI is noisy (the same commit has scored
    // 52→62 across runs as runner CPU contention varies). Run each config N
    // times and keep the MEDIAN so the budget gate reflects the real page, not
    // a slow-runner roll of the dice. Override with LH_RUNS.
    const RUNS = Math.max(1, Number(process.env.LH_RUNS || 3));

    for (const route of ROUTES) {
      summary[route.name] = {};
      for (const config of CONFIGS) {
        const url = `${BASE}${route.path}`;
        console.log(`[lighthouse] ${config.name} ${url} (median of ${RUNS})`);

        const samples = [];
        for (let i = 0; i < RUNS; i++) {
          const result = await lighthouse(url, {
            port: chrome.port,
            output: "json",
            onlyCategories: ["performance", "accessibility"],
            ...config.settings,
          });
          const report = JSON.parse(result.report);
          samples.push({
            perf: report.categories.performance.score * 100,
            a11y: report.categories.accessibility.score * 100,
            fcp: report.audits["first-contentful-paint"].numericValue,
            lcp: report.audits["largest-contentful-paint"].numericValue,
            tbt: report.audits["total-blocking-time"].numericValue,
            report: result.report,
          });
          const s = samples[i];
          console.log(`  run ${i + 1}/${RUNS}: perf=${s.perf} FCP=${Math.round(s.fcp)}ms LCP=${Math.round(s.lcp)}ms TBT=${Math.round(s.tbt)}ms`);
        }

        // Median by performance score (the metric the budget gates on).
        samples.sort((a, b) => a.perf - b.perf);
        const median = samples[Math.floor(samples.length / 2)];

        summary[route.name][config.name] = {
          perf: median.perf,
          a11y: median.a11y,
          fcp: median.fcp,
          lcp: median.lcp,
          tbt: median.tbt,
        };
        console.log(`  median: perf=${median.perf} a11y=${median.a11y} FCP=${Math.round(median.fcp)}ms LCP=${Math.round(median.lcp)}ms TBT=${Math.round(median.tbt)}ms`);

        const reportPath = resolve(reportsDir, `${route.name}-${config.name}.json`);
        writeFileSync(reportPath, median.report);
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
