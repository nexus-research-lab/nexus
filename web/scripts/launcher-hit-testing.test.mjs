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

test("the Launcher Agent pile only captures pointer events on real tokens", async () => {
  const { LauncherHeroStage } = await server.ssrLoadModule(
    "/src/features/launcher/hero/launcher-hero-stage.tsx",
  );
  const { I18N_CONTEXT } = await server.ssrLoadModule(
    "/src/shared/i18n/i18n-context.ts",
  );
  const html = renderToStaticMarkup(
    React.createElement(
      I18N_CONTEXT.Provider,
      {
        value: {
          locale: "zh",
          setLocale: () => {},
          t: (key) => key,
        },
      },
      React.createElement(LauncherHeroStage, {
        currentAgentId: null,
        decorativeTokens: [{
          agent_id: "agent-1",
          key: "agent-1",
          kind: "agent",
          label: "AG",
          swatch: {
            fill: "#7c86f8",
            ring: "#ffffff",
            text: "#14172b",
          },
        }],
        isQueryLoading: false,
        mentionTargets: [],
        onEnterHome: () => {},
        onOpenMainAgentDm: () => {},
        onOpenRecentEntry: () => {},
        onQueryChange: () => {},
        onSelectAgent: () => {},
        onSubmit: () => true,
        query: "",
        recentEntries: [{
          agent_id: "agent-1",
          key: "recent-agent-1",
          label: "测试",
          last_activity_at: 1,
          type: "dm",
        }],
      }),
    ),
  );

  assert.match(
    html,
    /class="pointer-events-none absolute bottom-0 left-1\/2 origin-bottom"/,
    "the absolute pile wrapper must not cover the recent-entry and handoff buttons",
  );
  assert.match(
    html,
    /class="pointer-events-auto absolute left-0 top-0 border opacity-0 rounded-full"/,
    "real Agent tokens must remain clickable through the inert wrapper",
  );
});
