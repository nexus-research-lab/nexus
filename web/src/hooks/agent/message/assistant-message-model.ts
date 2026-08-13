import type {
  AssistantMessage,
  Message,
} from "@/types/conversation/message/entity";
import type {
  ContentBlock,
  ImageContent,
} from "@/types/conversation/message/content";
import {
  isLiveStreamAssistant,
  markLiveStreamRevealBlock,
  preserveLiveStreamRevealMarker,
} from "@/lib/conversation/live-stream-reveal";

type ContentBlockType = ContentBlock["type"];
type ContentBlockOf<Type extends ContentBlockType> = Extract<
  ContentBlock,
  { type: Type }
>;
type ContentBlockKeyResolverMap = {
  [Type in ContentBlockType]: (
    block: ContentBlockOf<Type>,
  ) => string | null;
};
type ImageIdentityResolver = (
  block: ImageContent,
) => string | null | undefined;

export const DEFAULT_ASSISTANT_ERROR_MESSAGE =
  "本轮执行失败，模型或工具没有正常完成。请稍后重试。";

// 顺序是图片跨快照合并的身份优先级，新增来源时必须显式选择位置。
const IMAGE_IDENTITY_RESOLVERS: readonly ImageIdentityResolver[] = [
  (block) => block.path,
  (block) => block.url,
  (block) => block.uri,
  (block) => block.source?.path,
  (block) => block.source?.url,
  (block) => block.source?.uri,
  (block) => block.data,
  (block) => block.source?.data,
];

const CONTENT_BLOCK_KEY_RESOLVERS = {
  document: (block) => `document:${block.title ?? ""}:${stableBlockValue(block.source)}`,
  image: (block) => imageContentBlockKey(block),
  redacted_thinking: () => "redacted_thinking",
  resource_link: (block) => `resource_link:${block.uri ?? ""}:${block.name ?? ""}`,
  search_result: (block) => `search_result:${block.url ?? ""}:${block.title ?? ""}`,
  system_event: (block) => [
    "system_event",
    block.source_message_id,
    block.subtype ?? "",
    block.tool_use_id ?? "",
    block.content,
  ].join(":"),
  task_progress: (block) => (
    block.task_id ? `task_progress:${block.task_id}` : null
  ),
  text: (block) => `text:${block.text}`,
  thinking: () => "thinking",
  tool_result: (block) => (
    block.tool_use_id ? `tool_result:${block.tool_use_id}` : null
  ),
  tool_use: (block) => (block.id ? `tool_use:${block.id}` : null),
  tool_use_error: (block) => `tool_use_error:${block.content}`,
  unsupported: (block) => `unsupported:${block.original_type}:${stableBlockValue(block.payload)}`,
  workspace_file_artifact: (block) => (
    block.id
      ? `workspace_file_artifact:${block.id}`
      : `workspace_file_artifact:${block.path}:${block.operation ?? ""}`
  ),
} satisfies ContentBlockKeyResolverMap;

/**
 * 后端 is_complete 服务于持久化，并不等于前端整轮终态。
 * Assistant 自身只依据 stop_reason 或显式 stream_status 收口。
 */
export function normalizeAssistantMessage(
  incoming: AssistantMessage,
): AssistantMessage {
  return {
    ...incoming,
    stream_status:
      incoming.stream_status ??
      (incoming.stop_reason || incoming.is_complete ? "done" : "streaming"),
  };
}

export function normalizeAssistantMessages(messages: Message[]): Message[] {
  let hasChanges = false;
  const normalizedMessages = messages.map((message) => {
    if (message.role !== "assistant") {
      return message;
    }
    const normalized = normalizeAssistantMessage(message);
    if (normalized.stream_status === message.stream_status) {
      return message;
    }
    hasChanges = true;
    return normalized;
  });
  return hasChanges ? normalizedMessages : messages;
}

