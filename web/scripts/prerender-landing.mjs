// Build-time prerender of the public landing page.
//
// Runs after `vite build` (see the `build` npm script). It renders the landing
// route to static HTML with react-dom/server and injects it into the built
// `dist/index.html`, so the marketing copy/headings are present in the initial
// HTML response (SEO + fast first paint) without any runtime/JS dependency.
//
// Pure Node — no headless browser — so it runs identically in local dev, CI,
// and the Alpine Docker build stage. Lazy below-the-fold sections are resolved
// via `renderToPipeableStream`'s `onAllReady` callback.
//
// A clean copy of the shell is written to `dist/app-shell.html`: the Go SPA
// handler serves that for non-landing routes so the authenticated app never
// briefly flashes the marketing page before the bundle mounts.

import { createServer } from "vite";
import { renderToPipeableStream } from "react-dom/server";
import { createElement } from "react";
import { Writable } from "node:stream";
import { readFileSync, writeFileSync, copyFileSync, existsSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";

const webRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const indexHtmlPath = resolve(webRoot, "dist/index.html");
const shellPath = resolve(webRoot, "dist/app-shell.html");
const ROOT_DIV = '<div id="root"></div>';

function fail(message) {
  console.error(`[prerender] ${message}`);
  process.exit(1);
}

if (!existsSync(indexHtmlPath)) {
  fail("dist/index.html not found — run `vite build` first.");
}

const template = readFileSync(indexHtmlPath, "utf8");
if (!template.includes(ROOT_DIV)) {
  fail(`expected ${ROOT_DIV} in dist/index.html; aborting to avoid a bad write.`);
}

// Preserve the empty shell for the SPA fallback before we inject content.
copyFileSync(indexHtmlPath, shellPath);

// `ssrLoadModule` transforms TSX, the `@` alias, and CSS imports on the fly, so
// no separate SSR bundle is needed.
const vite = await createServer({
  root: webRoot,
  logLevel: "error",
  server: { middlewareMode: true },
  appType: "custom",
});

try {
  const { createLandingElement } = await vite.ssrLoadModule(
    "/src/prerender/entry-landing.tsx",
  );

  const appHtml = await new Promise((resolvePromise, reject) => {
    let html = "";
    const sink = new Writable({
      write(chunk, _encoding, callback) {
        html += chunk;
        callback();
      },
    });
    sink.on("finish", () => resolvePromise(html));
    sink.on("error", reject);

    // `onAllReady` fires once every Suspense boundary (the lazy below-the-fold
    // sections) has resolved, giving us the complete static markup.
    const { pipe } = renderToPipeableStream(createElement(createLandingElement), {
      onAllReady() {
        pipe(sink);
      },
      onError(error) {
        reject(error);
      },
    });
  });

  const prerendered = template.replace(ROOT_DIV, `<div id="root">${appHtml}</div>`);
  writeFileSync(indexHtmlPath, prerendered);
  console.log(
    `[prerender] landing → dist/index.html (+${appHtml.length} bytes); clean shell → dist/app-shell.html`,
  );
} finally {
  await vite.close();
}
