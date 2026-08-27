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

test("resource failures only project access loss from HTTP facts", async () => {
  const { ApiRequestError, UnauthorizedError } = await server.ssrLoadModule(
    "/src/lib/api/core/http-error.ts",
  );
  const { getResourceFailure } = await server.ssrLoadModule(
    "/src/lib/error-message.ts",
  );

  assert.deepEqual(
    getResourceFailure(new Error("network unavailable"), "fallback"),
    { access: null, message: "network unavailable" },
  );
  assert.equal(
    getResourceFailure(new UnauthorizedError("sign in"), "fallback").access,
    "authentication_required",
  );
  assert.equal(
    getResourceFailure(new ApiRequestError("forbidden", 403), "fallback").access,
    "forbidden",
  );
  assert.equal(
    getResourceFailure(new ApiRequestError("conflict", 409), "fallback").access,
    null,
  );
  assert.equal(
    getResourceFailure(new ApiRequestError("denied", 500, {
      category: "authorization",
      code: "test.authorization",
      effect: "not_applicable",
      version: 1,
    }), "fallback").access,
    "forbidden",
  );

  const errorSource = await read("src/lib/error-message.ts");
  const resourceStateSource = await read("src/shared/ui/display/resource-state.tsx");
  assert.doesNotMatch(errorSource, /navigator\.onLine/);
  assert.doesNotMatch(resourceStateSource, /"offline"|"permission"/);
  assert.match(resourceStateSource, /primaryAction\?: UiResourceStateAction/);
});

test("Loop picker keeps a loaded snapshot during refresh and transient failure", async () => {
  const { projectLoopPickerContentKind } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/components/loop-picker/loop-picker-model.ts",
  );

  assert.equal(projectLoopPickerContentKind({
    accessBlocked: false,
    error: null,
    hasSnapshot: false,
    isLoading: true,
    loopCount: 0,
  }), "loading");
  assert.equal(projectLoopPickerContentKind({
    accessBlocked: false,
    error: null,
    hasSnapshot: true,
    isLoading: true,
    loopCount: 2,
  }), "list");
  assert.equal(projectLoopPickerContentKind({
    accessBlocked: false,
    error: new Error("refresh failed"),
    hasSnapshot: true,
    isLoading: false,
    loopCount: 2,
  }), "list");
  assert.equal(projectLoopPickerContentKind({
    accessBlocked: true,
    error: new Error("forbidden"),
    hasSnapshot: true,
    isLoading: false,
    loopCount: 2,
  }), "error");
});

test("sensitive snapshots are blocked by access state and refresh stays non-destructive", async () => {
  const [
    memoryView,
    memoryDocument,
    scheduledHistory,
    loopsDirectory,
    workGraphDirectory,
    loopController,
    memoryDocumentResource,
    scheduledDialog,
  ] = await Promise.all([
    read("src/features/memory/agent-memory-view.tsx"),
    read("src/features/memory/document/memory-document-panel.tsx"),
    read("src/features/capability/scheduled/history/view/scheduled-task-run-history-content.tsx"),
    read("src/features/capability/loops/loops-directory.tsx"),
    read("src/features/capability/workgraph-distillations/workgraph-distillations-directory.tsx"),
    read("src/features/conversation/shared/composer/components/loop-picker/use-loop-picker-controller.ts"),
    read("src/features/memory/document/use-memory-document-resource.ts"),
    read("src/features/capability/scheduled/history/scheduled-task-run-history-dialog.tsx"),
  ]);

  assert.ok(
    memoryView.indexOf("memory.resource.error?.access")
      < memoryView.indexOf("memory.resource.isLoading && !memory.resource.snapshot"),
  );
  assert.ok(
    memoryDocument.indexOf("controller.resourceError?.access")
      < memoryDocument.indexOf("controller.isLoading && !controller.content"),
  );
  assert.match(scheduledHistory, /accessBlocked && failure[\s\S]*isLoading && !hasSnapshot/);
  assert.match(loopsDirectory, /loading && !hasSnapshot/);
  assert.match(workGraphDirectory, /loading && !hasSnapshot/);
  assert.match(loopController, /current\.scopeKey === locale[\s\S]*\.\.\.current/);
  assert.doesNotMatch(loopController, /setResource\(INITIAL_RESOURCE\)/);
  assert.match(memoryView, /isOpen=\{!accessBlocked && deleteTarget !== null\}/);
  assert.match(workGraphDirectory, /isOpen=\{!accessBlocked && Boolean\(deleteCandidate\)\}/);
  assert.match(workGraphDirectory, /\{!accessBlocked && editingPreview \? \(/);
  assert.match(scheduledDialog, /isOpen=\{!accessBlocked && actions\.recoveryTarget !== null\}/);
  assert.match(
    memoryDocumentResource,
    /if \(accessBlocked\) \{[\s\S]*consumedLiveVersionRef\.current[\s\S]*return;/,
  );
  assert.match(
    memoryDocumentResource,
    /current\.resourceError\?\.access[\s\S]*\? current/,
  );
});

test("uncertain mutation copy never claims that nothing changed", async () => {
  const [capabilityZh, capabilityEn, conversationZh, conversationEn] = await Promise.all([
    read("src/shared/i18n/catalog/zh/capability.ts"),
    read("src/shared/i18n/catalog/en/capability.ts"),
    read("src/shared/i18n/catalog/zh/conversation.ts"),
    read("src/shared/i18n/catalog/en/conversation.ts"),
  ]);
  const copy = [capabilityZh, capabilityEn, conversationZh, conversationEn].join("\n");

  assert.doesNotMatch(copy, /这张工作图仍然保留|对话内容没有改变/);
  assert.doesNotMatch(copy, /This WorkGraph is still available|The conversation was not changed/);
  assert.match(copy, /无法确认删除是否已经生效/);
  assert.match(copy, /cannot yet confirm whether deletion took effect/);
});

function read(relativePath) {
  return readFile(path.join(webRoot, relativePath), "utf8");
}
