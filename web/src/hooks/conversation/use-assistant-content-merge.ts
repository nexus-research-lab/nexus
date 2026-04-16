/**
 * useAssistantContentMerge — 合并并去重 assistant 消息内容块
 *
 * 将一轮对话中多条 assistant 消息的内容块合并为单一列表，
 * 自动去重 tool_use / tool_result，并追踪流式输出的 block 索引。
 */

import { useMemo } from "react";

import { ContentBlock, Message, ResultMessage } from "@/types/conversation/message";

interface UseAssistantContentMergeOptions {
  messages: Message[];
  is_last_round?: boolean;
  is_loading?: boolean;
}

interface UseAssistantContentMergeReturn {
  /** 用户消息 */
  user_message: Message | undefined;
  /** 所有 assistant 消息 */
  assistant_messages: Message[];
  /** result 消息 */
  result_message: ResultMessage | undefined;
  /** 当前正在流式输出的 assistant 消息 ID */
  streaming_assistant_message_id: string | null;
  /** 合并去重后的所有内容块 */
  merged_content: ContentBlock[];
  /** merged_content 每个块对应的来源 assistant 消息 ID */
  merged_content_source_message_ids: string[];
  /** 正在流式输出的 block 在 merged_content 中的索引 */
  streaming_block_indexes: Set<number>;
  /** 可见的 assistant 文本内容块 */
  visible_assistant_text_content: ContentBlock[];
  /** 正在流式输出的文本在 visible_assistant_text_content 中的索引 */
  assistant_text_streaming_indexes: Set<number>;
  /** 纯文本内容（用于复制） */
  assistant_text_content: string;
}

export function useAssistantContentMerge({
  messages,
  is_last_round,
  is_loading,
}: UseAssistantContentMergeOptions): UseAssistantContentMergeReturn {
  // 分离消息
  const { user_message, assistant_messages, result_message } = useMemo(() => {
    const user = messages.find((m) => m.role === "user");
    const result = messages.find((m) => m.role === "result") as ResultMessage | undefined;
    const assistant = messages.filter((m) => m.role === "assistant");
    return { user_message: user, assistant_messages: assistant, result_message: result };
  }, [messages]);

  const streaming_assistant_message_id = useMemo(() => {
    if (!is_last_round || !is_loading) {
      return null;
    }

    for (let index = assistant_messages.length - 1; index >= 0; index -= 1) {
      const message = assistant_messages[index];
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
  }, [assistant_messages, is_last_round, is_loading]);

  // 合并并去重 assistant 内容
  const { merged_content, merged_content_source_message_ids, streaming_block_indexes } = useMemo(() => {
    const allBlocks: ContentBlock[] = [];
    const sourceMessageIds: string[] = [];
    const nextStreamingBlockIndexes = new Set<number>();
    const seenToolIds = new Set<string>();

    for (const msg of assistant_messages) {
      if (!Array.isArray(msg.content)) continue;
      const isStreamingMessage = msg.message_id === streaming_assistant_message_id;
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
        merged_content: allBlocks,
        merged_content_source_message_ids: sourceMessageIds,
        streaming_block_indexes: nextStreamingBlockIndexes,
      };
  }, [assistant_messages, streaming_assistant_message_id]);

  const visible_assistant_text_content = useMemo(() => {
    return merged_content.filter(
      (block) => block.type === "text" && Boolean(block.text.trim()),
    );
  }, [merged_content]);

  const assistant_text_streaming_indexes = useMemo(() => {
    const nextIndexes = new Set<number>();
    let textIndex = 0;

    merged_content.forEach((block, index) => {
      if (block.type === "text" && Boolean(block.text.trim())) {
        if (streaming_block_indexes.has(index)) {
          nextIndexes.add(textIndex);
        }
        textIndex += 1;
      }
    });

    return nextIndexes;
  }, [merged_content, streaming_block_indexes]);

  const assistant_text_content = useMemo(() => {
    const texts: string[] = [];
    for (const block of visible_assistant_text_content) {
      if (block.type === "text" && block.text) {
        texts.push(block.text);
      }
    }
    return texts.join("\n\n");
  }, [visible_assistant_text_content]);

  return {
    user_message,
    assistant_messages,
    result_message,
    streaming_assistant_message_id,
    merged_content,
    merged_content_source_message_ids,
    streaming_block_indexes,
    visible_assistant_text_content,
    assistant_text_streaming_indexes,
    assistant_text_content,
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
