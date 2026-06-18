// Build-time prerender of public pages (landing, login, signup) and app-shell.
//
// Runs after `vite build` (see the `build` npm script). It renders each route
// to static HTML with react-dom/server and injects it into the built template,
// so the pages are present in the initial HTML response (SEO + fast first paint)
// without any runtime/JS dependency.
//
// Pure Node — no headless browser — so it runs identically in local dev, CI,
// and the Alpine Docker build stage. Lazy below-the-fold sections are resolved
// via `renderToPipeableStream`'s `onAllReady` callback.
//
// A clean copy of the shell is written to `dist/app-shell.html` with a static
// skeleton layout: the Go SPA handler serves that for authenticated routes so
// they show structure at FCP instead of a spinner.

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
const loginPath = resolve(webRoot, "dist/login.html");
const signupPath = resolve(webRoot, "dist/signup.html");
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

function renderToString(element) {
  return new Promise((resolvePromise, reject) => {
    let html = "";
    const sink = new Writable({
      write(chunk, _encoding, callback) {
        html += chunk;
        callback();
      },
    });
    sink.on("finish", () => resolvePromise(html));
    sink.on("error", reject);

    const { pipe } = renderToPipeableStream(createElement(element), {
      onAllReady() {
        pipe(sink);
      },
      onError(error) {
        reject(error);
      },
    });
  });
}

const APP_SHELL_SKELETON = `<div id="root"><div style="display:flex;min-height:100vh;background:var(--background,#F4F6FA)"><aside style="width:256px;border-inline-end:1px solid var(--border,#DFE4ED);padding:16px;display:none" class="sm:!block"><div style="height:32px;width:120px;border-radius:8px;background:var(--muted,#EEF1F6);margin-bottom:24px"></div><div style="display:flex;flex-direction:column;gap:8px">${Array.from({ length: 5 }, () => '<div style="height:36px;border-radius:8px;background:var(--muted,#EEF1F6)"></div>').join("")}</div></aside><main style="flex:1;padding:24px"><div style="display:grid;gap:16px;grid-template-columns:repeat(auto-fill,minmax(240px,1fr));margin-bottom:24px">${Array.from({ length: 3 }, () => '<div style="height:96px;border-radius:12px;background:var(--muted,#EEF1F6)"></div>').join("")}</div><div style="height:320px;border-radius:12px;background:var(--muted,#EEF1F6)"></div></main></div></div>`;

// `ssrLoadModule` transforms TSX, the `@` alias, and CSS imports on the fly, so
// no separate SSR bundle is needed.
const vite = await createServer({
  root: webRoot,
  logLevel: "error",
  server: { middlewareMode: true },
  appType: "custom",
});

try {
  // Preserve the empty shell before injecting content, then add skeleton.
  copyFileSync(indexHtmlPath, shellPath);
  const shellHtml = readFileSync(shellPath, "utf8").replace(ROOT_DIV, APP_SHELL_SKELETON);
  writeFileSync(shellPath, shellHtml);

  // --- Landing page ---
  const { createLandingElement } = await vite.ssrLoadModule(
    "/src/prerender/entry-landing.tsx",
  );
  const landingHtml = await renderToString(createLandingElement);
  const prerendered = template.replace(ROOT_DIV, `<div id="root">${landingHtml}</div>`);
  writeFileSync(indexHtmlPath, prerendered);
  console.log(
    `[prerender] landing → dist/index.html (+${landingHtml.length} bytes)`,
  );

  // --- Auth pages ---
  const { createLoginElement, createSignupElement } = await vite.ssrLoadModule(
    "/src/prerender/entry-auth.tsx",
  );

  const loginHtml = await renderToString(createLoginElement);
  writeFileSync(loginPath, template.replace(ROOT_DIV, `<div id="root">${loginHtml}</div>`));
  console.log(`[prerender] login → dist/login.html (+${loginHtml.length} bytes)`);

  const signupHtml = await renderToString(createSignupElement);
  writeFileSync(signupPath, template.replace(ROOT_DIV, `<div id="root">${signupHtml}</div>`));
  console.log(`[prerender] signup → dist/signup.html (+${signupHtml.length} bytes)`);

  console.log(`[prerender] skeleton → dist/app-shell.html`);
} finally {
  await vite.close();
}
