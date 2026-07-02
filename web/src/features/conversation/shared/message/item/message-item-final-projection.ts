import type {
  AssistantMessage,
  ContentBlock,
  Message,
  ResultSummary,
} from "@/types/conversation/message";
import { get_result_summary_display_text } from "./message-item-stats";
import {
  extract_text_from_content_blocks,
  projection_from_ordered_entries,
  type AssistantContentMode,
  type AssistantTurnEntry,
  type ContentProjection,
  type OrderedAssistantEntry,
} from "./message-item-support";

interface FinalProjectionInput {
  assistant_content_mode: AssistantContentMode;
  assistant_messages: Message[];
  ordered_projection: ContentProjection;
  result_summary: ResultSummary | undefined;
  round_id: string;
  streaming_block_indexes: Set<number>;
  visible_assistant_turns: AssistantTurnEntry[];
  visible_ordered_assistant_entries: OrderedAssistantEntry[];
}

export function resolve_message_item_final_projection({
  assistant_content_mode: assistantContentMode,
  assistant_messages: assistantMessages,
  ordered_projection: orderedProjection,
  result_summary: resultSummary,
  round_id: roundId,
  streaming_block_indexes: streamingBlockIndexes,
  visible_assistant_turns: visibleAssistantTurns,
  visible_ordered_assistant_entries: visibleOrderedAssistantEntries,
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
    final_assistant_turn: finalAssistantTurn,
    final_tail_entries: finalTailEntries,
    result_summary: resultSummary,
    streaming_block_indexes: streamingBlockIndexes,
    visible_ordered_assistant_entries: visibleOrderedAssistantEntries,
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
    assistant_content_mode: assistantContentMode,
    fallback_final_assistant_content: fallbackFinalAssistantContent,
    final_assistant_turn: finalAssistantTurn,
    final_tail_entries: finalTailEntries,
    result_summary: resultSummary,
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
      : extract_text_from_content_blocks(finalAssistantContent);

  return {
    direct_ordered_projection: directOrderedProjection,
    process_projection: processProjection,
    final_assistant_content: finalAssistantContent,
    final_assistant_streaming_indexes: finalAssistantStreamingIndexes,
    final_assistant_text: finalAssistantText,
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
          (turn) => turn.message_id === message.message_id,
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
    if (entry.source_message_id !== finalAssistantTurn.message_id) {
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
  final_assistant_turn: finalAssistantTurn,
  final_tail_entries: finalTailEntries,
  result_summary: resultSummary,
  streaming_block_indexes: streamingBlockIndexes,
  visible_ordered_assistant_entries: visibleOrderedAssistantEntries,
}: {
  final_assistant_turn: AssistantTurnEntry | null;
  final_tail_entries: OrderedAssistantEntry[];
  result_summary: ResultSummary | undefined;
  streaming_block_indexes: Set<number>;
  visible_ordered_assistant_entries: OrderedAssistantEntry[];
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
      finalTailEntries.map((entry) => entry.merged_index),
    );
    return projection_from_ordered_entries(
      visibleOrderedAssistantEntries.filter(
        (entry) => !tailIndexes.has(entry.merged_index),
      ),
      streamingBlockIndexes,
    );
  }

  if (!resultText && finalAssistantTurn) {
    const finalAssistantTextMergedIndexes =
      finalAssistantTurn.text_content.length > 0
        ? textEntryIndexesForTurn(
          finalAssistantTurn,
          visibleOrderedAssistantEntries,
        )
        : new Set<number>();
    return projection_from_ordered_entries(
      visibleOrderedAssistantEntries.filter(
        (entry) =>
          entry.source_message_id !== finalAssistantTurn.message_id ||
          !finalAssistantTextMergedIndexes.has(entry.merged_index),
      ),
      streamingBlockIndexes,
    );
  }

  return projection_from_ordered_entries(
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
  if (finalAssistantTurn.text_content.length > 0) {
    return finalAssistantTurn.text_content;
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
      if (streamingBlockIndexes.has(entry.merged_index)) {
        nextIndexes.add(index);
      }
    });
    return nextIndexes;
  }
  if (!finalAssistantTurn) {
    return new Set<number>();
  }
  if (finalAssistantTurn.text_content.length > 0) {
    return finalAssistantTurn.text_streaming_indexes;
  }
  return finalAssistantTurn.streaming_indexes;
}

function resolveFinalAssistantContent({
  assistant_content_mode: assistantContentMode,
  fallback_final_assistant_content: fallbackFinalAssistantContent,
  final_assistant_turn: finalAssistantTurn,
  final_tail_entries: finalTailEntries,
  result_summary: resultSummary,
}: {
  assistant_content_mode: AssistantContentMode;
  fallback_final_assistant_content: ContentBlock[] | null;
  final_assistant_turn: AssistantTurnEntry | null;
  final_tail_entries: OrderedAssistantEntry[];
  result_summary: ResultSummary | undefined;
}) {
  if (
    assistantContentMode === "dm_live" ||
    assistantContentMode === "room_thread"
  ) {
    return null;
  }

  const resultText = get_result_summary_display_text(resultSummary);
  if (resultText) {
    return resultText;
  }

  if (assistantContentMode === "dm_archived") {
    if (finalTailEntries.length > 0) {
      return finalTailEntries.map((entry) => entry.block);
    }
    if (finalAssistantTurn?.text_content.length) {
      return finalAssistantTurn.text_content;
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
    if (entry.source_message_id !== finalAssistantTurn.message_id) {
      continue;
    }
    if (entry.block.type !== "text" || !entry.block.text.trim()) {
      continue;
    }
    nextIndexes.add(entry.merged_index);
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
  return { content: [], streaming_indexes: new Set<number>() };
}
