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
import { PermissionDialog } from '@/shared/ui/dialog/permission-dialog';
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
  Bash: '执行命令',
  Read: '读取内容',
  Write: '写入内容',
  Edit: '修改内容',
  MultiEdit: '批量修改',
  Grep: '查找内容',
  Glob: '浏览文件',
  LS: '查看目录',
  TodoWrite: '更新计划',
  AskUserQuestion: '等待你的确认',
  WebSearch: '网络搜索',
  WebFetch: '抓取网页',
  Skill: '调用技能',
  Task: '委派任务',
};

const getToolTitle = (tool_name: string): string => {
  return TOOL_TITLE_MAP[tool_name] ?? tool_name;
};

/** 获取工具输入的简短摘要 */
const getInputSummary = (input: any): string | null => {
  if (!input) return null;
  if (input.file_path) return input.file_path;
  if (input.path) return input.path;
  if (input.url) return input.url;
  if (input.query) return input.query;
  if (input.pattern) return input.pattern;
  if (input.description) return input.description;
  if (input.task) return input.task;
  if (input.prompt) return input.prompt;
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
  const inputSummary = useMemo(() => getInputSummary(tool_use.input), [tool_use.input]);
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

  return (
    <div className="border-l border-slate-200/90 pl-4">
      <div
        className={cn(
          "flex min-w-0 flex-wrap cursor-pointer select-none items-center gap-x-2 gap-y-1 py-1 text-xs transition-colors sm:flex-nowrap",
          isRunning && "animate-pulse"
        )}
        onClick={() => hasResult && setIsExpanded(!isExpanded)}
      >
        {/* 工具图标 */}
        <div className={cn(
          "flex h-5 w-5 items-center justify-center rounded-full",
          isSuccess && "text-green-500",
          isError && "text-red-500",
          isRunning && "text-sky-500",
          isWaiting && "text-orange-500",
          !isSuccess && !isError && !isRunning && !isWaiting && "text-slate-400",
        )}>
          {isRunning ? (
            <Loader className="h-3.5 w-3.5 animate-spin" />
          ) : isSuccess ? (
            <CheckCircle className="h-3.5 w-3.5" />
          ) : isError ? (
            <XCircle className="h-3.5 w-3.5" />
          ) : isWaiting ? (
            <Clock className="h-3.5 w-3.5 animate-pulse" />
          ) : (
            <Sparkles className="h-3.5 w-3.5" />
          )}
        </div>

        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 items-center gap-2">
            <span className={cn(
              "shrink-0 text-[11px] font-medium",
              isSuccess && "text-green-600",
              isError && "text-red-500",
              isRunning && "text-sky-500",
              isWaiting && "text-orange-500"
            )}>
              {toolTitle}
            </span>
            {durationText ? (
              <span className="shrink-0 text-[11px] text-slate-400">{durationText}</span>
            ) : null}
          </div>
          <div className="mt-0.5 min-w-0 text-[12px] text-slate-500">
            {hasResult && !isExpanded && resultSummary ? (
              <span className="block truncate">{resultSummary}</span>
            ) : inputSummary ? (
              <span className="block truncate">{inputSummary}</span>
            ) : (
              <span>{isWaiting ? '等待确认' : '处理中…'}</span>
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
              "rounded px-1.5 py-0.5 text-[10px] transition-all",
              copied
                ? "bg-green-50 text-green-500"
                : "text-slate-400 hover:bg-slate-100 hover:text-slate-700"
            )}
          >
            {copied ? '✓' : '复制'}
          </button>
        )}

        {/* 展开指示器 */}
        {hasResult && (
          <div className="shrink-0 text-slate-300">
            {isExpanded ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
          </div>
        )}
      </div>

      {isRunning && (
        <div className="ml-7 h-px bg-slate-200/80" />
      )}

      {hasResult && isExpanded && (
        <div className="ml-7 mt-2 max-h-[300px] overflow-y-auto custom-scrollbar">
            {typeof tool_result.content === 'string' ? (
              <pre className="bg-slate-100/80 p-3 text-xs whitespace-pre-wrap break-all text-slate-800">
                {tool_result.content}
              </pre>
            ) : (
              <CodeBlock language="json" value={JSON.stringify(tool_result.content, null, 2)} />
            )}
        </div>
      )}

      {!hasResult && isRunning && (
        <div className="ml-7 mt-2 text-xs text-slate-500">
          <div className="flex items-center gap-2">
            <div className="flex gap-1">
              <div className="w-1.5 h-1.5 bg-primary rounded-full animate-[pulse_1s_ease-in-out_infinite]" />
              <div className="w-1.5 h-1.5 bg-primary rounded-full animate-[pulse_1s_ease-in-out_0.2s_infinite]" />
              <div className="w-1.5 h-1.5 bg-primary rounded-full animate-[pulse_1s_ease-in-out_0.4s_infinite]" />
            </div>
            <span className="text-[11px] text-slate-400">处理中</span>
          </div>
        </div>
      )}

      {permission_request && isWaiting && (
        <div className="ml-7 mt-2 rounded-xl border border-amber-200/70 bg-amber-50/45 p-3">
          <div className="space-y-2">
            <div className="flex flex-wrap items-center gap-2 text-[11px]">
              <span className="rounded-full bg-amber-100 px-2 py-1 font-medium text-amber-700">
                {permission_request.risk_label || '需要确认'}
              </span>
              {permission_request.expires_at ? (
                <span className="text-slate-400">
                  {new Date(permission_request.expires_at).toLocaleTimeString()}
                  {' '}前确认
                </span>
              ) : null}
            </div>
            {permission_request.summary ? (
              <p className="text-[13px] leading-6 text-slate-700">
                {permission_request.summary}
              </p>
            ) : null}
          </div>

          <div className="mt-3 flex flex-wrap items-center justify-end gap-2">
            <button
              onClick={() => setShowDetailModal(true)}
              className="workspace-chip radius-shell-sm px-3 py-1 text-xs font-medium transition-colors hover:text-slate-950"
            >
              详情
            </button>
            <button
              onClick={() => permission_request.on_deny()}
              className="workspace-chip radius-shell-sm px-3 py-1 text-xs font-medium transition-colors hover:text-slate-950"
            >
              拒绝
            </button>
            <button
              onClick={() => permission_request.on_allow()}
              className="radius-shell-sm bg-slate-900 px-3 py-1 text-xs font-medium text-white transition-colors hover:bg-slate-800"
            >
              允许
            </button>
          </div>
        </div>
      )}

      {/* 详情弹窗 */}
      {permission_request && showDetailModal && (
        <PermissionDialog
          is_open={showDetailModal}
          tool_name={tool_use.name}
          tool_input={permission_request.tool_input}
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
