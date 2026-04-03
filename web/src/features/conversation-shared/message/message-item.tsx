/**
 * Message Component
 *
 *
 */

"use client";

import { memo, useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { prepare, layout } from "@chenglou/pretext";
import { Bot, Check, ChevronDown, ChevronRight, Copy, Edit2, Square, User, Wrench } from "lucide-react";
import { cn } from "@/lib/utils";
import { useAssistantContentMerge } from "@/hooks/use-assistant-content-merge";
import { AssistantMessage, ContentBlock, Message, ResultMessage } from "@/types/message";
import { PendingPermission, PermissionDecisionPayload } from "@/types/permission";
import { ContentRenderer } from "./content-renderer";
import { MessageStats } from "./message-stats";
import { ToolBlock } from "./block/tool-block";

interface OrderedAssistantEntry {
  block: ContentBlock;
  merged_index: number;
  source_message_id: string;
}

interface AssistantTurnEntry {
  message_id: string;
  content: ContentBlock[];
  text_content: ContentBlock[];
  streaming_indexes: Set<number>;
  text_streaming_indexes: Set<number>;
}

interface ContentProjection {
  content: ContentBlock[];
  streaming_indexes: Set<number>;
}

type AssistantContentMode = "dm_live" | "dm_archived" | "room_thread" | "room_result";

interface MessageItemProps {
  compact?: boolean;
  current_agent_name?: string | null;
  round_id: string;
  messages: Message[];
  is_last_round?: boolean;
  is_loading?: boolean;
  pending_permission?: PendingPermission | null;
  on_permission_response?: (payload: PermissionDecisionPayload) => boolean;
  hidden_tool_names?: string[];
  on_edit_user_message?: (message_id: string, new_content: string) => void;
  on_open_workspace_file?: (path: string) => void;
  /** Called when user clicks the per-message stop button in Room mode. */
  on_stop_message?: (msg_id: string) => void;
  /** 初始化时 process 区域是否默认展开 */
  default_process_expanded?: boolean;
  /** 助手头部右侧附加操作，例如查看 Thread */
  assistant_header_action?: ReactNode;
  /** 助手内容渲染模式。 */
  assistant_content_mode?: AssistantContentMode;
  class_name?: string;
}

function MessageItemInner(
  {
    compact = false,
    current_agent_name,
    messages,
    is_last_round,
    is_loading,
    pending_permission,
    on_permission_response,
    hidden_tool_names = ['TodoWrite'],
    on_edit_user_message,
    on_open_workspace_file,
    on_stop_message,
    default_process_expanded = false,
    assistant_header_action,
    assistant_content_mode = "dm_archived",
    class_name,
  }: MessageItemProps) {
  const [copiedUser, setCopiedUser] = useState(false);
  const [copiedAssistant, setCopiedAssistant] = useState(false);
  const [isProcessExpanded, setIsProcessExpanded] = useState(default_process_expanded);

  // 分离消息 + 合并内容
  const {
    userMessage,
    assistantMessages,
    resultMessage,
    mergedContent,
    mergedContentSourceMessageIds,
    streamingBlockIndexes,
  } = useAssistantContentMerge({
    messages,
    is_last_round,
    is_loading,
  });

  // 元数据
  const firstAssistant = assistantMessages[0];
  const model = firstAssistant && 'model' in firstAssistant ? firstAssistant.model : undefined;
  const timestamp = firstAssistant?.timestamp || resultMessage?.timestamp;

  // Room 并发场景：读取 stream_status（仅 AssistantMessage 携带此字段）
  const stream_status = useMemo(() => {
    const a = firstAssistant as AssistantMessage | undefined;
    return a?.stream_status ?? null;
  }, [firstAssistant]);

  // 统计信息
  const stats = useMemo(() => {
    if (!resultMessage) return null;
    const cacheHit = resultMessage.usage?.cache_read_input_tokens;
    return {
      duration: resultMessage.duration_ms >= 1000
        ? `${(resultMessage.duration_ms / 1000).toFixed(1)}s`
        : `${resultMessage.duration_ms}ms`,
      tokens: resultMessage.usage
        ? `↑ ${resultMessage.usage.input_tokens} ↓ ${resultMessage.usage.output_tokens}`
        : null,
      cost: resultMessage.total_cost_usd !== undefined
        ? `$ ${resultMessage.total_cost_usd ? resultMessage.total_cost_usd.toFixed(4) : null}`
        : null,
      cache_hit: cacheHit && cacheHit > 0 ? `💾 ${cacheHit}` : null,
    };
  }, [resultMessage]);

  // 状态
  const userContent = useMemo(() => {
    if (!userMessage || userMessage.role !== 'user') return '';
    return typeof userMessage.content === 'string' ? userMessage.content : '';
  }, [userMessage]);

  const hasInlinePendingTool = useMemo(() => {
    if (!pending_permission) {
      return false;
    }

    const pendingToolUseIds = new Set<string>();
    const resolvedToolUseIds = new Set<string>();

    for (const block of mergedContent) {
      if (block.type === 'tool_use' && block.name === pending_permission.tool_name) {
        pendingToolUseIds.add(block.id);
      }
      if (block.type === 'tool_result') {
        resolvedToolUseIds.add(block.tool_use_id);
      }
    }

    for (const toolUseId of pendingToolUseIds) {
      if (!resolvedToolUseIds.has(toolUseId)) {
        return true;
      }
    }

    return false;
  }, [mergedContent, pending_permission]);

  const hiddenToolUseIds = useMemo(() => {
    const nextIds = new Set<string>();
    for (const block of mergedContent) {
      if (block.type === "tool_use" && hidden_tool_names.includes(block.name)) {
        nextIds.add(block.id);
      }
    }
    return nextIds;
  }, [mergedContent, hidden_tool_names]);

  const visibleOrderedAssistantEntries = useMemo<OrderedAssistantEntry[]>(() => {
    const entries: OrderedAssistantEntry[] = [];

    mergedContent.forEach((block, mergedIndex) => {
      if (block.type === "text") {
        if (block.text.trim()) {
          entries.push({
            block,
            merged_index: mergedIndex,
            source_message_id: mergedContentSourceMessageIds[mergedIndex] || "",
          });
        }
        return;
      }
      if (block.type === "thinking") {
        if (block.thinking?.trim()) {
          entries.push({
            block,
            merged_index: mergedIndex,
            source_message_id: mergedContentSourceMessageIds[mergedIndex] || "",
          });
        }
        return;
      }
      if (block.type === "tool_use") {
        if (!hidden_tool_names.includes(block.name)) {
          entries.push({
            block,
            merged_index: mergedIndex,
            source_message_id: mergedContentSourceMessageIds[mergedIndex] || "",
          });
        }
        return;
      }
      if (block.type === "tool_result") {
        if (!hiddenToolUseIds.has(block.tool_use_id)) {
          entries.push({
            block,
            merged_index: mergedIndex,
            source_message_id: mergedContentSourceMessageIds[mergedIndex] || "",
          });
        }
      }
    });

    return entries;
  }, [hiddenToolUseIds, hidden_tool_names, mergedContent, mergedContentSourceMessageIds]);

  const visibleOrderedAssistantContent = useMemo(() => {
    return visibleOrderedAssistantEntries.map((entry) => entry.block);
  }, [visibleOrderedAssistantEntries]);

  const orderedAssistantStreamingIndexes = useMemo(() => {
    const nextIndexes = new Set<number>();

    visibleOrderedAssistantEntries.forEach((entry, visibleIndex) => {
      if (streamingBlockIndexes.has(entry.merged_index)) {
        nextIndexes.add(visibleIndex);
      }
    });

    return nextIndexes;
  }, [streamingBlockIndexes, visibleOrderedAssistantEntries]);

  const visibleAssistantTurns = useMemo<AssistantTurnEntry[]>(() => {
    const turn_map = new Map<string, AssistantTurnEntry>();
    assistantMessages.forEach((message) => {
      turn_map.set(message.message_id, {
        message_id: message.message_id,
        content: [],
        text_content: [],
        streaming_indexes: new Set<number>(),
        text_streaming_indexes: new Set<number>(),
      });
    });

    visibleOrderedAssistantEntries.forEach((entry) => {
      const turn = turn_map.get(entry.source_message_id);
      if (!turn) {
        return;
      }

      const content_index = turn.content.length;
      turn.content.push(entry.block);
      if (streamingBlockIndexes.has(entry.merged_index)) {
        turn.streaming_indexes.add(content_index);
      }

      if (entry.block.type === "text" && entry.block.text.trim()) {
        const text_index = turn.text_content.length;
        turn.text_content.push(entry.block);
        if (streamingBlockIndexes.has(entry.merged_index)) {
          turn.text_streaming_indexes.add(text_index);
        }
      }
    });

    return assistantMessages
      .map((message) => turn_map.get(message.message_id))
      .filter((turn): turn is AssistantTurnEntry => Boolean(turn && turn.content.length > 0));
  }, [assistantMessages, streamingBlockIndexes, visibleOrderedAssistantEntries]);

  const orderedProjection = useMemo<ContentProjection>(() => ({
    content: visibleOrderedAssistantContent,
    streaming_indexes: orderedAssistantStreamingIndexes,
  }), [orderedAssistantStreamingIndexes, visibleOrderedAssistantContent]);

  const lastAssistantTurn = useMemo(
    () => visibleAssistantTurns.at(-1) ?? null,
    [visibleAssistantTurns],
  );

  const finalTailEntries = useMemo<OrderedAssistantEntry[]>(() => {
    if (!lastAssistantTurn) {
      return [];
    }

    const tail_entries: OrderedAssistantEntry[] = [];
    for (let index = visibleOrderedAssistantEntries.length - 1; index >= 0; index -= 1) {
      const entry = visibleOrderedAssistantEntries[index];
      if (entry.source_message_id !== lastAssistantTurn.message_id) {
        break;
      }
      if (entry.block.type !== "text" || !entry.block.text.trim()) {
        break;
      }
      tail_entries.unshift(entry);
    }
    return tail_entries;
  }, [lastAssistantTurn, visibleOrderedAssistantEntries]);

  const finalTailText = useMemo(() => {
    return finalTailEntries
      .map((entry) => entry.block)
      .filter((block): block is Extract<ContentBlock, { type: "text" }> => block.type === "text")
      .map((block) => block.text)
      .join("\n\n")
      .trim();
  }, [finalTailEntries]);

  const archivedProcessProjection = useMemo<ContentProjection>(() => {
    const result_text = resultMessage?.result?.trim();
    const should_strip_tail = finalTailEntries.length > 0
      && (
        !result_text
        || finalTailText === result_text
        || finalTailEntries
          .map((entry) => entry.block)
          .filter((block): block is Extract<ContentBlock, { type: "text" }> => block.type === "text")
          .map((block) => block.text)
          .join("")
          .trim() === result_text
      );

    if (should_strip_tail) {
      const tail_indexes = new Set(finalTailEntries.map((entry) => entry.merged_index));
      return projectionFromOrderedEntries(
        visibleOrderedAssistantEntries.filter((entry) => !tail_indexes.has(entry.merged_index)),
        streamingBlockIndexes,
      );
    }

    if (!result_text && lastAssistantTurn) {
      return projectionFromOrderedEntries(
        visibleOrderedAssistantEntries.filter((entry) => entry.source_message_id !== lastAssistantTurn.message_id),
        streamingBlockIndexes,
      );
    }

    return projectionFromOrderedEntries(visibleOrderedAssistantEntries, streamingBlockIndexes);
  }, [
    finalTailEntries,
    finalTailText,
    lastAssistantTurn,
    resultMessage,
    streamingBlockIndexes,
    visibleOrderedAssistantEntries,
  ]);

  const fallbackFinalAssistantContent = useMemo(() => {
    if (finalTailEntries.length > 0) {
      return finalTailEntries.map((entry) => entry.block);
    }
    if (!lastAssistantTurn) {
      return null;
    }
    if (lastAssistantTurn.text_content.length > 0) {
      return lastAssistantTurn.text_content;
    }
    if (lastAssistantTurn.content.length > 0) {
      return lastAssistantTurn.content;
    }
    return null;
  }, [finalTailEntries, lastAssistantTurn]);

  const fallbackFinalAssistantStreamingIndexes = useMemo(() => {
    if (finalTailEntries.length > 0) {
      const next_indexes = new Set<number>();
      finalTailEntries.forEach((entry, index) => {
        if (streamingBlockIndexes.has(entry.merged_index)) {
          next_indexes.add(index);
        }
      });
      return next_indexes;
    }
    if (!lastAssistantTurn) {
      return new Set<number>();
    }
    if (lastAssistantTurn.text_content.length > 0) {
      return lastAssistantTurn.text_streaming_indexes;
    }
    return lastAssistantTurn.streaming_indexes;
  }, [finalTailEntries, lastAssistantTurn, streamingBlockIndexes]);

  const directOrderedProjection = useMemo<ContentProjection>(() => {
    if (assistant_content_mode === "dm_live" || assistant_content_mode === "room_thread") {
      return orderedProjection;
    }
    return { content: [], streaming_indexes: new Set<number>() };
  }, [assistant_content_mode, orderedProjection]);

  const processProjection = useMemo<ContentProjection>(() => {
    if (assistant_content_mode === "dm_archived") {
      return archivedProcessProjection;
    }
    return { content: [], streaming_indexes: new Set<number>() };
  }, [archivedProcessProjection, assistant_content_mode]);

  const finalAssistantContent = useMemo<string | ContentBlock[] | null>(() => {
    if (assistant_content_mode === "dm_live" || assistant_content_mode === "room_thread") {
      return null;
    }

    const result_text = resultMessage?.result?.trim();
    if (result_text) {
      return result_text;
    }

    return fallbackFinalAssistantContent;
  }, [assistant_content_mode, fallbackFinalAssistantContent, resultMessage]);

  const finalAssistantStreamingIndexes = useMemo(() => {
    if (assistant_content_mode === "dm_live" || assistant_content_mode === "room_thread") {
      return new Set<number>();
    }
    if (typeof finalAssistantContent === "string") {
      return new Set<number>();
    }
    return fallbackFinalAssistantStreamingIndexes;
  }, [assistant_content_mode, fallbackFinalAssistantStreamingIndexes, finalAssistantContent]);

  const finalAssistantText = useMemo(() => {
    if (typeof finalAssistantContent === "string") {
      return finalAssistantContent;
    }
    return extractTextFromContentBlocks(finalAssistantContent);
  }, [finalAssistantContent]);
  const shouldRenderDirectAssistantContent = directOrderedProjection.content.length > 0;
  const hasVisibleProcess = processProjection.content.length > 0 || (pending_permission && !hasInlinePendingTool);
  const shouldRenderProcessCallchain = assistant_content_mode === "dm_archived" && hasVisibleProcess;

  const processSummary = useMemo(() => {
    let toolCount = 0;
    let thinkingCount = 0;
    let errorCount = 0;

    for (const block of processProjection.content) {
      if (block.type === "thinking") {
        thinkingCount += 1;
        continue;
      }
      if (block.type === "tool_use") {
        toolCount += 1;
        continue;
      }
      if (block.type === "tool_result" && block.is_error) {
        errorCount += 1;
      }
    }

    if (pending_permission) {
      return "等待你的确认后继续";
    }

    const summaryParts: string[] = [];
    if (thinkingCount > 0) {
      summaryParts.push(`${thinkingCount} 段思路`);
    }
    if (toolCount > 0) {
      summaryParts.push(`${toolCount} 次动作`);
    }
    if (errorCount > 0) {
      summaryParts.push(`${errorCount} 个异常`);
    }

    return summaryParts.length > 0 ? summaryParts.join(" · ") : "查看过程";
  }, [pending_permission, processProjection.content]);

  const shouldHideAssistantContent = useMemo(() => {
    if (
      stream_status === 'pending'
      || stream_status === 'streaming'
      || stream_status === 'cancelled'
      || stream_status === 'error'
    ) {
      return false;
    }

    if (directOrderedProjection.content.length > 0) {
      return false;
    }
    if (processProjection.content.length > 0) {
      return false;
    }
    if (typeof finalAssistantContent === "string") {
      return !finalAssistantContent.trim();
    }
    if (finalAssistantContent && finalAssistantContent.length > 0) {
      return false;
    }
    return !resultMessage;
  }, [
    directOrderedProjection.content.length,
    finalAssistantContent,
    processProjection.content.length,
    resultMessage,
    stream_status,
  ]);

  const shouldRenderAssistantText = Boolean(
    typeof finalAssistantContent === "string"
      ? finalAssistantContent.trim()
      : finalAssistantContent?.length,
  );

  useEffect(() => {
    if (pending_permission || (is_last_round && is_loading)) {
      setIsProcessExpanded(true);
    }
  }, [is_last_round, is_loading, pending_permission]);

  // 操作
  const handleCopyUser = useCallback(async () => {
    if (!userContent) return;
    try {
      await navigator.clipboard.writeText(userContent);
      setCopiedUser(true);
      setTimeout(() => setCopiedUser(false), 2000);
    } catch (error) {
      console.error('Failed to copy:', error);
    }
  }, [userContent]);

  const handleCopyAssistant = useCallback(async () => {
    const text = finalAssistantText;
    if (!text) return;
    try {
      await navigator.clipboard.writeText(text);
      setCopiedAssistant(true);
      setTimeout(() => setCopiedAssistant(false), 2000);
    } catch (error) {
      console.error('Failed to copy:', error);
    }
  }, [finalAssistantText]);

  const showCursor = is_last_round && is_loading && streamingBlockIndexes.size > 0;
  const canCopyAssistant = Boolean(finalAssistantText?.trim());
  const shouldShowAssistantFooter = (
    assistant_content_mode === "dm_archived" || assistant_content_mode === "room_result"
  ) && !is_loading && (Boolean(stats) || canCopyAssistant);

  // Per-message stop: visible when this bubble is actively pending/streaming
  const can_stop_message = on_stop_message && (stream_status === 'pending' || stream_status === 'streaming');
  const handle_stop_message = useCallback(() => {
    if (!on_stop_message || !firstAssistant) return;
    on_stop_message(firstAssistant.message_id);
  }, [on_stop_message, firstAssistant]);
  const pendingPermissionBlock = pending_permission && !hasInlinePendingTool ? (
    <div className="mt-3 rounded-xl bg-slate-50/70 p-3">
      <ToolBlock
        tool_use={{
          type: "tool_use",
          id: `pending_${pending_permission.request_id}`,
          name: pending_permission.tool_name,
          input: pending_permission.tool_input,
        }}
        status="waiting_permission"
        permission_request={{
          request_id: pending_permission.request_id,
          tool_input: pending_permission.tool_input,
          risk_level: pending_permission.risk_level,
          risk_label: pending_permission.risk_label,
          summary: pending_permission.summary,
          suggestions: pending_permission.suggestions,
          expires_at: pending_permission.expires_at,
          on_allow: (updated_permissions) => on_permission_response?.({
            decision: "allow",
            updated_permissions,
          }),
          on_deny: (updated_permissions) => on_permission_response?.({
            decision: "deny",
            updated_permissions,
          }),
        }}
      />
    </div>
  ) : null;

  // Pretext-based streaming min-height: measure the current assistant text
  // and hold the container at that height so scroll doesn't jump on each token.
  // Throttled to run at most once every 150ms — pretext layout is fast but
  // calling it on every token (100/sec) would still burn meaningful CPU.
  const contentAreaRef = useRef<HTMLDivElement>(null);
  const streamingMinHeight = useRef(60);
  const layoutThrottleRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    const layout_text = assistant_content_mode === "dm_live" || assistant_content_mode === "room_thread"
      ? extractTextFromContentBlocks(directOrderedProjection.content)
      : finalAssistantText;

    if (!showCursor || !layout_text) return;
    if (layoutThrottleRef.current !== null) return; // already scheduled

    layoutThrottleRef.current = setTimeout(() => {
      layoutThrottleRef.current = null;
      const el = contentAreaRef.current;
      if (!el) return;
      try {
        const width = el.offsetWidth || 640;
        const prepared = prepare(layout_text, "400 14px ui-sans-serif, system-ui, sans-serif");
        const result = layout(prepared, width, 28);
        streamingMinHeight.current = Math.max(streamingMinHeight.current, result.height);
      } catch { /* keep previous estimate */ }
    }, 150);
  }, [assistant_content_mode, directOrderedProjection.content, finalAssistantText, showCursor]);

  // Reset on new stream; cancel any pending throttled layout
  useEffect(() => {
    if (!showCursor) {
      streamingMinHeight.current = 60;
      if (layoutThrottleRef.current !== null) {
        clearTimeout(layoutThrottleRef.current);
        layoutThrottleRef.current = null;
      }
    }
  }, [showCursor]);

  // 格式化时间
  const formatTime = (ts: number) => {
    const date = new Date(ts);
    return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
  };

  return (
    <div
      className={cn(
        "w-full min-w-0 animate-in fade-in slide-in-from-bottom-2 duration-300",
        "space-y-2 py-3",
        !compact && "border-b border-slate-200/75",
        class_name,
      )}>

      {/* ═══════════════════════ 用户消息 ═══════════════════════ */}
      {userMessage && (
        <div className={cn("w-full", compact ? "px-0.5" : "px-2 sm:px-3")}>
          <div className={cn("mx-auto w-full", compact ? "max-w-full" : "max-w-[980px]")}>
            <div className="group grid min-w-0 grid-cols-[40px_minmax(0,1fr)] gap-3">
              <div className="flex h-10 w-10 items-center justify-center rounded-lg border border-slate-200 bg-slate-50 text-slate-600">
                <User className="h-4 w-4" />
              </div>
              <div className="relative min-w-0">
                {/* 头部 */}
                <div className={cn(
                  "flex items-center gap-2",
                  compact ? "h-[26px]" : "h-7",
                )}>
                  <span className="shrink-0 text-sm font-bold text-slate-900">你</span>
                  <span className="hidden shrink-0 text-xs text-slate-500 sm:inline">
                    {userMessage.timestamp ? formatTime(userMessage.timestamp) : "--:--"}
                  </span>
                  <div className="flex-1" />

                  {/* 操作按钮 */}
                  <div className="flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
                    <button
                      aria-label="复制消息"
                      onClick={handleCopyUser}
                      className={cn(
                        "p-1 rounded transition-colors focus-visible:ring-2 focus-visible:ring-primary/50",
                        copiedUser ? "text-success" : "text-muted-foreground/50 hover:text-foreground"
                      )}
                    >
                      {copiedUser ? <Check className="w-3 h-3" /> : <Copy className="w-3 h-3" />}
                    </button>
                    {on_edit_user_message && (
                      <button
                        aria-label="编辑消息"
                        onClick={() => {
                          const newContent = prompt('编辑消息:', userContent);
                          if (newContent && newContent !== userContent) {
                            on_edit_user_message(userMessage.message_id, newContent);
                          }
                        }}
                        className="p-1 rounded text-muted-foreground/50 hover:text-foreground transition-colors focus-visible:ring-2 focus-visible:ring-primary/50"
                      >
                        <Edit2 className="w-3 h-3" />
                      </button>
                    )}
                  </div>
                </div>

                {/* 内容 */}
                <div className="pb-1 pt-1">
                  <p className={cn(
                    "whitespace-pre-wrap text-slate-900 wrap-anywhere",
                    compact ? "text-[13px] leading-6" : "text-[15px] leading-7",
                  )}>
                    {userContent}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* ═══════════════════════ 助手消息 ═══════════════════════ */}
      {!shouldHideAssistantContent && (
        <div className={cn("w-full", compact ? "px-0.5" : "px-2 sm:px-3")}>
          <div className={cn("mx-auto w-full", compact ? "max-w-full" : "max-w-[980px]")}>
            <div className={cn("group grid min-w-0 grid-cols-[40px_minmax(0,1fr)]", compact ? "gap-2" : "gap-3")}>
              <div className="flex h-10 w-10 items-center justify-center rounded-lg border border-slate-200 bg-slate-50 text-slate-600">
                <Bot className="h-4 w-4" />
              </div>

              <div className="relative min-w-0">
                {/* 优雅的头部栏 */}
                <div className={cn(
                  "flex min-w-0 items-center gap-2",
                  compact ? "h-7 pb-0.5" : "h-7 pb-0.5",
                )}>
                  <span className="shrink-0 text-sm font-bold text-slate-900">
                    {current_agent_name || "协作成员"}
                  </span>

                  {/* 时间 */}
                  <span className="hidden shrink-0 text-xs text-slate-500 sm:inline">
                    {timestamp ? formatTime(timestamp) : "--:--"}
                  </span>

                  {/* 模型 */}
                  {model ? <span className="min-w-0 truncate text-xs text-slate-400">{model}</span> : null}

                  <div className="flex-1" />

                  {assistant_header_action ? (
                    <div className="shrink-0">
                      {assistant_header_action}
                    </div>
                  ) : null}

                  {/* Per-message stop button (Room 并发模式) */}
                  {can_stop_message && (
                    <button
                      type="button"
                      aria-label="停止生成"
                      onClick={handle_stop_message}
                      className="flex items-center gap-1 rounded px-1.5 py-0.5 text-xs text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-700"
                    >
                      <Square className="h-3 w-3 fill-current" />
                      <span>停止</span>
                    </button>
                  )}

                </div>

                {/* 内容区 */}
                <div
                  ref={contentAreaRef}
                  className={cn(
                    compact ? "min-w-0 max-w-full overflow-x-hidden pb-2 pt-1 text-[13px] leading-6" : "min-w-0 max-w-full overflow-x-hidden pb-2 pt-1 text-[15px] leading-7",
                  )}
                  style={showCursor ? { minHeight: streamingMinHeight.current } : undefined}
                >

                  {/* Room 并发：pending 占位动画 */}
                  {stream_status === 'pending' && mergedContent.length === 0 && (
                    <div className="flex items-center gap-1.5 py-1">
                      <span className="h-1.5 w-1.5 animate-bounce rounded-full bg-slate-400 [animation-delay:0ms]" />
                      <span className="h-1.5 w-1.5 animate-bounce rounded-full bg-slate-400 [animation-delay:150ms]" />
                      <span className="h-1.5 w-1.5 animate-bounce rounded-full bg-slate-400 [animation-delay:300ms]" />
                    </div>
                  )}

                  {/* Room 并发：已取消标记 */}
                  {stream_status === 'cancelled' && mergedContent.length === 0 && (
                    <span className="text-xs text-slate-400 italic">已停止</span>
                  )}

                  {stream_status === 'error' && mergedContent.length === 0 && (
                    <span className="text-xs text-rose-500 italic">执行失败</span>
                  )}

                  {shouldRenderDirectAssistantContent ? (
                    <div>
                      <ContentRenderer
                        content={directOrderedProjection.content}
                        is_streaming={showCursor}
                        streaming_block_indexes={directOrderedProjection.streaming_indexes}
                        pending_permission={pending_permission}
                        on_permission_response={on_permission_response}
                        on_open_workspace_file={on_open_workspace_file}
                        hidden_tool_names={hidden_tool_names}
                      />
                      {pendingPermissionBlock}
                    </div>
                  ) : null}

                  {shouldRenderProcessCallchain ? (
                    <div>
                      <button
                        className="flex w-full items-center gap-2 px-0 py-1.5 text-left transition-colors hover:text-slate-700"
                        onClick={() => setIsProcessExpanded((previous) => !previous)}
                        type="button"
                      >
                        <Wrench className="h-3 w-3 shrink-0 text-slate-300" />
                        <div className="min-w-0 flex-1 truncate text-[12px] font-medium text-slate-500">
                          {processSummary}
                        </div>
                        <div className="text-slate-300">
                          {isProcessExpanded ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
                        </div>
                      </button>

                      {isProcessExpanded ? (
                        <div className="pt-1">
                          <ContentRenderer
                            content={processProjection.content}
                            is_streaming={showCursor}
                            streaming_block_indexes={processProjection.streaming_indexes}
                            pending_permission={pending_permission}
                            on_permission_response={on_permission_response}
                            on_open_workspace_file={on_open_workspace_file}
                            hidden_tool_names={hidden_tool_names}
                          />

                          {pendingPermissionBlock}
                        </div>
                      ) : null}
                    </div>
                  ) : null}

                  {shouldRenderAssistantText ? (
                    <div className={cn(shouldRenderProcessCallchain)}>
                      <ContentRenderer
                        content={finalAssistantContent ?? []}
                        is_streaming={showCursor}
                        streaming_block_indexes={finalAssistantStreamingIndexes}
                        on_open_workspace_file={on_open_workspace_file}
                      />
                    </div>
                  ) : null}
                </div>

                {/* 底部统计栏（完成后显示） */}
                {shouldShowAssistantFooter && (
                  <MessageStats
                    stats={stats || undefined}
                    show_cursor={showCursor}
                    compact={compact}
                    copied_assistant={copiedAssistant}
                    on_copy_assistant={canCopyAssistant ? handleCopyAssistant : undefined}
                  />
                )}

              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// 仅在影响视觉输出的关键属性变化时重新渲染，避免流式阶段产生无效更新。
export const MessageItem = memo(MessageItemInner, (prev, next) => {
  if (prev.round_id !== next.round_id) return false;
  if (prev.is_last_round !== next.is_last_round) return false;
  if (prev.is_loading !== next.is_loading) return false;
  if (prev.compact !== next.compact) return false;
  if (prev.current_agent_name !== next.current_agent_name) return false;
  if (prev.pending_permission !== next.pending_permission) return false;
  if (prev.assistant_header_action !== next.assistant_header_action) return false;
  if (prev.assistant_content_mode !== next.assistant_content_mode) return false;
  if (prev.class_name !== next.class_name) return false;
  // 消息数组按引用比较，上游流式合并会返回新数组，足以标记内容变化。
  if (prev.messages !== next.messages) return false;
  // 回调由上游 useCallback 保持稳定，这里不做深比较以避免额外开销。
  return true;
});

function projectionFromOrderedEntries(
  entries: OrderedAssistantEntry[],
  streaming_block_indexes: Set<number>,
): ContentProjection {
  const content: ContentBlock[] = [];
  const streaming_indexes = new Set<number>();

  entries.forEach((entry, index) => {
    content.push(entry.block);
    if (streaming_block_indexes.has(entry.merged_index)) {
      streaming_indexes.add(index);
    }
  });

  return { content, streaming_indexes };
}

function extractTextFromContentBlocks(content?: ContentBlock[] | null): string {
  if (!content || content.length === 0) {
    return "";
  }

  const texts: string[] = [];
  content.forEach((block) => {
    if (block.type === "text" && block.text.trim()) {
      texts.push(block.text);
    }
  });
  return texts.join("\n\n");
}

export default MessageItem;
