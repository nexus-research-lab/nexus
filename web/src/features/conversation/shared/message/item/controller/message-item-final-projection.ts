import type {
  AssistantMessage,
  ContentBlock,
  Message,
  ResultSummary,
} from "@/types/conversation/message";
import { getResultSummaryDisplayText } from "./message-item-stats";
import {
  extractTextFromContentBlocks,
  projectionFromOrderedEntries,
  type AssistantContentMode,
  type AssistantTurnEntry,
  type ContentProjection,
  type OrderedAssistantEntry,
} from "../message-item-support";

interface FinalProjectionInput {
  assistantContentMode: AssistantContentMode;
  assistantMessages: Message[];
  orderedProjection: ContentProjection;
  resultSummary: ResultSummary | undefined;
  roundId: string;
  /** 本轮 durable user message id；新协议下顶层 assistant 的 parent_id 指向它。 */
  userMessageId?: string | null;
  streamingBlockIndexes: Set<number>;
  visibleAssistantTurns: AssistantTurnEntry[];
  visibleOrderedAssistantEntries: OrderedAssistantEntry[];
}

export function resolveMessageItemFinalProjection({
  assistantContentMode,
  assistantMessages,
  orderedProjection,
  resultSummary,
  roundId,
  userMessageId,
  streamingBlockIndexes,
  visibleAssistantTurns,
  visibleOrderedAssistantEntries,
}: FinalProjectionInput) {
  const finalAssistantTurn = resolveFinalAssistantTurn(
    assistantMessages,
    roundId,
    userMessageId ?? null,
    visibleAssistantTurns,
  );
  const finalTailEntries = resolveFinalTailEntries(
    finalAssistantTurn,
    visibleOrderedAssistantEntries,
  );
  const archivedProcessProjection = buildArchivedProcessProjection({
    finalAssistantTurn,
    finalTailEntries,
    streamingBlockIndexes,
    visibleOrderedAssistantEntries,
  });
  const fallbackFinalAssistantContent = resolveFallbackFinalAssistantContent(
    finalAssistantTurn,
    finalTailEntries,
  );
  const fallbackFinalAssistantStreamingIndexes =
    resolveFallbackFinalAssistantStreamingIndexes(
      finalAssistantTurn,
      finalTailEntries,
      streamingBlockIndexes,
    );

  const directOrderedProjection =
    assistantContentMode === "dm_live" ||
    assistantContentMode === "room_thread"
      ? orderedProjection
      : emptyProjection();
  const processProjection =
    assistantContentMode === "dm_archived"
      ? archivedProcessProjection
      : emptyProjection();
  const finalAssistantContent = resolveFinalAssistantContent({
    assistantContentMode,
    fallbackFinalAssistantContent,
    finalAssistantTurn,
    finalTailEntries,
    resultSummary,
  });
  const finalAssistantStreamingIndexes =
    assistantContentMode === "dm_live" ||
    assistantContentMode === "room_thread" ||
    typeof finalAssistantContent === "string"
      ? new Set<number>()
      : fallbackFinalAssistantStreamingIndexes;
  const finalAssistantText =
    typeof finalAssistantContent === "string"
      ? finalAssistantContent
      : extractTextFromContentBlocks(finalAssistantContent);

  return {
    directOrderedProjection,
    processProjection,
    finalAssistantContent,
    finalAssistantStreamingIndexes,
    finalAssistantText,
  };
}

function resolveFinalAssistantTurn(
  assistantMessages: Message[],
  roundId: string,
  userMessageId: string | null,
  visibleAssistantTurns: AssistantTurnEntry[],
) {
  // 顶层 assistant 的 parent 指向本轮 user message（旧数据指向 round_id）；
  // 其他 parent（tool_use / slot msg）属于子执行，不能当最终回复。
  const isTopLevelParent = (parentId: string | undefined) =>
    !parentId ||
    parentId === roundId ||
    (userMessageId != null && parentId === userMessageId);
  for (let index = assistantMessages.length - 1; index >= 0; index -= 1) {
    const message = assistantMessages[index] as AssistantMessage;
    if (isTopLevelParent(message.parent_id)) {
      return (
        visibleAssistantTurns.find(
          (turn) => turn.messageId === message.message_id,
        ) ?? null
      );
    }
  }
  return visibleAssistantTurns.at(-1) ?? null;
}

function resolveFinalTailEntries(
  finalAssistantTurn: AssistantTurnEntry | null,
  visibleOrderedAssistantEntries: OrderedAssistantEntry[],
) {
  if (!finalAssistantTurn) {
    return [];
  }

  const tailEntries: OrderedAssistantEntry[] = [];
  for (
    let index = visibleOrderedAssistantEntries.length - 1;
    index >= 0;
    index -= 1
  ) {
    const entry = visibleOrderedAssistantEntries[index];
    if (entry.sourceMessageId !== finalAssistantTurn.messageId) {
      break;
    }
    if (entry.block.type !== "text" || !entry.block.text.trim()) {
      break;
    }
    tailEntries.unshift(entry);
  }
  return tailEntries;
}

