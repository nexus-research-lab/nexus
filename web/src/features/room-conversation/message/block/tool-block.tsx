/**
 * Tool Execution Block Component - 革命性时间线设计
 *
 * 视觉化工具执行：进度条融入卡片、悬浮预览、内联复制
 */

"use client";

import { useState, useCallback, useMemo } from 'react';
import { CheckCircle, ChevronDown, ChevronRight, Clock, Loader, Sparkles, XCircle } from 'lucide-react';
import { cn } from '@/lib/utils';
import { CodeBlock } from './code-block';
import { PermissionDialog } from '@/shared/ui/permission-dialog';
import { ToolResultContent, ToolUseContent } from '@/types/message';
import { PermissionRiskLevel, PermissionUpdate } from '@/types/permission';

interface ToolPermissionRequest {
  request_id: string;
  tool_input: Record<string, any>;
  risk_level?: PermissionRiskLevel;
  risk_label?: string;
  summary?: string;
  suggestions?: PermissionUpdate[];
  expires_at?: string;
  on_allow: (updated_permissions?: PermissionUpdate[]) => void;
  on_deny: (updated_permissions?: PermissionUpdate[]) => void;
}

interface ToolBlockProps {
  tool_use: ToolUseContent;
  tool_result?: ToolResultContent;
  status?: "pending" | "running" | "success" | "error" | "waiting_permission";
  start_time?: number;
  end_time?: number;
  permission_request?: ToolPermissionRequest;
}

// ==================== 辅助函数 ====================

const TOOL_TITLE_MAP: Record<string, string> = {
  Bash: '执行动作',
  Read: '读取内容',
  Write: '写入内容',
  Edit: '修改内容',
  MultiEdit: '批量修改',
  Grep: '查找内容',
  Glob: '浏览文件',
  LS: '查看目录',
  TodoWrite: '更新计划',
  AskUserQuestion: '等待你的确认',
};

