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

test("Room deletion commands only replay an explicitly not-applied mutation", async () => {
  const recovery = await server.ssrLoadModule(
    "/src/features/home/sidebar/room-deletion-recovery.ts",
  );
  const {
    getRoomDeletionCommand,
    projectRoomDeletionFailure,
  } = recovery;

  assert.equal(getRoomDeletionCommand(null), "delete");
  assert.equal(getRoomDeletionCommand({
    directoryCheck: "not_checked",
    kind: "not_applied",
  }), "delete");
  assert.equal(getRoomDeletionCommand({
    directoryCheck: "target_present",
    kind: "outcome_unknown",
  }), "reconcile");
  assert.equal(getRoomDeletionCommand({
    directoryCheck: "target_present",
    kind: "committed_cleanup_incomplete",
  }), "reconcile");
  assert.equal(getRoomDeletionCommand({
    directoryCheck: "target_absent",
    kind: "committed_cleanup_incomplete",
  }), "dismiss");

  assert.deepEqual(projectRoomDeletionFailure({
    code: null,
    effect: "unknown",
  }), {
    directoryCheck: "not_checked",
    kind: "outcome_unknown",
  });
  assert.deepEqual(projectRoomDeletionFailure({
    code: "room.not_found",
    effect: "not_applied",
  }), {
    directoryCheck: "not_checked",
    kind: "resource_absent",
  });
});

test("Room deletion recovery copy answers result, impact, and next step", async () => {
  const { getRoomDeletionRecoveryPresentation } = await server.ssrLoadModule(
    "/src/features/home/sidebar/room-deletion-recovery.ts",
  );
  const unknown = { directoryCheck: "not_checked", kind: "outcome_unknown" };
  const retryable = { directoryCheck: "not_checked", kind: "not_applied" };

  const chineseUnknown = getRoomDeletionRecoveryPresentation(unknown, "zh");
  assert.match(chineseUnknown.failure.title, /无法确认/);
  assert.match(chineseUnknown.failure.impact, /删除结果待核对/);
  assert.match(chineseUnknown.failure.impact, /没有自动再次删除/);
  assert.match(chineseUnknown.failure.nextStep, /不要再次删除/);
  assert.equal(chineseUnknown.confirmText, "核对 Room 列表");
  assert.equal(chineseUnknown.variant, "default");

  const englishUnknown = getRoomDeletionRecoveryPresentation(unknown, "en");
  assert.match(englishUnknown.failure.title, /can’t confirm/);
  assert.match(englishUnknown.failure.impact, /deletion result needs verification/i);
  assert.match(englishUnknown.failure.impact, /did not delete it again/i);
  assert.match(englishUnknown.failure.nextStep, /Don’t delete it again/);

  const retry = getRoomDeletionRecoveryPresentation(retryable, "zh");
  assert.equal(retry.confirmText, "再次删除");
  assert.match(retry.failure.impact, /仍然保留/);
  assert.equal(retry.variant, "danger");

  assert.doesNotMatch(
    JSON.stringify([chineseUnknown, englishUnknown, retry]),
    /not_applied|outcome_unknown|FailureCore|request[_ ]id/i,
  );
});

test("Room deletion controller locks execution and reconciles unknown outcomes", async () => {
  const [controller, panel, directory] = await Promise.all([
    read("src/features/home/sidebar/use-chat-sidebar-controller.ts"),
    read("src/features/home/sidebar/chat-sidebar-panel.tsx"),
    read("src/features/home/sidebar/sidebar-directory.ts"),
  ]);

  assert.match(controller, /deletionRunningRef\.current/);
  assert.match(controller, /unresolvedDeletionsRef/);
  assert.match(controller, /projectMutationFailure\(/);
  assert.match(
    controller,
    /failure\.kind !== "not_applied"[\s\S]*await reconcileDeletion\(target, failure, ownerGeneration\)/,
  );
  assert.doesNotMatch(controller, /console\.error\("\[Sidebar\] 删除 Room 失败"/);
  assert.match(panel, /busy=\{controller\.deletion\.action !== null\}/);
  assert.match(panel, /failure=\{deletionRecovery\?\.failure\}/);
  assert.match(directory, /await reconcileHomeDirectory\(\)/);
  assert.match(directory, /refreshed\.rooms\.some\(\(room\) => room\.id === roomId\)/);
});

function read(relativePath) {
  return readFile(path.join(webRoot, relativePath), "utf8");
}
