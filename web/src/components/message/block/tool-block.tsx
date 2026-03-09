/**
 * Tool Execution Block Component - 革命性时间线设计
 *
 * 视觉化工具执行：进度条融入卡片、悬浮预览、内联复制
 */

"use client";

import { useState, useCallback, useMemo } from 'react';
import { Check, CheckCircle, ChevronDown, ChevronRight, Clock, Copy, Loader, Terminal, XCircle } from 'lucide-react';
import { cn } from '@/lib/utils';
import { ToolResultContent, ToolUseContent } from '@/types/message';
import { CodeBlock } from './code-block';
import { PermissionDialog } from '@/components/permission/permission-dialog';

// ==================== 类型定义 ====================

interface ToolExecutionBlockProps {
  toolUse: ToolUseContent;
  toolResult?: ToolResultContent;
  status?: 'pending' | 'running' | 'success' | 'error' | 'waiting_permission';
  startTime?: number;
  endTime?: number;
  permissionRequest?: {
    request_id: string;
    tool_input: Record<string, any>;
    onAllow: () => void;
    onDeny: () => void;
  };
}

// ==================== 辅助函数 ====================

/** 获取文件路径的简短显示 */
const getPathDisplay = (input: any): string | null => {
  if (!input) return null;
  if (input.file_path) return input.file_path;
  if (input.path) return input.path;
  if (input.command) return `$ ${input.command.slice(0, 50)}${input.command.length > 50 ? '...' : ''}`;
  return null;
};

/** 获取结果摘要 */
const getResultSummary = (content: any): string => {
  if (typeof content === 'string') {
    return content.slice(0, 80) + (content.length > 80 ? '...' : '');
  }
  return 'JSON 数据';
};

// ==================== 主组件 ====================

