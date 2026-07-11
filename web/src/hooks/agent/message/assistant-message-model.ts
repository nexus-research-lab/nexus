import type {
  AssistantMessage,
  Message,
} from "@/types/conversation/message/entity";
import type {
  ContentBlock,
  ImageContent,
} from "@/types/conversation/message/content";

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

const CONTENT_BLOCK_KEY_RESOLVERS = {
  image: (block) => imageContentBlockKey(block),
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
    ),
    is_complete:
      normalizedIncoming.is_complete ?? normalizedExisting.is_complete,
    result_summary:
      normalizedIncoming.result_summary ?? normalizedExisting.result_summary,
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
): ContentBlock[] {
  if (existingBlocks.length === 0) {
    return [...incomingBlocks];
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
      mergedBlocks[existingIndex] = incomingBlock;
      continue;
    }

    const key = assistantContentBlockKey(incomingBlock);
    if (key) {
      indexByKey.set(key, mergedBlocks.length);
    }
    mergedBlocks.push(incomingBlock);
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
  const resolver = CONTENT_BLOCK_KEY_RESOLVERS[block.type] as (
    value: ContentBlock
  ) => string | null;
  return resolver(block);
}

function imageContentBlockKey(block: ImageContent): string | null {
  const rawKey =
    block.path ||
    block.url ||
    block.uri ||
    block.source?.path ||
    block.source?.url ||
    block.source?.uri ||
    block.data ||
    block.source?.data ||
    null;
  return rawKey ? `image:${rawKey}` : null;
}