function buildArchivedProcessProjection({
  finalAssistantTurn,
  finalTailEntries,
  streamingBlockIndexes,
  visibleOrderedAssistantEntries,
}: {
  finalAssistantTurn: AssistantTurnEntry | null;
  finalTailEntries: OrderedAssistantEntry[];
  streamingBlockIndexes: Set<number>;
  visibleOrderedAssistantEntries: OrderedAssistantEntry[];
}) {
  // 最终回复由独立区域渲染（tail / turn 文本 / result 摘要），
  // 过程链无条件剥离它，避免同一段答案在过程和最终各出现一次。
  if (finalTailEntries.length > 0) {
    const tailIndexes = new Set(
      finalTailEntries.map((entry) => entry.mergedIndex),
    );
    return projectionFromOrderedEntries(
      visibleOrderedAssistantEntries.filter(
        (entry) => !tailIndexes.has(entry.mergedIndex),
      ),
      streamingBlockIndexes,
    );
  }

  if (finalAssistantTurn && finalAssistantTurn.textContent.length > 0) {
    const finalAssistantTextMergedIndexes = textEntryIndexesForTurn(
      finalAssistantTurn,
      visibleOrderedAssistantEntries,
    );
    return projectionFromOrderedEntries(
      visibleOrderedAssistantEntries.filter(
        (entry) =>
          entry.sourceMessageId !== finalAssistantTurn.messageId ||
          !finalAssistantTextMergedIndexes.has(entry.mergedIndex),
      ),
      streamingBlockIndexes,
    );
  }

  return projectionFromOrderedEntries(
    visibleOrderedAssistantEntries,
    streamingBlockIndexes,
  );
}

function resolveFallbackFinalAssistantContent(
  finalAssistantTurn: AssistantTurnEntry | null,
  finalTailEntries: OrderedAssistantEntry[],
) {
  if (finalTailEntries.length > 0) {
    return finalTailEntries.map((entry) => entry.block);
  }
  if (!finalAssistantTurn) {
    return null;
  }
  if (finalAssistantTurn.textContent.length > 0) {
    return finalAssistantTurn.textContent;
  }
  if (finalAssistantTurn.content.length > 0) {
    return finalAssistantTurn.content;
  }
  return null;
}

function resolveFallbackFinalAssistantStreamingIndexes(
  finalAssistantTurn: AssistantTurnEntry | null,
  finalTailEntries: OrderedAssistantEntry[],
  streamingBlockIndexes: Set<number>,
) {
  if (finalTailEntries.length > 0) {
    const nextIndexes = new Set<number>();
    finalTailEntries.forEach((entry, index) => {
      if (streamingBlockIndexes.has(entry.mergedIndex)) {
        nextIndexes.add(index);
      }
    });
    return nextIndexes;
  }
  if (!finalAssistantTurn) {
    return new Set<number>();
  }
  if (finalAssistantTurn.textContent.length > 0) {
    return finalAssistantTurn.textStreamingIndexes;
  }
  return finalAssistantTurn.streamingIndexes;
}

function resolveFinalAssistantContent({
  assistantContentMode,
  fallbackFinalAssistantContent,
  finalAssistantTurn,
  finalTailEntries,
  resultSummary,
}: {
  assistantContentMode: AssistantContentMode;
  fallbackFinalAssistantContent: ContentBlock[] | null;
  finalAssistantTurn: AssistantTurnEntry | null;
  finalTailEntries: OrderedAssistantEntry[];
  resultSummary: ResultSummary | undefined;
}) {
  if (
    assistantContentMode === "dm_live" ||
    assistantContentMode === "room_thread"
  ) {
    return null;
  }

  const resultText = getResultSummaryDisplayText(resultSummary);

  if (assistantContentMode === "dm_archived") {
    // 优先用消息正文（过程链已剥离同一段内容）；
    // result 摘要文本只在正文缺失时兜底，避免两边措辞不一致时重复展示。
    if (finalTailEntries.length > 0) {
      return finalTailEntries.map((entry) => entry.block);
    }
    if (finalAssistantTurn?.textContent.length) {
      return finalAssistantTurn.textContent;
    }
    return resultText || null;
  }

  if (resultText) {
    return resultText;
  }

  return fallbackFinalAssistantContent;
}

function textEntryIndexesForTurn(
  finalAssistantTurn: AssistantTurnEntry,
  visibleOrderedAssistantEntries: OrderedAssistantEntry[],
) {
  const nextIndexes = new Set<number>();
  for (const entry of visibleOrderedAssistantEntries) {
    if (entry.sourceMessageId !== finalAssistantTurn.messageId) {
      continue;
    }
    if (entry.block.type !== "text" || !entry.block.text.trim()) {
      continue;
    }
    nextIndexes.add(entry.mergedIndex);
  }
  return nextIndexes;
}

function emptyProjection(): ContentProjection {
  return { content: [], streamingIndexes: new Set<number>() };
}