export function ToolBlock({
  toolUse,
  toolResult,
  status = 'success',
  startTime,
  endTime,
  permissionRequest,
}: ToolExecutionBlockProps) {
  const [isExpanded, setIsExpanded] = useState(false);
  const [showDetailModal, setShowDetailModal] = useState(false);
  const [copied, setCopied] = useState(false);

  // 复制工具执行结果
  const handleCopyResult = useCallback(async (e: React.MouseEvent) => {
    e.stopPropagation();
    if (!toolResult) return;
    const contentToCopy = typeof toolResult.content === 'string'
      ? toolResult.content
      : JSON.stringify(toolResult.content, null, 2);
    try {
      await navigator.clipboard.writeText(contentToCopy);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (error) {
      console.error('Failed to copy:', error);
    }
  }, [toolResult]);

  // 计算执行时间
  const duration = useMemo(() => {
    if (endTime && startTime) return endTime - startTime;
    if (startTime) return Date.now() - startTime;
    return 0;
  }, [endTime, startTime]);

  // 格式化时间
  const durationText = useMemo(() => {
    if (duration === 0) return '';
    return duration >= 1000 ? `${(duration / 1000).toFixed(1)}s` : `${duration}ms`;
  }, [duration]);

  // 路径显示
  const pathDisplay = useMemo(() => getPathDisplay(toolUse.input), [toolUse.input]);

  // 最终状态
  const finalStatus = toolResult?.is_error ? 'error' : status;
  const hasResult = !!toolResult;
  const isRunning = finalStatus === 'running';
  const isSuccess = finalStatus === 'success';
  const isError = finalStatus === 'error';
  const isWaiting = finalStatus === 'waiting_permission';

  // 状态配色
  const statusColors = {
    pending: 'border-muted-foreground/30',
    running: 'border-primary/50 shadow-[0_0_10px_rgba(0,240,255,0.1)]',
    success: 'border-green-500/40',
    error: 'border-red-500/40',
    waiting_permission: 'border-orange-500/40 shadow-[0_0_10px_rgba(255,165,0,0.1)]',
  };

  return (
    <div className={cn(
      "my-2 border rounded overflow-hidden bg-background/50 backdrop-blur-sm transition-all duration-300",
      statusColors[finalStatus]
    )}>
      {/* ═══════════ 头部栏：工具名+路径+状态+时间 ═══════════ */}
      <div
        className={cn(
          "h-9 px-3 flex items-center gap-2 font-mono text-xs cursor-pointer select-none",
          "hover:bg-primary/5 transition-colors",
          isRunning && "animate-pulse"
        )}
        onClick={() => hasResult && setIsExpanded(!isExpanded)}
      >
        {/* 工具图标 */}
        <div className={cn(
          "w-5 h-5 flex items-center justify-center rounded",
          isSuccess && "text-green-500",
          isError && "text-red-500",
          isRunning && "text-primary",
          isWaiting && "text-orange-500"
        )}>
          {isRunning ? (
            <Loader className="w-3.5 h-3.5 animate-spin" />
          ) : isSuccess ? (
            <CheckCircle className="w-3.5 h-3.5" />
          ) : isError ? (
            <XCircle className="w-3.5 h-3.5" />
          ) : isWaiting ? (
            <Clock className="w-3.5 h-3.5 animate-pulse" />
          ) : (
            <Terminal className="w-3.5 h-3.5" />
          )}
        </div>

        {/* 工具名 */}
        <span className={cn(
          "font-medium uppercase tracking-wider",
          isSuccess && "text-green-500",
          isError && "text-red-500",
          isRunning && "text-primary",
          isWaiting && "text-orange-500"
        )}>
          {toolUse.name}
        </span>

        {/* 分隔符 */}
        <span className="text-muted-foreground/30">│</span>

        {/* 路径/命令 */}
        {pathDisplay && (
          <span className="text-muted-foreground truncate max-w-[300px]">
            {pathDisplay}
          </span>
        )}

        {/* 弹性空间 */}
        <div className="flex-1" />

        {/* 结果摘要（折叠时） */}
        {hasResult && !isExpanded && (
          <span className="text-muted-foreground/60 truncate max-w-[200px] hidden sm:block">
            {getResultSummary(toolResult.content)}
          </span>
        )}

        {/* 复制按钮（有结果时） */}
        {hasResult && (
          <button
            onClick={handleCopyResult}
            className={cn(
              "px-2 py-0.5 rounded text-[10px] uppercase tracking-wider transition-all",
              copied
                ? "text-green-500 bg-green-500/10"
                : "text-muted-foreground hover:text-foreground hover:bg-muted/50"
            )}
          >
            {copied ? '✓' : 'copy'}
          </button>
        )}

        {/* 时间 */}
        {durationText && (
          <>
            <span className="text-muted-foreground/30">│</span>
            <span className="text-muted-foreground/60 tabular-nums">{durationText}</span>
          </>
        )}

        {/* 展开指示器 */}
        {hasResult && (
          <div className="text-muted-foreground/40">
            {isExpanded ? <ChevronDown className="w-3.5 h-3.5" /> : <ChevronRight className="w-3.5 h-3.5" />}
          </div>
        )}
      </div>

      {/* ═══════════ 进度条（运行时） ═══════════ */}
      {isRunning && (
        <div className="h-px bg-gradient-to-r from-transparent via-primary to-transparent animate-pulse" />
      )}

      {/* ═══════════ 展开的结果内容 ═══════════ */}
      {hasResult && isExpanded && (
        <div className="border-t border-border/30">
          <div className="p-3 max-h-[300px] overflow-y-auto custom-scrollbar">
            {typeof toolResult.content === 'string' ? (
              <pre className="text-xs font-mono whitespace-pre-wrap break-all text-foreground/80">
                {toolResult.content}
              </pre>
            ) : (
              <CodeBlock language="json" value={JSON.stringify(toolResult.content, null, 2)} />
            )}
          </div>
        </div>
      )}

      {/* ═══════════ 运行中指示 ═══════════ */}
      {!hasResult && isRunning && (
        <div className="h-8 px-3 flex items-center gap-2 text-xs text-muted-foreground border-t border-border/20">
          <div className="flex gap-1">
            <div className="w-1.5 h-1.5 bg-primary rounded-full animate-[pulse_1s_ease-in-out_infinite]" />
            <div className="w-1.5 h-1.5 bg-primary rounded-full animate-[pulse_1s_ease-in-out_0.2s_infinite]" />
            <div className="w-1.5 h-1.5 bg-primary rounded-full animate-[pulse_1s_ease-in-out_0.4s_infinite]" />
          </div>
          <span className="font-mono text-[10px] uppercase tracking-wider">executing...</span>
        </div>
      )}

      {/* ═══════════ 权限确认 ═══════════ */}
      {permissionRequest && isWaiting && (
        <div className="border-t border-orange-500/20 bg-orange-500/5">
          {/* 参数预览 */}
          <div className="px-3 py-2 max-h-[120px] overflow-y-auto custom-scrollbar border-b border-orange-500/10">
            <pre className="text-[11px] font-mono text-foreground/70 whitespace-pre-wrap break-all">
              {JSON.stringify(permissionRequest.tool_input, null, 2)}
            </pre>
          </div>

          {/* 操作栏 */}
          <div className="h-10 px-3 flex items-center gap-2">
            <span className="text-xs text-orange-500 font-medium flex items-center gap-1.5">
              <Clock className="w-3 h-3" />
              AWAITING_PERMISSION
            </span>
            <div className="flex-1" />
            <button
              onClick={permissionRequest.onDeny}
              className="px-3 py-1 rounded text-xs font-medium border border-border/50 hover:bg-muted transition-colors"
            >
              拒绝
            </button>
            <button
              onClick={permissionRequest.onAllow}
              className="px-3 py-1 rounded text-xs font-medium bg-primary text-primary-foreground hover:bg-primary/90 transition-colors"
            >
              允许执行
            </button>
            <button
              onClick={() => setShowDetailModal(true)}
              className="px-3 py-1 rounded text-xs font-medium border border-border/50 hover:bg-muted transition-colors"
            >
              查看详情
            </button>
          </div>
        </div>
      )}

      {/* 详情弹窗 */}
      {permissionRequest && showDetailModal && (
        <PermissionDialog
          isOpen={showDetailModal}
          toolName={toolUse.name}
          toolInput={toolUse.input}
          onAllow={() => {
            setShowDetailModal(false);
            permissionRequest.onAllow();
          }}
          onDeny={() => {
            setShowDetailModal(false);
            permissionRequest.onDeny();
          }}
          onClose={() => setShowDetailModal(false)}
        />
      )}
    </div>
  );
}
