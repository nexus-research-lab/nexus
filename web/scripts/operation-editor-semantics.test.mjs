import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

import { createElement } from "react";
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

const { buildEditorSessionView } = await server.ssrLoadModule(
  "/src/features/conversation/operation/apps/editor-session-model.ts",
);
const { EditorActivityBar } = await server.ssrLoadModule(
  "/src/features/conversation/operation/apps/editor-activity-bar.tsx",
);
const { collectOperationFileContext } = await server.ssrLoadModule(
  "/src/features/conversation/operation/operation-file-documents.ts",
);
const { resolveOperationToolProfile } = await server.ssrLoadModule(
  "/src/features/conversation/operation/operation-tool-catalog.ts",
);
const { SourceCodeView } = await server.ssrLoadModule(
  "/src/features/conversation/shared/editor/text/source-code-view.tsx",
);
const { resolveSourceFocusRanges } = await server.ssrLoadModule(
  "/src/features/conversation/shared/editor/text/source-code-focus.ts",
);

const now = Date.now();

function editorEvent({
  id,
  input,
  kind = "workspace_edit",
  phase = "done",
  surface = "editor",
  target = "src/app.ts",
  toolName,
  timestamp,
}) {
  return {
    agent_id: "agent:editor-test",
    id,
    input_preview: input,
    kind,
    message_id: `message:${id}`,
    phase,
    round_id: "round:editor-test",
    session_key: "session:editor-test",
    surface,
    target,
    title: toolName,
    tool_name: toolName,
    tool_use_id: `tool:${id}`,
    updated_at: timestamp,
  };
}

test("Claude Code and nxs editor tools use exact mappings without fuzzy aliases", () => {
  assert.equal(resolveOperationToolProfile("Read").action, "read");
  const fileReadProfile = resolveOperationToolProfile("FileRead");
  assert.equal(fileReadProfile.action, "read");
  assert.deepEqual(fileReadProfile.target_keys, ["file_path"]);
  const viewImageProfile = resolveOperationToolProfile("ViewImage");
  assert.equal(viewImageProfile.action, "read");
  assert.equal(viewImageProfile.kind, "workspace_read");
  assert.equal(viewImageProfile.surface, "workspace");
  assert.equal(viewImageProfile.evidence_type, "file");
  assert.deepEqual(viewImageProfile.target_keys, ["source"]);
  const fileWriteProfile = resolveOperationToolProfile("FileWrite");
  assert.equal(fileWriteProfile.action, "create");
  assert.deepEqual(fileWriteProfile.target_keys, ["file_path"]);
  const fileEditProfile = resolveOperationToolProfile("FileEdit");
  assert.equal(fileEditProfile.action, "edit");
  assert.deepEqual(fileEditProfile.target_keys, ["file_path"]);
  assert.equal(resolveOperationToolProfile("filesystem.read").action, "read");
  assert.equal(resolveOperationToolProfile("filesystem.write").action, "create");
  assert.equal(resolveOperationToolProfile("patch.apply").action, "edit");
  assert.equal(resolveOperationToolProfile("notebook.edit").action, "edit");
  assert.equal(resolveOperationToolProfile("apply_patch").action, "generic");
  assert.equal(resolveOperationToolProfile("read_customer_record").action, "generic");
  assert.equal(resolveOperationToolProfile("view_image_metadata").action, "generic");
});

test("Read focuses the real requested line range", () => {
  const read = editorEvent({
    id: "read-range",
    input: { file_path: "src/app.ts", limit: 6, offset: 12 },
    kind: "workspace_read",
    surface: "workspace",
    toolName: "Read",
    timestamp: now,
  });
  const session = buildEditorSessionView({
    event: read,
    path: "src/app.ts",
    relatedEvents: [],
  });

  assert.equal(session.activeAction.kind, "read");
  assert.deepEqual(session.activeAction.lineRange, { start: 12, end: 17 });
  assert.equal(session.detailLabel, "L12-L17");
  assert.deepEqual(session.sourceFocus, {
    startLine: 12,
    endLine: 17,
    tone: "read",
  });

  const fullRead = editorEvent({
    id: "read-full",
    input: { file_path: "src/app.ts" },
    kind: "workspace_read",
    surface: "workspace",
    toolName: "Read",
    timestamp: now + 1,
  });
  const fullSession = buildEditorSessionView({
    event: fullRead,
    path: "src/app.ts",
    relatedEvents: [],
  });
  assert.equal(fullSession.activeAction.lineRange, null);
  assert.equal(fullSession.detailLabel, "全文");
  assert.equal(fullSession.sourceFocus, null);
});

test("Write reports live creation progress and source size", () => {
  const write = editorEvent({
    id: "write-live",
    input: { file_path: "src/app.ts", content: "const ready = true;\nexport { ready };" },
    phase: "done",
    toolName: "Write",
    timestamp: now,
  });
  const session = buildEditorSessionView({
    event: write,
    liveStatus: "writing",
    path: "src/app.ts",
    relatedEvents: [],
  });

  assert.equal(session.activeAction.kind, "write");
  assert.equal(session.activeAction.phase, "running");
  assert.equal(session.activeAction.statusLabel, "正在写入");
  assert.equal(session.activeAction.contentLines, 2);
  assert.equal(session.detailLabel, "2 行 · 37 字符");
  assert.deepEqual(session.sourceFocus, {
    startLine: 1,
    endLine: 2,
    tone: "write",
  });
});

