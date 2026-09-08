// INPUT: Web 浅色主题 token 与 Windows 原生主题的显式 ARGB 投影。
// OUTPUT: 当前每个原生语义颜色必须与 Web 真相源一致，否则前端门禁失败。
// POS: 跨平台 token 漂移检查；不证明 WPF 渲染、深色主题或原生窗口已实机验收。

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const tokenURL = new URL("../src/app/styles/theme-tokens.css", import.meta.url);
const nativeURL = new URL("../../desktop/windows/Nexus.Desktop/Theme/NexusNativeTheme.cs", import.meta.url);

// Names map platform APIs to semantic ownership; color values live only in CSS.
const projection = {
  CanvasBrush: { token: "--material-shell-header-background", opaque: true },
  OverlayBrush: { token: "--material-overlay-background" },
  OverlayBorderBrush: { token: "--material-overlay-border" },
  DividerSubtleBrush: { token: "--divider-subtle-color" },
  TextStrongBrush: { token: "--text-strong" },
  TextDefaultBrush: { token: "--text-default" },
  TextMutedBrush: { token: "--text-muted" },
  IconDefaultBrush: { token: "--icon-default" },
  InteractiveHoverBrush: { token: "--interaction-hover-background" },
  InteractiveActiveBrush: { token: "--interaction-active-background" },
  PrimaryBrush: { token: "--primary" },
  FocusBrush: { token: "--ring" },
  BrandActionBrush: { token: "--brand-action" },
  BrandActionHoverBrush: { token: "--brand-action-hover" },
  DestructiveBrush: { token: "--destructive" },
};

function readLightTokens(source) {
  const body = source.match(/:root,\s*:root\[data-theme="light"\]\s*\{([\s\S]*?)\n\}/)?.[1];
  assert.ok(body, "Keep the explicit light theme source identifiable.");
  const result = new Map();
  for (const [, name, value] of body.matchAll(/(--[\w-]+)\s*:\s*([^;]+);/g)) {
    assert.ok(!result.has(name), `Duplicate light token: ${name}`);
    result.set(name, value.trim());
  }
  return result;
}

function argb(value) {
  assert.ok(value, "Missing native color source token.");
  const hex = value.match(/^#([\da-f]{6})$/i);
  if (hex) return [255, ...hex[1].match(/../g).map((part) => parseInt(part, 16))];
  const rgb = value.match(/^rgba?\(\s*(\d+)[ ,]+(\d+)[ ,]+(\d+)(?:\s*,\s*([\d.]+))?\s*\)$/);
  assert.ok(rgb, `Native projection needs an explicit supported RGB value: ${value}`);
  return [Math.round(Number(rgb[4] ?? 1) * 255), ...rgb.slice(1, 4).map(Number)];
}

test("Windows semantic brushes are an exact projection of the current Web light tokens", async () => {
  const [css, native] = await Promise.all([readFile(tokenURL, "utf8"), readFile(nativeURL, "utf8")]);
  const tokens = readLightTokens(css);
  const brushes = new Map([...native.matchAll(/public static SolidColorBrush (\w+) \{ get; \} = CreateBrush\(([^)]+)\);/g)]
    .map(([, name, args]) => [name, args.split(",").map((value) => Number(value.trim()))]));
  assert.deepEqual([...brushes.keys()].sort(), Object.keys(projection).sort(), "Every native brush needs a named Web token owner.");
  for (const [name, { token, opaque }] of Object.entries(projection)) {
    const expected = argb(tokens.get(token));
    // WPF owns the opaque top-level window; preserve the Web header's hue.
    if (opaque) expected[0] = 255;
    assert.deepEqual(brushes.get(name), expected, `${name} drifted from ${token}`);
  }
  const shadow = native.match(/ShadowColor\s*\{ get; \}\s*=\s*System\.Windows\.Media\.Color\.FromRgb\(([^)]+)\)/)?.[1];
  assert.ok(shadow);
  assert.deepEqual(shadow.split(",").map((value) => Number(value.trim())), argb(tokens.get("--shadow-color")).slice(1));
  assert.match(native, /brush\.Freeze\(\)/);
});
