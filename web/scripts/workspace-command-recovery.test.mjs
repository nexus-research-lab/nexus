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

test("workspace unknown-result locks are exact to Agent, command, and paths", async () => {
  const { getWorkspaceMutationIntentKey } = await recoveryModule();
  const base = {
    agentId: "agent-a",
    command: "rename",
    isDirectory: false,
    path: "docs/a|b.md",
    newPath: "docs/c.md",
  };

  const key = getWorkspaceMutationIntentKey(base);
  assert.notEqual(key, getWorkspaceMutationIntentKey({...base, agentId: "agent-b"}));
  assert.notEqual(key, getWorkspaceMutationIntentKey({...base, path: "docs/a"}));
  assert.notEqual(key, getWorkspaceMutationIntentKey({...base, newPath: "b.md|docs/c.md"}));
  assert.notEqual(key, getWorkspaceMutationIntentKey({
    agentId: "agent-a",
    command: "delete",
    path: "docs/a|b.md",
  }));

  const upload = {
    agentId: "agent-a",
    command: "upload",
    file: {lastModified: 7, name: "report.pdf", size: 42, type: "application/pdf"},
    targetDirectory: "docs",
  };
  assert.notEqual(
    getWorkspaceMutationIntentKey(upload),
    getWorkspaceMutationIntentKey({
      ...upload,
      file: {...upload.file, lastModified: 8},
    }),
  );
});

test("authoritative lists only reconcile create, rename, and delete target states", async () => {
  const { reconcileWorkspaceMutation } = await recoveryModule();
  const files = [
    entry("notes.md", false),
    entry("renamed", true),
    entry("renamed/child.txt", false),
  ];

  assert.deepEqual(reconcileWorkspaceMutation({
    agentId: "agent-a",
    command: "create",
    entryType: "file",
    path: "notes.md",
  }, files), {
    command: "create",
    entryType: "file",
    result: {path: "notes.md"},
  });
  assert.equal(reconcileWorkspaceMutation({
    agentId: "agent-a",
    command: "create",
    entryType: "directory",
    path: "notes.md",
  }, files), null);

  assert.deepEqual(reconcileWorkspaceMutation({
    agentId: "agent-a",
    command: "rename",
    isDirectory: true,
    path: "original",
    newPath: "renamed",
  }, files), {
    command: "rename",
    result: {path: "original", new_path: "renamed"},
  });
  assert.equal(reconcileWorkspaceMutation({
    agentId: "agent-a",
    command: "rename",
    isDirectory: true,
    path: "renamed",
    newPath: "other",
  }, files), null);

  assert.deepEqual(reconcileWorkspaceMutation({
    agentId: "agent-a",
    command: "delete",
    path: "removed",
  }, files), {command: "delete", result: {path: "removed"}});
  assert.equal(reconcileWorkspaceMutation({
    agentId: "agent-a",
    command: "delete",
    path: "renamed",
  }, files), null);
  assert.equal(reconcileWorkspaceMutation({
    agentId: "agent-a",
    command: "upload",
    file: {lastModified: 1, name: "notes.md", size: 1, type: "text/plain"},
    targetDirectory: null,
  }, files), null);
});

test("multi-file uploads retain an item-level terminal or pending status", async () => {
  const { groupWorkspaceUploadOutcomes } = await recoveryModule();
  assert.deepEqual(groupWorkspaceUploadOutcomes([
    {name: "done.txt", status: "completed"},
    {name: "maybe.txt", status: "unconfirmed"},
    {name: "rejected.txt", status: "not_applied"},
    {name: "later-a.txt", status: "not_started"},
    {name: "later-b.txt", status: "not_started"},
  ]), {
    completed: ["done.txt"],
    not_applied: ["rejected.txt"],
    not_started: ["later-a.txt", "later-b.txt"],
    unconfirmed: ["maybe.txt"],
  });
});

test("workspace mutations separate committed API results from list refresh and never replay during reconciliation", async () => {
  const source = await read(
    "src/features/conversation/room/workspace/controller/use-workspace-commands.ts",
  );
  const reconciliation = source.slice(
    source.indexOf("const reconcilePendingRecovery"),
    source.indexOf("const allowNewIntent"),
  );

  assert.match(
    source,
    /const result = await mutation\(token\.scopeKey\);[\s\S]*await refreshAfterCommittedMutation\(command, token\);[\s\S]*return isCurrentToken\(token\) \? result : null/,
  );
  assert.match(source, /recoveryLocksRef\.current\.get\(recoveryKey\)/);
  assert.match(source, /failure\.effect === "not_applied"/);
  assert.match(source, /canStartNewIntent: failure\.effect === "unknown"/);
  assert.match(source, /status: "unconfirmed"/);
  assert.match(source, /status: "not_started"/);
  assert.doesNotMatch(
    reconciliation,
    /createWorkspaceEntryApi|renameWorkspaceEntryApi|deleteWorkspaceEntryApi|uploadWorkspaceFileApi/,
  );
});

function entry(filePath, isDirectory) {
  return {
    depth: filePath.split("/").length - 1,
    is_dir: isDirectory,
    modified_at: "2026-08-28T00:00:00Z",
    name: filePath.split("/").at(-1),
    path: filePath,
  };
}

function read(relativePath) {
  return readFile(path.join(webRoot, relativePath), "utf8");
}

function recoveryModule() {
  return server.ssrLoadModule(
    "/src/features/conversation/room/workspace/controller/workspace-command-recovery.ts",
  );
}
