import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

import { createServer } from "vite";

const { createElement } = await import("react");
const { renderToStaticMarkup } = await import("react-dom/server");

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
const {
  deriveStageDesktopIntents,
  operationEventFromRuntimeEvent,
} = await server.ssrLoadModule(
  "/src/features/conversation/operation/operation-desktop-intents.ts",
);
const { planOperationDesktop } = await server.ssrLoadModule(
  "/src/features/conversation/operation/operation-scene-planner.ts",
);
const { resolveOperationImageSource } = await server.ssrLoadModule(
  "/src/features/conversation/operation/operation-image-source.ts",
);
const { StageWindowContent } = await server.ssrLoadModule(
  "/src/features/conversation/operation/apps/operation-app-renderers.tsx",
);
const { appSurfaceForWindowKind } = await server.ssrLoadModule(
  "/src/features/conversation/operation/apps/operation-app-surface-policy.ts",
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
  operationUserFileWindowId,
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

test("artifact status labels do not become files or duplicate document windows", () => {
  const event = fileEvent({
    kind: "workspace_edit",
    surface: "editor",
    target: "gomoku.html",
    tool_name: "Write",
    input_preview: {
      file_path: "gomoku.html",
      content: "<!doctype html><title>Gomoku</title>",
    },
    evidence: [
      { type: "file", label: "创建", value: "gomoku.html" },
      { type: "artifact", label: "HTML", value: "内嵌预览已准备" },
    ],
  });
  const workspaceItem = {
    id: "workspace:gomoku",
    agent_id: event.agent_id,
    path: "gomoku.html",
    status: "updated",
    version: 1,
    source: "agent",
    session_key: event.session_key,
    tool_use_id: event.tool_use_id,
    event_type: "file_write_end",
    live_content: event.input_preview.content,
    updated_at: now,
  };
  const snapshot = {
    key: event.session_key,
    session_key: event.session_key,
    active_event: event,
    events: [event],
    recent_evidence: event.evidence,
    workspace_events: [workspaceItem],
    updated_at: now,
  };

  const context = collectOperationFileContext(event, snapshot, [event]);
  const desktop = planOperationDesktop({ event, snapshot });

  assert.deepEqual(context.file_documents.map((document) => document.target), ["gomoku.html"]);
  assert.equal(new Set(desktop.windows.map((window) => window.id)).size, desktop.windows.length);
  assert.ok(!desktop.windows.some((window) => window.title === "内嵌预览已准备"));
});

test("multi-file edits assign one stable window identity per file", () => {
  const event = fileEvent({
    id: "patch-two-files",
    kind: "workspace_edit",
    surface: "editor",
    target: "src/app.ts",
    tool_name: "Edit",
    input_preview: {
      edits: [
        { path: "src/app.ts", old_text: "idle", new_text: "ready" },
        { path: "src/state.ts", old_text: "idle", new_text: "ready" },
      ],
    },
  });
  const workspaceEvents = ["src/app.ts", "src/state.ts"].map((workspacePath, index) => ({
    id: `workspace:${index}`,
    agent_id: event.agent_id,
    path: workspacePath,
    status: "updated",
    version: 1,
    source: "agent",
    session_key: event.session_key,
    tool_use_id: event.tool_use_id,
    event_type: "file_write_end",
    live_content: "ready",
    updated_at: now + index,
  }));
  const snapshot = {
    key: event.session_key,
    session_key: event.session_key,
    active_event: event,
    events: [event],
    recent_evidence: [],
    workspace_events: workspaceEvents,
    updated_at: now,
  };

  const desktop = planOperationDesktop({ event, snapshot });
  const documentWindows = desktop.windows.filter((window) => window.kind === "code_editor");

  assert.equal(documentWindows.length, 2);
  assert.equal(new Set(documentWindows.map((window) => window.id)).size, 2);
  assert.deepEqual(
    documentWindows.map((window) => window.payload.target).sort(),
    ["src/app.ts", "src/state.ts"],
  );
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
    agentId: "a39a2dd5fd86",
    path: "/Users/test/.nexus-dev/instances/operation-stage/workspace/a39a2dd5fd86/gobang.py",
  }), "gobang.py");
  assert.equal(resolveOperationWorkspaceFilePath({
    agentId: "another-agent",
    path: "/Users/test/.nexus-dev/instances/operation-stage/workspace/a39a2dd5fd86/gobang.py",
  }), null);
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

