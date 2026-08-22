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
  assert.match(markup, /与来源图对照/);
  assert.match(markup, /data-workgraph-sketch-node="draft"/);
  assert.match(markup, />v2</);
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
