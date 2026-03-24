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
import { PermissionDialog } from '@/shared/ui/permission-dialog';

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
    pending: 'workspace-card',
    running: 'workspace-card shadow-[0_18px_30px_rgba(133,119,255,0.12)]',
    success: 'workspace-card shadow-[0_18px_30px_rgba(102,217,143,0.10)]',
    error: 'workspace-card shadow-[0_18px_30px_rgba(235,90,81,0.10)]',
    waiting_permission: 'workspace-card shadow-[0_18px_30px_rgba(255,157,86,0.12)]',
  };

  return (
    <div className={cn(
      "radius-shell-md my-2 overflow-hidden transition-all duration-300",
      statusColors[finalStatus]
    )}>
      {/* ═══════════ 头部栏：工具名+路径+状态+时间 ═══════════ */}
      <div
        className={cn(
          "flex min-w-0 flex-wrap cursor-pointer select-none items-center gap-x-2 gap-y-1 px-3 py-2 font-mono text-xs transition-colors sm:h-10 sm:flex-nowrap",
          "hover:bg-white/18",
          isRunning && "animate-pulse"
        )}
        onClick={() => hasResult && setIsExpanded(!isExpanded)}
      >
        {/* 工具图标 */}
        <div className={cn(
          "workspace-chip radius-shell-sm flex h-6 w-6 items-center justify-center",
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
          "shrink-0",
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
          <span className="order-2 min-w-0 text-muted-foreground break-all sm:order-none w-auto sm:flex-1 sm:truncate sm:break-normal sm:max-w-[240px] xl:max-w-[300px]">
            {pathDisplay}
          </span>
        )}

        {/* 弹性空间 */}
        <div className="hidden flex-1 sm:block" />

        {/* 结果摘要（折叠时） */}
        {/*{hasResult && !isExpanded && (*/}
        {/*  <span className="hidden max-w-[100px] truncate text-muted-foreground/60 lg:block">*/}
        {/*    {getResultSummary(toolResult.content)}*/}
        {/*  </span>*/}
        {/*)}*/}

        {/* 复制按钮（有结果时） */}
        {hasResult && (
          <button
            onClick={handleCopyResult}
            className={cn(
              "ml-auto sm:ml-0",
              "workspace-chip radius-shell-sm px-2 py-0.5 text-[10px] uppercase tracking-wider transition-all",
              copied
                ? "text-green-500 bg-green-500/10"
                : "text-slate-700/58 hover:text-slate-950 hover:bg-white/18"
            )}
          >
            {copied ? '✓' : '复制'}
          </button>
        )}

        {/* 时间 */}
        {durationText && (
          <>
            <span className="hidden text-muted-foreground/30 sm:inline">│</span>
            <span className="hidden text-muted-foreground/60 tabular-nums sm:inline">{durationText}</span>
          </>
        )}

        {/* 展开指示器 */}
        {hasResult && (
          <div className="shrink-0 text-muted-foreground/40">
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
        <div className="border-t workspace-divider">
          <div className="max-h-[300px] overflow-y-auto p-3 custom-scrollbar">
            {typeof toolResult.content === 'string' ? (
              <pre className="workspace-card radius-shell-sm p-4 text-xs font-mono whitespace-pre-wrap break-all text-slate-900/80">
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
        <div className="flex h-8 items-center gap-2 border-t workspace-divider px-3 text-xs text-slate-700/56">
          <div className="flex gap-1">
            <div className="w-1.5 h-1.5 bg-primary rounded-full animate-[pulse_1s_ease-in-out_infinite]" />
            <div className="w-1.5 h-1.5 bg-primary rounded-full animate-[pulse_1s_ease-in-out_0.2s_infinite]" />
            <div className="w-1.5 h-1.5 bg-primary rounded-full animate-[pulse_1s_ease-in-out_0.4s_infinite]" />
          </div>
          <span className="font-mono text-[10px] uppercase tracking-wider">处理中…</span>
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
            <pre className="workspace-card radius-shell-sm p-3 text-[11px] font-mono whitespace-pre-wrap break-all text-slate-900/74">
              {JSON.stringify(permissionRequest.tool_input, null, 2)}
            </pre>
          </div>

          {/* 操作栏 */}
          <div className="flex flex-wrap items-center gap-2 px-3 py-3 sm:h-11 sm:flex-nowrap sm:py-0">
            <span className="flex items-center gap-1.5 text-xs font-medium text-orange-500">
              <Clock className="w-3 h-3" />
              等待确认
            </span>
            <div className="hidden flex-1 sm:block" />
            <button
              onClick={() => permissionRequest.onDeny()}
              className="workspace-chip radius-shell-sm px-3 py-1 text-xs font-medium transition-colors hover:text-slate-950"
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
              className="workspace-chip radius-shell-sm px-3 py-1 text-xs font-medium transition-colors hover:text-slate-950"
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
