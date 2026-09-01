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
  const [recovery, catalogs] = await Promise.all([
    server.ssrLoadModule(
      "/src/features/home/sidebar/room-deletion-recovery.ts",
    ),
    Promise.all([
      server.ssrLoadModule("/src/shared/i18n/catalog/zh/index.ts"),
      server.ssrLoadModule("/src/shared/i18n/catalog/en/index.ts"),
    ]),
  ]);
  const { getRoomDeletionRecoveryPresentation } = recovery;
  const [{ zhMessages }, { enMessages }] = catalogs;
  const unknown = { directoryCheck: "not_checked", kind: "outcome_unknown" };
  const retryable = { directoryCheck: "not_checked", kind: "not_applied" };

  const unknownPresentation = getRoomDeletionRecoveryPresentation(unknown);
  assert.match(zhMessages[unknownPresentation.failure.titleKey], /无法确认/);
  assert.match(zhMessages[unknownPresentation.failure.impactKey], /删除结果待核对/);
  assert.match(zhMessages[unknownPresentation.failure.impactKey], /没有自动再次删除/);
  assert.match(zhMessages[unknownPresentation.failure.nextStepKey], /不要再次删除/);
  assert.equal(zhMessages[unknownPresentation.confirmTextKey], "核对 Room 列表");
  assert.match(enMessages[unknownPresentation.failure.titleKey], /can’t confirm/);
  assert.match(
    enMessages[unknownPresentation.failure.impactKey],
    /deletion result needs verification/i,
  );
  assert.match(
    enMessages[unknownPresentation.failure.nextStepKey],
    /Don’t delete it again/,
  );
  assert.equal(unknownPresentation.variant, "default");

  const retry = getRoomDeletionRecoveryPresentation(retryable);
  assert.equal(zhMessages[retry.confirmTextKey], "再次删除");
  assert.match(zhMessages[retry.failure.impactKey], /仍然保留/);
  assert.equal(retry.variant, "danger");

  assert.doesNotMatch(
    JSON.stringify([
      zhMessages[unknownPresentation.failure.titleKey],
      zhMessages[unknownPresentation.failure.impactKey],
      zhMessages[unknownPresentation.failure.nextStepKey],
      zhMessages[retry.failure.titleKey],
      zhMessages[retry.failure.impactKey],
      zhMessages[retry.failure.nextStepKey],
    ]),
    /not_applied|outcome_unknown|FailureCore|request[_ ]id/i,
  );
});
