// @vitest-environment node

import assert from "node:assert/strict";

import { test } from "vitest";

import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import ReactMarkdown from "react-markdown";

test("高速输出追赶且大块终态按固定截止时间排空", async () => {
  const { AdaptiveStreamClock } = await import("@/shared/ui/markdown/streaming/adaptive-stream-clock.ts");
  for (const cps of [36, 300, 3_000]) {
    const clock = new AdaptiveStreamClock(0);
    let backlog = 0;
    let maxBacklog = 0;
    for (let timestamp = 0; timestamp < 3_000; timestamp += 34) {
      const appended = Math.round(cps * 0.034);
      backlog += appended;
      clock.observeAppend(timestamp, appended);
      const frame = clock.resolveFrame({ backlog, frameIntervalMs: 34,
        maxRevealCount: 2048, streaming: true, timestamp });
      assert.ok(frame.revealCount >= 0 && frame.revealCount <= backlog);
      backlog -= frame.revealCount;
      maxBacklog = Math.max(maxBacklog, backlog);
    }
    assert.ok(maxBacklog < Math.max(180, cps * 0.5), `arrival ${cps}/s: backlog ${maxBacklog}`);
  }
  const clock = new AdaptiveStreamClock(0);
  let backlog = 12_000;
  clock.observeAppend(0, backlog);
  const first = clock.resolveFrame({ backlog, frameIntervalMs: 34,
    maxRevealCount: 2048, streaming: false, timestamp: 34 });
  assert.ok(first.revealCount > 4 && first.revealCount < backlog);
  backlog -= first.revealCount;
  for (let timestamp = 68; timestamp <= 578 && backlog; timestamp += 34) {
    backlog -= clock.resolveFrame({ backlog, frameIntervalMs: 34,
      maxRevealCount: 2048, streaming: false, timestamp }).revealCount;
  }
  assert.equal(backlog, 0, "terminal must finish, not restart a sliding drain window");
});

test("公式分隔符兼容保留代码与链接，流式空行不拆公式", async () => {
  const { normalizeMarkdownContent, MARKDOWN_PLUGINS, REHYPE_PLUGINS } = await import("@/shared/ui/markdown/core/markdown-renderer-shared.tsx");
  const { splitStreamingMarkdownBlocks } = await import("@/shared/ui/markdown/streaming/markdown-stream-blocks.ts");
  const normalize = (text) => normalizeMarkdownContent(text, () => null);
  for (const input of [String.raw`\(x^2\)`, String.raw`\[x^2\]`, "$x^2$", "$$\nx^2\n$$"]) {
    const html = renderToStaticMarkup(React.createElement(ReactMarkdown, { remarkPlugins: MARKDOWN_PLUGINS, rehypePlugins: REHYPE_PLUGINS }, normalize(input)));
    assert.match(html, /class="katex"/);
  }
  for (const input of ["`\\(x\\)`", "``\\(x\\)``", "```tex\n\\[x\\]\n```", "~~~tex\n\\[x\\]\n~~~", "```tex\n\\[x\\]", "[link](https://example.com/\\(x\\))", String.raw`\\(literal\\)`]) {
    assert.equal(normalize(input), input);
  }
  const formula = "$$\na+b\n\n+c\n$$\n";
  assert.deepEqual(splitStreamingMarkdownBlocks(formula).map((block) => block.content), [formula]);
  assert.equal(splitStreamingMarkdownBlocks("$$\na+b\n\n+c").length, 1);
  assert.equal(splitStreamingMarkdownBlocks(normalize(String.raw`\[a+b

+c`)).filter((block) => block.content.trim()).length, 1);
  assert.match(normalize("```tex\ncode\n````\n\\(x\\)"), /\\\(x\\\)/);
  const literalFormula = String.raw`\(\mathrm{file.txt}\)`;
  assert.equal(normalizeMarkdownContent(literalFormula, () => "file.txt", () => {}), literalFormula);
});

test("阅读偏好持久化与损坏数据恢复", async () => {
  const { normalizeChatTypography, DEFAULT_CHAT_TYPOGRAPHY, useChatTypography } = await import("@/shared/theme/chat-typography.ts");
  assert.deepEqual(normalizeChatTypography(null), DEFAULT_CHAT_TYPOGRAPHY);
  assert.deepEqual(normalizeChatTypography({ font: 123, fontSize: 99, lineHeight: 0 }), { font: "default", fontSize: 26, lineHeight: 1.2 });
  assert.deepEqual(normalizeChatTypography({ fontSize: NaN, lineHeight: Infinity }), DEFAULT_CHAT_TYPOGRAPHY);
  assert.equal(normalizeChatTypography({ font: "Times New Roman" }).font, "Times New Roman");
  assert.equal(normalizeChatTypography({ font: "PT " }).font, "PT ");
  assert.equal(normalizeChatTypography({ font: "\n" }).font, "default");
  let stored;
  useChatTypography.persist.setOptions({ storage: { getItem: () => stored, setItem: (_key, value) => { stored = value; }, removeItem: () => { stored = undefined; } } });
  useChatTypography.getState().setTypography({ font: "Times New Roman", fontSize: 20, lineHeight: 1.8 });
  const saved = stored;
  useChatTypography.setState({ typography: DEFAULT_CHAT_TYPOGRAPHY });
  stored = saved;
  await useChatTypography.persist.rehydrate();
  assert.deepEqual(useChatTypography.getState().typography, { font: "Times New Roman", fontSize: 20, lineHeight: 1.8 });
});

test("外观恢复的主题默认值跟随系统配色", async () => {
  const { defaultTheme } = await import("@/shared/theme/theme-context.ts");
  const previousWindow = globalThis.window;
  try {
    globalThis.window = { matchMedia: () => ({ matches: true }) };
    assert.equal(defaultTheme(), "dark");
    globalThis.window = { matchMedia: () => ({ matches: false }) };
    assert.equal(defaultTheme(), "light");
  } finally {
    if (previousWindow === undefined) delete globalThis.window;
    else globalThis.window = previousWindow;
  }
});

test("字体目录通过版本化桌面桥接读取且不发送字体文件", async () => {
  const { getDesktopSystemFonts } = await import("@/lib/desktop-bridge/desktop-bridge.ts");
  const previousWindow = globalThis.window;
  let request;
  try {
    globalThis.window = { __NEXUS_DESKTOP_BRIDGE__: { invoke: async (message) => {
      request = message;
      return { families: ["Academy Engraved LET", "Al Bayan", "PT Mono"] };
    } } };
    assert.deepEqual(await getDesktopSystemFonts(), { families: ["Academy Engraved LET", "Al Bayan", "PT Mono"] });
    assert.deepEqual(request, { schema_version: 1, kind: "app.get_system_fonts", payload: {} });
  } finally {
    if (previousWindow === undefined) delete globalThis.window;
    else globalThis.window = previousWindow;
  }
});
