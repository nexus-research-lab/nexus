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
  assistant_content_mode,
  assistant_messages,
  ordered_projection,
  result_summary,
  round_id,
  streaming_block_indexes,
  visible_assistant_turns,
  visible_ordered_assistant_entries,
}: FinalProjectionInput) {
  const final_assistant_turn = resolve_final_assistant_turn(
    assistant_messages,
    round_id,
    visible_assistant_turns,
  );
  const final_tail_entries = resolve_final_tail_entries(
    final_assistant_turn,
    visible_ordered_assistant_entries,
  );
  const archived_process_projection = build_archived_process_projection({
    final_assistant_turn,
    final_tail_entries,
    result_summary,
    streaming_block_indexes,
    visible_ordered_assistant_entries,
  });
  const fallback_final_assistant_content = resolve_fallback_final_assistant_content(
    final_assistant_turn,
    final_tail_entries,
  );
  const fallback_final_assistant_streaming_indexes =
    resolve_fallback_final_assistant_streaming_indexes(
      final_assistant_turn,
      final_tail_entries,
      streaming_block_indexes,
    );

  const direct_ordered_projection =
    assistant_content_mode === "dm_live" ||
    assistant_content_mode === "room_thread"
      ? ordered_projection
      : empty_projection();
  const process_projection =
    assistant_content_mode === "dm_archived"
      ? archived_process_projection
      : empty_projection();
  const final_assistant_content = resolve_final_assistant_content({
    assistant_content_mode,
    fallback_final_assistant_content,
    final_assistant_turn,
    final_tail_entries,
    result_summary,
  });
  const final_assistant_streaming_indexes =
    assistant_content_mode === "dm_live" ||
    assistant_content_mode === "room_thread" ||
    typeof final_assistant_content === "string"
      ? new Set<number>()
      : fallback_final_assistant_streaming_indexes;
  const final_assistant_text =
    typeof final_assistant_content === "string"
      ? final_assistant_content
      : extract_text_from_content_blocks(final_assistant_content);

  return {
    direct_ordered_projection,
    process_projection,
    final_assistant_content,
    final_assistant_streaming_indexes,
    final_assistant_text,
  };
}

function resolve_final_assistant_turn(
  assistant_messages: Message[],
  round_id: string,
  visible_assistant_turns: AssistantTurnEntry[],
) {
  for (let index = assistant_messages.length - 1; index >= 0; index -= 1) {
    const message = assistant_messages[index] as AssistantMessage;
    if (!message.parent_id || message.parent_id === round_id) {
      return (
        visible_assistant_turns.find(
          (turn) => turn.message_id === message.message_id,
        ) ?? null
      );
    }
  }
  return visible_assistant_turns.at(-1) ?? null;
}

function resolve_final_tail_entries(
  final_assistant_turn: AssistantTurnEntry | null,
  visible_ordered_assistant_entries: OrderedAssistantEntry[],
) {
  if (!final_assistant_turn) {
    return [];
  }

  const tail_entries: OrderedAssistantEntry[] = [];
  for (
    let index = visible_ordered_assistant_entries.length - 1;
    index >= 0;
    index -= 1
  ) {
    const entry = visible_ordered_assistant_entries[index];
    if (entry.source_message_id !== final_assistant_turn.message_id) {
      break;
    }
    if (entry.block.type !== "text" || !entry.block.text.trim()) {
      break;
    }
    tail_entries.unshift(entry);
  }
  return tail_entries;
}

