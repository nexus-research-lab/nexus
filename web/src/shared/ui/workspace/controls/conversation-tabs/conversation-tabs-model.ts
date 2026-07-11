import { getSessionChannelLabel } from "@/features/conversation/external-session-labels";
import { RoomConversationView } from "@/types/conversation/conversation";

const CREATE_CONVERSATION_BUTTON_SPACE = 88;
const TRACK_HORIZONTAL_PADDING = 2;

export const ACTIVE_TAB_MIN_WIDTH = 142;
export const INACTIVE_TAB_MIN_WIDTH = 92;

const ACTIVE_TAB_WIDTH_WEIGHT = 1.32;

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

export function getInitialOpenConversationIds(
  conversationId: string | null,
  recentConversationIds: string[],
): string[] {
  if (conversationId && recentConversationIds.includes(conversationId)) {
    return [conversationId];
  }
  return recentConversationIds[0] ? [recentConversationIds[0]] : [];
}

export function reconcileOpenConversationIds({
  conversationId,
  currentIds,
  pendingClosedId,
  recentIds,
}: {
  conversationId: string | null;
  currentIds: string[];
  pendingClosedId: string | null;
  recentIds: string[];
}): string[] {
  const liveIds = new Set(recentIds);
  const selectedId = conversationId && liveIds.has(conversationId) ? conversationId : null;
  const retainedIds = currentIds.filter((id) => liveIds.has(id));
  const shouldOpenSelected = selectedId !== null &&
    selectedId !== pendingClosedId &&
    !retainedIds.includes(selectedId);
  const nextIds = shouldOpenSelected
    ? [...retainedIds, selectedId]
    : retainedIds;
  const fallbackId = selectedId ?? recentIds[0] ?? null;
  const resolvedIds = nextIds.length === 0 && fallbackId ? [fallbackId] : nextIds;

  return areIdsEqual(currentIds, resolvedIds) ? currentIds : resolvedIds;
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

export function isExternalSessionConversation(
  conversation?: RoomConversationView,
): boolean {
  return conversation?.options?.external_session === true;
}

export function getExternalSessionLabel(
  conversation: RoomConversationView,
): string | null {
  if (!isExternalSessionConversation(conversation)) {
    return null;
  }
  return getSessionChannelLabel(
    readStringOption(conversation.options, "channel_type"),
    conversation.session_key,
  );
}

export function calculateConversationTabWidths({
  activeConversationId,
  hasCreateButton,
  orderedConversations,
  trackWidth,
}: {
  activeConversationId: string | null;
  hasCreateButton: boolean;
  orderedConversations: RoomConversationView[];
  trackWidth: number;
}): Map<string, number> {
  const widths = new Map<string, number>();
  if (!trackWidth || orderedConversations.length === 0) {
    return widths;
  }

  const availableWidth = Math.max(
    0,
    trackWidth - TRACK_HORIZONTAL_PADDING - (
      hasCreateButton ? CREATE_CONVERSATION_BUTTON_SPACE : 0
    ),
  );
  if (orderedConversations.length === 1) {
    widths.set(
      orderedConversations[0].conversation_id,
      Math.max(ACTIVE_TAB_MIN_WIDTH, availableWidth),
    );
    return widths;
  }

  const inactiveCount = orderedConversations.length - 1;
  const minimumTotalWidth = ACTIVE_TAB_MIN_WIDTH + INACTIVE_TAB_MIN_WIDTH * inactiveCount;
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

function areIdsEqual(leftIds: string[], rightIds: string[]): boolean {
  return leftIds.length === rightIds.length && leftIds.every(
    (id, index) => id === rightIds[index],
  );
}

function readStringOption(options: Record<string, unknown>, key: string): string | null {
  const value = options[key];
  return typeof value === "string" ? value : null;
}
