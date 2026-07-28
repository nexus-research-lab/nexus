import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

import { createElement } from "react";
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

test("Safari uses the static liquid-glass material", async () => {
  const { canUseTrueLiquidGlass, isSafariUserAgent } =
    await server.ssrLoadModule(
      "/src/shared/ui/liquid-glass/liquid-glass-engine.ts",
    );
  const safariUserAgent =
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
    + "AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.5 Safari/605.1.15";

  assert.equal(isSafariUserAgent(safariUserAgent), true);
  assert.equal(
    canUseTrueLiquidGlass({
      prefersReducedMotion: false,
      saveData: false,
      supportsBackdrop: true,
      userAgent: safariUserAgent,
    }),
    false,
    "Safari must not mount an SVG backdrop-filter that can leave stale pixels after unmount",
  );
});

test("the static switch uses one non-composited thumb", async () => {
  const { GlassSwitch } = await server.ssrLoadModule(
    "/src/shared/ui/liquid-glass/glass-switch.tsx",
  );
  const renderSwitch = (checked) => renderToStaticMarkup(
    createElement(GlassSwitch, {
      checked,
      onChange: () => {},
      size: "xs",
    }),
  );
  const uncheckedMarkup = renderSwitch(false);
  const checkedMarkup = renderSwitch(true);
  const leftPosition = (markup) => markup.match(/left:([^;"]+)/)?.[1];

  assert.equal(
    (uncheckedMarkup.match(/data-liquid-glass-material="static"/g) ?? []).length,
    1,
  );
  assert.equal((uncheckedMarkup.match(/aria-hidden="true"/g) ?? []).length, 1);
  assert.doesNotMatch(
    uncheckedMarkup,
    /backdrop-filter|opacity:|transform:|will-change-transform/,
  );
  assert.notEqual(leftPosition(uncheckedMarkup), leftPosition(checkedMarkup));
});

test("capable Chromium browsers retain the SVG liquid-glass material", async () => {
  const { canUseTrueLiquidGlass, isSafariUserAgent } =
    await server.ssrLoadModule(
      "/src/shared/ui/liquid-glass/liquid-glass-engine.ts",
    );
  const chromeUserAgent =
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
    + "AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36";

  assert.equal(isSafariUserAgent(chromeUserAgent), false);
  assert.equal(
    canUseTrueLiquidGlass({
      prefersReducedMotion: false,
      saveData: false,
      supportsBackdrop: true,
      userAgent: chromeUserAgent,
    }),
    true,
  );
});

test("resource and accessibility limits keep the static material", async () => {
  const { canUseTrueLiquidGlass } = await server.ssrLoadModule(
    "/src/shared/ui/liquid-glass/liquid-glass-engine.ts",
  );
  const baseCapability = {
    prefersReducedMotion: false,
    saveData: false,
    supportsBackdrop: true,
    userAgent:
      "Mozilla/5.0 Chrome/138.0.0.0 Safari/537.36",
  };

  assert.equal(
    canUseTrueLiquidGlass({
      ...baseCapability,
      prefersReducedMotion: true,
    }),
    false,
  );
  assert.equal(
    canUseTrueLiquidGlass({ ...baseCapability, saveData: true }),
    false,
  );
  assert.equal(
    canUseTrueLiquidGlass({ ...baseCapability, supportsBackdrop: false }),
    false,
  );
  assert.equal(
    canUseTrueLiquidGlass({
      ...baseCapability,
      userAgent: "Mozilla/5.0 Firefox/140.0",
    }),
    false,
  );
});
