/**
 * Message Component
 *
 *
 */

"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Bot, Check, ChevronDown, ChevronRight, Copy, Edit2, User, Wrench } from "lucide-react";
import { cn } from "@/lib/utils";
import { ContentBlock, Message, ResultMessage } from "@/types/message";
import { PendingPermission, PermissionDecisionPayload } from "@/types/permission";
import { ContentRenderer } from "./content-renderer";
import { MessageStats } from "./message-stats";
import { ToolBlock } from "./block/tool-block";

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
  on_delete?: (round_id: string) => Promise<void>;
  on_regenerate?: (round_id: string) => Promise<void>;
  on_edit_user_message?: (message_id: string, new_content: string) => void;
  on_open_workspace_file?: (path: string) => void;
  class_name?: string;
}

export function MessageItem(
  {
    compact = false,
    current_agent_name,
    round_id,
    messages,
    is_last_round,
    is_loading,
    pending_permission,
    on_permission_response,
    hidden_tool_names = ['TodoWrite'],
    on_delete,
    on_regenerate,
    on_edit_user_message,
    on_open_workspace_file,
    class_name,
  }: MessageItemProps) {
  const [copiedUser, setCopiedUser] = useState(false);
  const [copiedAssistant, setCopiedAssistant] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [isRegenerating, setIsRegenerating] = useState(false);
  const [isProcessExpanded, setIsProcessExpanded] = useState(false);

  // 分离消息
  const { userMessage, assistantMessages, resultMessage } = useMemo(() => {
    const user = messages.find(m => m.role === 'user');
    const result = messages.find(m => m.role === 'result') as ResultMessage | undefined;
    const assistant = messages.filter(m => m.role === 'assistant');
    return { userMessage: user, assistantMessages: assistant, resultMessage: result };
  }, [messages]);

  const streamingAssistantMessageId = useMemo(() => {
    if (!is_last_round || !is_loading) {
      return null;
    }

    for (let index = assistantMessages.length - 1; index >= 0; index -= 1) {
      const message = assistantMessages[index];
      if (message.is_complete === false) {
        return message.message_id;
      }
    }

    return null;
  }, [assistantMessages, is_last_round, is_loading]);

  // 合并并去重 assistant 内容
  const { mergedContent, streamingBlockIndexes } = useMemo(() => {
    const allBlocks: ContentBlock[] = [];
    const nextStreamingBlockIndexes = new Set<number>();
    const seenToolIds = new Set<string>();

    for (const msg of assistantMessages) {
      if (!Array.isArray(msg.content)) continue;
      const isStreamingMessage = msg.message_id === streamingAssistantMessageId;
      const streamingContentIndex = isStreamingMessage
        ? findLastStreamableBlockIndex(msg.content)
        : -1;

      msg.content.forEach((block, blockIndex) => {
        if (!block) {
          return;
        }
        if (block.type === 'tool_use' && block.id) {
          if (seenToolIds.has(block.id)) return;
          seenToolIds.add(block.id);
        }
        if (block.type === 'tool_result' && block.tool_use_id) {
          if (seenToolIds.has(`result_${block.tool_use_id}`)) return;
          seenToolIds.add(`result_${block.tool_use_id}`);
        }

        const nextIndex = allBlocks.length;
        allBlocks.push(block);
        if (isStreamingMessage && blockIndex === streamingContentIndex) {
          nextStreamingBlockIndexes.add(nextIndex);
        }
      });
    }
    return {
      mergedContent: allBlocks,
      streamingBlockIndexes: nextStreamingBlockIndexes,
    };
  }, [assistantMessages, streamingAssistantMessageId]);

  const visibleAssistantTextContent = useMemo(() => {
    return mergedContent.filter((block) => (
      block.type === "text" && Boolean(block.text.trim())
    ));
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

  // 获取纯文本内容用于复制
  const assistantTextContent = useMemo(() => {
    const texts: string[] = [];
    for (const block of visibleAssistantTextContent) {
      if (block.type === "text" && block.text) {
        texts.push(block.text);
      }
    }
    return texts.join("\n\n");
  }, [visibleAssistantTextContent]);

  // 元数据
  const firstAssistant = assistantMessages[0];
  const model = firstAssistant && 'model' in firstAssistant ? firstAssistant.model : undefined;
  const timestamp = firstAssistant?.timestamp || resultMessage?.timestamp;

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
  const hasFinalAnswer = !!resultMessage;
  const userContent = useMemo(() => {
    if (!userMessage) return '';
    return typeof userMessage.content === 'string' ? userMessage.content : '';
  }, [userMessage]);

  const shouldHideAssistantContent = useMemo(() => {
    if (mergedContent.length === 0) return true;
    return mergedContent.every(block => {
      if (block.type === 'text') return !block.text?.trim();
      if (block.type === 'tool_use') return hidden_tool_names.includes(block.name);
      return block.type === 'tool_result';
    });
  }, [mergedContent, hidden_tool_names]);

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

  const visibleProcessContent = useMemo(() => {
    return mergedContent.filter((block) => {
      if (block.type === "thinking") {
        return Boolean(block.thinking?.trim());
      }
      if (block.type === "tool_use") {
        return !hidden_tool_names.includes(block.name);
      }
      if (block.type === "tool_result") {
        return !hiddenToolUseIds.has(block.tool_use_id);
      }
      return false;
    });
  }, [hiddenToolUseIds, hidden_tool_names, mergedContent]);

  const processStreamingIndexes = useMemo(() => {
    const nextIndexes = new Set<number>();
    let processIndex = 0;

    mergedContent.forEach((block, index) => {
      const isVisibleThinking = block.type === "thinking" && Boolean(block.thinking?.trim());
      const isVisibleToolUse = block.type === "tool_use" && !hidden_tool_names.includes(block.name);
      const isVisibleToolResult = block.type === "tool_result" && !hiddenToolUseIds.has(block.tool_use_id);

      if (isVisibleThinking || isVisibleToolUse || isVisibleToolResult) {
        if (streamingBlockIndexes.has(index)) {
          nextIndexes.add(processIndex);
        }
        processIndex += 1;
      }
    });

    return nextIndexes;
  }, [hiddenToolUseIds, hidden_tool_names, mergedContent, streamingBlockIndexes]);

  const processSummary = useMemo(() => {
    let toolCount = 0;
    let thinkingCount = 0;
    let errorCount = 0;

    for (const block of visibleProcessContent) {
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
    if (is_last_round && is_loading) {
      return toolCount > 0 ? `正在处理 ${toolCount} 个动作` : "正在整理过程";
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
  }, [is_last_round, is_loading, pending_permission, visibleProcessContent]);

  const hasVisibleProcess = visibleProcessContent.length > 0 || (pending_permission && !hasInlinePendingTool);
  const shouldRenderAssistantText = visibleAssistantTextContent.length > 0;

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
    if (!assistantTextContent) return;
    try {
      await navigator.clipboard.writeText(assistantTextContent);
      setCopiedAssistant(true);
      setTimeout(() => setCopiedAssistant(false), 2000);
    } catch (error) {
      console.error('Failed to copy:', error);
    }
  }, [assistantTextContent]);

  const handleDelete = useCallback(async () => {
    if (!on_delete || isDeleting) return;
    setIsDeleting(true);
    try {
      await on_delete(round_id);
    } finally {
      setIsDeleting(false);
    }
  }, [on_delete, round_id, isDeleting]);

  const handleRegenerate = useCallback(async () => {
    if (!on_regenerate || isRegenerating) return;
    setIsRegenerating(true);
    try {
      await on_regenerate(round_id);
    } finally {
      setIsRegenerating(false);
    }
  }, [on_regenerate, round_id, isRegenerating]);

  const showCursor = is_last_round && is_loading && streamingBlockIndexes.size > 0;
  const isCompleted = hasFinalAnswer && !is_loading;
  const canOperateRound = !!userMessage && !is_loading;

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
                    "whitespace-pre-wrap text-slate-900 [overflow-wrap:anywhere]",
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
      {/* 没有可见 assistant 内容时，仍渲染容器以提供删除/重试操作 */}
      {(!shouldHideAssistantContent || canOperateRound) && (
        <div className={cn("w-full", compact ? "px-0.5" : "px-2 sm:px-3")}>
          <div className={cn("mx-auto w-full", compact ? "max-w-full" : "max-w-[980px]")}>
            <div className={cn("group grid min-w-0 grid-cols-[40px_minmax(0,1fr)]", compact ? "gap-2" : "gap-3")}>
              <div className="flex h-10 w-10 items-center justify-center rounded-lg border border-slate-200 bg-slate-50 text-slate-600">
                <Bot className="h-4 w-4" />
              </div>

              <div className="relative min-w-0">
                {/* 扫描线效果 */}
                {showCursor && (
                  <div className="absolute inset-0 pointer-events-none overflow-hidden">
                    <div className="absolute inset-0 animate-scan" />
                  </div>
                )}

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

                </div>

                {/* 内容区 */}
                <div className={cn(
                  compact ? "min-w-0 pb-2 pt-1 text-[13px] leading-6" : "min-w-0 pb-2 pt-1 text-[15px] leading-7",
                  showCursor && "min-h-[60px]"
                )}>

                  {hasVisibleProcess ? (
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
                            content={visibleProcessContent}
                            is_streaming={showCursor}
                            streaming_block_indexes={processStreamingIndexes}
                            pending_permission={pending_permission}
                            on_permission_response={on_permission_response}
                            on_open_workspace_file={on_open_workspace_file}
                            hidden_tool_names={hidden_tool_names}
                          />

                          {pending_permission && !hasInlinePendingTool ? (
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
                          ) : null}
                        </div>
                      ) : null}
                    </div>
                  ) : null}

                  {shouldRenderAssistantText ? (
                    <div className={cn(hasVisibleProcess)}>
                      <ContentRenderer
                        content={visibleAssistantTextContent}
                        is_streaming={showCursor}
                        streaming_block_indexes={assistantTextStreamingIndexes}
                        on_open_workspace_file={on_open_workspace_file}
                      />
                    </div>
                  ) : null}
                </div>

                {/* 底部统计栏（完成后显示） */}
                {canOperateRound && (
                  <MessageStats
                    stats={stats || undefined}
                    show_cursor={showCursor}
                    copied_assistant={copiedAssistant}
                    is_regenerating={isRegenerating}
                    is_deleting={isDeleting}
                    on_copy_assistant={handleCopyAssistant}
                    on_regenerate={on_regenerate ? handleRegenerate : undefined}
                    on_delete={on_delete ? handleDelete : undefined}
                  />
                )}

                {/* 底部进度条（流式时） */}
                {showCursor && (
                  <div className="h-0.5 animate-progress-bar" />
                )}
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export default MessageItem;

function findLastStreamableBlockIndex(blocks: ContentBlock[]): number {
  for (let index = blocks.length - 1; index >= 0; index -= 1) {
    const block = blocks[index];
    if (!block) {
      continue;
    }
    if (block.type === 'text' || block.type === 'thinking') {
      return index;
    }
  }

  return -1;
}
