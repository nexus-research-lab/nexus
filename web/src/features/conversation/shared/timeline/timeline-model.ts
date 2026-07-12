import type {
  AssistantMessage,
  Message,
  UserMessage,
} from "@/types/conversation/message/entity";
import type { ContentBlock } from "@/types/conversation/message/content";
import type { RoomPendingAgentSlotState } from "@/types/agent/agent-conversation";
import type { PendingPermission } from "@/types/conversation/interaction/permission";
import type { SessionRoundIndexItem } from "@/types/conversation/room";
import { stripRoomControlMarkers } from "../message/message-content-model";

/** DM / Room 共用的唯一时间线投影。 */
export interface ConversationTimeline {
  message_groups: Map<string, Message[]>;
  pending_slot_groups: Map<string, RoomPendingAgentSlotState[]>;
  pending_permission_groups: Map<string, PendingPermission[]>;
  loaded_round_ids: string[];
  feed_round_ids: string[];
  round_index_items: SessionRoundIndexItem[];
  live_round_ids: string[];
}

function groupByRound<T>(
  items: T[],
  getRoundId: (item: T) => string | null | undefined,
): Map<string, T[]> {
  const groups = new Map<string, T[]>();
  for (const item of items) {
    const roundId = getRoundId(item)?.trim();
    if (!roundId) {
      continue;
    }
    const group = groups.get(roundId);
    if (group) {
      group.push(item);
    } else {
      groups.set(roundId, [item]);
    }
  }
  return groups;
}

/** 消息的 round_id 已由后端归一为根轮次。 */
export function groupMessagesByRound(
  messages: Message[],
): Map<string, Message[]> {
  return groupByRound(messages, (message) => message.round_id);
}

export function groupPendingPermissionsByRound(
  permissions: PendingPermission[],
): Map<string, PendingPermission[]> {
  return groupByRound(permissions, (permission) => permission.round_id);
}

export function groupPendingSlotsByRound(
  slots: RoomPendingAgentSlotState[],
): Map<string, RoomPendingAgentSlotState[]> {
  return groupByRound(slots, (slot) => slot.round_id);
}

// 终态轮次里 assistant 仅剩无回复标记（剥离后无文本、无工具/图片等块）时，
// 视为纯 no-reply，不在时间线显示。保守判定：任何工具/非文本块都算可见输出。
function hasVisibleUserContent(message: UserMessage): boolean {
  return Boolean(message.content.trim()) || Boolean(message.attachments?.length);
}

function hasVisibleAssistantBlock(block: ContentBlock): boolean {
  switch (block.type) {
    case "thinking":
      return false;
    case "text":
      return Boolean(stripRoomControlMarkers(block.text));
    default:
      // 工具、图片等非文本块即使没有摘要，也属于用户可见输出。
      return true;
  }
}

function hasVisibleAssistantOutput(message: AssistantMessage): boolean {
  const result = message.result_summary?.result ?? "";
  return message.content.some(hasVisibleAssistantBlock)
    || Boolean(stripRoomControlMarkers(result));
}

function isBlankNoReplyRound(messages: Message[]): boolean {
  // 用户消息不能因 Assistant 无回复而被整轮吞掉。
  const hasVisibleUserMessage = messages
    .filter((message): message is UserMessage => message.role === "user")
    .some(hasVisibleUserContent);
  const assistants = messages.filter(
    (message): message is AssistantMessage => message.role === "assistant",
  );
  return !hasVisibleUserMessage
    && assistants.length > 0
    && !assistants.some(hasVisibleAssistantOutput);
}

/** 时间线除历史消息外，也要显示已启动但尚未产生消息的运行轮次。 */
export function buildTimelineRoundIds(
  messageGroups: Map<string, Message[]>,
  liveRoundIds: string[] = [],
  extraRoundIds: Iterable<string> = [],
): string[] {
  const live = new Set(liveRoundIds);
  const roundIds = Array.from(messageGroups.keys()).filter(
    (roundId) =>
      live.has(roundId) ||
      !isBlankNoReplyRound(messageGroups.get(roundId) ?? []),
  );
  const seen = new Set(roundIds);

  for (const roundId of extraRoundIds) {
    appendUniqueRoundId(roundIds, seen, roundId);
  }
  for (const roundId of liveRoundIds) {
    appendUniqueRoundId(roundIds, seen, roundId);
  }
  return roundIds;
}

function appendUniqueRoundId(
  roundIds: string[],
  seen: Set<string>,
  roundId: string | null | undefined,
) {
  const normalized = roundId?.trim();
  if (!normalized || seen.has(normalized)) {
    return;
  }
  seen.add(normalized);
  roundIds.push(normalized);
}

function getIndexedLoadedRoundIndexes(
  indexedRoundIds: string[],
  loadedRoundIds: string[],
): number[] {
  const indexByRoundId = new Map<string, number>();
  indexedRoundIds.forEach((roundId, index) => {
    indexByRoundId.set(roundId, index);
  });

  const indexes = new Set<number>();
  for (const roundId of loadedRoundIds) {
    const index = indexByRoundId.get(roundId);
    if (index !== undefined) {
      indexes.add(index);
    }
  }
  return Array.from(indexes).sort((left, right) => left - right);
}

function isLatestLoadedWindow(
  indexedRoundIds: string[],
  loadedIndexes: number[],
): boolean {
  if (loadedIndexes.length === 0) {
    return false;
  }
  const firstLoadedIndex = loadedIndexes[0];
  const expectedLength = indexedRoundIds.length - firstLoadedIndex;
  if (expectedLength !== loadedIndexes.length) {
    return false;
  }
  return loadedIndexes.every(
    (index, offset) => index === firstLoadedIndex + offset,
  );
}

/**
 * 用完整索引确定 feed 顺序，但正文只渲染已加载窗口。
 *
 * 最新历史页不插入未加载占位，避免新打开旧 session 时因为全量索引
 * 直接产生很长的空滚动；非最新窗口保留相邻占位，让点击定位后还能
 * 继续通过正常滚动触发局部加载。
 */
export function buildIndexedTimelineRoundIds(
  roundIndexItems: SessionRoundIndexItem[],
  loadedRoundIds: string[],
): string[] {
  if (roundIndexItems.length === 0) {
    return loadedRoundIds;
  }

  const indexedRoundIds = roundIndexItems
    .map((item) => item.roundId.trim())
    .filter(Boolean);
  const indexedRoundIdSet = new Set(indexedRoundIds);
  const loadedIndexes = getIndexedLoadedRoundIndexes(
    indexedRoundIds,
    loadedRoundIds,
  );
  const shouldIncludeBoundaryPlaceholders =
    !isLatestLoadedWindow(indexedRoundIds, loadedIndexes);
  const seen = new Set<string>();
  const roundIds: string[] = [];

  const visibleIndexSet = new Set(loadedIndexes);
  if (shouldIncludeBoundaryPlaceholders) {
    for (const index of loadedIndexes) {
      if (index > 0) {
        visibleIndexSet.add(index - 1);
      }
      if (index < indexedRoundIds.length - 1) {
        visibleIndexSet.add(index + 1);
      }
    }
  }

  for (const index of Array.from(visibleIndexSet).sort((left, right) => left - right)) {
    appendUniqueRoundId(roundIds, seen, indexedRoundIds[index]);
  }
  for (const roundId of loadedRoundIds) {
    if (!indexedRoundIdSet.has(roundId)) {
      appendUniqueRoundId(roundIds, seen, roundId);
    }
  }
  return roundIds;
}
