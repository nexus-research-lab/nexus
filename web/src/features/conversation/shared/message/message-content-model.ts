import type {
  ContentBlock,
  ToolResultContent,
  TextContent,
} from "@/types/conversation/message/content";

const TOOL_USE_ERROR_TAG_PATTERN =
  /<tool_use_error>([\s\S]*?)<\/tool_use_error>/g;

// 该标记只控制 Room 编排，任何面向用户的文本投影都必须先剥离。
const ROOM_CONTROL_MARKER = /<nexus_room_no_reply\s*\/>/g;

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
  return text.replace(ROOM_CONTROL_MARKER, "").trim();
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
