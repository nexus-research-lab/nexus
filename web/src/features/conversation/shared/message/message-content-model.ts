/**
 * INPUT: Assistant 内容块、结果文本与 Room 内部控制标记。
 * OUTPUT: 用户可见输出判定、控制标记清理及消息内容提取/归一化工具。
 * POS: DM/Room 消息内容语义的共享纯模型。
 */
import type {
  ContentBlock,
  SystemEventContent,
  ToolResultContent,
  TextContent,
} from "@/types/conversation/message/content";
import type { AssistantMessage } from "@/types/conversation/message/entity";

const TOOL_USE_ERROR_TAG_PATTERN =
  /<tool_use_error>([\s\S]*?)<\/tool_use_error>/g;
const API_RETRY_VISIBLE_ATTEMPT = 4;

// 这些标记只控制 Room 编排；历史、流式、结果与复制投影必须共用同一清理入口。
const ROOM_CONTROL_MARKER_PATTERN =
  /<nexus_room_(?:fanout|no_reply)\s*\/>/gi;
const EMPTY_HIDDEN_TOOL_NAMES = new Set<string>();
const NON_VISUAL_ASSISTANT_BLOCK_TYPES = new Set<ContentBlock["type"]>([
  "document",
  "redacted_thinking",
  "resource_link",
  "search_result",
  "task_progress",
  "thinking",
  "tool_result",
  "unsupported",
]);

// SDK 用内部元数据标记可恢复的工具结果，模型仍能看到 is_error，用户界面不应把它当成异常。
export const INTERNAL_TOOL_RESULT_KIND_KEY = "_nexus_internal_kind";
export const MALFORMED_TOOL_INPUT_RESULT_KIND = "malformed_tool_input";

export function isRecoverableToolResult(
  block: ToolResultContent,
): boolean {
  return block.metadata?.[INTERNAL_TOOL_RESULT_KIND_KEY] ===
    MALFORMED_TOOL_INPUT_RESULT_KIND;
}

export function isRecoverableToolUse(
  block: Extract<ContentBlock, { type: "tool_use" }>,
): boolean {
  return block.metadata?.[INTERNAL_TOOL_RESULT_KIND_KEY] ===
    MALFORMED_TOOL_INPUT_RESULT_KIND;
}

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
    appendTextBlock(blocks, block.text.slice(cursor, index));
    appendToolUseErrorBlock(blocks, match[1] ?? "");
    cursor = index + match[0].length;
  }

  appendTextBlock(blocks, block.text.slice(cursor));
  return blocks;
}

function appendTextBlock(blocks: ContentBlock[], text: string): void {
  if (text.trim()) {
    blocks.push({ type: "text", text });
  }
}

function appendToolUseErrorBlock(
  blocks: ContentBlock[],
  rawContent: string,
): void {
  const content = rawContent.trim();
  if (content) {
    blocks.push({ type: "tool_use_error", content });
  }
}

export function stripRoomControlMarkers(text: string): string {
  return text.replace(ROOM_CONTROL_MARKER_PATTERN, "").trim();
}

export function hasVisibleAssistantOutput(
  message: AssistantMessage,
  hiddenToolNames: ReadonlySet<string> = EMPTY_HIDDEN_TOOL_NAMES,
): boolean {
  const result = message.result_summary?.result ?? "";
  return message.content.some((block) => hasVisibleAssistantBlock(
    block,
    hiddenToolNames,
  ))
    || Boolean(stripRoomControlMarkers(result));
}

function hasVisibleAssistantBlock(
  block: ContentBlock,
  hiddenToolNames: ReadonlySet<string>,
): boolean {
  switch (block.type) {
    case "text":
      return Boolean(stripRoomControlMarkers(block.text));
    case "tool_use":
      return !hiddenToolNames.has(block.name) && !isRecoverableToolUse(block);
    case "system_event":
      return !isHiddenSystemEvent(block);
    default:
      return !NON_VISUAL_ASSISTANT_BLOCK_TYPES.has(block.type);
  }
}

export function isHiddenSystemEvent(block: SystemEventContent): boolean {
  return block.subtype === "api_retry"
    && typeof block.attempt === "number"
    && block.attempt < API_RETRY_VISIBLE_ATTEMPT;
}

export function extractTextFromContentBlocks(
  content?: ContentBlock[] | null,
): string {
  if (!content?.length) {
    return "";
  }

  return stripRoomControlMarkers(
    content
      .filter((block): block is TextContent => block.type === "text")
      .map((block) => block.text)
      .filter((text) => text.trim())
      .join("\n\n"),
  );
}
