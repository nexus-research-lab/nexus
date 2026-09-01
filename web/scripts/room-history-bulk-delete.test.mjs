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
  externalIdentity = null,
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
      ? {
          external_identity: externalIdentity,
          external_session: true,
          channel_type: "telegram",
        }
      : {},
    room_id: "room-a",
    session_id: null,
    session_key: `session:${id}`,
    title,
  };
}

function historyItemCopy(locale = "zh") {
  const isEnglish = locale === "en";
  return {
    actionLabels: {
      delete: isEnglish ? "Delete conversation" : "删除对话",
      rename: isEnglish ? "Rename" : "重命名",
    },
    editorLabels: {
      cancel: isEnglish ? "Cancel" : "取消",
      confirm: isEnglish ? "Confirm rename" : "确认重命名",
      input: isEnglish ? "Edit conversation title" : "编辑对话标题",
    },
    locale,
    untitled: isEnglish ? "Untitled conversation" : "未命名会话",
  };
}

test("滚动历史批量选择包含当前会话但排除外部 Session", async () => {
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
    ["current", "older"],
    "当前本地会话必须允许全量清空，外部 Session 仍不可批量删除",
  );
  const selected = toggleAllRoomHistorySelection(new Set(), entries);
  assert.deepEqual([...selected], ["current", "older"]);
  assert.equal(getRoomHistorySelectionState(selected, entries), "all");
  assert.equal(
    getRoomHistorySelectionState(selected, [entries[0], entries[1]]),
    "all",
    "全选状态只统计具有批量资格的会话",
  );
  assert.deepEqual(
    [...reconcileRoomHistorySelection(
      new Set(["current", "older", "removed"]),
      getBulkSelectableConversationIds(entries),
    )],
    ["current", "older"],
    "刷新后必须剔除已经消失的选中项",
  );
  const stableSelection = new Set(["current", "older"]);
  assert.equal(
    reconcileRoomHistorySelection(
      stableSelection,
      getBulkSelectableConversationIds(entries),
    ),
    stableSelection,
    "可选集合对象重建但内容不变时必须复用选择快照",
  );
});

