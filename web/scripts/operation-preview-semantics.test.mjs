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
const { resolveOperationWorkspaceFilePath } = await server.ssrLoadModule(
  "/src/features/conversation/operation/operation-workspace-file-path.ts",
);
const {
  appendOperationUserFilePath,
  findOperationWorkspaceWindow,
  mergeOperationUserFileWindows,
} = await server.ssrLoadModule(
  "/src/features/conversation/operation/operation-user-file-windows.ts",
);
const {
  buildFinderSessionView,
  finderResultPaths,
} = await server.ssrLoadModule(
  "/src/features/conversation/operation/apps/finder-session.ts",
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

test("SDK absolute paths resolve only inside the active Agent workspace", () => {
  assert.equal(resolveOperationWorkspaceFilePath({
    path: "/Users/test/.nexus/workspace/Devi/reports/brief.md",
    workspacePath: "/Users/test/.nexus/workspace/Devi",
  }), "reports/brief.md");
  assert.equal(resolveOperationWorkspaceFilePath({
    path: "C:\\Users\\test\\.nexus\\workspace\\Devi\\src\\main.ts",
    workspacePath: "C:\\Users\\test\\.nexus\\workspace\\Devi",
  }), "src/main.ts");
  assert.equal(resolveOperationWorkspaceFilePath({
    knownPaths: ["reports/brief.md"],
    path: "/private/runtime/session/reports/brief.md",
  }), "reports/brief.md");
  assert.equal(resolveOperationWorkspaceFilePath({
    knownPaths: ["report.md", "b/report.md"],
    path: "/private/runtime/session/b/report.md",
  }), null);
  assert.equal(resolveOperationWorkspaceFilePath({
    path: "../outside.txt",
    workspacePath: "/Users/test/.nexus/workspace/Devi",
  }), null);
});

test("Files uses real workspace entries, search, and the current-round change scope", () => {
  const event = fileEvent({
    target: "/tmp/runtime/src/main.ts",
    result_preview: "src/main.ts\nREADME.md\nnot a path summary.",
  });
  const files = [
    { path: "src", name: "src", is_dir: true, modified_at: "2026-07-17T08:00:00Z", depth: 0 },
    { path: "src/main.ts", name: "main.ts", is_dir: false, size: 1200, modified_at: "2026-07-17T08:01:00Z", depth: 1 },
    { path: "README.md", name: "README.md", is_dir: false, size: 90, modified_at: "2026-07-17T08:02:00Z", depth: 0 },
  ];
  const items = [{
    id: "live:main",
    agent_id: event.agent_id,
    event_type: "file_write_end",
    path: "src/main.ts",
    source: "agent",
    status: "updated",
    updated_at: now,
    version: 2,
  }];

  const workspaceView = buildFinderSessionView({ event, files, items });
  assert.equal(workspaceView.item_count, 2);
  assert.equal(workspaceView.selected_path, "src/main.ts");
  assert.equal(workspaceView.selected_entry?.size, 1200);

  const searchView = buildFinderSessionView({ event, files, items, query: "readme" });
  assert.equal(searchView.item_count, 1);
  assert.deepEqual(searchView.rows.map((row) => row.path), ["README.md"]);

  const changesView = buildFinderSessionView({ event, files, items, scope: "changes" });
  assert.ok(changesView.rows.some((row) => row.path === "src" && row.type === "folder"));
  assert.ok(changesView.rows.some((row) => row.path === "src/main.ts"));
  assert.ok(!changesView.rows.some((row) => row.path === "README.md"));
  assert.deepEqual(finderResultPaths(event.result_preview), ["src/main.ts", "README.md"]);
});

test("workspace search results stay in Files until a concrete file is opened", () => {
  const event = fileEvent({
    kind: "workspace_search",
    surface: "workspace",
    target: "web/src/**/*.tsx",
    result_preview: [
      "web/src/dev/operation-stage-preview.tsx",
      "web/src/features/conversation/operation/stage/operation-stage-desktop.tsx",
    ],
  });
  const context = collectOperationFileContext(event, null, [event]);
  const view = buildFinderSessionView({ event, items: [] });

  assert.equal(context.file_documents.length, 0);
  assert.equal(view.changed_count, 0);
  assert.ok(!view.rows.some((row) => row.path.includes("**")));
  assert.deepEqual(
    view.rows.filter((row) => row.type === "file").map((row) => row.path),
    event.result_preview,
  );
});

test("Files opens one truthful preview window per workspace path", () => {
  const event = fileEvent();
  const plannedWindow = {
    id: "planned:brief",
    kind: "markdown_reader",
    title: "brief.md",
    target: "reports/brief.md",
    phase: "background",
    z: 10,
    layout: "primary",
    payload: { event, snapshot: null, target: "reports/brief.md", workspace_preview: true },
  };
  const openedPaths = appendOperationUserFilePath(
    appendOperationUserFilePath([], "games/gomoku.html"),
    "games/gomoku.html",
  );
  openedPaths.push("reports/brief.md", "reports/final.pdf");
  const windows = mergeOperationUserFileWindows({
    event,
    openedPaths,
    plannedWindows: [plannedWindow],
    snapshot: null,
  });

  assert.equal(windows.length, 3);
  assert.equal(windows.filter((window) => window.payload.target === "reports/brief.md").length, 1);
  assert.equal(findOperationWorkspaceWindow(windows, "/tmp/reports/brief.md")?.id, plannedWindow.id);
  assert.equal(findOperationWorkspaceWindow(windows, "games/gomoku.html")?.kind, "browser");
  assert.equal(findOperationWorkspaceWindow(windows, "reports/final.pdf")?.kind, "pdf_reader");
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
