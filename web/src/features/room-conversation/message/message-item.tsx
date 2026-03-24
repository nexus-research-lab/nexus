/**
 * Message Component
 *
 *
 */

"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Check, Copy, Edit2, Terminal, User } from "lucide-react";
import { cn } from "@/lib/utils";
import { ContentBlock, Message, ResultMessage } from "@/types/message";
import { PendingPermission, PermissionDecisionPayload } from "@/types/permission";
import { MessageItemProps } from "@/types/room-conversation";
import { ContentRenderer } from "./content-renderer";
import { MessageStats } from "./message-stats";
import { ToolBlock } from "./block/tool-block";

export function MessageItem(
  {
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
  const roundRef = useRef<HTMLDivElement>(null);
  const [copiedUser, setCopiedUser] = useState(false);
  const [copiedAssistant, setCopiedAssistant] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [isRegenerating, setIsRegenerating] = useState(false);

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

  // 获取纯文本内容用于复制
  const assistantTextContent = useMemo(() => {
    const texts: string[] = [];
    for (const block of mergedContent) {
      if (block.type === 'text' && block.text) {
        texts.push(block.text);
      }
    }
    // 如果有最终回答，也加入
    if (resultMessage?.result) {
      texts.push(resultMessage.result);
    }
    return texts.join('\n\n');
  }, [mergedContent, resultMessage]);

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

  // 滚动
  useEffect(() => {
    if (is_last_round && roundRef.current) {
      roundRef.current.scrollIntoView({ behavior: 'smooth', block: 'end' });
    }
  }, [is_last_round]);

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
    <div ref={roundRef}
      className={cn("w-full min-w-0 space-y-3 animate-in fade-in slide-in-from-bottom-2 duration-300", class_name)}>

      {/* ═══════════════════════ 用户消息 ═══════════════════════ */}
      {userMessage && (
        <div className="w-full px-1 sm:px-4">
          <div className="mx-auto w-full max-w-[980px]">
            <div className="group flex min-w-0 items-end justify-end gap-3">
              <div className={cn(
                "radius-shell-lg relative min-w-0 max-w-[78%] overflow-hidden transition-all duration-300",
                "workspace-card px-4 sm:px-5"
              )}>
                {/* 头部 */}
                <div className="flex h-10 items-center justify-end gap-2 border-b border-white/18">
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

                  {/* 时间 */}
                  <span className="text-[10px] font-mono text-slate-700/42 sm:inline">
                    {userMessage.timestamp ? formatTime(userMessage.timestamp) : '--:--'} ｜
                  </span>

                  {/* 头像在右边 */}
                  <span className="text-[10px] font-semibold uppercase tracking-[0.16em] text-sky-700/62">你</span>
                  <User className="w-3 h-3 text-sky-700/62" />
                </div>

                {/* 内容 */}
                <div className="py-4">
                  <p className="text-sm leading-relaxed text-right whitespace-pre-wrap text-slate-900/86 [overflow-wrap:anywhere]">
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
        <div className="w-full px-1 sm:px-4">
          <div className="mx-auto w-full max-w-[980px]">
            <div className="group flex min-w-0 items-start gap-3">

              <div className={cn(
                "radius-shell-lg relative min-w-0 flex-1 overflow-hidden workspace-card transition-all duration-500",
                showCursor && "shadow-[0_24px_44px_rgba(133,119,255,0.16)]",
                isCompleted && "shadow-[0_24px_44px_rgba(102,217,143,0.14)]"
              )}>
                {/* 扫描线效果 */}
                {showCursor && (
                  <div className="absolute inset-0 pointer-events-none overflow-hidden">
                    <div className="absolute inset-0 animate-scan" />
                  </div>
                )}

                {/* 优雅的头部栏 */}
                <div className="flex min-w-0 h-10 items-center gap-2 border-b border-white/28 px-3 sm:px-4">
                  <div className="workspace-chip flex h-6 w-6 shrink-0 items-center justify-center rounded-full">
                    <Terminal className="w-3 h-3 text-slate-800/70" />
                  </div>
                  <span className="shrink-0 text-[10px] font-semibold uppercase tracking-[0.16em] text-slate-900/72">
                    {current_agent_name || "协作成员"}
                  </span>

                  {/* 时间 */}
                  <span className="hidden shrink-0 text-[10px] font-mono text-slate-700/40 sm:inline">
                    | {timestamp ? formatTime(timestamp) : '--:--'} |
                  </span>

                  {/* 模型 */}
                  {model && (
                    <span className="min-w-0 truncate text-[10px] text-slate-700/40">{model}</span>
                  )}

                </div>

                {/* 内容区 */}
                <div className={cn(
                  "min-w-0 p-4 text-sm leading-relaxed sm:p-5",
                  showCursor && "min-h-[60px]"
                )}>

                  <ContentRenderer
                    content={mergedContent}
                    is_streaming={showCursor}
                    streaming_block_indexes={streamingBlockIndexes}
                    pending_permission={pending_permission}
                    on_permission_response={on_permission_response}
                    on_open_workspace_file={on_open_workspace_file}
                    hidden_tool_names={hidden_tool_names}
                  />

                  {pending_permission && !hasInlinePendingTool && (
                    <div className="mt-4">
                      <ToolBlock
                        tool_use={{
                          type: 'tool_use',
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
                            decision: 'allow',
                            updated_permissions,
                          }),
                          on_deny: (updated_permissions) => on_permission_response?.({
                            decision: 'deny',
                            updated_permissions,
                          }),
                        }}
                      />
                    </div>
                  )}
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
