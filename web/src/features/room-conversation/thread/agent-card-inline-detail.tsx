"use client";

import { useMemo } from "react";
import { cn } from "@/lib/utils";
import { useAssistantContentMerge } from "@/hooks/use-assistant-content-merge";
import { AssistantMessage, Message, ResultMessage } from "@/types/message";
import { ContentRenderer } from "@/features/conversation-shared/message/content-renderer";
import { MessageStats } from "@/features/conversation-shared/message/message-stats";

interface AgentCardInlineDetailProps {
  /** 该 Agent 在此轮中的 assistant 消息 */
  agent_messages: AssistantMessage[];
  /** 对应的 result 消息（可能不存在，如仍在流式输出） */
  result_message?: ResultMessage;
  is_loading?: boolean;
  on_open_workspace_file?: (path: string) => void;
}

/**
 * Agent 卡片内联展开内容 — 在 AgentStatusCard 下方直接渲染完整回复。
 * 复用 useAssistantContentMerge 和 ContentRenderer。
 */
export function AgentCardInlineDetail({
  agent_messages,
  result_message,
  is_loading = false,
  on_open_workspace_file,
}: AgentCardInlineDetailProps) {
  // 将 agent_messages + result 组合成 useAssistantContentMerge 需要的格式
  const messages_for_merge: Message[] = useMemo(() => {
    const arr: Message[] = [...agent_messages];
    if (result_message) arr.push(result_message);
    return arr;
  }, [agent_messages, result_message]);

  const {
    mergedContent,
    streamingBlockIndexes,
    visibleAssistantTextContent,
    assistantTextStreamingIndexes,
    assistantTextContent,
    resultMessage: merged_result,
  } = useAssistantContentMerge({
    messages: messages_for_merge,
    is_last_round: true,
    is_loading,
  });

  // 过滤可见的 process 内容（thinking + tool_use + tool_result）
  const visibleProcessContent = useMemo(() => {
    return mergedContent.filter((block) => {
      if (block.type === "thinking") return Boolean(block.thinking?.trim());
      if (block.type === "tool_use") return true;
      if (block.type === "tool_result") return true;
      return false;
    });
  }, [mergedContent]);

  const should_show_text = visibleAssistantTextContent.length > 0;
  const show_cursor = is_loading && streamingBlockIndexes.size > 0;

  // 统计信息
  const stats = useMemo(() => {
    if (!merged_result) return undefined;
    return {
      duration: merged_result.duration_ms >= 1000
        ? `${(merged_result.duration_ms / 1000).toFixed(1)}s`
        : `${merged_result.duration_ms}ms`,
      tokens: merged_result.usage
        ? `↑ ${merged_result.usage.input_tokens} ↓ ${merged_result.usage.output_tokens}`
        : null,
      cost: merged_result.total_cost_usd !== undefined
        ? `$ ${merged_result.total_cost_usd ? merged_result.total_cost_usd.toFixed(4) : null}`
        : null,
      cache_hit: merged_result.usage?.cache_read_input_tokens && merged_result.usage.cache_read_input_tokens > 0
        ? `💾 ${merged_result.usage.cache_read_input_tokens}`
        : null,
    };
  }, [merged_result]);

  if (mergedContent.length === 0) return null;

  return (
    <div className="border-t border-slate-100 px-3 pb-2 pt-2">
      {/* Process 内容 (thinking / tool calls) */}
      {visibleProcessContent.length > 0 && (
        <div className="mb-2">
          <ContentRenderer
            content={visibleProcessContent}
            is_streaming={show_cursor}
            on_open_workspace_file={on_open_workspace_file}
          />
        </div>
      )}

      {/* 文本回复 */}
      {should_show_text && (
        <div className="text-[14px] leading-7">
          <ContentRenderer
            content={visibleAssistantTextContent}
            is_streaming={show_cursor}
            streaming_block_indexes={assistantTextStreamingIndexes}
            on_open_workspace_file={on_open_workspace_file}
          />
        </div>
      )}

      {/* 统计栏 */}
      {stats && !is_loading && (
        <MessageStats stats={stats} show_cursor={show_cursor} />
      )}
    </div>
  );
}