// resolveAssistantResultErrorMessage 统一解析 result_summary 的终态错误。
// 错误可能只存在 errors/terminal_reason 中，不能只读取 result 字段。
export function resolveAssistantResultErrorMessage(
  summary: AssistantMessage["result_summary"],
): string | null {
  if (!summary || (!summary.is_error && summary.subtype !== "error")) {
    return null;
  }
  const resultText = typeof summary.result === "string"
    ? summary.result.trim()
    : "";
  if (resultText) {
    return resultText;
  }
  const errorText = (Array.isArray(summary.errors) ? summary.errors : [])
    .filter((item) => typeof item === "string")
    .map((item) => item.trim())
    .filter(Boolean)
    .join("; ");
  if (errorText) {
    return errorText;
  }
  const terminalReason = typeof summary.terminal_reason === "string"
    ? summary.terminal_reason.trim()
    : "";
  return terminalReason || DEFAULT_ASSISTANT_ERROR_MESSAGE;
}

// resolveAssistantResultErrorBannerMessage 只返回尚未由最终回复承载的错误。
// result 是唯一正文或与正文相同时，消息气泡已经完整说明失败，无需再叠加系统气泡。
export function resolveAssistantResultErrorBannerMessage(
  message: AssistantMessage,
): string | null {
  const error = resolveAssistantResultErrorMessage(message.result_summary);
  if (!error) {
    return null;
  }
  const resultText = normalizeDisplayText(message.result_summary?.result ?? "");
  if (!resultText) {
    return error;
  }
  const assistantText = normalizeDisplayText(
    message.content
      .filter((block): block is Extract<ContentBlock, { type: "text" }> => (
        block.type === "text"
      ))
      .map((block) => block.text)
      .join("\n\n"),
  );
  return !assistantText || assistantText === resultText ? null : error;
}

function normalizeDisplayText(value: string): string {
  return value.replaceAll("\r\n", "\n").trim();
}

// latestAssistantResultErrorMessage 只检查终态 result_summary，不把工具自身
// 的 is_error 当成整轮失败；这样可恢复工具错误不会污染会话错误栏。
export function latestAssistantResultErrorMessage(
  messages: readonly Message[],
): string | null {
  let latestAssistant: AssistantMessage | null = null;
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index];
    if (message.role === "assistant") {
      latestAssistant = message;
      break;
    }
  }
  if (!latestAssistant) {
    return null;
  }

  const latestRoundId = typeof latestAssistant.round_id === "string"
    ? latestAssistant.round_id.trim()
    : "";
  // 正常消息都带 root round_id；缺失时只能信任最新一条，避免把旧轮次错误
  // 误挂到一个没有身份的历史快照上。
  if (!latestRoundId) {
    return resolveAssistantResultErrorBannerMessage(latestAssistant);
  }

  // Room 同一 root round 可能有多个 Agent。不能只看最后一条 Assistant，
  // 否则前一个 slot 的真实失败会被后一个 slot 的成功快照覆盖。
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index];
    if (
      message.role !== "assistant"
      || (typeof message.round_id === "string"
        ? message.round_id.trim()
        : "") !== latestRoundId
    ) {
      continue;
    }
    const error = resolveAssistantResultErrorBannerMessage(message);
    if (error) {
      return error;
    }
  }
  return null;
}

export function mergeAssistantMessage(
  existing: AssistantMessage,
  incoming: AssistantMessage,
): AssistantMessage {
  const normalizedExisting = normalizeAssistantMessage(existing);
  const normalizedIncoming = normalizeAssistantMessage(incoming);
  return normalizeAssistantMessage({
    ...normalizedExisting,
    ...normalizedIncoming,
    content: mergeAssistantContentBlocks(
      normalizedExisting.content,
      normalizedIncoming.content,
      isLiveStreamAssistant(normalizedExisting),
    ),
    is_complete:
      normalizedIncoming.is_complete ?? normalizedExisting.is_complete,
    result_summary:
      normalizedIncoming.result_summary ?? normalizedExisting.result_summary,
    recalled_memories:
      normalizedIncoming.recalled_memories ?? normalizedExisting.recalled_memories,
    stop_reason:
      normalizedIncoming.stop_reason ?? normalizedExisting.stop_reason,
    stream_status:
      normalizedIncoming.stream_status ?? normalizedExisting.stream_status,
    usage: normalizedIncoming.usage ?? normalizedExisting.usage,
  });
}