function build_archived_process_projection({
  final_assistant_turn,
  final_tail_entries,
  result_summary,
  streaming_block_indexes,
  visible_ordered_assistant_entries,
}: {
  final_assistant_turn: AssistantTurnEntry | null;
  final_tail_entries: OrderedAssistantEntry[];
  result_summary: ResultSummary | undefined;
  streaming_block_indexes: Set<number>;
  visible_ordered_assistant_entries: OrderedAssistantEntry[];
}) {
  const result_text = result_summary?.result?.trim();
  const final_tail_text = text_from_entries(final_tail_entries, "\n\n");
  const should_strip_tail =
    final_tail_entries.length > 0 &&
    (!result_text ||
      final_tail_text === result_text ||
      text_from_entries(final_tail_entries, "").trim() === result_text);

  if (should_strip_tail) {
    const tail_indexes = new Set(
      final_tail_entries.map((entry) => entry.merged_index),
    );
    return projection_from_ordered_entries(
      visible_ordered_assistant_entries.filter(
        (entry) => !tail_indexes.has(entry.merged_index),
      ),
      streaming_block_indexes,
    );
  }

  if (!result_text && final_assistant_turn) {
    const final_assistant_text_merged_indexes =
      final_assistant_turn.text_content.length > 0
        ? text_entry_indexes_for_turn(
          final_assistant_turn,
          visible_ordered_assistant_entries,
        )
        : new Set<number>();
    return projection_from_ordered_entries(
      visible_ordered_assistant_entries.filter(
        (entry) =>
          entry.source_message_id !== final_assistant_turn.message_id ||
          !final_assistant_text_merged_indexes.has(entry.merged_index),
      ),
      streaming_block_indexes,
    );
  }

  return projection_from_ordered_entries(
    visible_ordered_assistant_entries,
    streaming_block_indexes,
  );
}

function resolve_fallback_final_assistant_content(
  final_assistant_turn: AssistantTurnEntry | null,
  final_tail_entries: OrderedAssistantEntry[],
) {
  if (final_tail_entries.length > 0) {
    return final_tail_entries.map((entry) => entry.block);
  }
  if (!final_assistant_turn) {
    return null;
  }
  if (final_assistant_turn.text_content.length > 0) {
    return final_assistant_turn.text_content;
  }
  if (final_assistant_turn.content.length > 0) {
    return final_assistant_turn.content;
  }
  return null;
}

function resolve_fallback_final_assistant_streaming_indexes(
  final_assistant_turn: AssistantTurnEntry | null,
  final_tail_entries: OrderedAssistantEntry[],
  streaming_block_indexes: Set<number>,
) {
  if (final_tail_entries.length > 0) {
    const next_indexes = new Set<number>();
    final_tail_entries.forEach((entry, index) => {
      if (streaming_block_indexes.has(entry.merged_index)) {
        next_indexes.add(index);
      }
    });
    return next_indexes;
  }
  if (!final_assistant_turn) {
    return new Set<number>();
  }
  if (final_assistant_turn.text_content.length > 0) {
    return final_assistant_turn.text_streaming_indexes;
  }
  return final_assistant_turn.streaming_indexes;
}

function resolve_final_assistant_content({
  assistant_content_mode,
  fallback_final_assistant_content,
  final_assistant_turn,
  final_tail_entries,
  result_summary,
}: {
  assistant_content_mode: AssistantContentMode;
  fallback_final_assistant_content: ContentBlock[] | null;
  final_assistant_turn: AssistantTurnEntry | null;
  final_tail_entries: OrderedAssistantEntry[];
  result_summary: ResultSummary | undefined;
}) {
  if (
    assistant_content_mode === "dm_live" ||
    assistant_content_mode === "room_thread"
  ) {
    return null;
  }

  const result_text = get_result_summary_display_text(result_summary);
  if (result_text) {
    return result_text;
  }

  if (assistant_content_mode === "dm_archived") {
    if (final_tail_entries.length > 0) {
      return final_tail_entries.map((entry) => entry.block);
    }
    if (final_assistant_turn?.text_content.length) {
      return final_assistant_turn.text_content;
    }
    return null;
  }

  return fallback_final_assistant_content;
}

function text_entry_indexes_for_turn(
  final_assistant_turn: AssistantTurnEntry,
  visible_ordered_assistant_entries: OrderedAssistantEntry[],
) {
  const next_indexes = new Set<number>();
  for (const entry of visible_ordered_assistant_entries) {
    if (entry.source_message_id !== final_assistant_turn.message_id) {
      continue;
    }
    if (entry.block.type !== "text" || !entry.block.text.trim()) {
      continue;
    }
    next_indexes.add(entry.merged_index);
  }
  return next_indexes;
}

function text_from_entries(entries: OrderedAssistantEntry[], separator: string) {
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

function empty_projection(): ContentProjection {
  return { content: [], streaming_indexes: new Set<number>() };
}
