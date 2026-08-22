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

async function renderWithI18n(element) {
  const { I18N_CONTEXT } = await server.ssrLoadModule(
    "/src/shared/i18n/i18n-context.ts",
  );
  const { MESSAGES } = await server.ssrLoadModule(
    "/src/shared/i18n/messages.ts",
  );
  return renderToStaticMarkup(
    React.createElement(
      I18N_CONTEXT.Provider,
      {
        value: {
          locale: "zh",
          setLocale: () => {},
          t: (key) => MESSAGES.zh[key] ?? key,
        },
      },
      element,
    ),
  );
}

test("conversation WorkGraph result renders one compact current-sketch card", async () => {
  const { WorkGraphArtifactBlock } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/blocks/artifact/workgraph/workgraph-artifact-block.tsx",
  );
  const markup = await renderWithI18n(React.createElement(WorkGraphArtifactBlock, {
    artifact: {
      type: "workgraph_artifact",
      state: "draft",
      operation: "revise_workgraph_preview",
      head_revision: 3,
      selected_revision: 2,
      version_count: 3,
      preview: {
        preview_id: "preview-1",
        slash_name: "briefing",
        title: "协作简报",
        description: "把已完成流程抽象成可复用工作图",
        source_execution_id: "execution-1",
        source_session_key: "room:conversation-1",
        objective: "形成简报",
        nodes: [{
          logical_key: "draft",
          role: "key",
          kind: "produce",
          subject: "起草简报",
          objective: "整合来源",
          deliverable: "简报",
          required: true,
          terminal: true,
          position: 0,
        }],
        dependencies: [],
        expires_at: "2026-08-23T00:00:00Z",
      },
    },
  }));

  assert.match(markup, /data-workgraph-artifact="draft"/);
  assert.match(markup, /当前草图/);
  assert.match(markup, /协作简报/);
  assert.match(markup, /来源对照/);
  assert.match(markup, /data-workgraph-sketch-node="draft"/);
  assert.match(markup, />v2</);
});

test("WorkGraph source comparison is a flat canvas pair without explanatory chrome", async () => {
  const source = await readFile(path.join(
    webRoot,
    "src/features/conversation/shared/message/blocks/artifact/workgraph/workgraph-artifact-block.tsx",
  ), "utf8");

  assert.match(source, /<UiDialogCloseButton/);
  assert.doesNotMatch(source, /<UiDialogHeader/);
  assert.doesNotMatch(source, /workflow_artifact_(?:source_description|draft_description)/);
  assert.match(source, /hidden min-h-0 grid-cols-2/);
  assert.match(source, /className="sr-only" id="workgraph-compare-title"/);
});

test("conversation ordering preserves the WorkGraph artifact for final projection", async () => {
  const { buildVisibleOrderedAssistantEntries } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/controller/projection/message-item-ordering.ts",
  );
  const artifact = {
    type: "workgraph_artifact",
    id: "workgraph:tool-1",
    state: "draft",
    operation: "get_workgraph_preview",
    preview: {
      preview_id: "preview-1",
      slash_name: "briefing",
      title: "协作简报",
      source_execution_id: "execution-1",
      source_session_key: "room:conversation-1",
      objective: "形成简报",
      nodes: [{
        logical_key: "draft",
        role: "key",
        kind: "produce",
        subject: "起草简报",
        objective: "整合来源",
        deliverable: "简报",
        required: true,
        terminal: true,
        position: 0,
      }],
      dependencies: [],
      expires_at: "2026-08-23T00:00:00Z",
    },
  };
  const entries = buildVisibleOrderedAssistantEntries({
    hiddenToolNames: new Set(),
    hiddenToolUseIds: new Set(),
    isLoading: false,
    mergedContent: [artifact],
    mergedContentSourceMessageIds: ["assistant-artifact"],
    sourceMessageOrderById: new Map([["assistant-artifact", 0]]),
    systemEventBlocks: [],
  });

  assert.equal(entries.length, 1);
  assert.equal(entries[0].block, artifact);
});

test("WorkGraph delivery stays after the final text and outside process folding", async () => {
  const { resolveMessageItemFinalProjection } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/controller/projection/message-item-final-projection.ts",
  );
  const thinking = { type: "thinking", thinking: "检查草图" };
  const artifact = {
    type: "workgraph_artifact",
    id: "workgraph:tool-1",
    state: "draft",
    operation: "get_workgraph_preview",
    preview: {
      preview_id: "preview-1",
      slash_name: "briefing",
      title: "协作简报",
      source_execution_id: "execution-1",
      source_session_key: "room:conversation-1",
      objective: "形成简报",
      nodes: [{
        logical_key: "draft",
        role: "key",
        kind: "produce",
        subject: "起草简报",
        objective: "整合来源",
        deliverable: "简报",
        required: true,
        terminal: true,
        position: 0,
      }],
      dependencies: [],
      expires_at: "2026-08-23T00:00:00Z",
    },
  };
  const finalText = { type: "text", text: "草图已读取并检查完毕。" };
  const processMessage = {
    role: "assistant",
    message_id: "assistant-process",
    round_id: "round-1",
    parent_id: "tool-1",
    content: [thinking, artifact],
  };
  const finalMessage = {
    role: "assistant",
    message_id: "assistant-final",
    round_id: "round-1",
    parent_id: "user-1",
    content: [finalText],
  };
  const orderedEntries = [
    [thinking, processMessage.message_id],
    [artifact, processMessage.message_id],
    [finalText, finalMessage.message_id],
  ].map(([block, sourceMessageId], mergedIndex) => ({
    block,
    mergedIndex,
    sourceMessageId,
    sourceOrder: mergedIndex,
  }));
  const visibleAssistantTurns = [
    {
      content: processMessage.content,
      messageId: processMessage.message_id,
      streamingIndexes: new Set(),
      textContent: [],
      textStreamingIndexes: new Set(),
    },
    {
      content: finalMessage.content,
      messageId: finalMessage.message_id,
      streamingIndexes: new Set(),
      textContent: [finalText],
      textStreamingIndexes: new Set(),
    },
  ];
  const project = (assistantContentMode) => resolveMessageItemFinalProjection({
    assistantContentMode,
    assistantMessages: [processMessage, finalMessage],
    orderedProjection: {
      content: [thinking, artifact, finalText],
      streamingIndexes: new Set(),
    },
    resultSummary: undefined,
    roundId: "round-1",
    userMessageId: "user-1",
    streamingBlockIndexes: new Set(),
    visibleAssistantTurns,
    visibleOrderedAssistantEntries: orderedEntries,
  });

  const live = project("dm_live");
  const archived = project("dm_archived");
  assert.deepEqual(
    live.directOrderedProjection.content.map((block) => block.type),
    ["thinking"],
  );
  assert.deepEqual(
    archived.processProjection.content.map((block) => block.type),
    ["thinking"],
  );
  assert.deepEqual(
    archived.finalAssistantContent.map((block) => block.type),
    ["text", "workgraph_artifact"],
  );
});
