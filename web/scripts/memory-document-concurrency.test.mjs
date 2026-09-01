import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
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

test.after(async () => {
  await server.close();
});

test("Memory live updates preserve an active draft and carry an exact revision", async () => {
  const { resolveMemoryLiveUpdateIntent } = await server.ssrLoadModule(
    "/src/features/memory/document/memory-document-model.ts",
  );
  const base = {
    consumed: { scopeKey: "agent:memory/topic.md", version: 3 },
    contentRevision: "sha256:old",
    editing: true,
    saving: false,
    scopeKey: "agent:memory/topic.md",
  };
  const updated = {
    agent_id: "agent",
    path: "memory/topic.md",
    status: "updated",
    version: 4,
    source: "agent",
    live_content: "agent version",
    content_revision: "sha256:agent",
    updated_at: Date.now(),
  };

  assert.deepEqual(
    resolveMemoryLiveUpdateIntent({ ...base, liveState: updated }),
    { kind: "conflict", version: 4 },
  );
  assert.deepEqual(
    resolveMemoryLiveUpdateIntent({
      ...base,
      contentRevision: "sha256:agent",
      liveState: updated,
    }),
    { kind: "consume", version: 4 },
  );
  assert.deepEqual(
    resolveMemoryLiveUpdateIntent({
      ...base,
      editing: false,
      liveState: updated,
    }),
    {
      content: "agent version",
      kind: "apply",
      revision: "sha256:agent",
      version: 4,
    },
  );
  assert.deepEqual(
    resolveMemoryLiveUpdateIntent({
      ...base,
      editing: false,
      liveState: { ...updated, content_revision: null },
    }),
    { kind: "reload", version: 4 },
  );
});

test("a successful save advances only the submitted baseline and keeps newer typing", async () => {
  const {
    classifyMemorySaveReconciliation,
    mergeSavedMemoryDocument,
  } = await server.ssrLoadModule(
    "/src/features/memory/document/use-memory-document-state.ts",
  );
  const current = {
    command: "save",
    commandError: null,
    content: "old",
    draft: "newer typing",
    editing: true,
    isLoading: false,
    resourceError: null,
    revision: "sha256:old",
    saveIssue: null,
    scopeKey: "agent:memory/topic.md",
  };

  const merged = mergeSavedMemoryDocument(
    current,
    "submitted draft",
    "submitted draft",
    "sha256:submitted",
  );
  assert.equal(merged.content, "submitted draft");
  assert.equal(merged.revision, "sha256:submitted");
  assert.equal(merged.draft, "newer typing");
  assert.equal(merged.editing, true);

  const issue = {
    attemptedDraft: "submitted draft",
    expectedRevision: "sha256:old",
    kind: "outcome_unknown",
    reconciliationFailed: false,
  };
  assert.equal(classifyMemorySaveReconciliation(issue, {
    content: "submitted draft",
    path: "memory/topic.md",
    revision: "sha256:submitted",
  }), "saved");
  assert.equal(classifyMemorySaveReconciliation(issue, {
    content: "old",
    path: "memory/topic.md",
    revision: "sha256:old",
  }), "not_applied");
  assert.equal(classifyMemorySaveReconciliation(issue, {
    content: "agent update",
    path: "memory/topic.md",
    revision: "sha256:agent",
  }), "conflict");
});

test("Memory writes send the read revision and never replay conflicts implicitly", async () => {
  const { updateWorkspaceFileContentApi } = await server.ssrLoadModule(
    "/src/lib/api/agent/agent-api.ts",
  );
  const previousFetch = globalThis.fetch;
  let requestBody = null;
  globalThis.fetch = async (_input, init) => {
    requestBody = JSON.parse(init.body);
    return new Response(JSON.stringify({
      code: "0000",
      message: "success",
      success: true,
      data: {
        path: "memory/topic.md",
        content: "draft",
        revision: "sha256:new",
      },
    }), {
      headers: { "Content-Type": "application/json" },
      status: 200,
    });
  };
  try {
    await updateWorkspaceFileContentApi(
      "agent",
      "memory/topic.md",
      "draft",
      "sha256:base",
    );
  } finally {
    globalThis.fetch = previousFetch;
  }
  assert.deepEqual(requestBody, {
    path: "memory/topic.md",
    content: "draft",
    expected_revision: "sha256:base",
  });

  const [saveSource, stateSource, panelSource] = await Promise.all([
    read("src/features/memory/document/use-memory-document-save.ts"),
    read("src/features/memory/document/use-memory-document-state.ts"),
    read("src/features/memory/document/memory-document-panel.tsx"),
  ]);
  assert.match(saveSource, /workspace\.file_revision_conflict/);
  assert.match(saveSource, /getWorkspaceFileContentApi[\s\S]*classifyMemorySaveReconciliation/);
  assert.match(stateSource, /response\.revision === issue\.expectedRevision/);
  assert.match(saveSource, /reconcileRunningRef\.current/);
  assert.doesNotMatch(saveSource, /setTimeout|retry\(/);
  assert.match(panelSource, /memory_conflict_impact/);
  assert.match(panelSource, /memory_conflict_review_impact/);
  assert.match(panelSource, /memory_use_latest/);
  assert.match(panelSource, /overwriteConflict/);
});

function read(relativePath) {
  return readFile(path.join(webRoot, relativePath), "utf8");
}
