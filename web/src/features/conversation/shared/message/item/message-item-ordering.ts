import type {
  ContentBlock,
  SystemEventContent,
} from "@/types/conversation/message";

import {
  split_text_block_by_tool_use_error,
  type AssistantTurnEntry,
  type OrderedAssistantEntry,
} from "./message-item-support";

export function build_visible_ordered_assistant_entries({
  hidden_tool_names: hiddenToolNames,
  hidden_tool_use_ids: hiddenToolUseIds,
  is_loading: isLoading,
  merged_content: mergedContent,
  merged_content_source_message_ids: mergedContentSourceMessageIds,
  source_message_order_by_id: sourceMessageOrderById,
  system_event_blocks: systemEventBlocks,
}: {
  hidden_tool_names: string[];
  hidden_tool_use_ids: ReadonlySet<string>;
  is_loading?: boolean;
  merged_content: ContentBlock[];
  merged_content_source_message_ids: string[];
  source_message_order_by_id: ReadonlyMap<string, number>;
  system_event_blocks: SystemEventContent[];
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
      const splitBlocks = split_text_block_by_tool_use_error(block);
      splitBlocks.forEach((splitBlock) => {
        assistantEntries.push({
          block: splitBlock,
          merged_index: mergedIndex,
          source_message_id: sourceMessageId,
          source_order: sourceOrder,
        });
      });
      return;
    }

    if (block.type === "thinking") {
      if (block.thinking?.trim()) {
        assistantEntries.push({
          block,
          merged_index: mergedIndex,
          source_message_id: sourceMessageId,
          source_order: sourceOrder,
        });
      }
      return;
    }

    if (block.type === "tool_use") {
      if (!hiddenToolNames.includes(block.name)) {
        assistantEntries.push({
          block,
          merged_index: mergedIndex,
          source_message_id: sourceMessageId,
          source_order: sourceOrder,
        });
      }
      return;
    }

    if (block.type === "tool_result") {
      if (!hiddenToolUseIds.has(block.tool_use_id)) {
        assistantEntries.push({
          block,
          merged_index: mergedIndex,
          source_message_id: sourceMessageId,
          source_order: sourceOrder,
        });
      }
      return;
    }

    if (block.type === "task_progress") {
      if (shouldShowTaskProgressInline) {
        assistantEntries.push({
          block,
          merged_index: mergedIndex,
          source_message_id: sourceMessageId,
          source_order: sourceOrder,
        });
      }
      return;
    }

    if (block.type === "tool_use_error") {
      if (block.content.trim()) {
        assistantEntries.push({
          block,
          merged_index: mergedIndex,
          source_message_id: sourceMessageId,
          source_order: sourceOrder,
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
        merged_index: -1,
        source_message_id: block.source_message_id,
        source_order: resolveSourceOrder(block.source_message_id),
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
      merged_index: -1,
      source_message_id: block.source_message_id,
      source_order: resolveSourceOrder(block.source_message_id),
    }))
    .sort((left, right) => {
      if (left.source_order !== right.source_order) {
        return left.source_order - right.source_order;
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
      unmatchedOrderedEntries[systemIndex].source_order <
        entry.source_order
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

export function build_visible_assistant_turns({
  assistant_messages: assistantMessages,
  streaming_block_indexes: streamingBlockIndexes,
  visible_ordered_assistant_entries: visibleOrderedAssistantEntries,
}: {
  assistant_messages: Array<{ message_id: string }>;
  streaming_block_indexes: ReadonlySet<number>;
  visible_ordered_assistant_entries: OrderedAssistantEntry[];
}): AssistantTurnEntry[] {
  const turnMap = new Map<string, AssistantTurnEntry>();
  assistantMessages.forEach((message) => {
    turnMap.set(message.message_id, {
      message_id: message.message_id,
      content: [],
      text_content: [],
      streaming_indexes: new Set<number>(),
      text_streaming_indexes: new Set<number>(),
    });
  });

  visibleOrderedAssistantEntries.forEach((entry) => {
    const turn = turnMap.get(entry.source_message_id);
    if (!turn) {
      return;
    }

    const contentIndex = turn.content.length;
    turn.content.push(entry.block);
    if (streamingBlockIndexes.has(entry.merged_index)) {
      turn.streaming_indexes.add(contentIndex);
    }

    if (entry.block.type === "text" && entry.block.text.trim()) {
      const textIndex = turn.text_content.length;
      turn.text_content.push(entry.block);
      if (streamingBlockIndexes.has(entry.merged_index)) {
        turn.text_streaming_indexes.add(textIndex);
      }
    }
  });

  return assistantMessages
    .map((message) => turnMap.get(message.message_id))
    .filter((turn): turn is AssistantTurnEntry =>
      Boolean(turn && turn.content.length > 0),
    );
}
