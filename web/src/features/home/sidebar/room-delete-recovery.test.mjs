// @vitest-environment node

import assert from "node:assert/strict";

import { test } from "vitest";

test("Room deletion commands only replay an explicitly not-applied mutation", async () => {
  const recovery = await import("@/features/home/sidebar/room-deletion-recovery.ts");
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