test("generic tool output cannot fabricate workspace file windows", () => {
  const bashEvent = fileEvent({
    id: "build-json",
    kind: "command_run",
    result_preview: "Created dist/report.json",
    surface: "terminal",
    target: "pnpm build",
    tool_name: "Bash",
  });
  const webEvent = fileEvent({
    id: "fetch-page",
    kind: "web_research",
    result_preview: "The page loads app.js and package.json.",
    surface: "web",
    target: "https://example.com",
    tool_name: "WebFetch",
  });
  const taskEvent = fileEvent({
    id: "task-output",
    kind: "task_progress",
    result_preview: "Finished package.json analysis.",
    surface: "task",
    target: "task-1",
    tool_name: "TaskOutput",
  });
  for (const event of [bashEvent, webEvent, taskEvent]) {
    assert.equal(collectOperationFileContext(event, null, [event]).file_documents.length, 0, event.tool_name);
  }
});

test("structured file evidence opens the exact artifact path", () => {
  const event = fileEvent({
    evidence: [{
      label: "created",
      preview: "# 总结",
      type: "artifact",
      value: "报告/总结.md",
    }],
    id: "generated-report",
    kind: "command_run",
    result_preview: "Command completed successfully.",
    surface: "terminal",
    target: "generate report",
    tool_name: "Bash",
  });
  const context = collectOperationFileContext(event, null, [event]);

  assert.equal(context.file_documents.length, 1);
  assert.equal(context.file_documents[0].target, "报告/总结.md");
  assert.equal(context.file_documents[0].preview, "# 总结");

  const remoteEvidenceEvent = fileEvent({
    evidence: [{
      label: "remote image",
      type: "artifact",
      value: "https://example.com/report.png",
    }],
    id: "remote-artifact",
    kind: "command_run",
    surface: "terminal",
    target: "download report",
    tool_name: "Bash",
  });
  assert.equal(collectOperationFileContext(remoteEvidenceEvent, null, [remoteEvidenceEvent]).file_documents.length, 0);
});

