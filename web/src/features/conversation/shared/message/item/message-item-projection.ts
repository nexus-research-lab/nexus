import type { ContentBlock } from "@/types/conversation/message";

export type AssistantContentMode =
  | "dm_archived"
  | "dm_live"
  | "room_result"
  | "room_thread";

export interface OrderedAssistantEntry {
  block: ContentBlock;
  mergedIndex: number;
  sourceMessageId: string;
  sourceOrder: number;
}

export interface AssistantTurnEntry {
  content: ContentBlock[];
  messageId: string;
  streamingIndexes: Set<number>;
  textContent: ContentBlock[];
  textStreamingIndexes: Set<number>;
}

export interface ContentProjection {
  content: ContentBlock[];
  streamingIndexes: Set<number>;
}

export function projectionFromOrderedEntries(
  entries: OrderedAssistantEntry[],
  streamingBlockIndexes: ReadonlySet<number>,
): ContentProjection {
  const content: ContentBlock[] = [];
  const streamingIndexes = new Set<number>();
  entries.forEach((entry, index) => {
    content.push(entry.block);
    if (streamingBlockIndexes.has(entry.mergedIndex)) {
      streamingIndexes.add(index);
    }
  });
  return { content, streamingIndexes };
}
