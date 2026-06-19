// Perf-budget gate: reads reports/lighthouse-summary.json and fails if any
// metric exceeds its threshold. Designed to run as a CI check.
//
// Usage: node scripts/check-budget.mjs

import { readFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const summaryPath = resolve(__dirname, "../reports/lighthouse-summary.json");

const BUDGETS = {
  landing: {
    mobile: { perf: 55 },
    desktop: { perf: 80 },
  },
};

let summary;
try {
  summary = JSON.parse(readFileSync(summaryPath, "utf8"));
} catch (err) {
  console.error(`Failed to read ${summaryPath}: ${err.message}`);
  process.exit(1);
}

let failed = false;

for (const [route, configs] of Object.entries(BUDGETS)) {
  const routeData = summary[route];
  if (!routeData) {
    console.error(`Missing route "${route}" in summary`);
    failed = true;
    continue;
  }

  for (const [config, thresholds] of Object.entries(configs)) {
    const data = routeData[config];
    if (!data) {
      console.error(`Missing config "${config}" for route "${route}"`);
      failed = true;
      continue;
    }

    for (const [metric, threshold] of Object.entries(thresholds)) {
      const actual = data[metric];
      if (typeof actual !== "number" || !Number.isFinite(actual)) {
        console.error(
          `FAIL: ${route}/${config} missing or invalid metric "${metric}"`,
        );
        failed = true;
        continue;
      }
      if (actual < threshold) {
        console.error(
          `FAIL: ${route}/${config} ${metric}=${actual} < ${threshold}`,
        );
        failed = true;
      } else {
        console.log(
          `PASS: ${route}/${config} ${metric}=${actual} >= ${threshold}`,
        );
      }
    }
  }
}

if (failed) {
  console.error("\nPerf budget check FAILED");
  process.exit(1);
} else {
  console.log("\nPerf budget check PASSED");
}
