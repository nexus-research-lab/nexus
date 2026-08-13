import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const webRoot = fileURLToPath(new URL("..", import.meta.url));

test("联络只显示好友并复用现有聊天组件", async () => {
  const [view, controller, detail] = await Promise.all([
    readFile(path.join(webRoot, "src/features/contacts/agent-communication-view.tsx"), "utf8"),
    readFile(path.join(webRoot, "src/pages/contacts/controller/use-agent-communication.ts"), "utf8"),
    readFile(path.join(webRoot, "src/features/contacts/contacts-agent-detail.tsx"), "utf8"),
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
  assert.match(controller, /ApiRequestError && loadError\.status === 404/);
  assert.match(controller, /room_directed_message/);
  assert.match(controller, /agent_contact_changed/);
  assert.match(controller, /subscribe_room/);
  assert.match(controller, /MESSAGE_FALLBACK_POLL_INTERVAL_MS = 30_000/);
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
});