function mergeAssistantContentBlocks(
  existingBlocks: ContentBlock[],
  incomingBlocks: ContentBlock[],
  markNewLiveBlocks: boolean,
): ContentBlock[] {
  if (existingBlocks.length === 0) {
    return incomingBlocks.map((block) => (
      markNewLiveBlocks ? markLiveStreamRevealBlock(block) : block
    ));
  }
  if (incomingBlocks.length === 0) {
    return [...existingBlocks];
  }

  const mergedBlocks = [...existingBlocks];
  const indexByKey = buildContentBlockIndex(mergedBlocks);
  for (const incomingBlock of incomingBlocks) {
    const existingIndex = findMergeTargetIndex(
      mergedBlocks,
      indexByKey,
      incomingBlock,
    );
    if (existingIndex !== null) {
      mergedBlocks[existingIndex] = preserveLiveStreamRevealMarker(
        mergedBlocks[existingIndex],
        incomingBlock,
      );
      continue;
    }

    const nextBlock = markNewLiveBlocks
      ? markLiveStreamRevealBlock(incomingBlock)
      : incomingBlock;
    const key = assistantContentBlockKey(nextBlock);
    if (key) {
      indexByKey.set(key, mergedBlocks.length);
    }
    mergedBlocks.push(nextBlock);
  }
  return mergedBlocks;
}

function buildContentBlockIndex(blocks: ContentBlock[]): Map<string, number> {
  const indexByKey = new Map<string, number>();
  blocks.forEach((block, index) => {
    const key = assistantContentBlockKey(block);
    if (key && !indexByKey.has(key)) {
      indexByKey.set(key, index);
    }
  });
  return indexByKey;
}

function findMergeTargetIndex(
  blocks: ContentBlock[],
  indexByKey: Map<string, number>,
  incomingBlock: ContentBlock,
): number | null {
  const textIndex = findMergeableTextBlockIndex(blocks, incomingBlock);
  if (textIndex !== -1) {
    return textIndex;
  }
  const key = assistantContentBlockKey(incomingBlock);
  return key ? (indexByKey.get(key) ?? null) : null;
}

function findMergeableTextBlockIndex(
  blocks: ContentBlock[],
  incomingBlock: ContentBlock,
): number {
  if (incomingBlock.type !== "text") {
    return -1;
  }
  for (let index = blocks.length - 1; index >= 0; index -= 1) {
    const currentBlock = blocks[index];
    if (currentBlock.type !== "text") {
      continue;
    }
    if (
      currentBlock.text === incomingBlock.text ||
      currentBlock.text.startsWith(incomingBlock.text) ||
      incomingBlock.text.startsWith(currentBlock.text)
    ) {
      return index;
    }
  }
  return -1;
}

function assistantContentBlockKey(block: ContentBlock): string | null {
  const resolver = CONTENT_BLOCK_KEY_RESOLVERS[block.type] as ((
    value: ContentBlock
  ) => string | null) | undefined;
  return resolver?.(block) ?? null;
}

function stableBlockValue(value: unknown): string {
  try {
    return JSON.stringify(value) ?? "";
  } catch {
    return "";
  }
}

function imageContentBlockKey(block: ImageContent): string | null {
  for (const resolveIdentity of IMAGE_IDENTITY_RESOLVERS) {
    const identity = resolveIdentity(block);
    if (identity) {
      return `image:${identity}`;
    }
  }
  return null;
}
