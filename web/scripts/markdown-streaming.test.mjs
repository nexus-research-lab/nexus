import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import ReactMarkdown from "react-markdown";

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

test("流式 Markdown 保持空行分隔的相邻有序列表项为一个语义块", async () => {
  const { splitStreamingMarkdownBlocks } = await server.ssrLoadModule(
    "/src/shared/ui/markdown/streaming/markdown-stream-blocks.ts",
  );
  const content = [
    "结果如下：",
    "",
    "1. **第一项**",
    "   摘要一",
    "",
    "2. **第二项**",
    "   摘要二",
    "",
    "3. **第三项**",
    "   摘要三",
  ].join("\n");

  const blocks = splitStreamingMarkdownBlocks(content);

  assert.equal(blocks.length, 2);
  assert.equal(blocks[0].content, "结果如下：\n\n");
  assert.equal(blocks[1].start_offset, "结果如下：\n\n".length);
  assert.match(blocks[1].content, /^1\./);
  assert.match(blocks[1].content, /\n2\./);
  assert.match(blocks[1].content, /\n3\./);
});

test("Markdown 有序列表透传非默认起始序号", async () => {
  const { createMarkdownComponents } = await server.ssrLoadModule(
    "/src/shared/ui/markdown/core/markdown-components.tsx",
  );
  const components = createMarkdownComponents(() => null);
  const html = renderToStaticMarkup(
    React.createElement(
      ReactMarkdown,
      { components },
      "4. 第四项",
    ),
  );

  assert.match(html, /<ol[^>]*start="4"/);
  assert.match(html, />第四项</);
});

test("Markdown 中文语义加粗使用可辨识的 600 字重", async () => {
  const {
    MARKDOWN_PLUGINS,
    MARKDOWN_SUMMARY_CLASS_NAME,
  } = await server.ssrLoadModule(
    "/src/shared/ui/markdown/core/markdown-renderer-shared.tsx",
  );
  const html = renderToStaticMarkup(
    React.createElement(
      ReactMarkdown,
      { remarkPlugins: MARKDOWN_PLUGINS },
      "普通 **重点** 正文",
    ),
  );
  const styles = await readFile(
    path.join(webRoot, "src/app/styles/theme-recipes.css"),
    "utf8",
  );

  assert.match(
    html,
    /<strong><span class="message-cjk-text">重点<\/span><\/strong>/,
  );
  assert.match(MARKDOWN_SUMMARY_CLASS_NAME, /\[&_strong\]:font-semibold/);
  assert.match(
    styles,
    /\.nexus-chat-feed \.nexus-chat-markdown strong \.message-cjk-text\s*\{[^}]*font-synthesis: weight style;[^}]*font-weight: 600;/,
  );
});

test("流式 Markdown 只平滑内容节奏，不叠加正文视觉动画", async () => {
  const [sharedEntry, messageEntry, rendererSource] = await Promise.all([
    readFile(
      path.join(
        webRoot,
        "src/shared/ui/markdown/markdown-content.tsx",
      ),
      "utf8",
    ),
    readFile(
      path.join(
        webRoot,
        "src/features/conversation/shared/message/markdown-renderer.tsx",
      ),
      "utf8",
    ),
    readFile(
      path.join(
        webRoot,
        "src/shared/ui/markdown/streaming/markdown-streaming.tsx",
      ),
      "utf8",
    ),
  ]);
  const sources = [sharedEntry, messageEntry, rendererSource].join("\n");
  assert.doesNotMatch(
    sources,
    /nexus-streaming-response|streaming-word-fade|fade-in-0|streaming-content-tail/,
    "正文不得叠加透明度淡入、尾光标或相关生命周期",
  );
  assert.match(rendererSource, /useDeferredValue\(nextBlocks\)/);
});

test("共享 Markdown 操作与 Mermaid 状态跟随界面语言", async () => {
  const { MESSAGES } = await server.ssrLoadModule("/src/shared/i18n/messages.ts");
  assert.equal(MESSAGES.zh["markdown.code.copy"], "复制 {language} 代码");
  assert.equal(MESSAGES.en["markdown.code.copy"], "Copy {language} code");
  assert.equal(MESSAGES.zh["markdown.mermaid.preview"], "预览");
  assert.equal(MESSAGES.en["markdown.mermaid.preview"], "Preview");
  assert.equal(MESSAGES.en["markdown.mermaid.render_failed"], "Failed to render Mermaid");

  const sharedMarkdownSources = await Promise.all([
    "src/shared/ui/markdown/code/code-block-content.tsx",
    "src/shared/ui/markdown/code/streaming-code-block.tsx",
    "src/shared/ui/markdown/mermaid/lazy-mermaid-view.tsx",
    "src/shared/ui/markdown/mermaid/mermaid-preview-dialog.tsx",
    "src/shared/ui/markdown/mermaid/mermaid-view-parts.tsx",
    "src/shared/ui/markdown/mermaid/mermaid-view.tsx",
  ].map((file) => readFile(path.join(webRoot, file), "utf8")));
  sharedMarkdownSources.forEach((source) => {
    assert.match(source, /useI18n/);
    assert.doesNotMatch(
      source,
      /已复制|复制源码|输出中|预览 Mermaid|等待完整图表|正在渲染图表|暂无图表预览|更新中/,
    );
  });
});
