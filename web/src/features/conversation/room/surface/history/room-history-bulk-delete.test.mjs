// @vitest-environment node

import assert from "node:assert/strict";

import { test } from "vitest";

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
  const { buildRoomHistoryEntries } = await import("@/features/conversation/room/surface/history/room-history-model.ts");
  const {
    getBulkSelectableConversationIds,
    getRoomHistorySelectionState,
    reconcileRoomHistorySelection,
    toggleAllRoomHistorySelection,
  } = await import("@/features/conversation/room/surface/history/room-history-selection.ts");
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
  const { buildRoomHistoryEntries } = await import("@/features/conversation/room/surface/history/room-history-model.ts");
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

  const { buildRoomHistoryItemPresentation } = await import("@/features/conversation/room/surface/history/room-history-item-model.ts");
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

test("批量删除串行执行并保留未确认项供核对", async () => {
  const { deleteRoomHistoryConversationBatch } = await import("@/features/conversation/room/surface/history/room-history-bulk-delete.ts");
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
