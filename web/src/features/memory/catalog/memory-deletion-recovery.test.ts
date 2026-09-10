// INPUT: 删除结果证据与精确身份、目录核对状态。
// OUTPUT: 保留安全重试、显式新意图及继续核对的恢复边界断言。
// POS: 纯恢复模型共置测试；无需 Vite SSR server 或文件监听。
import assert from "node:assert/strict";
import { test } from "vitest";
import * as recovery from "./memory-deletion-recovery";

test("Memory deletion recovery only retries not-applied and requires a new intent otherwise", () => {
  const identity = {
    agentId: "agent-a",
    ownerGeneration: 7,
    path: "memory/project.md",
    title: "Project memory",
  };

  const notApplied = recovery.projectMemoryDeletionFailure(
    { effect: "not_applied" },
    identity,
  );
  assert.equal(
    recovery.getMemoryDeletionRecoveryPresentation(notApplied).primaryAction,
    "retry",
  );

  const unknownPresent = {
    ...recovery.projectMemoryDeletionFailure({ effect: "unknown" }, identity),
    directoryCheck: "target_present" as const,
  };
  const presentRecovery = recovery.getMemoryDeletionRecoveryPresentation(
    unknownPresent,
  );
  assert.equal(presentRecovery.primaryAction, "start_new_intent");
  assert.equal(
    recovery.canStartNewMemoryDeletionIntent(unknownPresent),
    true,
  );

  const unknownFailed = { ...unknownPresent, directoryCheck: "failed" as const };
  assert.equal(
    recovery.getMemoryDeletionRecoveryPresentation(unknownFailed)
      .primaryAction,
    "reconcile",
  );
  assert.equal(
    recovery.canStartNewMemoryDeletionIntent(unknownFailed),
    false,
  );

  const committedFailed = {
    ...recovery.projectCommittedMemoryDeletion(identity),
    directoryCheck: "failed" as const,
  };
  const committedRecovery = recovery.getMemoryDeletionRecoveryPresentation(
    committedFailed,
  );
  assert.equal(committedRecovery.primaryAction, "reconcile");
  assert.equal(
    recovery.canStartNewMemoryDeletionIntent(committedFailed),
    false,
  );
});

