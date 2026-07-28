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

function conversation(id, {
  draft,
  external = false,
  lastActivity = 0,
  title = id,
  type = "topic",
} = {}) {
  return {
    conversation_id: id,
    conversation_type: type,
    created_at: lastActivity,
    is_draft: draft,
    last_activity_at: lastActivity,
    options: external
      ? { external_session: true, channel_type: "telegram" }
      : {},
    room_id: "room-a",
    session_id: null,
    session_key: `session:${id}`,
    title,
  };
}

test("滚动历史批量选择排除当前会话和外部 Session", async () => {
  const { buildRoomHistoryEntries } = await server.ssrLoadModule(
    "/src/features/conversation/room/surface/history/room-history-model.ts",
  );
  const {
    getBulkSelectableConversationIds,
    getRoomHistorySelectionState,
    reconcileRoomHistorySelection,
    toggleAllRoomHistorySelection,
  } = await server.ssrLoadModule(
    "/src/features/conversation/room/surface/history/room-history-selection.ts",
  );
  const entries = buildRoomHistoryEntries({
    canManageConversations: true,
    canUpdateConversationTitle: true,
    conversations: [
      conversation("current", {lastActivity: 30}),
      conversation("older", {lastActivity: 20}),
      conversation("external", {external: true, lastActivity: 10}),
    ],
    currentConversationId: "current",
  });

  assert.deepEqual(
    [...getBulkSelectableConversationIds(entries)],
    ["older"],
    "当前会话和外部 Session 必须留作不可批量删除项",
  );
  const selected = toggleAllRoomHistorySelection(new Set(), entries);
  assert.deepEqual([...selected], ["older"]);
  assert.equal(getRoomHistorySelectionState(selected, entries), "all");
  assert.equal(
    getRoomHistorySelectionState(selected, [entries[0], entries[1]]),
    "all",
    "全选状态只统计具有批量资格的会话",
  );
  assert.deepEqual(
    [...reconcileRoomHistorySelection(
      new Set(["older", "removed"]),
      getBulkSelectableConversationIds(entries),
    )],
    ["older"],
    "刷新后必须剔除已经消失的选中项",
  );
  const stableSelection = new Set(["older"]);
  assert.equal(
    reconcileRoomHistorySelection(
      stableSelection,
      getBulkSelectableConversationIds(entries),
    ),
    stableSelection,
    "可选集合对象重建但内容不变时必须复用选择快照",
  );
});

test("历史只排除内部草稿且不依据标题推断", async () => {
  const {
    buildRoomHistoryEntries,
    filterRoomHistoryConversations,
  } = await server.ssrLoadModule(
    "/src/features/conversation/room/surface/history/room-history-model.ts",
  );
  const currentDraft = conversation("current-draft", {
    draft: true,
    lastActivity: 30,
    title: "已经生成标题",
  });
  const untitledStarted = conversation("untitled-started", {
    draft: false,
    lastActivity: 20,
    title: "",
  });
  const externalSession = conversation("external", {
    draft: true,
    external: true,
    lastActivity: 10,
  });

  assert.deepEqual(
    filterRoomHistoryConversations([
      currentDraft,
      untitledStarted,
      externalSession,
    ]).map((conversation) => conversation.conversation_id),
    ["untitled-started", "external"],
    "桌面历史与移动切换器共用的过滤器只能排除内部 is_draft=true",
  );

  const draftEntries = buildRoomHistoryEntries({
    canManageConversations: true,
    canUpdateConversationTitle: true,
    conversations: [currentDraft, untitledStarted, externalSession],
    currentConversationId: currentDraft.conversation_id,
  });

  assert.deepEqual(
    draftEntries.map((entry) => entry.conversation.conversation_id),
    ["untitled-started", "external"],
    "有标题的内部草稿仍不进历史，无标题的已开始会话和外部 Session 保持可见",
  );

  const startedEntries = buildRoomHistoryEntries({
    canManageConversations: true,
    canUpdateConversationTitle: true,
    conversations: [
      {...currentDraft, is_draft: false},
      untitledStarted,
      externalSession,
    ],
    currentConversationId: currentDraft.conversation_id,
  });

  assert.deepEqual(
    startedEntries.map((entry) => entry.conversation.conversation_id),
    ["current-draft", "untitled-started", "external"],
    "首条用户输入使 is_draft 收敛为 false 后，同一会话才进入历史",
  );
});

