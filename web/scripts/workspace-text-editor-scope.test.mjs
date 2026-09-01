import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const webRoot = fileURLToPath(new URL("..", import.meta.url));
const recoveryModule = loadTypescriptModule(
  "src/features/conversation/shared/editor/text/text-file-editor-recovery.ts",
);

test("workspace text drafts and async responses are fenced by exact Agent and path", async () => {
  const [hook, panel] = await Promise.all([
    read("src/features/conversation/shared/editor/text/use-text-file-editor.ts"),
    read("src/features/conversation/shared/editor/workspace-file-preview-panel.tsx"),
  ]);

  assert.match(
    hook,
    /const scopeKey = `\$\{ownerGeneration\}\\u0000\$\{agentId\}\\u0000\$\{path\}`/,
  );
  assert.match(hook, /useSyncExternalStore\(/);
  assert.match(hook, /isAuthOwnerScopeGenerationCurrent/);
  assert.match(hook, /scopeRef\.current\.key === expectedScope\.key/);
  assert.match(hook, /saveTokenRef\.current/);
  assert.match(hook, /reconcileTokenRef\.current/);
  assert.match(hook, /failure\.access[\s\S]*clearTextFileForAccess/);
  assert.match(
    hook,
    /clearTextFileForAccess[\s\S]*createTextFileEditorState\(current\.scopeKey\)/,
  );
  assert.match(panel, /key=\{`\$\{agentId\}\\u0000\$\{path\}`\}/);
});

test("an unread file cannot enter editing or saving state", async () => {
  const [hook, model] = await Promise.all([
    read("src/features/conversation/shared/editor/text/use-text-file-editor.ts"),
    read("src/features/conversation/shared/editor/text/text-file-editor-model.ts"),
  ]);

  assert.match(hook, /!state\.hasLoadedContent[\s\S]*\|\| !state\.revision/);
  assert.match(hook, /current\.hasLoadedContent[\s\S]*Boolean\(current\.revision\)/);
  assert.match(
    model,
    /editDisabled: !isEditing[\s\S]*\(!isAvailable \|\| !revisionReady \|\| isExternalWriting\)/,
  );
  assert.match(model, /saveDisabled: !isAvailable[\s\S]*\|\| !revisionReady[\s\S]*\|\| !isDirty/);
});

test("workspace text saves carry the loaded revision and unknown writes only reconcile by GET", async () => {
  const [hook, recovery] = await Promise.all([
    read("src/features/conversation/shared/editor/text/use-text-file-editor.ts"),
    read("src/features/conversation/shared/editor/text/text-file-editor-recovery.ts"),
  ]);

  assert.match(
    hook,
    /updateWorkspaceFileContentApi\([\s\S]*token\.attemptedDraft,[\s\S]*token\.expectedRevision/,
  );
  assert.match(hook, /workspace\.file_revision_conflict/);
  assert.match(hook, /failure\.effect === "not_applied"/);
  assert.match(
    hook,
    /getWorkspaceFileContentApi\(token\.agentId, token\.path\)[\s\S]*classifyTextFileSaveReconciliation/,
  );
  assert.doesNotMatch(hook, /setTimeout|retry\(/);
  assert.match(recovery, /response\.revision === issue\.expectedRevision/);
});

test("workspace text reconciliation preserves newer typing and distinguishes exact outcomes", async () => {
  const {
    classifyTextFileSaveReconciliation,
    mergeSavedTextFile,
  } = await recoveryModule;
  const issue = {
    attemptedDraft: "submitted",
    expectedRevision: "sha256:base",
    kind: "outcome_unknown",
    reconciliationFailed: false,
  };

  assert.equal(classifyTextFileSaveReconciliation(issue, {
    content: "submitted",
    path: "notes.md",
    revision: "sha256:submitted",
  }), "intent_present");
  assert.equal(classifyTextFileSaveReconciliation(issue, {
    content: "base",
    path: "notes.md",
    revision: "sha256:base",
  }), "retry_ready");
  assert.equal(classifyTextFileSaveReconciliation(issue, {
    content: "other writer",
    path: "notes.md",
    revision: "sha256:other",
  }), "conflict");

  assert.deepEqual(mergeSavedTextFile({
    draftContent: "newer typing",
    isEditing: true,
    revision: "sha256:base",
    savedContent: "base",
  }, "submitted", {
    content: "submitted",
    path: "notes.md",
    revision: "sha256:submitted",
  }), {
    draftContent: "newer typing",
    isEditing: true,
    revision: "sha256:submitted",
    savedContent: "submitted",
  });
});

test("external live updates never overwrite a dirty or outcome-unknown draft", async () => {
  const { resolveTextFileLiveUpdateIntent } = await recoveryModule;
  const liveState = {
    agent_id: "agent-a",
    path: "notes.md",
    status: "updated",
    version: 4,
    source: "agent",
    live_content: "external",
    content_revision: "sha256:external",
    updated_at: Date.now(),
  };
  const base = {
    agentId: "agent-a",
    consumed: { scopeKey: "scope", version: 3 },
    hasLoadedContent: true,
    isEditing: false,
    isSaving: false,
    liveState,
    path: "notes.md",
    revision: "sha256:base",
    saveIssue: null,
    scopeKey: "scope",
  };

  assert.deepEqual(resolveTextFileLiveUpdateIntent({
    ...base,
    isDirty: true,
  }), { kind: "conflict", version: 4 });
  assert.deepEqual(resolveTextFileLiveUpdateIntent({
    ...base,
    isDirty: false,
  }), {
    content: "external",
    kind: "apply",
    revision: "sha256:external",
    version: 4,
  });
  assert.deepEqual(resolveTextFileLiveUpdateIntent({
    ...base,
    isDirty: true,
    saveIssue: {
      attemptedDraft: "draft",
      expectedRevision: "sha256:base",
      kind: "outcome_unknown",
      reconciliationFailed: false,
    },
  }), { kind: "ignore" });
  assert.deepEqual(resolveTextFileLiveUpdateIntent({
    ...base,
    isDirty: false,
    revision: "sha256:external",
  }), { kind: "consume", version: 4 });
});

test("workspace text issues use one message and safe actions", async () => {
  const [editor, notice, zh] = await Promise.all([
    read("src/features/conversation/shared/editor/text/text-file-editor.tsx"),
    read("src/features/conversation/shared/editor/text/text-file-editor-reliability.tsx"),
    read("src/shared/i18n/catalog/zh/core.ts"),
  ]);

  assert.match(editor, /<TextFileEditorReliability/);
  assert.match(notice, /impact=\{/);
  assert.doesNotMatch(notice, /nextStep=\{/);
  assert.match(notice, /onReconcile/);
  assert.match(notice, /onAdoptLatest/);
  assert.match(notice, /onOverwrite/);
  assert.match(notice, /\[overflow-wrap:anywhere\]/);
  assert.match(zh, /workspace_file\.save_unknown_impact/);
  assert.match(zh, /workspace_file\.conflict_review_next_step/);
});

function read(relativePath) {
  return readFile(path.join(webRoot, relativePath), "utf8");
}

async function loadTypescriptModule(relativePath) {
  const [{ default: typescript }, source] = await Promise.all([
    import("typescript"),
    read(relativePath),
  ]);
  const output = typescript.transpileModule(source, {
    compilerOptions: {
      module: typescript.ModuleKind.ESNext,
      target: typescript.ScriptTarget.ES2022,
    },
    fileName: relativePath,
  }).outputText;
  return import(`data:text/javascript;base64,${Buffer.from(output).toString("base64")}`);
}
