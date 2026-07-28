/**
 * INPUT: Assistant 内容模式与有序 ContentBlock 条目。
 * OUTPUT: 穷尽模式策略选择的正文 surface、人工介入 owner 及 direct/final 内容投影。
 * POS: MessageItem 跨 controller/view 的纯模式契约与有序内容投影入口。
 */
import type { ContentBlock } from "@/types/conversation/message/content";

export type AssistantContentMode =
  | "dm_archived"
  | "dm_live"
  | "room_result"
  | "room_thread";

export type AssistantResponseSurface = "direct" | "final";
export type PendingInteractionOwner = "composer" | "content" | "list";

interface AssistantContentModePolicy {
  pendingInteractionOwner: PendingInteractionOwner;
  responseSurface: AssistantResponseSurface;
}

const ASSISTANT_CONTENT_MODE_POLICIES: Readonly<Record<
  AssistantContentMode,
  AssistantContentModePolicy
>> = {
  dm_archived: {
    pendingInteractionOwner: "composer",
    responseSurface: "final",
  },
  dm_live: {
    pendingInteractionOwner: "composer",
    responseSurface: "final",
  },
  room_result: {
    pendingInteractionOwner: "list",
    responseSurface: "final",
  },
  room_thread: {
    pendingInteractionOwner: "list",
    responseSurface: "direct",
  },
};

export function resolveAssistantResponseSurface(
  mode: AssistantContentMode,
): AssistantResponseSurface {
  return ASSISTANT_CONTENT_MODE_POLICIES[mode].responseSurface;
}

export function resolvePendingInteractionOwner(
  mode: AssistantContentMode,
): PendingInteractionOwner {
  return ASSISTANT_CONTENT_MODE_POLICIES[mode].pendingInteractionOwner;
}

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