test("当前会话在仍有其他会话时常驻提供单项删除动作", async () => {
  const { buildRoomHistoryEntries } = await server.ssrLoadModule(
    "/src/features/conversation/room/surface/history/room-history-model.ts",
  );
  const { buildRoomHistoryItemPresentation } = await server.ssrLoadModule(
    "/src/features/conversation/room/surface/history/room-history-item-model.ts",
  );
  const currentConversation = conversation("current", {lastActivity: 20});
  const entry = buildRoomHistoryEntries({
    canManageConversations: true,
    canUpdateConversationTitle: true,
    conversations: [
      currentConversation,
      conversation("fallback", {lastActivity: 10}),
    ],
    currentConversationId: currentConversation.conversation_id,
  })[0];
  const presentation = buildRoomHistoryItemPresentation(
    entry,
    {
      isEditing: false,
      isSelected: false,
      isSelecting: false,
    },
    {untitled: "未命名会话"},
  );

  assert.equal(presentation.actionsPersistent, true);
  assert.deepEqual(presentation.actions, ["rename", "delete"]);

  const lastEntry = buildRoomHistoryEntries({
    canManageConversations: true,
    canUpdateConversationTitle: true,
    conversations: [currentConversation],
    currentConversationId: currentConversation.conversation_id,
  })[0];
  const lastPresentation = buildRoomHistoryItemPresentation(
    lastEntry,
    {
      isEditing: false,
      isSelected: false,
      isSelecting: false,
    },
    {untitled: "未命名会话"},
  );

  assert.equal(lastPresentation.actions.includes("delete"), false);
});

test("主对话只在它是最后一个本地会话时禁止删除", async () => {
  const { buildRoomHistoryEntries } = await server.ssrLoadModule(
    "/src/features/conversation/room/surface/history/room-history-model.ts",
  );
  const mainConversation = conversation("main", {
    lastActivity: 10,
    type: "room_main",
  });
  const entries = buildRoomHistoryEntries({
    canManageConversations: true,
    canUpdateConversationTitle: true,
    conversations: [
      conversation("current", {lastActivity: 20}),
      mainConversation,
    ],
    currentConversationId: "current",
  });
  const mainEntry = entries.find(
    (entry) => entry.conversation.conversation_id === mainConversation.conversation_id,
  );

  assert.equal(mainEntry.canDelete, true);

  const lastLocalEntry = buildRoomHistoryEntries({
    canManageConversations: true,
    canUpdateConversationTitle: true,
    conversations: [
      mainConversation,
      conversation("external", {external: true, lastActivity: 20}),
    ],
    currentConversationId: mainConversation.conversation_id,
  }).find(
    (entry) => entry.conversation.conversation_id === mainConversation.conversation_id,
  );

  assert.equal(
    lastLocalEntry.canDelete,
    false,
    "外部 Session 不能使最后一个本地会话获得删除资格",
  );
});

test("批量删除串行执行并保留失败项供重试", async () => {
  const { deleteRoomHistoryConversationBatch } = await server.ssrLoadModule(
    "/src/features/conversation/room/surface/history/room-history-bulk-delete.ts",
  );
  const calls = [];
  const result = await deleteRoomHistoryConversationBatch(
    ["first", "failed", "last"],
    async (conversationId) => {
      calls.push(conversationId);
      if (conversationId === "failed") {
        throw new Error("delete failed");
      }
    },
  );

  assert.deepEqual(calls, ["first", "failed", "last"]);
  assert.deepEqual(result.failedConversationIds, ["failed"]);
});