const getToolTitle = (tool_name: string): string => {
  return TOOL_TITLE_MAP[tool_name] ?? '执行动作';
};

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
  tool_use,
  tool_result,
  status = 'success',
  start_time,
  end_time,
  permission_request,
}: ToolBlockProps) {
  const [isExpanded, setIsExpanded] = useState(false);
  const [showDetailModal, setShowDetailModal] = useState(false);
  const [copied, setCopied] = useState(false);

  // 复制工具执行结果
  const handleCopyResult = useCallback(async (e: React.MouseEvent) => {
    e.stopPropagation();
    if (!tool_result) return;
    const contentToCopy = typeof tool_result.content === 'string'
      ? tool_result.content
      : JSON.stringify(tool_result.content, null, 2);
    try {
      await navigator.clipboard.writeText(contentToCopy);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (error) {
      console.error('Failed to copy:', error);
    }
  }, [tool_result]);

  // 计算执行时间
  const duration = useMemo(() => {
    if (end_time && start_time) return end_time - start_time;
    if (start_time) return Date.now() - start_time;
    return 0;
  }, [end_time, start_time]);

  // 格式化时间
  const durationText = useMemo(() => {
    if (duration === 0) return '';
    return duration >= 1000 ? `${(duration / 1000).toFixed(1)}s` : `${duration}ms`;
  }, [duration]);

  // 路径显示
  const pathDisplay = useMemo(() => getPathDisplay(tool_use.input), [tool_use.input]);
  const toolTitle = useMemo(() => getToolTitle(tool_use.name), [tool_use.name]);
  const resultSummary = useMemo(() => {
    if (!tool_result) return null;
    return getResultSummary(tool_result.content);
  }, [tool_result]);

  // 最终状态
  const finalStatus = tool_result?.is_error ? 'error' : status;
  const hasResult = !!tool_result;
  const isRunning = finalStatus === 'running';
  const isSuccess = finalStatus === 'success';
  const isError = finalStatus === 'error';
  const isWaiting = finalStatus === 'waiting_permission';

  // 状态配色
  const statusColors = {
    pending: 'border-white/24 bg-white/12',
    running: 'border-white/28 bg-white/14 shadow-[0_18px_30px_rgba(133,119,255,0.08)]',
    success: 'border-white/28 bg-white/14 shadow-[0_18px_30px_rgba(102,217,143,0.08)]',
    error: 'border-white/28 bg-white/14 shadow-[0_18px_30px_rgba(235,90,81,0.08)]',
    waiting_permission: 'border-white/28 bg-white/14 shadow-[0_18px_30px_rgba(255,157,86,0.1)]',
  };

  return (
    <div className={cn(
      "my-2 overflow-hidden rounded-[20px] border transition-all duration-300",
      statusColors[finalStatus]
    )}>
      {/* ═══════════ 头部栏：工具名+路径+状态+时间 ═══════════ */}
      <div
        className={cn(
          "flex min-w-0 flex-wrap cursor-pointer select-none items-center gap-x-2 gap-y-1 px-3 py-2 text-xs transition-colors sm:min-h-11 sm:flex-nowrap",
          "hover:bg-white/12",
          isRunning && "animate-pulse"
        )}
        onClick={() => hasResult && setIsExpanded(!isExpanded)}
      >
        {/* 工具图标 */}
        <div className={cn(
          "flex h-7 w-7 items-center justify-center rounded-full border border-white/28 bg-white/24",
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
            <Sparkles className="w-3.5 h-3.5" />
          )}
        </div>

        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 items-center gap-2">
            <span className={cn(
              "shrink-0 font-medium uppercase tracking-[0.14em]",
              isSuccess && "text-green-600",
              isError && "text-red-500",
              isRunning && "text-primary",
              isWaiting && "text-orange-500"
            )}>
              {toolTitle}
            </span>
            {durationText ? (
              <span className="shrink-0 text-[11px] text-slate-700/46">{durationText}</span>
            ) : null}
          </div>
          <div className="mt-0.5 min-w-0 text-[12px] text-slate-700/60">
            {hasResult && !isExpanded && resultSummary ? (
              <span className="block truncate">{resultSummary}</span>
            ) : pathDisplay ? (
              <span className="block truncate">{pathDisplay}</span>
            ) : (
              <span>{isWaiting ? '等你确认后继续' : '查看执行详情'}</span>
            )}
          </div>
        </div>

        <div className="hidden flex-1 sm:block" />

        {/* 复制按钮（有结果时） */}
        {hasResult && (
          <button
            onClick={handleCopyResult}
            className={cn(
              "ml-auto sm:ml-0",
              "rounded-full border border-white/28 bg-white/18 px-2 py-0.5 text-[10px] uppercase tracking-wider transition-all",
              copied
                ? "text-green-500 bg-green-500/10"
                : "text-slate-700/58 hover:text-slate-950 hover:bg-white/18"
            )}
          >
            {copied ? '✓' : '复制'}
          </button>
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
        <div className="border-t border-white/18">
          <div className="max-h-[300px] overflow-y-auto p-3 custom-scrollbar">
            {pathDisplay ? (
              <div className="mb-3 rounded-[14px] border border-white/24 bg-white/18 px-3 py-2 text-[11px] text-slate-700/62">
                <div className="mb-1 font-semibold uppercase tracking-[0.12em] text-slate-700/48">
                  执行详情
                </div>
                <div className="break-all">{pathDisplay}</div>
              </div>
            ) : null}
            {typeof tool_result.content === 'string' ? (
              <pre className="rounded-[16px] border border-white/24 bg-white/22 p-4 text-xs whitespace-pre-wrap break-all text-slate-900/80">
                {tool_result.content}
              </pre>
            ) : (
              <CodeBlock language="json" value={JSON.stringify(tool_result.content, null, 2)} />
            )}
          </div>
        </div>
      )}

      {/* ═══════════ 运行中指示 ═══════════ */}
      {!hasResult && isRunning && (
        <div className="flex h-8 items-center gap-2 border-t border-white/18 px-3 text-xs text-slate-700/56">
          <div className="flex gap-1">
            <div className="w-1.5 h-1.5 bg-primary rounded-full animate-[pulse_1s_ease-in-out_infinite]" />
            <div className="w-1.5 h-1.5 bg-primary rounded-full animate-[pulse_1s_ease-in-out_0.2s_infinite]" />
            <div className="w-1.5 h-1.5 bg-primary rounded-full animate-[pulse_1s_ease-in-out_0.4s_infinite]" />
          </div>
          <span className="font-mono text-[10px] uppercase tracking-wider">处理中…</span>
        </div>
      )}

      {/* ═══════════ 权限确认 ═══════════ */}
      {permission_request && isWaiting && (
        <div className="border-t border-orange-500/20 bg-orange-500/5">
          {/* 参数预览 */}
          <div className="max-h-[120px] overflow-y-auto border-b border-orange-500/10 px-3 py-3 custom-scrollbar">
            {permission_request.summary && (
              <div className="mb-2 flex items-center gap-2 text-[11px] text-orange-500">
                <span className="font-semibold uppercase tracking-wider">
                  {permission_request.risk_label || '需要确认'}
                </span>
                <span className="truncate">{permission_request.summary}</span>
              </div>
            )}
            <pre className="rounded-[16px] border border-white/24 bg-white/18 p-3 text-[11px] whitespace-pre-wrap break-all text-slate-900/74">
              {JSON.stringify(permission_request.tool_input, null, 2)}
            </pre>
          </div>

          {/* 操作栏 */}
          <div className="flex flex-wrap items-center gap-2 px-3 py-3 sm:h-11 sm:flex-nowrap sm:py-0">
            <span className="flex items-center gap-1.5 text-xs font-medium text-orange-500">
              <Clock className="w-3 h-3" />
              等你确认后继续
            </span>
            <div className="hidden flex-1 sm:block" />
            <button
              onClick={() => permission_request.on_deny()}
              className="workspace-chip radius-shell-sm px-3 py-1 text-xs font-medium transition-colors hover:text-slate-950"
            >
              拒绝
            </button>
            <button
              onClick={() => permission_request.on_allow()}
              className="radius-shell-sm bg-primary px-3 py-1 text-xs font-medium text-primary-foreground shadow-[0_14px_24px_rgba(133,119,255,0.18)] transition-colors hover:bg-primary/90"
            >
              允许执行
            </button>
            <button
              onClick={() => setShowDetailModal(true)}
              className="workspace-chip radius-shell-sm px-3 py-1 text-xs font-medium transition-colors hover:text-slate-950"
            >
              查看执行详情
            </button>
          </div>
        </div>
      )}

      {/* 详情弹窗 */}
      {permission_request && showDetailModal && (
        <PermissionDialog
          is_open={showDetailModal}
          tool_name={tool_use.name}
          tool_input={tool_use.input}
          risk_level={permission_request.risk_level}
          risk_label={permission_request.risk_label}
          summary={permission_request.summary}
          suggestions={permission_request.suggestions}
          expires_at={permission_request.expires_at}
          on_allow={(updated_permissions) => {
            setShowDetailModal(false);
            permission_request.on_allow(updated_permissions);
          }}
          on_deny={(updated_permissions) => {
            setShowDetailModal(false);
            permission_request.on_deny(updated_permissions);
          }}
          on_close={() => setShowDetailModal(false)}
        />
      )}
    </div>
  );
}
