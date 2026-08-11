import assert from "node:assert/strict";
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

test.after(async () => server.close());

test("生成式 UI 流式空壳不执行模型脚本，完成文档才包含完整代码", async () => {
  const {
    buildGenerativeUIFinalDocument,
    buildGenerativeUIShellDocument,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/blocks/tool/generative-ui-document.ts",
  );
  const widget = `<button>运行</button><script>window.widgetReady = true;</script>`;
  const shell = buildGenerativeUIShellDocument(false);
  const finalDocument = buildGenerativeUIFinalDocument(widget, true);

  assert.match(shell, /morphdom@2\.7\.4/);
  assert.match(shell, /id="nexus-widget-root" inert/);
  assert.doesNotMatch(shell, /window\.widgetReady/);
  assert.match(finalDocument, /window\.widgetReady/);
  assert.match(finalDocument, /--nexus-background: #0e151f/);
});

test("show_widget 在 DM live 中保持独立内容段", async () => {
  const { projectDmToolRunSegments } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/process/dm-tool-run-segments.ts",
  );
  const widget = {
    type: "tool_use",
    id: "widget-1",
    name: "mcp__nexus_visualize__show_widget",
    input: { title: "曲线", widget_code: "<svg />" },
  };
  const [segment] = projectDmToolRunSegments({
    interactiveToolUseIds: new Set(),
    live: true,
    projection: { content: [widget], streamingIndexes: new Set([0]) },
    responseResumed: false,
  });

  assert.equal(segment.kind, "content");
  assert.equal(segment.id, "interactive-tool:widget-1");
});

test("show_widget 工具块渲染为仅允许脚本的 iframe", async () => {
  const { ContentToolBlock } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/view/content/content-tool-block.tsx",
  );
  const { THEME_CONTEXT } = await server.ssrLoadModule(
    "/src/shared/theme/theme-context.ts",
  );
  const toolUse = {
    type: "tool_use",
    id: "widget-render-1",
    name: "mcp__nexus_visualize__show_widget",
    input: { title: "曲线", widget_code: "<svg />" },
  };
  const result = {
    type: "tool_result",
    tool_use_id: toolUse.id,
    content: '{"rendered":true}',
  };
  const markup = renderToStaticMarkup(
    React.createElement(
      THEME_CONTEXT.Provider,
      { value: { theme: "light", setTheme: () => undefined } },
      React.createElement(ContentToolBlock, {
        block: toolUse,
        context: {
          canRespondToPermissions: false,
          pendingInteractionOwner: "composer",
          projection: {
            consumedBlockIndexes: new Set([1]),
            resolvedToolUseIds: new Set([toolUse.id]),
            taskProgressByToolUseId: new Map(),
            toolUseById: new Map([[toolUse.id, {
              index: 0,
              result,
              use: toolUse,
            }]]),
          },
        },
      }),
    ),
  );

  assert.match(markup, /data-generative-ui="true"/);
  assert.match(markup, /sandbox="allow-scripts"/);
  assert.doesNotMatch(markup, /allow-same-origin/);
});
