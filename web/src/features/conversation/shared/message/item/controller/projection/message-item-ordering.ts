import type {
  ContentBlock,
  SystemEventContent,
} from "@/types/conversation/message/content";

import { splitTextBlockByToolUseError } from "../../../message-content-model";
import type {
  AssistantTurnEntry,
  OrderedAssistantEntry,
} from "../../message-item-projection";

interface BlockProjectionContext {
  hiddenToolNames: ReadonlySet<string>;
  hiddenToolUseIds: ReadonlySet<string>;
  showTaskProgress: boolean;
}

type BlockProjector = (
  block: ContentBlock,
  context: BlockProjectionContext,
) => ContentBlock[] | null;

const BLOCK_PROJECTORS: BlockProjector[] = [
  (block) => block.type === "text"
    ? splitTextBlockByToolUseError(block)
    : null,
  (block) => block.type === "thinking"
    ? (block.thinking.trim() ? [block] : [])
    : null,
  (block, context) => block.type === "tool_use"
    ? (context.hiddenToolNames.has(block.name) ? [] : [block])
    : null,
  (block, context) => block.type === "tool_result"
    ? (context.hiddenToolUseIds.has(block.tool_use_id) ? [] : [block])
    : null,
  (block, context) => block.type === "task_progress"
    ? (context.showTaskProgress ? [block] : [])
    : null,
  (block) => block.type === "tool_use_error"
    ? (block.content.trim() ? [block] : [])
    : null,
];

type ResolveSourceOrder = (sourceMessageId: string) => number;

interface SystemEventPartition {
  byToolUseId: Map<string, SystemEventContent[]>;
  unmatched: SystemEventContent[];
}

interface AttachedSystemEvents {
  entries: OrderedAssistantEntry[];
  unmatched: SystemEventContent[];
}

interface OrderedSystemEventEntry extends OrderedAssistantEntry {
  block: SystemEventContent;
}

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
  const resolveSourceOrder: ResolveSourceOrder = (sourceMessageId) =>
    sourceMessageOrderById.get(sourceMessageId) ?? Number.MAX_SAFE_INTEGER;
  const projectionContext: BlockProjectionContext = {
    hiddenToolNames,
    hiddenToolUseIds,
    showTaskProgress: Boolean(isLoading) || !hasVisibleText(mergedContent),
  };
  const assistantEntries = projectAssistantEntries({
    context: projectionContext,
    mergedContent,
    mergedContentSourceMessageIds,
    resolveSourceOrder,
  });
  const attached = attachSystemEvents(
    assistantEntries,
    partitionSystemEvents(systemEventBlocks),
    resolveSourceOrder,
  );
  const unmatchedEntries = orderSystemEventEntries(
    attached.unmatched,
    resolveSourceOrder,
  );
  return mergeEntriesBySourceOrder(attached.entries, unmatchedEntries);
}

function hasVisibleText(content: ContentBlock[]): boolean {
  return content.some(
    (block) => block.type === "text" && Boolean(block.text.trim()),
  );
}

function projectAssistantEntries({
  context,
  mergedContent,
  mergedContentSourceMessageIds,
  resolveSourceOrder,
}: {
  context: BlockProjectionContext;
  mergedContent: ContentBlock[];
  mergedContentSourceMessageIds: string[];
  resolveSourceOrder: ResolveSourceOrder;
}): OrderedAssistantEntry[] {
  return mergedContent.flatMap((block, mergedIndex) => {
    const sourceMessageId = mergedContentSourceMessageIds[mergedIndex] ?? "";
    const sourceOrder = resolveSourceOrder(sourceMessageId);
    return projectVisibleBlocks(block, context).map((visibleBlock) => ({
      block: visibleBlock,
      mergedIndex,
      sourceMessageId,
      sourceOrder,
    }));
  });
}

function projectVisibleBlocks(
  block: ContentBlock,
  context: BlockProjectionContext,
): ContentBlock[] {
  for (const projectBlock of BLOCK_PROJECTORS) {
    const projected = projectBlock(block, context);
    if (projected !== null) {
      return projected;
    }
  }
  return [];
}

function partitionSystemEvents(
  blocks: SystemEventContent[],
): SystemEventPartition {
  const partition: SystemEventPartition = {
    byToolUseId: new Map(),
    unmatched: [],
  };
  for (const block of blocks) {
    if (!block.tool_use_id) {
      partition.unmatched.push(block);
      continue;
    }
    const matchedBlocks = partition.byToolUseId.get(block.tool_use_id) ?? [];
    matchedBlocks.push(block);
    partition.byToolUseId.set(block.tool_use_id, matchedBlocks);
  }
  return partition;
}

function attachSystemEvents(
  assistantEntries: OrderedAssistantEntry[],
  partition: SystemEventPartition,
  resolveSourceOrder: ResolveSourceOrder,
): AttachedSystemEvents {
  const entries: OrderedAssistantEntry[] = [];
  for (const entry of assistantEntries) {
    entries.push(entry);
    if (entry.block.type !== "tool_use") {
      continue;
    }

    const matchedBlocks = partition.byToolUseId.get(entry.block.id) ?? [];
    entries.push(...matchedBlocks.map(
      (block) => createSystemEventEntry(block, resolveSourceOrder),
    ));
    partition.byToolUseId.delete(entry.block.id);
  }

  const unmatched = [...partition.unmatched];
  partition.byToolUseId.forEach((blocks) => unmatched.push(...blocks));
  return { entries, unmatched };
}

function createSystemEventEntry(
  block: SystemEventContent,
  resolveSourceOrder: ResolveSourceOrder,
): OrderedSystemEventEntry {
  return {
    block,
    mergedIndex: -1,
    sourceMessageId: block.source_message_id,
    sourceOrder: resolveSourceOrder(block.source_message_id),
  };
}

function orderSystemEventEntries(
  blocks: SystemEventContent[],
  resolveSourceOrder: ResolveSourceOrder,
): OrderedSystemEventEntry[] {
  return blocks
    .map((block) => createSystemEventEntry(block, resolveSourceOrder))
    .sort((left, right) =>
      left.sourceOrder - right.sourceOrder
      || left.block.timestamp - right.block.timestamp,
    );
}

function mergeEntriesBySourceOrder(
  entries: OrderedAssistantEntry[],
  systemEntries: OrderedSystemEventEntry[],
): OrderedAssistantEntry[] {
  if (systemEntries.length === 0) {
    return entries;
  }

  const mergedEntries: OrderedAssistantEntry[] = [];
  let systemIndex = 0;
  for (const entry of entries) {
    while (
      systemIndex < systemEntries.length
      && systemEntries[systemIndex].sourceOrder < entry.sourceOrder
    ) {
      mergedEntries.push(systemEntries[systemIndex]);
      systemIndex += 1;
    }
    mergedEntries.push(entry);
  }
  mergedEntries.push(...systemEntries.slice(systemIndex));
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
