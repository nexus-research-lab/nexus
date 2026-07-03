/**
 * =====================================================
 * @File   ：message-item-support.ts
 * @Date   ：2026-04-15 18:25
 * @Author ：leemysw
 * 2026-04-15 18:25   Create
 * =====================================================
 */

import { AgentConversationRuntimePhase } from "@/types/agent/agent-conversation";
import { isAskUserQuestionTimedOutResult } from "@/types/conversation/ask-user-question";
import {
  ContentBlock,
  SystemEventTone,
  TextContent,
} from "@/types/conversation/message";

export interface OrderedAssistantEntry {
  block: ContentBlock;
  mergedIndex: number;
  sourceMessageId: string;
  sourceOrder: number;
}

export interface AssistantTurnEntry {
  messageId: string;
  content: ContentBlock[];
  textContent: ContentBlock[];
  streamingIndexes: Set<number>;
  textStreamingIndexes: Set<number>;
}

export interface ContentProjection {
  content: ContentBlock[];
  streamingIndexes: Set<number>;
}

const TOOL_USE_ERROR_TAG_PATTERN =
  /<tool_use_error>([\s\S]*?)<\/tool_use_error>/g;

export function splitTextBlockByToolUseError(
  block: TextContent,
): ContentBlock[] {
  if (!block.text.includes("<tool_use_error>")) {
    return [block];
  }

  const blocks: ContentBlock[] = [];
  let cursor = 0;
  for (const match of block.text.matchAll(TOOL_USE_ERROR_TAG_PATTERN)) {
    const index = match.index ?? 0;
    const text = block.text.slice(cursor, index);
    if (text.trim()) {
      blocks.push({ type: "text", text });
    }
    const content = (match[1] ?? "").trim();
    if (content) {
      blocks.push({ type: "tool_use_error", content });
    }
    cursor = index + match[0].length;
  }

  const tail = block.text.slice(cursor);
  if (tail.trim()) {
    blocks.push({ type: "text", text: tail });
  }
  return blocks.length > 0 ? blocks : [];
}

export type AssistantContentMode =
  | "dm_live"
  | "dm_archived"
  | "room_thread"
  | "room_result";
export const DEFAULT_TIMELINE_DOT_TOP = 12;

export function mapRuntimePhaseToActivityState(
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

export function findLatestStreamingBlock(
  content: ContentBlock[],
  streamingBlockIndexes: ReadonlySet<number>,
): ContentBlock | null {
  const indexes = Array.from(streamingBlockIndexes).sort(
    (left, right) => right - left,
  );
  for (const index of indexes) {
    const block = content[index];
    if (!block) {
      continue;
    }
    if (block.type === "text" && !block.text.trim()) {
      continue;
    }
    if (block.type === "tool_use_error" && !block.content.trim()) {
      continue;
    }
    if (block.type === "thinking" && !block.thinking.trim()) {
      continue;
    }
    return block;
  }
  return null;
}

export function hasTimedOutAskUserQuestion(
  content: ContentBlock[],
): boolean {
  const askToolUseIds = new Set<string>();

  for (const block of content) {
    if (block.type === "tool_use" && block.name === "AskUserQuestion") {
      askToolUseIds.add(block.id);
    }
  }

  for (const block of content) {
    if (block.type !== "tool_result" || !block.is_error) {
      continue;
    }
    if (!askToolUseIds.has(block.tool_use_id)) {
      continue;
    }
    if (isAskUserQuestionTimedOutResult(block)) {
      return true;
    }
  }

  return false;
}

export function getSystemMessageIconClassName(
  tone: SystemEventTone,
): string {
  if (tone === "warning") {
    return "text-(--warning)";
  }
  return "text-(--icon-muted)";
}

export function getSystemMessageLabelClassName(
  tone: SystemEventTone,
): string {
  if (tone === "warning") {
    return "text-amber-800/80";
  }
  return "text-(--text-muted)";
}

export function projectionFromOrderedEntries(
  entries: OrderedAssistantEntry[],
  streamingBlockIndexes: Set<number>,
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

// Backend room control token, never shown to humans.
// Mirrors the backend NoReplyMarker.
const ROOM_CONTROL_MARKER = /<nexus_room_no_reply\s*\/>/g;

export function stripRoomControlMarkers(text: string): string {
  return text.replace(ROOM_CONTROL_MARKER, "").trim();
}

export function extractTextFromContentBlocks(
  content?: ContentBlock[] | null,
): string {
  if (!content || content.length === 0) {
    return "";
  }

  const texts: string[] = [];
  content.forEach((block) => {
    if (block.type === "text" && block.text.trim()) {
      texts.push(block.text);
    }
  });
  return stripRoomControlMarkers(texts.join("\n\n"));
}

export function formatMessageTime(timestamp?: number | null): string {
  if (!timestamp) {
    return "-- --:--";
  }

  const messageDate = new Date(timestamp);
  const now = new Date();
  const isSameYear = messageDate.getFullYear() === now.getFullYear();

  return messageDate.toLocaleString("zh-CN", {
    ...(isSameYear ? {} : { year: "numeric" }),
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

export function getTimelineAnchorElement(
  contentElement: HTMLElement,
): HTMLElement | null {
  return (
    contentElement.querySelector<HTMLElement>("[data-timeline-anchor]") ??
    contentElement.querySelector<HTMLElement>(
      "[data-markdown-anchor], button, li, h1, h2, h3, h4, pre, blockquote, th, td",
    )
  );
}

function getFirstTextLineTop(contentElement: HTMLElement): number | null {
  const textWalker = document.createTreeWalker(
    contentElement,
    NodeFilter.SHOW_TEXT,
    {
      acceptNode(node) {
        return node.textContent?.trim()
          ? NodeFilter.FILTER_ACCEPT
          : NodeFilter.FILTER_SKIP;
      },
    },
  );

  const firstTextNode = textWalker.nextNode();
  if (!(firstTextNode instanceof Text) || !firstTextNode.textContent) {
    return null;
  }

  const range = document.createRange();
  range.selectNodeContents(firstTextNode);
  const firstLineRect =
    range.getClientRects()[0] ?? range.getBoundingClientRect();
  if (!firstLineRect) {
    return null;
  }

  const contentRect = contentElement.getBoundingClientRect();
  return firstLineRect.top - contentRect.top + firstLineRect.height / 2;
}

export function getTimelineAnchorTop(
  contentElement: HTMLElement,
  anchorElement: HTMLElement | null,
): number {
  if (!anchorElement) {
    return getFirstTextLineTop(contentElement) ?? DEFAULT_TIMELINE_DOT_TOP;
  }

  const contentRect = contentElement.getBoundingClientRect();
  const candidateRect = anchorElement.getBoundingClientRect();
  const anchorMode = anchorElement.dataset.timelineAnchorMode;
  if (anchorMode === "box") {
    return candidateRect.top - contentRect.top + candidateRect.height / 2;
  }

  const computedStyle = window.getComputedStyle(anchorElement);
  const parsedLineHeight = Number.parseFloat(computedStyle.lineHeight);
  const anchorHeight = Number.isFinite(parsedLineHeight)
    ? parsedLineHeight
    : candidateRect.height;

  return candidateRect.top - contentRect.top + anchorHeight / 2;
}
