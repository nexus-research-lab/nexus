// @vitest-environment node

import assert from "node:assert/strict";

import { test } from "vitest";

import React from "react";
import { renderToStaticMarkup } from "react-dom/server";

test("生成式 UI 外壳包含挂载节点和语法有效的桥接脚本", async () => {
  const { buildGenerativeUIShellDocument } = await import("@/features/conversation/shared/message/blocks/tool/generative-ui-document.ts");
  const { JSDOM } = await import("jsdom");
  const dom = new JSDOM(buildGenerativeUIShellDocument("light"));
  try {
    const document = dom.window.document;
    assert.ok(document.getElementById("nexus-widget-root"));
    const bridge = document.querySelector("script:not([src])")?.textContent;
    assert.ok(bridge);
    assert.doesNotThrow(() => new Function(bridge));
  } finally {
    dom.window.close();
  }
});

test("show_widget 高度在流式阶段不回缩并只在终态结算一次", async () => {
  const { resolveGenerativeUIHeightRevision } = await import("@/features/conversation/shared/message/blocks/tool/generative-ui-height-model.ts");
  assert.deepEqual(
    resolveGenerativeUIHeightRevision(420, 260, false),
    { height: 420, settle: false },
  );
  assert.deepEqual(
    resolveGenerativeUIHeightRevision(420, 560, false),
    { height: 560, settle: false },
  );
  assert.deepEqual(
    resolveGenerativeUIHeightRevision(420, 260, true),
    { height: 260, settle: true },
  );
  assert.deepEqual(
    resolveGenerativeUIHeightRevision(420, 560, true),
    { height: 560, settle: false },
  );
});

test("show_widget 从工具过程提升到最终回复", async () => {
  const { resolveMessageItemFinalProjection } = await import("@/features/conversation/shared/message/item/controller/projection/message-item-final-projection.ts");
  const widget = {
    type: "tool_use",
    id: "widget-1",
    name: "mcp__nexus__show_widget",
    input: { title: "曲线", widget_code: "<svg />" },
  };
  const result = {
    type: "tool_result",
    tool_use_id: widget.id,
    content: '{"accepted":true}',
  };
  const finalText = { type: "text", text: "可以拖动年份查看变化。" };
  const hiddenThinking = { type: "thinking", thinking: "整理最终回复" };
  const toolMessage = {
    role: "assistant",
    message_id: "assistant-widget",
    parent_id: "user-1",
    content: [widget, result],
  };
  const finalMessage = {
    role: "assistant",
    message_id: "assistant-final",
    parent_id: "user-1",
    content: [hiddenThinking, finalText],
    agent_mentions: [{
      agent_id: "agent-2",
      content_block_index: 1,
      end_rune: 2,
      label: "同事",
      start_rune: 0,
    }],
  };
  const orderedEntries = [
    [widget, toolMessage.message_id],
    [result, toolMessage.message_id],
    [finalText, finalMessage.message_id],
  ].map(([block, sourceMessageId], mergedIndex) => ({
    block,
    mergedIndex,
    sourceMessageId,
    sourceOrder: mergedIndex,
  }));
  const projection = resolveMessageItemFinalProjection({
    assistantContentMode: "dm_live",
    assistantMessages: [toolMessage, finalMessage],
    orderedProjection: {
      content: [widget, result, finalText],
      streamingIndexes: new Set([2]),
    },
    resultSummary: undefined,
    roundId: "round-1",
    userMessageId: "user-1",
    streamingBlockIndexes: new Set([2]),
    visibleAssistantTurns: [
      {
        content: [widget, result],
        messageId: toolMessage.message_id,
        streamingIndexes: new Set(),
        textContent: [],
        textStreamingIndexes: new Set(),
      },
      {
        content: [finalText],
        messageId: finalMessage.message_id,
        streamingIndexes: new Set([0]),
        textContent: [finalText],
        textStreamingIndexes: new Set([0]),
      },
    ],
    visibleOrderedAssistantEntries: orderedEntries,
  });

  assert.deepEqual(
    projection.finalAssistantContent.map((block) => block.type),
    ["tool_use", "tool_result", "text"],
  );
  assert.deepEqual(projection.directOrderedProjection.content, []);
  assert.deepEqual([...projection.finalAssistantStreamingIndexes], [2]);
  assert.equal(projection.finalAssistantMentions[0].content_block_index, 2);
});

test("show_widget 工具块渲染为仅允许脚本的 iframe", async () => {
  const { ContentToolBlock } = await import("@/features/conversation/shared/message/item/view/content/content-tool-block.tsx");
  const { THEME_CONTEXT } = await import("@/shared/theme/theme-context.ts");
  const { I18N_CONTEXT } = await import("@/shared/i18n/i18n-context.ts");
  const { MESSAGES } = await import("@/shared/i18n/messages.ts");
  const toolUse = {
    type: "tool_use",
    id: "widget-render-1",
    name: "mcp__nexus__show_widget",
    input: { title: "曲线", widget_code: "<svg />" },
  };
  const result = {
    type: "tool_result",
    tool_use_id: toolUse.id,
    content: '{"accepted":true}',
  };
  const markup = renderToStaticMarkup(
    React.createElement(
      I18N_CONTEXT.Provider,
      {
        value: {
          locale: "zh",
          setLocale: () => undefined,
          t: (key) => MESSAGES.zh[key] ?? key,
        },
      },
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
    ),
  );

  assert.match(markup, /data-generative-ui="true"/);
  assert.match(markup, /data-generative-ui-status="loading"/);
  assert.match(markup, /sandbox="allow-scripts"/);



  assert.doesNotMatch(markup, /allow-same-origin/);
});
