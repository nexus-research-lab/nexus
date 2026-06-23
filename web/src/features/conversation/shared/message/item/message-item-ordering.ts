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
  hidden_tool_names,
  hidden_tool_use_ids,
  is_loading,
  merged_content,
  merged_content_source_message_ids,
  source_message_order_by_id,
  system_event_blocks,
}: {
  hidden_tool_names: string[];
  hidden_tool_use_ids: ReadonlySet<string>;
  is_loading?: boolean;
  merged_content: ContentBlock[];
  merged_content_source_message_ids: string[];
  source_message_order_by_id: ReadonlyMap<string, number>;
  system_event_blocks: SystemEventContent[];
}): OrderedAssistantEntry[] {
  const assistant_entries: OrderedAssistantEntry[] = [];
  const should_show_task_progress_inline =
    is_loading ||
    !merged_content.some(
      (block) => block.type === "text" && Boolean(block.text.trim()),
    );
  const resolve_source_order = (source_message_id: string) =>
    source_message_order_by_id.get(source_message_id) ??
    Number.MAX_SAFE_INTEGER;

  merged_content.forEach((block, merged_index) => {
    const source_message_id =
      merged_content_source_message_ids[merged_index] || "";
    const source_order = resolve_source_order(source_message_id);

    if (block.type === "text") {
      const split_blocks = split_text_block_by_tool_use_error(block);
      split_blocks.forEach((split_block) => {
        assistant_entries.push({
          block: split_block,
          merged_index,
          source_message_id,
          source_order,
        });
      });
      return;
    }

    if (block.type === "thinking") {
      if (block.thinking?.trim()) {
        assistant_entries.push({
          block,
          merged_index,
          source_message_id,
          source_order,
        });
      }
      return;
    }

    if (block.type === "tool_use") {
      if (!hidden_tool_names.includes(block.name)) {
        assistant_entries.push({
          block,
          merged_index,
          source_message_id,
          source_order,
        });
      }
      return;
    }

    if (block.type === "tool_result") {
      if (!hidden_tool_use_ids.has(block.tool_use_id)) {
        assistant_entries.push({
          block,
          merged_index,
          source_message_id,
          source_order,
        });
      }
      return;
    }

    if (block.type === "task_progress") {
      if (should_show_task_progress_inline) {
        assistant_entries.push({
          block,
          merged_index,
          source_message_id,
          source_order,
        });
      }
      return;
    }

    if (block.type === "tool_use_error") {
      if (block.content.trim()) {
        assistant_entries.push({
          block,
          merged_index,
          source_message_id,
          source_order,
        });
      }
    }
  });

  const ordered_entries: OrderedAssistantEntry[] = [];
  const system_blocks_by_tool_use_id = new Map<
    string,
    SystemEventContent[]
  >();
  const unmatched_system_blocks: SystemEventContent[] = [];

  system_event_blocks.forEach((block) => {
    if (block.tool_use_id) {
      const existing_blocks =
        system_blocks_by_tool_use_id.get(block.tool_use_id) ?? [];
      existing_blocks.push(block);
      system_blocks_by_tool_use_id.set(block.tool_use_id, existing_blocks);
      return;
    }
    unmatched_system_blocks.push(block);
  });

  assistant_entries.forEach((entry) => {
    ordered_entries.push(entry);
    if (entry.block.type !== "tool_use") {
      return;
    }

    const matched_system_blocks = system_blocks_by_tool_use_id.get(
      entry.block.id,
    );
    if (!matched_system_blocks?.length) {
      return;
    }

    matched_system_blocks.forEach((block) => {
      ordered_entries.push({
        block,
        merged_index: -1,
        source_message_id: block.source_message_id,
        source_order: resolve_source_order(block.source_message_id),
      });
    });
    system_blocks_by_tool_use_id.delete(entry.block.id);
  });

  system_blocks_by_tool_use_id.forEach((blocks) => {
    unmatched_system_blocks.push(...blocks);
  });
  const unmatched_ordered_entries = unmatched_system_blocks
    .map((block) => ({
      block,
      merged_index: -1,
      source_message_id: block.source_message_id,
      source_order: resolve_source_order(block.source_message_id),
    }))
    .sort((left, right) => {
      if (left.source_order !== right.source_order) {
        return left.source_order - right.source_order;
      }
      const left_timestamp =
        left.block.type === "system_event" ? left.block.timestamp : 0;
      const right_timestamp =
        right.block.type === "system_event" ? right.block.timestamp : 0;
      return left_timestamp - right_timestamp;
    });

  if (unmatched_ordered_entries.length === 0) {
    return ordered_entries;
  }

  const merged_entries: OrderedAssistantEntry[] = [];
  let system_index = 0;
  ordered_entries.forEach((entry) => {
    while (
      system_index < unmatched_ordered_entries.length &&
      unmatched_ordered_entries[system_index].source_order <
        entry.source_order
    ) {
      merged_entries.push(unmatched_ordered_entries[system_index]);
      system_index += 1;
    }
    merged_entries.push(entry);
  });
  while (system_index < unmatched_ordered_entries.length) {
    merged_entries.push(unmatched_ordered_entries[system_index]);
    system_index += 1;
  }

  return merged_entries;
}

export function build_visible_assistant_turns({
  assistant_messages,
  streaming_block_indexes,
  visible_ordered_assistant_entries,
}: {
  assistant_messages: Array<{ message_id: string }>;
  streaming_block_indexes: ReadonlySet<number>;
  visible_ordered_assistant_entries: OrderedAssistantEntry[];
}): AssistantTurnEntry[] {
  const turn_map = new Map<string, AssistantTurnEntry>();
  assistant_messages.forEach((message) => {
    turn_map.set(message.message_id, {
      message_id: message.message_id,
      content: [],
      text_content: [],
      streaming_indexes: new Set<number>(),
      text_streaming_indexes: new Set<number>(),
    });
  });

  visible_ordered_assistant_entries.forEach((entry) => {
    const turn = turn_map.get(entry.source_message_id);
    if (!turn) {
      return;
    }

    const content_index = turn.content.length;
    turn.content.push(entry.block);
    if (streaming_block_indexes.has(entry.merged_index)) {
      turn.streaming_indexes.add(content_index);
    }

    if (entry.block.type === "text" && entry.block.text.trim()) {
      const text_index = turn.text_content.length;
      turn.text_content.push(entry.block);
      if (streaming_block_indexes.has(entry.merged_index)) {
        turn.text_streaming_indexes.add(text_index);
      }
    }
  });

  return assistant_messages
    .map((message) => turn_map.get(message.message_id))
    .filter((turn): turn is AssistantTurnEntry =>
      Boolean(turn && turn.content.length > 0),
    );
}
