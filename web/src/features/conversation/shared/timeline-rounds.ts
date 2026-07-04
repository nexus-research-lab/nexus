import type { AssistantMessage, Message } from "@/types/conversation/message";
import type { SessionRoundIndexItem } from "@/types/conversation/room";
import { stripRoomControlMarkers } from "./message/item/message-item-support";

// 终态轮次里 assistant 仅剩无回复标记（剥离后无文本、无工具/图片等块）时，
// 视为纯 no-reply，不在时间线显示。保守判定：任何工具/非文本块都算可见输出。
function isBlankNoReplyRound(messages: Message[]): boolean {
  const assistants = messages.filter(
    (message): message is AssistantMessage => message.role === "assistant",
  );
  if (assistants.length === 0) {
    return false;
  }
  for (const assistant of assistants) {
    for (const block of assistant.content) {
      if (block.type === "thinking") {
        continue;
      }
      if (block.type === "text") {
        if (stripRoomControlMarkers(block.text)) {
          return false;
        }
        continue;
      }
      return false;
    }
    const resultText = assistant.result_summary?.result;
    if (resultText && stripRoomControlMarkers(resultText)) {
      return false;
    }
  }
  return true;
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
  const append = (roundId: string | null | undefined) => {
    const normalized = roundId?.trim();
    if (!normalized || seen.has(normalized)) {
      return;
    }
    seen.add(normalized);
    roundIds.push(normalized);
  };

  for (const roundId of extraRoundIds) {
    append(roundId);
  }
  for (const roundId of liveRoundIds) {
    append(roundId);
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
