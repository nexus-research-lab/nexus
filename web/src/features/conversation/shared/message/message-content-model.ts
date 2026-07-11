import type {
  ContentBlock,
  TextContent,
} from "@/types/conversation/message";

const TOOL_USE_ERROR_TAG_PATTERN =
  /<tool_use_error>([\s\S]*?)<\/tool_use_error>/g;

// 该标记只控制 Room 编排，任何面向用户的文本投影都必须先剥离。
const ROOM_CONTROL_MARKER = /<nexus_room_no_reply\s*\/>/g;

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
  return blocks;
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
