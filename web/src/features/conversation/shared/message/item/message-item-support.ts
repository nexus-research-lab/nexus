/**
 * =====================================================
 * @File   ：message-item-support.ts
 * @Date   ：2026-04-15 18:25
 * @Author ：leemysw
 * 2026-04-15 18:25   Create
 * =====================================================
 */

import { AgentConversationRuntimePhase } from "@/types/agent/agent-conversation";
import { is_ask_user_question_timed_out_result } from "@/types/conversation/ask-user-question";
import { ContentBlock } from "@/types/conversation/message";

export interface OrderedAssistantEntry {
  block: ContentBlock;
  merged_index: number;
  source_message_id: string;
}

export interface AssistantTurnEntry {
  message_id: string;
  content: ContentBlock[];
  text_content: ContentBlock[];
  streaming_indexes: Set<number>;
  text_streaming_indexes: Set<number>;
}

export interface ContentProjection {
  content: ContentBlock[];
  streaming_indexes: Set<number>;
}

export type AssistantContentMode = "dm_live" | "dm_archived" | "room_thread" | "room_result";

export function map_runtime_phase_to_activity_state(
  phase?: AgentConversationRuntimePhase | null,
) {
  switch (phase) {
    case "awaiting_permission":
      return "waiting_permission" as const;
    case "queued":
    case "running":
      return "thinking" as const;
    case "streaming":
      return "replying" as const;
    default:
      return null;
  }
}

export function find_latest_streaming_block(
  content: ContentBlock[],
  streaming_block_indexes: ReadonlySet<number>,
): ContentBlock | null {
  const indexes = Array.from(streaming_block_indexes).sort((left, right) => right - left);
  for (const index of indexes) {
    const block = content[index];
    if (!block) {
      continue;
    }
    if (block.type === "text" && !block.text.trim()) {
      continue;
    }
    if (block.type === "thinking" && !block.thinking.trim()) {
      continue;
    }
    return block;
  }
  return null;
}

export function has_timed_out_ask_user_question(content: ContentBlock[]): boolean {
  const ask_tool_use_ids = new Set<string>();

  for (const block of content) {
    if (block.type === "tool_use" && block.name === "AskUserQuestion") {
      ask_tool_use_ids.add(block.id);
    }
  }

  for (const block of content) {
    if (block.type !== "tool_result" || !block.is_error) {
      continue;
    }
    if (!ask_tool_use_ids.has(block.tool_use_id)) {
      continue;
    }
    if (is_ask_user_question_timed_out_result(block)) {
      return true;
    }
  }

  return false;
}

export function get_system_message_container_class_name(tone: "neutral" | "warning"): string {
  if (tone === "warning") {
    return "border border-amber-200/60 bg-amber-50/70 text-amber-950/88";
  }
  return "border border-(--surface-panel-subtle-border) bg-(--surface-inset-background) text-(--text-default)";
}

export function get_system_message_icon_class_name(tone: "neutral" | "warning"): string {
  if (tone === "warning") {
    return "text-amber-700/80";
  }
  return "text-(--icon-muted)";
}

export function projection_from_ordered_entries(
  entries: OrderedAssistantEntry[],
  streaming_block_indexes: Set<number>,
): ContentProjection {
  const content: ContentBlock[] = [];
  const streaming_indexes = new Set<number>();

  entries.forEach((entry, index) => {
    content.push(entry.block);
    if (streaming_block_indexes.has(entry.merged_index)) {
      streaming_indexes.add(index);
    }
  });

  return { content, streaming_indexes };
}

export function extract_text_from_content_blocks(content?: ContentBlock[] | null): string {
  if (!content || content.length === 0) {
    return "";
  }

  const texts: string[] = [];
  content.forEach((block) => {
    if (block.type === "text" && block.text.trim()) {
      texts.push(block.text);
    }
  });
  return texts.join("\n\n");
}

export function format_message_time(timestamp?: number | null): string {
  if (!timestamp) {
    return "--:--";
  }
  return new Date(timestamp).toLocaleTimeString("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
  });
}
