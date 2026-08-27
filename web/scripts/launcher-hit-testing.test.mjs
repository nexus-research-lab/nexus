import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";

import { createServer } from "vite";

const webRoot = fileURLToPath(new URL("..", import.meta.url));
const server = await createServer({
  configFile: false,
  logLevel: "silent",
  resolve: { alias: { "@": path.join(webRoot, "src") } },
  root: webRoot,
  server: { middlewareMode: true },
});

test.after(async () => {
  await server.close();
});

test("the bootstrap cat stays local and reduced-motion safe", async () => {
  const { AppLoadingState } = await server.ssrLoadModule(
    "/src/shared/ui/layout/app-loading-screen.tsx",
  );
  const html = renderToStaticMarkup(
    React.createElement(AppLoadingState, { message: "正在连接 Nexus" }),
  );
  const [animatedCat, staticCat] = await Promise.all([
    readFile(path.join(webRoot, "public/lotties/cat-loading.webp")),
    readFile(path.join(webRoot, "public/lotties/cat-loading-static.webp")),
  ]);

  assert.match(html, /role="status"/);
  assert.match(html, /cat-loading\.webp/);
  assert.match(html, /cat-loading-static\.webp/);
  assert.match(html, /prefers-reduced-motion: reduce/);
  assert.equal(animatedCat.includes(Buffer.from("ANIM")), true);
  assert.equal(staticCat.includes(Buffer.from("ANIM")), false);
  assert.ok(animatedCat.length < 64 * 1024);
  assert.ok(staticCat.length < 16 * 1024);
});
