// INPUT: Room 会话事实、显式选择、打开集合与旧路由收口事实。
// OUTPUT: 创建顺序、草稿资格、存活标签、活动项、关闭回退和持久化资格。
// POS: Room 标签导航纯模型；不拥有共享标签宽度、DOM 或 Store 副作用。

import { isExternalSessionConversation } from "@/lib/conversation/external-session";
import type { RoomConversationView } from "@/types/conversation/conversation";

export function getConversationIdsByCreationTime(
  conversations: RoomConversationView[],
): string[] {
  return [...conversations]
    .sort((left, right) => {
      if (left.created_at !== right.created_at) {
        return left.created_at - right.created_at;
      }
      return left.conversation_id.localeCompare(right.conversation_id);
    })
    .map((conversation) => conversation.conversation_id);
}

export function resolveSelectedDraftConversationId(
  conversations: RoomConversationView[],
  selectedConversationId: string | null,
): string | null {
  if (!selectedConversationId) {
    return null;
  }
  const selectedConversation = conversations.find(
    (conversation) => conversation.conversation_id === selectedConversationId,
  );
  return selectedConversation?.is_draft === true
    && !isExternalSessionConversation(selectedConversation)
    ? selectedConversation.conversation_id
    : null;
}

export function getInitialOpenConversationIds(
  conversationId: string | null,
  orderedConversationIds: string[],
): string[] {
  const selectedId = conversationId && orderedConversationIds.includes(conversationId)
    ? conversationId
    : orderedConversationIds[orderedConversationIds.length - 1] ?? null;
  return selectedId ? [selectedId] : [];
}

export function reconcileOpenConversationIds({
  conversationId,
  currentIds,
  orderedIds,
  pendingClosedId,
}: {
  conversationId: string | null;
  currentIds: string[];
  orderedIds: string[];
  pendingClosedId: string | null;
}): string[] {
  const liveIds = new Set(orderedIds);
  const selectedId = resolveLiveConversationId(conversationId, liveIds);
  const retainedIds = retainLiveConversationIds(currentIds, liveIds);
  const selectedIds = appendSelectedConversationId(
    retainedIds,
    selectedId,
    pendingClosedId,
  );
  const ensuredIds = ensureOpenConversationId(
    selectedIds,
    selectedId,
    orderedIds,
  );
  const resolvedIds = sortConversationIdsByReference(ensuredIds, orderedIds);

  return areIdsEqual(currentIds, resolvedIds) ? currentIds : resolvedIds;
}

function sortConversationIdsByReference(
  currentIds: string[],
  orderedIds: string[],
): string[] {
  const currentIdSet = new Set(currentIds);
  return orderedIds.filter((id) => currentIdSet.has(id));
}

function resolveLiveConversationId(
  conversationId: string | null,
  liveIds: Set<string>,
): string | null {
  return conversationId && liveIds.has(conversationId)
    ? conversationId
    : null;
}

function retainLiveConversationIds(
  currentIds: string[],
  liveIds: Set<string>,
): string[] {
  return currentIds.filter((id) => liveIds.has(id));
}

function appendSelectedConversationId(
  currentIds: string[],
  selectedId: string | null,
  pendingClosedId: string | null,
): string[] {
  if (
    !selectedId
    || selectedId === pendingClosedId
    || currentIds.includes(selectedId)
  ) {
    return currentIds;
  }
  return [...currentIds, selectedId];
}

function ensureOpenConversationId(
  currentIds: string[],
  selectedId: string | null,
  recentIds: string[],
): string[] {
  if (currentIds.length > 0) {
    return currentIds;
  }
  const fallbackId = selectedId ?? recentIds[0] ?? null;
  return fallbackId ? [fallbackId] : currentIds;
}

export function resolveActiveConversationId({
  conversationId,
  optimisticId,
  orderedConversations,
}: {
  conversationId: string | null;
  optimisticId: string | null;
  orderedConversations: RoomConversationView[];
}): string | null {
  const openIds = new Set(
    orderedConversations.map((conversation) => conversation.conversation_id),
  );
  if (optimisticId && openIds.has(optimisticId)) {
    return optimisticId;
  }
  if (conversationId && openIds.has(conversationId)) {
    return conversationId;
  }
  return orderedConversations[0]?.conversation_id ?? null;
}

export function shouldPersistConversationTabs({
  activeConversationId,
  routeConversationId,
}: {
  activeConversationId: string | null;
  routeConversationId: string | null;
}): boolean {
  return Boolean(
    activeConversationId
      && routeConversationId === activeConversationId,
  );
}

export function getCloseFallbackConversationId(
  orderedConversations: RoomConversationView[],
  targetConversationId: string,
): string | null {
  const targetIndex = orderedConversations.findIndex(
    (conversation) => conversation.conversation_id === targetConversationId,
  );
  if (targetIndex < 0) {
    return null;
  }
  return (
    orderedConversations[targetIndex + 1]?.conversation_id ??
    orderedConversations[targetIndex - 1]?.conversation_id ??
    null
  );
}

function areIdsEqual(leftIds: string[], rightIds: string[]): boolean {
  return leftIds.length === rightIds.length && leftIds.every(
    (id, index) => id === rightIds[index],
  );
}