test("IM 历史会话展示账号身份且解绑后即使有任务引用也可单项删除", async () => {
  const { buildRoomHistoryEntries } = await server.ssrLoadModule(
    "/src/features/conversation/room/surface/history/room-history-model.ts",
  );
  const current = conversation("current-im", {
    external: true,
    externalIdentity: {
      account_hint: "A1B2C3",
      can_delete: false,
      channel_type: "telegram",
      current_pairing: true,
      pairing_status: "active",
    },
    lastActivity: 30,
  });
  const referenced = conversation("referenced-im", {
    external: true,
    externalIdentity: {
      account_hint: "D4E5F6",
      can_delete: true,
      channel_type: "telegram",
      current_pairing: false,
      pairing_status: "disabled",
      task_reference_count: 2,
    },
    lastActivity: 20,
  });
  const historical = conversation("historical-im", {
    external: true,
    externalIdentity: {
      account_hint: "112233",
      can_delete: true,
      channel_type: "telegram",
      current_pairing: false,
      pairing_status: "disabled",
    },
    lastActivity: 10,
  });
  const legacy = conversation("legacy-im", {
    external: true,
    externalIdentity: {
      can_delete: true,
      channel_type: "telegram",
      current_pairing: false,
      legacy_session_hint: "445566",
      pairing_status: "disabled",
    },
    lastActivity: 5,
  });
  const entries = buildRoomHistoryEntries({
    canManageConversations: true,
    canUpdateConversationTitle: true,
    conversations: [current, referenced, historical, legacy],
    currentConversationId: current.conversation_id,
  });

  assert.deepEqual(entries.map((entry) => ({
    canDelete: entry.canDelete,
    label: entry.externalSessionLabel,
  })), [
    {canDelete: false, label: "Telegram · 账号 A1B2C3 · 当前"},
    {canDelete: true, label: "Telegram · 账号 D4E5F6 · 历史 · 2 个任务"},
    {canDelete: true, label: "Telegram · 账号 112233 · 历史"},
    {canDelete: true, label: "Telegram · 旧会话 445566 · 历史"},
  ]);
  assert.equal(entries.every((entry) => entry.canBulkDelete === false), true);

  const { buildRoomHistoryItemPresentation } = await server.ssrLoadModule(
    "/src/features/conversation/room/surface/history/room-history-item-model.ts",
  );
  const referencedPresentation = buildRoomHistoryItemPresentation(
    entries[1],
    {
      isEditing: false,
      isSelected: false,
      isSelecting: false,
    },
    historyItemCopy(),
  );
  assert.deepEqual(referencedPresentation.actions, ["delete"]);
  assert.equal(
    referencedPresentation.actionsPersistent,
    true,
    "所有可删除的 IM 历史会话都必须常显删除入口，而不是依赖 hover",
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
    historyItemCopy(),
  );

  assert.equal(presentation.actionsPersistent, true);
  assert.deepEqual(presentation.actions, ["rename", "delete"]);

  const oneDayAgo = Date.now() - 25 * 60 * 60 * 1000;
  const englishPresentation = buildRoomHistoryItemPresentation(
    {
      ...entry,
      conversation: {
        ...entry.conversation,
        last_activity_at: oneDayAgo,
      },
    },
    {
      isEditing: false,
      isSelected: false,
      isSelecting: false,
    },
    historyItemCopy("en"),
  );
  assert.equal(englishPresentation.activityLabel, "1 day ago");
  assert.equal(englishPresentation.actionLabels.rename, "Rename");
  assert.equal(
    englishPresentation.editorLabels.input,
    "Edit conversation title",
  );

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
    historyItemCopy(),
  );

  assert.equal(lastPresentation.actions.includes("delete"), false);
  const lastSelectionPresentation = buildRoomHistoryItemPresentation(
    lastEntry,
    {
      isEditing: false,
      isSelected: true,
      isSelecting: true,
    },
    historyItemCopy(),
  );
  assert.equal(
    lastSelectionPresentation.selection?.disabled,
    false,
    "最后一个已开始会话仍可通过全量清空进入批量选择",
  );
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

test("批量删除串行执行并保留未确认项供核对", async () => {
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
  assert.equal(result.replacementConversationId, null);
});

test("清空全部历史先创建新草稿并最后删除当前会话", async () => {
  const { deleteRoomHistoryConversationBatch } = await server.ssrLoadModule(
    "/src/features/conversation/room/surface/history/room-history-bulk-delete.ts",
  );
  const calls = [];
  const result = await deleteRoomHistoryConversationBatch(
    ["current", "older", "older"],
    async (conversationId) => {
      calls.push(`delete:${conversationId}`);
    },
    {
      currentConversationId: "current",
      createReplacementConversation: async () => {
        calls.push("create:fresh");
        return "fresh";
      },
    },
  );

  assert.deepEqual(
    calls,
    ["create:fresh", "delete:older", "delete:current"],
    "新草稿必须先成为安全锚点，当前会话必须最后删除",
  );
  assert.deepEqual(result.failedConversationIds, []);
  assert.equal(result.replacementConversationId, "fresh");
});

test("新草稿准备失败时不删除任何历史", async () => {
  const { deleteRoomHistoryConversationBatch } = await server.ssrLoadModule(
    "/src/features/conversation/room/surface/history/room-history-bulk-delete.ts",
  );
  const calls = [];
  const result = await deleteRoomHistoryConversationBatch(
    ["current", "older"],
    async (conversationId) => {
      calls.push(conversationId);
    },
    {
      currentConversationId: "current",
      createReplacementConversation: async () => null,
    },
  );

  assert.deepEqual(calls, []);
  assert.deepEqual(result.failedConversationIds, ["current", "older"]);
  assert.equal(result.replacementConversationId, null);
});
