import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const webRoot = fileURLToPath(new URL("..", import.meta.url));

test("contact communication preserves snapshots and makes recovery actionable", async () => {
  const [view, controller, recovery, detail, readFailureType, zhCatalog, enCatalog] =
    await Promise.all([
      read("src/features/contacts/agent-communication-view.tsx"),
      read("src/pages/contacts/controller/use-agent-communication.ts"),
      read("src/pages/contacts/controller/agent-communication-recovery.ts"),
      read("src/features/contacts/contacts-agent-detail.tsx"),
      read("src/types/agent/communication.ts"),
      read("src/shared/i18n/catalog/zh/agent.ts"),
      read("src/shared/i18n/catalog/en/agent.ts"),
    ]);

  assert.match(
    readFailureType,
    /"channel"[\s\S]*"directory"[\s\S]*"history"[\s\S]*"messages"/,
  );
  assert.match(controller, /directorySnapshotAgentIdRef/);
  assert.match(controller, /targetSnapshotKeyRef/);
  assert.match(controller, /messageSnapshotKeyRef/);
  assert.match(controller, /const invalidated = invalidatesReadSnapshot\(loadError\)/);
  assert.match(controller, /stale: !invalidated[\s\S]*messageSnapshotKeyRef\.current === requestKey/);
  assert.match(controller, /setConversationFailure\(\{[\s\S]*kind: "history"/);
  assert.match(controller, /case "directory":[\s\S]*void loadDirectory\(\)/);
  assert.match(controller, /case "channel":[\s\S]*void loadTarget\(\)/);
  assert.match(
    controller,
    /case "history":[\s\S]*void loadOlderMessages\(\)[\s\S]*void loadTarget\(\)/,
  );
  assert.match(controller, /case "messages":[\s\S]*void loadMessages\(true, true\)/);
  assert.equal(controller.match(/sendAgentCommunicationMessageApi\(/g)?.length, 1);
  assert.doesNotMatch(controller, /setTimeout|setInterval/);

  assert.match(controller, /blocksAgentCommunicationIntent/);
  assert.match(controller, /clearMatchingMutationFailure/);
  assert.match(controller, /reconcileContactDirectoryMutation/);
  assert.match(controller, /activeAgentIdRef\.current !== scopeAgentId/);
  assert.match(recovery, /projectMutationFailure/);
  assert.match(recovery, /projected\.effect !== "not_applied"/);
  assert.match(recovery, /effect: "not_applied"/);

  assert.match(view, /FeedbackBannerViewport/);
  assert.match(view, /mutation_unknown_impact/);
  assert.match(view, /mutation_committed_next_step/);
  assert.match(view, /failure\.blocksRepeat \? \{\} : \{ onDismiss: onClear \}/);
  assert.match(view, /readFailure && !readFailure\.stale/);
  assert.match(view, /<UiResourceState/);
  assert.match(view, /impact=\{t\(copy\.impact\)\}/);
  assert.match(view, /nextStep=\{t\(copy\.nextStep\)\}/);
  assert.match(view, /onRetry=\{\(\) => onRetryRead\(readFailure\.kind\)\}/);
  assert.match(detail, /AgentCommunicationReadFailureKind/);
  assert.match(detail, /data-agent-save-error-details/);
  assert.match(detail, /data-agent-save-error-popover/);

  for (const catalog of [zhCatalog, enCatalog]) {
    assert.match(catalog, /agent_options\.contact\.directory_unavailable_impact/);
    assert.match(catalog, /agent_options\.contact\.directory_stale_impact/);
    assert.match(catalog, /agent_options\.contact\.channel_unavailable_impact/);
    assert.match(catalog, /agent_options\.contact\.messages_stale_impact/);
    assert.match(catalog, /agent_options\.contact\.history_failure_next_step/);
    assert.match(catalog, /agent_options\.contact\.mutation_not_applied_impact/);
    assert.match(catalog, /agent_options\.contact\.mutation_unknown_next_step/);
    assert.match(catalog, /agent_options\.contact\.mutation_accepted_impact/);
    assert.match(catalog, /agent_options\.contact\.mutation_committed_next_step/);
  }
});

function read(relativePath) {
  return readFile(path.join(webRoot, relativePath), "utf8");
}
