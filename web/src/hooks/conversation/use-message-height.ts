import { prepare, layout } from "@chenglou/pretext";
import { isAutomationTriggerUserMessage } from "@/types/conversation/automation-message";
import { ContentBlock, Message } from "@/types/conversation/message";

// 与 Markdown 正文的字号和行高保持一致，避免虚拟列表初始估高跳动。
const PROSE_FONT = "400 14px ui-sans-serif, system-ui, sans-serif";
const PROSE_LINE_HEIGHT = 28;

// 每轮固定结构：用户头部、Agent 头部、内边距和分隔线。
const ROUND_CHROME_HEIGHT = 96;

const BLOCK_PADDING = 16;

const CODE_LINE_HEIGHT = 22;
const CODE_BLOCK_MIN_HEIGHT = 80;

const TOOL_BLOCK_HEIGHT = 60;

function estimateTextHeight(text: string, containerWidth: number): number {
  if (!text.trim()) return 0;
  try {
    const prepared = prepare(text, PROSE_FONT);
    const result = layout(prepared, containerWidth, PROSE_LINE_HEIGHT);
    return result.height + BLOCK_PADDING;
  } catch {
    // 字体测量失败时按平均字符宽度估算，保证虚拟列表仍可工作。
    const charsPerLine = Math.max(1, Math.floor(containerWidth / 8.4));
    const lines = Math.ceil(text.length / charsPerLine);
    return lines * PROSE_LINE_HEIGHT + BLOCK_PADDING;
  }
}

function extractTextFromMessages(messages: Message[]): string {
  const parts: string[] = [];
  for (const msg of messages) {
    if (msg.role === "user" && typeof msg.content === "string") {
      if (isAutomationTriggerUserMessage(msg)) {
        continue;
      }
      parts.push(msg.content);
    } else if (msg.role === "assistant") {
      if (typeof msg.content === "string") {
        parts.push(msg.content);
      } else if (Array.isArray(msg.content)) {
        for (const block of msg.content as ContentBlock[]) {
          if (block.type === "text") parts.push(block.text ?? "");
          if (block.type === "task_progress") parts.push(block.description ?? "");
        }
      }
    }
  }
  return parts.join("\n");
}

function countToolBlocks(messages: Message[]): number {
  let count = 0;
  for (const msg of messages) {
    if (msg.role === "assistant" && Array.isArray(msg.content)) {
      for (const block of msg.content as ContentBlock[]) {
        if (block.type === "tool_use") count++;
      }
    }
  }
  return count;
}

function estimateCodeBlockHeight(text: string): number {
  const blocks = text.match(/```[\s\S]*?```/g) ?? [];
  return blocks.reduce((sum, block) => {
    const lines = block.split("\n").length;
    return sum + Math.max(CODE_BLOCK_MIN_HEIGHT, lines * CODE_LINE_HEIGHT);
  }, 0);
}

/**
 * 批量估算轮次高度，共享 pretext 缓存并避免逐项触发 DOM 测量。
 */
export function estimateRoundHeights(
  roundIds: string[],
  messageGroups: Map<string, Message[]>,
  containerWidth: number,
): Map<string, number> {
  const result = new Map<string, number>();

  if (containerWidth <= 0) {
    roundIds.forEach((id) => result.set(id, 200));
    return result;
  }

  for (const id of roundIds) {
    const messages = messageGroups.get(id) ?? [];
    const text = extractTextFromMessages(messages);
    const toolCount = countToolBlocks(messages);
    const codeBlockHeight = estimateCodeBlockHeight(text);
    const proseText = text.replace(/```[\s\S]*?```/g, "");
    const proseHeight = estimateTextHeight(proseText, containerWidth);

    const height = Math.max(
      80,
      ROUND_CHROME_HEIGHT + proseHeight + codeBlockHeight + toolCount * TOOL_BLOCK_HEIGHT,
    );
    result.set(id, height);
  }

  return result;
}
