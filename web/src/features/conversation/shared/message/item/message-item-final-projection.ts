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
} from "./message-item-support";

interface FinalProjectionInput {
  assistantContentMode: AssistantContentMode;
  assistantMessages: Message[];
  orderedProjection: ContentProjection;
  resultSummary: ResultSummary | undefined;
  roundId: string;
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
  streamingBlockIndexes,
  visibleAssistantTurns,
  visibleOrderedAssistantEntries,
}: FinalProjectionInput) {
  const finalAssistantTurn = resolveFinalAssistantTurn(
    assistantMessages,
    roundId,
    visibleAssistantTurns,
  );
  const finalTailEntries = resolveFinalTailEntries(
    finalAssistantTurn,
    visibleOrderedAssistantEntries,
  );
  const archivedProcessProjection = buildArchivedProcessProjection({
    finalAssistantTurn,
    finalTailEntries,
    resultSummary,
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
  visibleAssistantTurns: AssistantTurnEntry[],
) {
  for (let index = assistantMessages.length - 1; index >= 0; index -= 1) {
    const message = assistantMessages[index] as AssistantMessage;
    if (!message.parent_id || message.parent_id === roundId) {
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
  resultSummary,
  streamingBlockIndexes,
  visibleOrderedAssistantEntries,
}: {
  finalAssistantTurn: AssistantTurnEntry | null;
  finalTailEntries: OrderedAssistantEntry[];
  resultSummary: ResultSummary | undefined;
  streamingBlockIndexes: Set<number>;
  visibleOrderedAssistantEntries: OrderedAssistantEntry[];
}) {
  const resultText = resultSummary?.result?.trim();
  const finalTailText = textFromEntries(finalTailEntries, "\n\n");
  const shouldStripTail =
    finalTailEntries.length > 0 &&
    (!resultText ||
      finalTailText === resultText ||
      textFromEntries(finalTailEntries, "").trim() === resultText);

  if (shouldStripTail) {
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

  if (!resultText && finalAssistantTurn) {
    const finalAssistantTextMergedIndexes =
      finalAssistantTurn.textContent.length > 0
        ? textEntryIndexesForTurn(
          finalAssistantTurn,
          visibleOrderedAssistantEntries,
        )
        : new Set<number>();
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
  if (resultText) {
    return resultText;
  }

  if (assistantContentMode === "dm_archived") {
    if (finalTailEntries.length > 0) {
      return finalTailEntries.map((entry) => entry.block);
    }
    if (finalAssistantTurn?.textContent.length) {
      return finalAssistantTurn.textContent;
    }
    return null;
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

function textFromEntries(entries: OrderedAssistantEntry[], separator: string) {
  return entries
    .map((entry) => entry.block)
    .filter(
      (block): block is Extract<ContentBlock, { type: "text" }> =>
        block.type === "text",
    )
    .map((block) => block.text)
    .join(separator)
    .trim();
}

function emptyProjection(): ContentProjection {
  return { content: [], streamingIndexes: new Set<number>() };
}
