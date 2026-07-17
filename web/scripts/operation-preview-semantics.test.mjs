import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

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

const {
  collectOperationFileContext,
  windowKindForFileTarget,
} = await server.ssrLoadModule(
  "/src/features/conversation/operation/operation-file-documents.ts",
);
const { deriveStageDesktopIntents } = await server.ssrLoadModule(
  "/src/features/conversation/operation/operation-desktop-intents.ts",
);
const { getWorkspaceFilePreviewKind } = await server.ssrLoadModule(
  "/src/features/conversation/shared/editor/workspace-file-preview-kind.ts",
);

const now = Date.now();

function fileEvent(overrides = {}) {
  return {
    id: "read-report",
    session_key: "session:preview",
    round_id: "round:preview",
    agent_id: "agent:preview",
    message_id: "message:preview",
    tool_use_id: "tool:read-report",
    tool_name: "Read",
    kind: "workspace_read",
    surface: "workspace",
    phase: "done",
    title: "读取文件",
    target: "reports/brief.md",
    result_preview: "# Brief",
    started_at: now,
    updated_at: now,
    ...overrides,
  };
}

test("absolute tool paths and relative workspace paths become one document", () => {
  const event = fileEvent({
    target: "/Users/test/.nexus/workspace/agent/reports/brief.md",
  });
  const workspaceItem = {
    id: "workspace:brief",
    agent_id: event.agent_id,
    path: "reports/brief.md",
    status: "updated",
    version: 1,
    source: "agent",
    session_key: event.session_key,
    tool_use_id: event.tool_use_id,
    event_type: "file_write_end",
    live_content: "# Canonical brief",
    updated_at: now,
  };
  const context = collectOperationFileContext(event, {
    key: event.session_key,
    session_key: event.session_key,
    active_event: event,
    events: [event],
    recent_evidence: [],
    workspace_events: [workspaceItem],
    updated_at: now,
  }, [event]);

  assert.equal(context.file_documents.length, 1);
  assert.equal(context.file_documents[0].target, "reports/brief.md");
  assert.equal(context.file_documents[0].workspace_item.id, workspaceItem.id);
});

test("previewable files route to truthful app kinds and shared renderers", () => {
  const cases = [
    ["README.mdx", "markdown_reader", "markdown"],
    ["report.docx", "word_reader", "document"],
    ["budget.xlsx", "spreadsheet", "spreadsheet"],
    ["deck.pptx", "presentation", "presentation"],
    ["paper.pdf", "pdf_reader", "pdf"],
    ["photo.tiff", "image_viewer", "image"],
    ["src/main.cpp", "code_editor", "text"],
    [".env.local", "code_editor", "text"],
  ];
  for (const [target, windowKind, previewKind] of cases) {
    assert.equal(windowKindForFileTarget(target), windowKind, target);
    assert.equal(getWorkspaceFilePreviewKind(target), previewKind, target);
  }
});

test("open HTML enters Navi while other files enter the preview app", () => {
  const htmlIntents = deriveStageDesktopIntents(fileEvent({
    id: "open-html",
    tool_use_id: "tool:open-html",
    tool_name: "Bash",
    kind: "command_run",
    surface: "terminal",
    target: "open games/gomoku.html",
    input_preview: { command: "open games/gomoku.html" },
  }));
  assert.ok(htmlIntents.some((intent) => intent.app === "browser" && intent.target === "games/gomoku.html"));
  assert.ok(!htmlIntents.some((intent) => intent.app === "preview"));

  const documentIntents = deriveStageDesktopIntents(fileEvent({
    id: "open-docx",
    tool_use_id: "tool:open-docx",
    tool_name: "Bash",
    kind: "command_run",
    surface: "terminal",
    target: "open reports/brief.docx",
    input_preview: { command: "open reports/brief.docx" },
  }));
  assert.ok(documentIntents.some((intent) => intent.app === "preview" && intent.target === "reports/brief.docx"));
  assert.ok(!documentIntents.some((intent) => intent.app === "browser"));
});
