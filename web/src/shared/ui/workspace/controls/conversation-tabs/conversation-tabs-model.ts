import { isExternalSessionConversation } from "@/lib/conversation/external-session";
import type { RoomConversationView } from "@/types/conversation/conversation";

// 中文注释：历史与创建入口共用轻量导航带的 32px 边缘占位。
const CONVERSATION_EDGE_CONTROL_SPACE = 32;
const CONVERSATION_TAB_GAP = 2;

export const ACTIVE_TAB_MIN_WIDTH = 156;
export const CONVERSATION_TABS_VIEWPORT_INSET = 4;
export const INACTIVE_TAB_MIN_WIDTH = 104;

const ACTIVE_TAB_WIDTH_WEIGHT = 1.32;

export function hasConversationTabsOverflow({
  conversationCount,
  hasCreateButton,
  hasLeadingControl,
  trackWidth,
}: {
  conversationCount: number;
  hasCreateButton: boolean;
  hasLeadingControl: boolean;
  trackWidth: number;
}): boolean {
  if (!trackWidth || conversationCount <= 1) {
    return false;
  }
  const tabViewportWidth = getAvailableConversationTabWidth({
    hasCreateButton,
    hasLeadingControl,
    trackWidth,
  });
  const inactiveCount = conversationCount - 1;
  const minimumTabsWidth = ACTIVE_TAB_MIN_WIDTH
    + INACTIVE_TAB_MIN_WIDTH * inactiveCount
    + CONVERSATION_TAB_GAP * inactiveCount;
  return minimumTabsWidth > tabViewportWidth;
}

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
  preserveEmpty,
  orderedIds,
  pendingClosedId,
}: {
  conversationId: string | null;
  currentIds: string[];
  preserveEmpty?: boolean;
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
    preserveEmpty === true,
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
  preserveEmpty: boolean,
): string[] {
  if (currentIds.length > 0 || preserveEmpty) {
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

export function calculateConversationTabWidths({
  activeConversationId,
  hasCreateButton,
  hasLeadingControl,
  hasTabsOverflow,
  orderedConversations,
  trackWidth,
}: {
  activeConversationId: string | null;
  hasCreateButton: boolean;
  hasLeadingControl: boolean;
  hasTabsOverflow: boolean;
  orderedConversations: RoomConversationView[];
  trackWidth: number;
}): Map<string, number> {
  const widths = new Map<string, number>();
  if (!trackWidth || orderedConversations.length === 0) {
    return widths;
  }

  const tabViewportWidth = getAvailableConversationTabWidth({
    hasCreateButton,
    hasLeadingControl,
    trackWidth,
  });
  const availableWidth = tabViewportWidth
    - CONVERSATION_TAB_GAP * Math.max(0, orderedConversations.length - 1);
  if (orderedConversations.length === 1) {
    widths.set(
      orderedConversations[0].conversation_id,
      Math.max(ACTIVE_TAB_MIN_WIDTH, tabViewportWidth),
    );
    return widths;
  }

  const inactiveCount = orderedConversations.length - 1;
  const minimumTotalWidth = ACTIVE_TAB_MIN_WIDTH + INACTIVE_TAB_MIN_WIDTH * inactiveCount;
  if (availableWidth < minimumTotalWidth && hasTabsOverflow) {
    return calculateOverflowConversationTabWidths({
      activeConversationId,
      orderedConversations,
      tabViewportWidth,
    });
  }

  let activeWidth = ACTIVE_TAB_MIN_WIDTH;
  let inactiveWidth = INACTIVE_TAB_MIN_WIDTH;

  if (availableWidth > minimumTotalWidth) {
    const weightedUnitWidth = availableWidth / (inactiveCount + ACTIVE_TAB_WIDTH_WEIGHT);
    const maximumActiveWidth = availableWidth - INACTIVE_TAB_MIN_WIDTH * inactiveCount;
    activeWidth = Math.min(
      maximumActiveWidth,
      Math.max(ACTIVE_TAB_MIN_WIDTH, weightedUnitWidth * ACTIVE_TAB_WIDTH_WEIGHT),
    );
    inactiveWidth = (availableWidth - activeWidth) / inactiveCount;
  }

  orderedConversations.forEach((conversation) => {
    widths.set(
      conversation.conversation_id,
      conversation.conversation_id === activeConversationId ? activeWidth : inactiveWidth,
    );
  });
  return widths;
}

function calculateOverflowConversationTabWidths({
  activeConversationId,
  orderedConversations,
  tabViewportWidth,
}: {
  activeConversationId: string | null;
  orderedConversations: RoomConversationView[];
  tabViewportWidth: number;
}): Map<string, number> {
  const widths = new Map<string, number>();
  const inactiveCount = orderedConversations.length - 1;
  // 中文注释：以活动标签为锚点，计算一屏能完整容纳的普通标签数。
  const visibleInactiveCount = Math.min(
    inactiveCount,
    Math.max(
      0,
      Math.floor(
        (tabViewportWidth - ACTIVE_TAB_MIN_WIDTH)
        / (INACTIVE_TAB_MIN_WIDTH + CONVERSATION_TAB_GAP),
      ),
    ),
  );
  let activeWidth = ACTIVE_TAB_MIN_WIDTH;
  let inactiveWidth = INACTIVE_TAB_MIN_WIDTH;

  if (visibleInactiveCount > 0) {
    const visibleWidth = tabViewportWidth
      - CONVERSATION_TAB_GAP * visibleInactiveCount;
    const weightedUnitWidth = visibleWidth
      / (visibleInactiveCount + ACTIVE_TAB_WIDTH_WEIGHT);
    const maximumActiveWidth = visibleWidth
      - INACTIVE_TAB_MIN_WIDTH * visibleInactiveCount;
    activeWidth = Math.min(
      maximumActiveWidth,
      Math.max(ACTIVE_TAB_MIN_WIDTH, weightedUnitWidth * ACTIVE_TAB_WIDTH_WEIGHT),
    );
    inactiveWidth = (visibleWidth - activeWidth) / visibleInactiveCount;
  }

  orderedConversations.forEach((conversation) => {
    widths.set(
      conversation.conversation_id,
      conversation.conversation_id === activeConversationId ? activeWidth : inactiveWidth,
    );
  });
  return widths;
}

function getAvailableConversationTabWidth({
  hasCreateButton,
  hasLeadingControl,
  trackWidth,
}: {
  hasCreateButton: boolean;
  hasLeadingControl: boolean;
  trackWidth: number;
}): number {
  return Math.max(
    0,
    trackWidth - CONVERSATION_TABS_VIEWPORT_INSET * 2 - (
      hasCreateButton ? CONVERSATION_EDGE_CONTROL_SPACE : 0
    ) - (
      hasLeadingControl ? CONVERSATION_EDGE_CONTROL_SPACE : 0
    ),
  );
}

function areIdsEqual(leftIds: string[], rightIds: string[]): boolean {
  return leftIds.length === rightIds.length && leftIds.every(
    (id, index) => id === rightIds[index],
  );
}
