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
import {
  ContentBlock,
  SystemEventTone,
} from "@/types/conversation/message";

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
export const DEFAULT_TIMELINE_DOT_TOP = 12;

export function map_runtime_phase_to_activity_state(
  phase?: AgentConversationRuntimePhase | null,
) {
  switch (phase) {
    case "awaiting_permission":
      return "waiting_permission" as const;
    case "sending":
      return "sending" as const;
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

export function get_system_message_icon_class_name(tone: SystemEventTone): string {
  if (tone === "warning") {
    return "text-amber-700/80";
  }
  return "text-(--icon-muted)";
}

export function get_system_message_label_class_name(tone: SystemEventTone): string {
  if (tone === "warning") {
    return "text-amber-800/80";
  }
  return "text-(--text-muted)";
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
    return "-- --:--";
  }

  const message_date = new Date(timestamp);
  const now = new Date();
  const is_same_year = message_date.getFullYear() === now.getFullYear();

  return message_date.toLocaleString("zh-CN", {
    ...(is_same_year ? {} : { year: "numeric" }),
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

export function get_timeline_anchor_element(content_element: HTMLElement): HTMLElement | null {
  return content_element.querySelector<HTMLElement>("[data-timeline-anchor]")
    ?? content_element.querySelector<HTMLElement>("[data-markdown-anchor], button, li, h1, h2, h3, h4, pre, blockquote, th, td");
}

function get_first_text_line_top(content_element: HTMLElement): number | null {
  const text_walker = document.createTreeWalker(
    content_element,
    NodeFilter.SHOW_TEXT,
    {
      acceptNode(node) {
        return node.textContent?.trim()
          ? NodeFilter.FILTER_ACCEPT
          : NodeFilter.FILTER_SKIP;
      },
    },
  );

  const first_text_node = text_walker.nextNode();
  if (!(first_text_node instanceof Text) || !first_text_node.textContent) {
    return null;
  }

  const range = document.createRange();
  range.selectNodeContents(first_text_node);
  const first_line_rect = range.getClientRects()[0] ?? range.getBoundingClientRect();
  if (!first_line_rect) {
    return null;
  }

  const content_rect = content_element.getBoundingClientRect();
  return first_line_rect.top - content_rect.top + first_line_rect.height / 2;
}

export function get_timeline_anchor_top(content_element: HTMLElement, anchor_element: HTMLElement | null): number {
  if (!anchor_element) {
    return get_first_text_line_top(content_element) ?? DEFAULT_TIMELINE_DOT_TOP;
  }

  const content_rect = content_element.getBoundingClientRect();
  const candidate_rect = anchor_element.getBoundingClientRect();
  const anchor_mode = anchor_element.dataset.timelineAnchorMode;
  if (anchor_mode === "box") {
    return candidate_rect.top - content_rect.top + candidate_rect.height / 2;
  }

  const computed_style = window.getComputedStyle(anchor_element);
  const parsed_line_height = Number.parseFloat(computed_style.lineHeight);
  const anchor_height = Number.isFinite(parsed_line_height) ? parsed_line_height : candidate_rect.height;

  return candidate_rect.top - content_rect.top + anchor_height / 2;
}
