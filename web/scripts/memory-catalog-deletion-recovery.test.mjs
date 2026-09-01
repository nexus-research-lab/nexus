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

test("Memory deletion recovery only retries not-applied and requires a new intent otherwise", async () => {
  const recovery = await server.ssrLoadModule(
    "/src/features/memory/catalog/memory-deletion-recovery.ts",
  );
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
    directoryCheck: "target_present",
  };
  const presentRecovery = recovery.getMemoryDeletionRecoveryPresentation(
    unknownPresent,
  );
  assert.equal(presentRecovery.primaryAction, "reconcile");
  assert.equal(presentRecovery.secondaryAction, "start_new_intent");
  assert.equal(
    recovery.canStartNewMemoryDeletionIntent(unknownPresent),
    true,
  );

  const unknownFailed = { ...unknownPresent, directoryCheck: "failed" };
  assert.equal(
    recovery.getMemoryDeletionRecoveryPresentation(unknownFailed)
      .secondaryAction,
    "start_new_intent",
  );

  const committedFailed = {
    ...recovery.projectCommittedMemoryDeletion(identity),
    directoryCheck: "failed",
  };
  const committedRecovery = recovery.getMemoryDeletionRecoveryPresentation(
    committedFailed,
  );
  assert.equal(committedRecovery.primaryAction, "reconcile");
  assert.equal(committedRecovery.secondaryAction, null);
  assert.equal(
    recovery.canStartNewMemoryDeletionIntent(committedFailed),
    false,
  );
});

test("Memory catalog fences delete and reconciliation by owner, Agent, path, and command", async () => {
  const [controller, notice, view] = await Promise.all([
    readFile(path.join(
      webRoot,
      "src/features/memory/catalog/use-agent-memory.ts",
    ), "utf8"),
    readFile(path.join(
      webRoot,
      "src/features/memory/catalog/memory-deletion-issue-notice.tsx",
    ), "utf8"),
    readFile(path.join(
      webRoot,
      "src/features/memory/agent-memory-view.tsx",
    ), "utf8"),
  ]);

  assert.match(controller, /useSyncExternalStore/);
  assert.match(controller, /`\$\{ownerGeneration\}\\u0000\$\{agentId\}`/);
  assert.match(controller, /issue\.ownerGeneration === ownerGeneration/);
  assert.match(controller, /issue\.agentId === agentId/);
  assert.match(controller, /issue\.path === path/);
  assert.match(controller, /activeCommand\.current\.path === token\.path/);
  assert.match(controller, /projectMutationFailure/);
  assert.match(controller, /issue\.kind !== "not_applied"/);
  assert.match(controller, /await reconcileDeletionIssue\(token, issue\)/);
  assert.match(controller, /projectCommittedMemoryDeletion/);
  assert.match(controller, /不能把已提交删除降级成普通失败或再次 DELETE/);
  assert.match(controller, /普通\/后台刷新只更新目录/);
  assert.match(controller, /if \(!snapshot\.truncated\)/);
  assert.match(controller, /getWorkspaceFileContentApi\(agentId, path\)/);
  assert.doesNotMatch(
    controller.slice(
      controller.indexOf("const refresh = useCallback"),
      controller.indexOf("useEffect(() =>"),
    ),
    /deleteIssues:/,
    "ordinary refresh must not resolve an exact-path deletion issue",
  );

  assert.match(notice, /impact=\{t\(presentation\.impactKey/);
  assert.match(notice, /primaryAction=\{getRecoveryAction/);
  assert.match(notice, /secondaryAction=\{presentation\.secondaryAction/);
  assert.match(view, /MemoryDeletionIssueNotices/);
  assert.match(view, /memory_delete_confirm_new_intent/);
});
