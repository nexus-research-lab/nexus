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
import { PermissionRiskLevel, PermissionUpdate } from '@/types/permission';
import { CodeBlock } from './code-block';
import { PermissionDialog } from '@/components/dialog/permission-dialog';

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
    risk_level?: PermissionRiskLevel;
    risk_label?: string;
    summary?: string;
    suggestions?: PermissionUpdate[];
    expires_at?: string;
    onAllow: (updatedPermissions?: PermissionUpdate[]) => void;
    onDeny: (updatedPermissions?: PermissionUpdate[]) => void;
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
    pending: 'neo-card-flat',
    running: 'neo-card shadow-[0_18px_30px_rgba(133,119,255,0.14)]',
    success: 'neo-card shadow-[0_18px_30px_rgba(102,217,143,0.12)]',
    error: 'neo-card shadow-[0_18px_30px_rgba(235,90,81,0.12)]',
    waiting_permission: 'neo-card shadow-[0_18px_30px_rgba(255,157,86,0.14)]',
  };

  return (
    <div className={cn(
      "radius-shell-md my-2 overflow-hidden transition-all duration-300",
      statusColors[finalStatus]
    )}>
      {/* ═══════════ 头部栏：工具名+路径+状态+时间 ═══════════ */}
      <div
        className={cn(
          "flex h-10 cursor-pointer select-none items-center gap-2 px-3 font-mono text-xs transition-colors",
          "hover:bg-white/20",
          isRunning && "animate-pulse"
        )}
        onClick={() => hasResult && setIsExpanded(!isExpanded)}
      >
        {/* 工具图标 */}
        <div className={cn(
          "neo-pill radius-shell-sm flex h-6 w-6 items-center justify-center",
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
              "neo-pill radius-shell-sm px-2 py-0.5 text-[10px] uppercase tracking-wider transition-all",
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
        <div className="border-t border-white/50">
          <div className="max-h-[300px] overflow-y-auto p-3 custom-scrollbar">
            {typeof toolResult.content === 'string' ? (
              <pre className="neo-inset radius-shell-sm p-4 text-xs font-mono whitespace-pre-wrap break-all text-foreground/80">
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
        <div className="flex h-8 items-center gap-2 border-t border-white/50 px-3 text-xs text-muted-foreground">
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
          <div className="max-h-[120px] overflow-y-auto border-b border-orange-500/10 px-3 py-3 custom-scrollbar">
            {permissionRequest.summary && (
              <div className="mb-2 text-[11px] text-orange-500 flex items-center gap-2">
                <span className="font-semibold uppercase tracking-wider">
                  {permissionRequest.risk_label || '待确认'}
                </span>
                <span className="truncate">{permissionRequest.summary}</span>
              </div>
            )}
            <pre className="neo-inset radius-shell-sm p-3 text-[11px] font-mono text-foreground/70 whitespace-pre-wrap break-all">
              {JSON.stringify(permissionRequest.tool_input, null, 2)}
            </pre>
          </div>

          {/* 操作栏 */}
          <div className="flex h-11 items-center gap-2 px-3">
            <span className="text-xs text-orange-500 font-medium flex items-center gap-1.5">
              <Clock className="w-3 h-3" />
              AWAITING_PERMISSION
            </span>
            <div className="flex-1" />
            <button
              onClick={() => permissionRequest.onDeny()}
              className="neo-pill radius-shell-sm px-3 py-1 text-xs font-medium transition-colors hover:text-foreground"
            >
              拒绝
            </button>
            <button
              onClick={() => permissionRequest.onAllow()}
              className="radius-shell-sm bg-primary px-3 py-1 text-xs font-medium text-primary-foreground shadow-[0_14px_24px_rgba(133,119,255,0.18)] transition-colors hover:bg-primary/90"
            >
              允许执行
            </button>
            <button
              onClick={() => setShowDetailModal(true)}
              className="neo-pill radius-shell-sm px-3 py-1 text-xs font-medium transition-colors hover:text-foreground"
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
          riskLevel={permissionRequest.risk_level}
          riskLabel={permissionRequest.risk_label}
          summary={permissionRequest.summary}
          suggestions={permissionRequest.suggestions}
          expiresAt={permissionRequest.expires_at}
          onAllow={(updatedPermissions) => {
            setShowDetailModal(false);
            permissionRequest.onAllow(updatedPermissions);
          }}
          onDeny={(updatedPermissions) => {
            setShowDetailModal(false);
            permissionRequest.onDeny(updatedPermissions);
          }}
          onClose={() => setShowDetailModal(false)}
        />
      )}
    </div>
  );
}
