import type {
  ContentBlock,
  SystemEventContent,
} from "@/types/conversation/message";

import {
  splitTextBlockByToolUseError,
  type AssistantTurnEntry,
  type OrderedAssistantEntry,
} from "../message-item-support";

export function buildVisibleOrderedAssistantEntries({
  hiddenToolNames,
  hiddenToolUseIds,
  isLoading,
  mergedContent,
  mergedContentSourceMessageIds,
  sourceMessageOrderById,
  systemEventBlocks,
}: {
  hiddenToolNames: ReadonlySet<string>;
  hiddenToolUseIds: ReadonlySet<string>;
  isLoading?: boolean;
  mergedContent: ContentBlock[];
  mergedContentSourceMessageIds: string[];
  sourceMessageOrderById: ReadonlyMap<string, number>;
  systemEventBlocks: SystemEventContent[];
}): OrderedAssistantEntry[] {
  const assistantEntries: OrderedAssistantEntry[] = [];
  const shouldShowTaskProgressInline =
    isLoading ||
    !mergedContent.some(
      (block) => block.type === "text" && Boolean(block.text.trim()),
    );
  const resolveSourceOrder = (sourceMessageId: string) =>
    sourceMessageOrderById.get(sourceMessageId) ??
    Number.MAX_SAFE_INTEGER;

  mergedContent.forEach((block, mergedIndex) => {
    const sourceMessageId =
      mergedContentSourceMessageIds[mergedIndex] || "";
    const sourceOrder = resolveSourceOrder(sourceMessageId);

    if (block.type === "text") {
      const splitBlocks = splitTextBlockByToolUseError(block);
      splitBlocks.forEach((splitBlock) => {
        assistantEntries.push({
          block: splitBlock,
          mergedIndex,
          sourceMessageId,
          sourceOrder,
        });
      });
      return;
    }

    if (block.type === "thinking") {
      if (block.thinking?.trim()) {
        assistantEntries.push({
          block,
          mergedIndex,
          sourceMessageId,
          sourceOrder,
        });
      }
      return;
    }

    if (block.type === "tool_use") {
      if (!hiddenToolNames.has(block.name)) {
        assistantEntries.push({
          block,
          mergedIndex,
          sourceMessageId,
          sourceOrder,
        });
      }
      return;
    }

    if (block.type === "tool_result") {
      if (!hiddenToolUseIds.has(block.tool_use_id)) {
        assistantEntries.push({
          block,
          mergedIndex,
          sourceMessageId,
          sourceOrder,
        });
      }
      return;
    }

    if (block.type === "task_progress") {
      if (shouldShowTaskProgressInline) {
        assistantEntries.push({
          block,
          mergedIndex,
          sourceMessageId,
          sourceOrder,
        });
      }
      return;
    }

    if (block.type === "tool_use_error") {
      if (block.content.trim()) {
        assistantEntries.push({
          block,
          mergedIndex,
          sourceMessageId,
          sourceOrder,
        });
      }
    }
  });

  const orderedEntries: OrderedAssistantEntry[] = [];
  const systemBlocksByToolUseId = new Map<
    string,
    SystemEventContent[]
  >();
  const unmatchedSystemBlocks: SystemEventContent[] = [];

  systemEventBlocks.forEach((block) => {
    if (block.tool_use_id) {
      const existingBlocks =
        systemBlocksByToolUseId.get(block.tool_use_id) ?? [];
      existingBlocks.push(block);
      systemBlocksByToolUseId.set(block.tool_use_id, existingBlocks);
      return;
    }
    unmatchedSystemBlocks.push(block);
  });

  assistantEntries.forEach((entry) => {
    orderedEntries.push(entry);
    if (entry.block.type !== "tool_use") {
      return;
    }

    const matchedSystemBlocks = systemBlocksByToolUseId.get(
      entry.block.id,
    );
    if (!matchedSystemBlocks?.length) {
      return;
    }

    matchedSystemBlocks.forEach((block) => {
      orderedEntries.push({
        block,
        mergedIndex: -1,
        sourceMessageId: block.source_message_id,
        sourceOrder: resolveSourceOrder(block.source_message_id),
      });
    });
    systemBlocksByToolUseId.delete(entry.block.id);
  });

  systemBlocksByToolUseId.forEach((blocks) => {
    unmatchedSystemBlocks.push(...blocks);
  });
  const unmatchedOrderedEntries = unmatchedSystemBlocks
    .map((block) => ({
      block,
      mergedIndex: -1,
      sourceMessageId: block.source_message_id,
      sourceOrder: resolveSourceOrder(block.source_message_id),
    }))
    .sort((left, right) => {
      if (left.sourceOrder !== right.sourceOrder) {
        return left.sourceOrder - right.sourceOrder;
      }
      const leftTimestamp =
        left.block.type === "system_event" ? left.block.timestamp : 0;
      const rightTimestamp =
        right.block.type === "system_event" ? right.block.timestamp : 0;
      return leftTimestamp - rightTimestamp;
    });

  if (unmatchedOrderedEntries.length === 0) {
    return orderedEntries;
  }

  const mergedEntries: OrderedAssistantEntry[] = [];
  let systemIndex = 0;
  orderedEntries.forEach((entry) => {
    while (
      systemIndex < unmatchedOrderedEntries.length &&
      unmatchedOrderedEntries[systemIndex].sourceOrder <
        entry.sourceOrder
    ) {
      mergedEntries.push(unmatchedOrderedEntries[systemIndex]);
      systemIndex += 1;
    }
    mergedEntries.push(entry);
  });
  while (systemIndex < unmatchedOrderedEntries.length) {
    mergedEntries.push(unmatchedOrderedEntries[systemIndex]);
    systemIndex += 1;
  }

  return mergedEntries;
}

export function buildVisibleAssistantTurns({
  assistantMessages,
  streamingBlockIndexes,
  visibleOrderedAssistantEntries,
}: {
  assistantMessages: Array<{ message_id: string }>;
  streamingBlockIndexes: ReadonlySet<number>;
  visibleOrderedAssistantEntries: OrderedAssistantEntry[];
}): AssistantTurnEntry[] {
  const turnMap = new Map<string, AssistantTurnEntry>();
  assistantMessages.forEach((message) => {
    turnMap.set(message.message_id, {
      messageId: message.message_id,
      content: [],
      textContent: [],
      streamingIndexes: new Set<number>(),
      textStreamingIndexes: new Set<number>(),
    });
  });

  visibleOrderedAssistantEntries.forEach((entry) => {
    const turn = turnMap.get(entry.sourceMessageId);
    if (!turn) {
      return;
    }

    const contentIndex = turn.content.length;
    turn.content.push(entry.block);
    if (streamingBlockIndexes.has(entry.mergedIndex)) {
      turn.streamingIndexes.add(contentIndex);
    }

    if (entry.block.type === "text" && entry.block.text.trim()) {
      const textIndex = turn.textContent.length;
      turn.textContent.push(entry.block);
      if (streamingBlockIndexes.has(entry.mergedIndex)) {
        turn.textStreamingIndexes.add(textIndex);
      }
    }
  });

  return assistantMessages
    .map((message) => turnMap.get(message.message_id))
    .filter((turn): turn is AssistantTurnEntry =>
      Boolean(turn && turn.content.length > 0),
    );
}
