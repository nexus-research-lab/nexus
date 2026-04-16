import { useMemo } from "react";
import { prepare, layout } from "@chenglou/pretext";
import { Message } from "@/types/conversation/message";

// Base font matching MarkdownRenderer prose text (text-sm = 14px, leading-7 = 28px)
const PROSE_FONT = "400 14px ui-sans-serif, system-ui, sans-serif";
const PROSE_LINE_HEIGHT = 28;

// Fixed structural overhead per round: user header + assistant header + padding + border
const ROUND_CHROME_HEIGHT = 96;

// Fixed overhead per text block (paragraph spacing, container padding)
const BLOCK_PADDING = 16;

// Code block: fixed height estimate per line of code
const CODE_LINE_HEIGHT = 22;
const CODE_BLOCK_MIN_HEIGHT = 80;

// Tool block fixed height
const TOOL_BLOCK_HEIGHT = 60;

type HeightEstimate = {
  /** Estimated total height in px for this round */
  height: number;
};

function estimate_text_height(text: string, containerWidth: number): number {
  if (!text.trim()) return 0;
  try {
    const prepared = prepare(text, PROSE_FONT);
    const result = layout(prepared, containerWidth, PROSE_LINE_HEIGHT);
    return result.height + BLOCK_PADDING;
  } catch {
    // Fallback: rough estimate at ~60 chars/line
    const charsPerLine = Math.max(1, Math.floor(containerWidth / 8.4));
    const lines = Math.ceil(text.length / charsPerLine);
    return lines * PROSE_LINE_HEIGHT + BLOCK_PADDING;
  }
}

function extract_text_from_messages(messages: Message[]): string {
  const parts: string[] = [];
  for (const msg of messages) {
    if (msg.role === "user" && typeof msg.content === "string") {
      parts.push(msg.content);
    } else if (msg.role === "assistant") {
      if (typeof msg.content === "string") {
        parts.push(msg.content);
      } else if (Array.isArray(msg.content)) {
        for (const block of msg.content as any[]) {
          if (block.type === "text") parts.push(block.text ?? "");
          if (block.type === "task_progress") parts.push(block.description ?? "");
        }
      }
    }
  }
  return parts.join("\n");
}

function count_tool_blocks(messages: Message[]): number {
  let count = 0;
  for (const msg of messages) {
    if (msg.role === "assistant" && Array.isArray(msg.content)) {
      for (const block of msg.content as any[]) {
        if (block.type === "tool_use") count++;
      }
    }
  }
  return count;
}

function estimate_code_block_height(text: string): number {
  const blocks = text.match(/```[\s\S]*?```/g) ?? [];
  return blocks.reduce((sum, block) => {
    const lines = block.split("\n").length;
    return sum + Math.max(CODE_BLOCK_MIN_HEIGHT, lines * CODE_LINE_HEIGHT);
  }, 0);
}

/**
 * Estimates the rendered height of a message round using pretext for text
 * measurement. Avoids DOM reflow — safe to call on many items at once.
 */
export function useMessageHeight(
  messages: Message[],
  containerWidth: number,
): HeightEstimate {
  return useMemo(() => {
    if (containerWidth <= 0) return { height: 200 };

    const text = extract_text_from_messages(messages);
    const toolCount = count_tool_blocks(messages);
    const codeBlockHeight = estimate_code_block_height(text);

    // Strip code blocks from text before measuring prose
    const proseText = text.replace(/```[\s\S]*?```/g, "");
    const proseHeight = estimate_text_height(proseText, containerWidth);

    const height =
      ROUND_CHROME_HEIGHT +
      proseHeight +
      codeBlockHeight +
      toolCount * TOOL_BLOCK_HEIGHT;

    return { height: Math.max(80, height) };
  }, [messages, containerWidth]);
}

/**
 * Batch height estimates for all rounds — call once, get all heights.
 * More efficient than calling useMessageHeight in a loop since we share
 * the prepare() cache across all messages.
 */
export function estimate_round_heights(
  round_ids: string[],
  message_groups: Map<string, Message[]>,
  containerWidth: number,
): Map<string, number> {
  const result = new Map<string, number>();

  if (containerWidth <= 0) {
    round_ids.forEach((id) => result.set(id, 200));
    return result;
  }

  for (const id of round_ids) {
    const messages = message_groups.get(id) ?? [];
    const text = extract_text_from_messages(messages);
    const toolCount = count_tool_blocks(messages);
    const codeBlockHeight = estimate_code_block_height(text);
    const proseText = text.replace(/```[\s\S]*?```/g, "");
    const proseHeight = estimate_text_height(proseText, containerWidth);

    const height = Math.max(
      80,
      ROUND_CHROME_HEIGHT + proseHeight + codeBlockHeight + toolCount * TOOL_BLOCK_HEIGHT,
    );
    result.set(id, height);
  }

  return result;
}
