/**
 * useAssistantContentMerge — 合并并去重 assistant 消息内容块
 *
 * 将一轮对话中多条 assistant 消息的内容块合并为单一列表，
 * 自动去重 toolUse / toolResult，并追踪流式输出的 block 索引。
 */

import { useMemo } from "react";

import { isAutomationTriggerUserMessage } from "@/types/conversation/automation-message";
import type {
  AssistantMessage,
  Message,
  ResultSummary,
  UserMessage,
} from "@/types/conversation/message/entity";
import type { ContentBlock } from "@/types/conversation/message/content";

interface UseAssistantContentMergeOptions {
  messages: Message[];
  isLastRound?: boolean;
  isLoading?: boolean;
}

interface UseAssistantContentMergeReturn {
  /** 用户消息 */
  userMessage: UserMessage | undefined;
  /** 所有 assistant 消息 */
  assistantMessages: AssistantMessage[];
  /** assistant 终态摘要 */
  resultSummary: ResultSummary | undefined;
  /** 合并去重后的所有内容块 */
  mergedContent: ContentBlock[];
  /** mergedContent 每个块对应的来源 assistant 消息 ID */
  mergedContentSourceMessageIds: string[];
  /** 正在流式输出的 block 在 mergedContent 中的索引 */
  streamingBlockIndexes: Set<number>;
}

interface AssistantContentMergeAccumulator {
  blocks: ContentBlock[];
  seenToolIds: Set<string>;
  sourceMessageIds: string[];
  streamingBlockIndexes: Set<number>;
}

export function useAssistantContentMerge({
  messages,
  isLastRound,
  isLoading,
}: UseAssistantContentMergeOptions): UseAssistantContentMergeReturn {
  const { userMessage, assistantMessages, resultSummary } = useMemo(() => {
    const user = messages.find(isVisibleUserMessage);
    const assistant = messages.filter(isAssistantMessage);
    const summary = getLatestResultSummary(assistant);
    return { userMessage: user, assistantMessages: assistant, resultSummary: summary };
  }, [messages]);

  const streamingAssistantMessageId = useMemo(() => {
    if (!isLastRound || !isLoading) {
      return null;
    }

    for (let index = assistantMessages.length - 1; index >= 0; index -= 1) {
      const message = assistantMessages[index];
      if (
        message.stream_status !== 'done'
        && message.stream_status !== 'cancelled'
        && message.stream_status !== 'error'
        && !message.stop_reason
      ) {
        return message.message_id;
      }
    }

    return null;
  }, [assistantMessages, isLastRound, isLoading]);

  const { mergedContent, mergedContentSourceMessageIds, streamingBlockIndexes } = useMemo(() => {
    const accumulator = createAssistantContentMergeAccumulator();
    assistantMessages.forEach((message) => appendAssistantMessageContent(
      accumulator,
      message,
      streamingAssistantMessageId,
    ));
    return {
      mergedContent: accumulator.blocks,
      mergedContentSourceMessageIds: accumulator.sourceMessageIds,
      streamingBlockIndexes: accumulator.streamingBlockIndexes,
    };
  }, [assistantMessages, streamingAssistantMessageId]);

  return {
    assistantMessages,
    mergedContent,
    mergedContentSourceMessageIds,
    resultSummary,
    streamingBlockIndexes,
    userMessage,
  };
}

function isVisibleUserMessage(message: Message): message is UserMessage {
  return message.role === "user" && !isAutomationTriggerUserMessage(message);
}

function isAssistantMessage(message: Message): message is AssistantMessage {
  return message.role === "assistant";
}

function createAssistantContentMergeAccumulator(): AssistantContentMergeAccumulator {
  return {
    blocks: [],
    seenToolIds: new Set<string>(),
    sourceMessageIds: [],
    streamingBlockIndexes: new Set<number>(),
  };
}

function appendAssistantMessageContent(
  accumulator: AssistantContentMergeAccumulator,
  message: AssistantMessage,
  streamingAssistantMessageId: string | null,
): void {
  const isStreamingMessage = message.message_id === streamingAssistantMessageId;
  const streamingContentIndex = isStreamingMessage
    ? findLastStreamableBlockIndex(message.content)
    : -1;
  message.content.forEach((block, blockIndex) => appendAssistantContentBlock(
    accumulator,
    block,
    blockIndex,
    message.message_id,
    streamingContentIndex,
  ));
}

function appendAssistantContentBlock(
  accumulator: AssistantContentMergeAccumulator,
  block: ContentBlock,
  blockIndex: number,
  messageId: string,
  streamingContentIndex: number,
): void {
  if (!claimAssistantContentBlock(accumulator.seenToolIds, block)) {
    return;
  }
  const nextIndex = accumulator.blocks.length;
  accumulator.blocks.push(block);
  accumulator.sourceMessageIds.push(messageId);
  if (blockIndex === streamingContentIndex) {
    accumulator.streamingBlockIndexes.add(nextIndex);
  }
}

function claimAssistantContentBlock(
  seenToolIds: Set<string>,
  block: ContentBlock,
): boolean {
  const dedupeKey = getAssistantContentBlockDedupeKey(block);
  if (!dedupeKey) {
    return true;
  }
  if (seenToolIds.has(dedupeKey)) {
    return false;
  }
  seenToolIds.add(dedupeKey);
  return true;
}

function getAssistantContentBlockDedupeKey(block: ContentBlock): string | null {
  if (block.type === "tool_use") {
    return block.id || null;
  }
  if (block.type === "tool_result") {
    return buildToolResultDedupeKey(block.tool_use_id);
  }
  return null;
}

function buildToolResultDedupeKey(toolUseId: string): string | null {
  return toolUseId ? `result_${toolUseId}` : null;
}

function getLatestResultSummary(
  assistantMessages: AssistantMessage[],
): ResultSummary | undefined {
  for (let index = assistantMessages.length - 1; index >= 0; index -= 1) {
    const summary = assistantMessages[index].result_summary;
    if (!summary) {
      continue;
    }
    return summary;
  }
  return undefined;
}

function findLastStreamableBlockIndex(blocks: ContentBlock[]): number {
  for (let index = blocks.length - 1; index >= 0; index -= 1) {
    const block = blocks[index];
    if (block.type === "text" || block.type === "thinking" || block.type === "image") {
      return index;
    }
  }

  return -1;
}