test("ViewImage classifies every exact SDK source without inventing workspace files", () => {
  const projectedEvent = operationEventFromRuntimeEvent({
    agent_id: "agent:preview",
    event_type: "tool_start",
    id: "runtime:view-image",
    input: { source: "assets/logo.png", question: "Check the layout" },
    message_id: "message:preview",
    phase: "running",
    round_id: "round:preview",
    session_key: "session:preview",
    timestamp: now,
    tool_name: "ViewImage",
    tool_use_id: "tool:view-image",
  });
  assert.equal(projectedEvent.target, "assets/logo.png");
  assert.equal(projectedEvent.kind, "workspace_read");
  assert.equal(resolveOperationImageSource(projectedEvent)?.kind, "workspace");

  const localEvent = fileEvent({
    id: "view-local-image",
    input_preview: { source: "assets/logo.png", question: "Check the layout" },
    target: "assets/logo.png",
    tool_name: "ViewImage",
  });
  const localDesktop = planOperationDesktop({ event: localEvent, snapshot: null });
  assert.ok(localDesktop.windows.some((window) => (
    window.kind === "image_viewer" &&
    window.payload.target === "assets/logo.png" &&
    window.payload.workspace_preview
  )));

  const remoteEvent = fileEvent({
    id: "view-remote-image",
    input_preview: { source: "https://example.com/logo.png" },
    target: "https://example.com/logo.png",
    tool_name: "ViewImage",
  });
  const remoteContext = collectOperationFileContext(remoteEvent, null, [remoteEvent]);
  const remoteDesktop = planOperationDesktop({ event: remoteEvent, snapshot: null });
  assert.equal(remoteContext.file_documents.length, 0);
  assert.equal(resolveOperationImageSource(remoteEvent)?.kind, "remote");
  assert.ok(remoteDesktop.windows.some((window) => (
    window.kind === "image_viewer" &&
    window.payload.image_source_kind === "remote" &&
    window.payload.image_source === "https://example.com/logo.png"
  )));

  const inlineEvent = fileEvent({
    id: "view-inline-image",
    input_preview: { source: "data:image/png;base64,iVBORw0KGgo=" },
    result_preview: "A small blue icon.",
    target: "data:image/png;base64,iVBORw0KGgo=",
    tool_name: "ViewImage",
  });
  const inlineDesktop = planOperationDesktop({ event: inlineEvent, snapshot: null });
  assert.equal(resolveOperationImageSource(inlineEvent)?.kind, "inline");
  assert.equal(inlineDesktop.windows[0]?.payload.image_source_kind, "inline");
  assert.ok(!inlineDesktop.windows[0]?.title.includes("base64"));

  const attachmentEvent = fileEvent({
    id: "view-attachment-image",
    input_preview: { source: "nexus-image://reference-1" },
    result_preview: "The screenshot shows a terminal window.",
    target: "nexus-image://reference-1",
    tool_name: "ViewImage",
  });
  const attachmentDesktop = planOperationDesktop({ event: attachmentEvent, snapshot: null });
  assert.equal(resolveOperationImageSource(attachmentEvent)?.kind, "attachment");
  assert.equal(attachmentDesktop.windows[0]?.payload.image_source_kind, "attachment");
  assert.equal(attachmentDesktop.windows[0]?.title, "会话图片");
  const attachmentMarkup = renderToStaticMarkup(createElement(StageWindowContent, {
    window: attachmentDesktop.windows[0],
  }));
  assert.match(attachmentMarkup, /data-stage-image-inspection/);
  assert.match(attachmentMarkup, /data-stage-image-analysis/);
  assert.match(attachmentMarkup, /The screenshot shows a terminal window\./);
  assert.doesNotMatch(attachmentMarkup, /&quot;The screenshot/);
  assert.doesNotMatch(attachmentMarkup, /无法打开工作区外的文件/);
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
  const htmlWindow = findOperationWorkspaceWindow(windows, "games/gomoku.html");
  assert.equal(htmlWindow?.kind, "browser");
  assert.equal(htmlWindow?.id, operationUserFileWindowId(event.round_id, "games/gomoku.html"));
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
    ["table.csv", "code_editor", "text"],
    ["table.tsv", "code_editor", "text"],
    ["site.html", "code_editor", "html"],
    ["diagram.mmd", "file_preview", "mermaid"],
    ["legacy.doc", "file_preview", "binary"],
    ["legacy.xls", "file_preview", "binary"],
    ["legacy.ppt", "file_preview", "binary"],
    ["src/main.cpp", "code_editor", "text"],
    [".env.local", "code_editor", "text"],
  ];
  for (const [target, windowKind, previewKind] of cases) {
    assert.equal(windowKindForFileTarget(target), windowKind, target);
    assert.equal(getWorkspaceFilePreviewKind(target), previewKind, target);
  }
});

test("file tool events open the real shared preview surface for every supported family", () => {
  const cases = [
    ["README.md", "markdown_reader"],
    ["report.docx", "word_reader"],
    ["budget.xlsx", "spreadsheet"],
    ["deck.pptx", "presentation"],
    ["paper.pdf", "pdf_reader"],
    ["photo.png", "image_viewer"],
    ["src/main.ts", "code_editor"],
    ["legacy.doc", "file_preview"],
  ];

  for (const [target, expectedKind] of cases) {
    const event = fileEvent({ id: `read:${target}`, target });
    const desktop = planOperationDesktop({ event, snapshot: null });
    const previewWindow = desktop.windows.find((window) => window.payload.target === target);

    assert.equal(previewWindow?.kind, expectedKind, target);
    assert.equal(previewWindow?.payload.workspace_preview, true, target);
    assert.equal(appSurfaceForWindowKind(expectedKind), "document", target);
  }
});

test("file_preview reaches the shared binary renderer", () => {
  const event = fileEvent({ id: "read:legacy.doc", target: "legacy.doc" });
  const desktop = planOperationDesktop({ event, snapshot: null });
  const previewWindow = desktop.windows.find((window) => window.kind === "file_preview");

  assert.ok(previewWindow);
  const markup = renderToStaticMarkup(createElement(StageWindowContent, { window: previewWindow }));
  assert.match(markup, /此文件类型不支持预览/);
  assert.match(markup, /legacy\.doc/);
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
