import type { RoomConversationView } from "@/types/conversation/conversation";

// 中文注释：创建入口固定占据统一托盘右端的 40px 分区。
const CREATE_CONVERSATION_BUTTON_SPACE = 40;
// 中文注释：真实溢出时，会话概览固定占据统一托盘左端的 36px 分区。
const CONVERSATION_OVERVIEW_BUTTON_SPACE = 36;
const CONVERSATION_TAB_GAP = 2;

export const ACTIVE_TAB_MIN_WIDTH = 156;
export const CONVERSATION_TABS_VIEWPORT_INSET = 5;
export const INACTIVE_TAB_MIN_WIDTH = 104;

const ACTIVE_TAB_WIDTH_WEIGHT = 1.32;

export function shouldShowConversationTabsOverview({
  conversationCount,
  hasCreateButton,
  trackWidth,
}: {
  conversationCount: number;
  hasCreateButton: boolean;
  trackWidth: number;
}): boolean {
  if (!trackWidth || conversationCount <= 1) {
    return false;
  }
  const tabViewportWidth = getAvailableConversationTabWidth({
    hasCreateButton,
    hasOverviewButton: false,
    trackWidth,
  });
  const inactiveCount = conversationCount - 1;
  const minimumTabsWidth = ACTIVE_TAB_MIN_WIDTH
    + INACTIVE_TAB_MIN_WIDTH * inactiveCount
    + CONVERSATION_TAB_GAP * inactiveCount;
  return minimumTabsWidth > tabViewportWidth;
}

export function getRecentConversationIds(
  conversations: RoomConversationView[],
): string[] {
  return [...conversations]
    .sort((left, right) => {
      if (left.last_activity_at !== right.last_activity_at) {
        return right.last_activity_at - left.last_activity_at;
      }
      return left.conversation_id.localeCompare(right.conversation_id);
    })
    .map((conversation) => conversation.conversation_id);
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

export function getInitialOpenConversationIds(
  conversationId: string | null,
  orderedConversationIds: string[],
  maxOpenCount = 1,
): string[] {
  const selectedId = conversationId && orderedConversationIds.includes(conversationId)
    ? conversationId
    : orderedConversationIds[0] ?? null;
  if (!selectedId) {
    return [];
  }

  const capacity = Math.max(1, maxOpenCount);
  const initialIds = orderedConversationIds.slice(0, capacity);
  if (initialIds.includes(selectedId)) {
    return initialIds;
  }
  const selectedIds = new Set([
    ...initialIds.slice(0, Math.max(0, capacity - 1)),
    selectedId,
  ]);
  return orderedConversationIds.filter((id) => selectedIds.has(id));
}

export function reconcileOpenConversationIds({
  conversationId,
  currentIds,
  excludedConversationIds,
  fillAvailable,
  maxOpenCount,
  orderedIds,
  pendingClosedId,
}: {
  conversationId: string | null;
  currentIds: string[];
  excludedConversationIds?: ReadonlySet<string>;
  fillAvailable?: boolean;
  maxOpenCount?: number;
  orderedIds: string[];
  pendingClosedId: string | null;
}): string[] {
  const liveIds = new Set(orderedIds);
  const capacity = Math.max(1, maxOpenCount ?? Number.MAX_SAFE_INTEGER);
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
  const expandedIds = fillAvailable
    ? appendAvailableConversationIds(
      ensuredIds,
      orderedIds,
      capacity,
      buildExcludedConversationIds(excludedConversationIds, pendingClosedId),
    )
    : ensuredIds;
  const sortedIds = sortConversationIdsByReference(expandedIds, orderedIds);
  const resolvedIds = limitOpenConversationIds(
    sortedIds,
    selectedId,
    capacity,
  );

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

function appendAvailableConversationIds(
  currentIds: string[],
  orderedIds: string[],
  maxOpenCount: number,
  excludedIds: ReadonlySet<string>,
): string[] {
  if (currentIds.length >= maxOpenCount) {
    return currentIds;
  }

  const nextIds = [...currentIds];
  for (const id of orderedIds) {
    if (
      nextIds.length >= maxOpenCount
      || excludedIds.has(id)
      || nextIds.includes(id)
    ) {
      continue;
    }
    nextIds.push(id);
  }
  return nextIds;
}

function buildExcludedConversationIds(
  excludedConversationIds: ReadonlySet<string> | undefined,
  pendingClosedId: string | null,
): ReadonlySet<string> {
  if (!pendingClosedId) {
    return excludedConversationIds ?? new Set<string>();
  }

  const excludedIds = new Set(excludedConversationIds);
  excludedIds.add(pendingClosedId);
  return excludedIds;
}

function limitOpenConversationIds(
  currentIds: string[],
  selectedId: string | null,
  maxOpenCount: number,
): string[] {
  if (currentIds.length <= maxOpenCount) {
    return currentIds;
  }

  const limitedIds = [...currentIds];
  while (limitedIds.length > maxOpenCount) {
    const removableIndex = findLastRemovableConversationIndex(
      limitedIds,
      selectedId,
    );
    limitedIds.splice(removableIndex >= 0 ? removableIndex : limitedIds.length - 1, 1);
  }
  return limitedIds;
}

function findLastRemovableConversationIndex(
  currentIds: string[],
  selectedId: string | null,
): number {
  for (let index = currentIds.length - 1; index >= 0; index -= 1) {
    if (currentIds[index] !== selectedId) {
      return index;
    }
  }
  return -1;
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
  hasOverviewButton,
  orderedConversations,
  trackWidth,
}: {
  activeConversationId: string | null;
  hasCreateButton: boolean;
  hasOverviewButton: boolean;
  orderedConversations: RoomConversationView[];
  trackWidth: number;
}): Map<string, number> {
  const widths = new Map<string, number>();
  if (!trackWidth || orderedConversations.length === 0) {
    return widths;
  }

  const tabViewportWidth = getAvailableConversationTabWidth({
    hasCreateButton,
    hasOverviewButton,
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
  if (availableWidth < minimumTotalWidth && hasOverviewButton) {
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
  hasOverviewButton,
  trackWidth,
}: {
  hasCreateButton: boolean;
  hasOverviewButton: boolean;
  trackWidth: number;
}): number {
  return Math.max(
    0,
    trackWidth - CONVERSATION_TABS_VIEWPORT_INSET * 2 - (
      hasCreateButton ? CREATE_CONVERSATION_BUTTON_SPACE : 0
    ) - (
      hasOverviewButton ? CONVERSATION_OVERVIEW_BUTTON_SPACE : 0
    ),
  );
}

function areIdsEqual(leftIds: string[], rightIds: string[]): boolean {
  return leftIds.length === rightIds.length && leftIds.every(
    (id, index) => id === rightIds[index],
  );
}
