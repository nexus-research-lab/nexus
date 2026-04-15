/**
 * useAssistantContentMerge — 合并并去重 assistant 消息内容块
 *
 * 将一轮对话中多条 assistant 消息的内容块合并为单一列表，
 * 自动去重 tool_use / tool_result，并追踪流式输出的 block 索引。
 */

import { useMemo } from "react";

import { ContentBlock, Message, ResultMessage } from "@/types/message";

interface UseAssistantContentMergeOptions {
  messages: Message[];
  is_last_round?: boolean;
  is_loading?: boolean;
}

interface UseAssistantContentMergeReturn {
  /** 用户消息 */
  userMessage: Message | undefined;
  /** 所有 assistant 消息 */
  assistantMessages: Message[];
  /** result 消息 */
  resultMessage: ResultMessage | undefined;
  /** 当前正在流式输出的 assistant 消息 ID */
  streamingAssistantMessageId: string | null;
  /** 合并去重后的所有内容块 */
  mergedContent: ContentBlock[];
  /** mergedContent 每个块对应的来源 assistant 消息 ID */
  mergedContentSourceMessageIds: string[];
  /** 正在流式输出的 block 在 mergedContent 中的索引 */
  streamingBlockIndexes: Set<number>;
  /** 可见的 assistant 文本内容块 */
  visibleAssistantTextContent: ContentBlock[];
  /** 正在流式输出的文本在 visibleAssistantTextContent 中的索引 */
  assistantTextStreamingIndexes: Set<number>;
  /** 纯文本内容（用于复制） */
  assistantTextContent: string;
}

export function useAssistantContentMerge({
  messages,
  is_last_round,
  is_loading,
}: UseAssistantContentMergeOptions): UseAssistantContentMergeReturn {
  // 分离消息
  const { userMessage, assistantMessages, resultMessage } = useMemo(() => {
    const user = messages.find((m) => m.role === "user");
    const result = messages.find((m) => m.role === "result") as ResultMessage | undefined;
    const assistant = messages.filter((m) => m.role === "assistant");
    return { userMessage: user, assistantMessages: assistant, resultMessage: result };
  }, [messages]);

  const streamingAssistantMessageId = useMemo(() => {
    if (!is_last_round || !is_loading) {
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
  }, [assistantMessages, is_last_round, is_loading]);

  // 合并并去重 assistant 内容
  const { mergedContent, mergedContentSourceMessageIds, streamingBlockIndexes } = useMemo(() => {
    const allBlocks: ContentBlock[] = [];
    const sourceMessageIds: string[] = [];
    const nextStreamingBlockIndexes = new Set<number>();
    const seenToolIds = new Set<string>();

    for (const msg of assistantMessages) {
      if (!Array.isArray(msg.content)) continue;
      const isStreamingMessage = msg.message_id === streamingAssistantMessageId;
      const streamingContentIndex = isStreamingMessage
        ? find_last_streamable_block_index(msg.content)
        : -1;

      msg.content.forEach((block, blockIndex) => {
        if (!block) {
          return;
        }
        if (block.type === "tool_use" && block.id) {
          if (seenToolIds.has(block.id)) return;
          seenToolIds.add(block.id);
        }
        if (block.type === "tool_result" && block.tool_use_id) {
          if (seenToolIds.has(`result_${block.tool_use_id}`)) return;
          seenToolIds.add(`result_${block.tool_use_id}`);
        }

        const nextIndex = allBlocks.length;
        allBlocks.push(block);
        sourceMessageIds.push(msg.message_id);
        if (isStreamingMessage && blockIndex === streamingContentIndex) {
          nextStreamingBlockIndexes.add(nextIndex);
        }
      });
    }
      return {
        mergedContent: allBlocks,
        mergedContentSourceMessageIds: sourceMessageIds,
        streamingBlockIndexes: nextStreamingBlockIndexes,
      };
  }, [assistantMessages, streamingAssistantMessageId]);

  const visibleAssistantTextContent = useMemo(() => {
    return mergedContent.filter(
      (block) => block.type === "text" && Boolean(block.text.trim()),
    );
  }, [mergedContent]);

  const assistantTextStreamingIndexes = useMemo(() => {
    const nextIndexes = new Set<number>();
    let textIndex = 0;

    mergedContent.forEach((block, index) => {
      if (block.type === "text" && Boolean(block.text.trim())) {
        if (streamingBlockIndexes.has(index)) {
          nextIndexes.add(textIndex);
        }
        textIndex += 1;
      }
    });

    return nextIndexes;
  }, [mergedContent, streamingBlockIndexes]);

  const assistantTextContent = useMemo(() => {
    const texts: string[] = [];
    for (const block of visibleAssistantTextContent) {
      if (block.type === "text" && block.text) {
        texts.push(block.text);
      }
    }
    return texts.join("\n\n");
  }, [visibleAssistantTextContent]);

  return {
    userMessage,
    assistantMessages,
    resultMessage,
    streamingAssistantMessageId,
    mergedContent,
    mergedContentSourceMessageIds,
    streamingBlockIndexes,
    visibleAssistantTextContent,
    assistantTextStreamingIndexes,
    assistantTextContent,
  };
}

function find_last_streamable_block_index(blocks: ContentBlock[]): number {
  for (let index = blocks.length - 1; index >= 0; index -= 1) {
    const block = blocks[index];
    if (!block) {
      continue;
    }
    if (block.type === "text" || block.type === "thinking") {
      return index;
    }
  }

  return -1;
}
