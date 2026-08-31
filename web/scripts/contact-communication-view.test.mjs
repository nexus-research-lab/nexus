import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const webRoot = fileURLToPath(new URL("..", import.meta.url));

test("联络只显示好友并复用现有聊天组件", async () => {
  const [
    view,
    controller,
    recovery,
    detail,
    readFailureType,
    zhCatalog,
    enCatalog,
  ] = await Promise.all([
    readFile(path.join(webRoot, "src/features/contacts/agent-communication-view.tsx"), "utf8"),
    readFile(path.join(webRoot, "src/pages/contacts/controller/use-agent-communication.ts"), "utf8"),
    readFile(path.join(webRoot, "src/pages/contacts/controller/agent-communication-recovery.ts"), "utf8"),
    readFile(path.join(webRoot, "src/features/contacts/contacts-agent-detail.tsx"), "utf8"),
    readFile(path.join(webRoot, "src/types/agent/communication.ts"), "utf8"),
    readFile(path.join(webRoot, "src/shared/i18n/catalog/zh/agent.ts"), "utf8"),
    readFile(path.join(webRoot, "src/shared/i18n/catalog/en/agent.ts"), "utf8"),
  ]);

  assert.match(view, /ConversationPanelLayout/);
  assert.match(view, /MessageItem/);
  assert.match(view, /ComposerPanel/);
  assert.match(view, /WorkspaceSurfaceHeader/);
  assert.match(view, /minmax\(240px,288px\).*md:gap-2/);
  assert.match(view, /items-center gap-2 px-2 py-3/);
  assert.doesNotMatch(view, /gap-2 border-b border-\(--divider-subtle-color\) p-3/);
  assert.match(detail, /WORKSPACE_CONTENT_GUTTER_CLASS_NAME/);
  assert.match(detail, /useState<AgentDetailTabKey>\("identity"\)/);
  assert.doesNotMatch(
    detail,
    /useResettableState<AgentDetailTabKey>[\s\S]*?agent\.agent_id/,
  );
  assert.match(
    detail,
    /contentMaxWidthClassName=\{WORKSPACE_CONTENT_MAX_WIDTH_CLASS_NAME\}/,
  );
  assert.doesNotMatch(detail, /max-w-\[860px\]/);
  assert.match(detail, /mx-\[var\(--workspace-content-gutter\)\].*border-t/);
  assert.match(controller, /conversation_id: conversationId \?\? undefined/);
  assert.match(controller, /deleteAgentContactApi/);
  assert.match(controller, /error instanceof ApiRequestError && error\.status === 404/);
  assert.match(controller, /room_directed_message/);
  assert.match(controller, /agent_contact_changed/);
  assert.match(controller, /subscribe_room/);
  assert.doesNotMatch(controller, /MESSAGE_FALLBACK_POLL_INTERVAL_MS|setInterval/);
  assert.match(controller, /previousRealtimeStateRef/);
  assert.match(controller, /before_message_id: historyCursor\.beforeMessageId/);
  assert.match(controller, /setHistoryPrependToken/);
  assert.match(view, /remove_friend_confirm/);
  assert.match(view, /useConversationHistoryLoader/);
  assert.match(view, /useFollowScroll/);
  assert.doesNotMatch(view, /UiRoomAvatar|CommunicationBubble|roomMessages/);
  assert.doesNotMatch(view, /<header className=/);
  assert.doesNotMatch(view, /inset-x-\[var\(--workspace-content-gutter\)\]/);
  assert.doesNotMatch(view, /\{conversationId \? \(\s*<ConversationPanelBottomArea/);
  assert.doesNotMatch(controller, /listRooms|getRoomConversationMessages|roomMessages/);
  assert.doesNotMatch(controller, /!conversationId \|\| isSending/);
  assert.match(readFailureType, /"channel"[\s\S]*"directory"[\s\S]*"history"[\s\S]*"messages"/);
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
  assert.doesNotMatch(controller, /setError\(/);
  assert.match(recovery, /projectMutationFailure/);
  assert.match(recovery, /projected\.effect !== "not_applied"/);
  assert.match(recovery, /effect: "not_applied"/);
  assert.match(view, /FeedbackBannerViewport/);
  assert.match(view, /mutation_unknown_impact/);
  assert.match(view, /mutation_committed_next_step/);
  assert.match(view, /failure\.blocksRepeat \? \{\} : \{ onDismiss: onClear \}/);
  assert.doesNotMatch(view, /session_load_failed/);

  const blockingDirectoryFailure = view.indexOf(
    "state.directoryFailure && !state.directoryFailure.stale",
  );
  const directoryEmptyState = view.indexOf("contacts.length === 0", blockingDirectoryFailure);
  assert.ok(blockingDirectoryFailure >= 0);
  assert.ok(directoryEmptyState > blockingDirectoryFailure);
  assert.match(view, /<UiResourceState/);
  assert.match(view, /impact=\{t\(copy\.impact\)\}/);
  assert.match(view, /nextStep=\{t\(copy\.nextStep\)\}/);
  assert.match(view, /primaryAction=\{\{/);
  assert.match(view, /onRetry=\{\(\) => onRefresh\("directory"\)\}/);
  assert.match(view, /onRetryRead\(readFailure\.kind\)/);
  assert.match(view, /readFailure && !readFailure\.stale/);
  assert.match(view, /state\.conversationFailure\?\.kind[\s\S]*"messages" : "channel"/);
  assert.match(detail, /AgentCommunicationReadFailureKind/);
  assert.match(detail, /data-agent-save-error-details/);
  assert.match(detail, /aria-label=\{state\.message\}/);
  assert.match(detail, /data-agent-save-error-popover/);
  assert.doesNotMatch(detail, /<span className="max-sm:hidden">\{state\.message\}<\/span>/);
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