test("Edit and MultiEdit preserve exact before and after fragments", () => {
  const edit = editorEvent({
    id: "edit-one",
    input: {
      file_path: "src/app.ts",
      new_string: "const state = 'ready';",
      old_string: "const state = 'idle';",
    },
    toolName: "Edit",
    timestamp: now,
  });
  const multi = editorEvent({
    id: "edit-many",
    input: {
      edits: [
        { old_string: "idle", new_string: "ready" },
        { old_string: "false", new_string: "true", replace_all: true },
      ],
      file_path: "src/app.ts",
    },
    toolName: "MultiEdit",
    timestamp: now + 1,
  });
  const session = buildEditorSessionView({
    diffStats: { additions: 2, deletions: 2 },
    event: multi,
    path: "src/app.ts",
    relatedEvents: [edit, multi],
  });

  assert.equal(session.history.length, 2);
  assert.equal(session.activeAction.kind, "multi_edit");
  assert.equal(session.changes.length, 2);
  assert.deepEqual(
    session.changes.map(({ before, after, replaceAll }) => ({ before, after, replaceAll })),
    [
      { before: "idle", after: "ready", replaceAll: false },
      { before: "false", after: "true", replaceAll: true },
    ],
  );
  assert.equal(session.detailLabel, "2 处修改 · +2 -2");
  assert.deepEqual(session.sourceFocus, {
    snippets: ["ready", "true"],
    tone: "edit",
  });
});

test("failed edits highlight the unchanged source instead of claiming success", () => {
  const failed = editorEvent({
    id: "edit-failed",
    input: {
      file_path: "src/app.ts",
      new_string: "next",
      old_string: "missing",
    },
    phase: "error",
    toolName: "Edit",
    timestamp: now,
  });
  const session = buildEditorSessionView({
    event: failed,
    path: "src/app.ts",
    relatedEvents: [],
  });

  assert.equal(session.activeAction.statusLabel, "执行失败");
  assert.deepEqual(session.sourceFocus, {
    snippets: ["missing"],
    tone: "error",
  });
});

test("one canonical patch event opens every real file target", () => {
  const patch = editorEvent({
    id: "patch-many-files",
    input: {
      creates: [{ path: "src/new.ts", content: "export const created = true;" }],
      edits: [{ path: "src/app.ts", old_text: "idle", new_text: "ready" }],
    },
    target: "src/app.ts",
    toolName: "patch.apply",
    timestamp: now,
  });
  const workspaceEvents = ["src/app.ts", "src/new.ts"].map((workspacePath, index) => ({
    agent_id: "agent:editor-test",
    diff_stats: { additions: 1, changed_lines: 1, deletions: index === 0 ? 1 : 0 },
    event_type: "file_write_end",
    id: `workspace:${index}`,
    live_content: index === 0 ? "ready" : "export const created = true;",
    path: workspacePath,
    session_key: "session:editor-test",
    source: "agent",
    status: "updated",
    tool_use_id: "tool:patch-many-files",
    updated_at: now + index,
    version: 1,
  }));
  const context = collectOperationFileContext(patch, {
    active_event: patch,
    events: [patch],
    key: "editor-test",
    recent_evidence: [],
    runtime_events: [],
    session_key: "session:editor-test",
    updated_at: now,
    workspace_events: workspaceEvents,
  }, [patch]);

  assert.deepEqual(
    context.file_documents.map((document) => document.target).sort(),
    ["src/app.ts", "src/new.ts"],
  );

  const editedSession = buildEditorSessionView({
    diffStats: { additions: 1, deletions: 1 },
    event: patch,
    path: "src/app.ts",
    relatedEvents: [patch],
  });
  assert.equal(editedSession.activeAction.kind, "edit");
  assert.deepEqual(
    editedSession.changes.map(({ before, after }) => ({ before, after })),
    [{ before: "idle", after: "ready" }],
  );

  const createdSession = buildEditorSessionView({
    diffStats: { additions: 1, deletions: 0 },
    event: patch,
    path: "src/new.ts",
    relatedEvents: [patch],
  });
  assert.equal(createdSession.activeAction.kind, "write");
  assert.equal(createdSession.activeAction.contentCharacters, 28);
  assert.deepEqual(createdSession.changes, []);
});

test("source view resolves ranges, line numbers, and operation chrome", () => {
  const content = "one\ntwo\nconst ready = true;\nfour";
  assert.deepEqual(
    resolveSourceFocusRanges(content, { snippets: ["const ready = true;"], tone: "edit" }),
    [{ start: 3, end: 3 }],
  );
  const read = editorEvent({
    id: "read-render",
    input: { file_path: "src/app.ts", limit: 2, offset: 2 },
    kind: "workspace_read",
    surface: "workspace",
    toolName: "Read",
    timestamp: now,
  });
  const session = buildEditorSessionView({ event: read, path: "src/app.ts", relatedEvents: [] });
  const markup = renderToStaticMarkup(createElement("div", null,
    createElement(EditorActivityBar, { session }),
    createElement(SourceCodeView, { content, focus: session.sourceFocus, isStreaming: false }),
  ));

  assert.match(markup, /读取文件/);
  assert.match(markup, /L2-L3/);
  assert.match(markup, /data-source-line="3"/);
  assert.match(markup, /data-focus-tone="read"/);
});
