/**
 * Tool Execution Block Component - 革命性时间线设计
 *
 * 视觉化工具执行：进度条融入卡片、悬浮预览、内联复制
 */

"use client";

import { useState, useCallback, useEffect, useMemo } from 'react';
import { CheckCircle, ChevronDown, ChevronRight, Clock, Loader, Sparkles, XCircle } from 'lucide-react';
import { cn } from '@/lib/utils';
import { CodeBlock } from './code-block';
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

const FIELD_LABEL_MAP: Record<string, string> = {
  query: '搜索内容',
  url: '网址',
  command: '命令',
  path: '路径',
  file_path: '文件路径',
  pattern: '匹配内容',
  prompt: '提示词',
  description: '说明',
  task: '任务',
  mode: '模式',
  directories: '目录',
  answers: '回答',
};

const PRIMARY_INPUT_KEYS = [
  'command',
  'query',
  'url',
  'path',
  'file_path',
  'pattern',
  'description',
  'prompt',
  'task',
] as const;

const formatPermissionValue = (value: unknown): string => {
  if (value == null || value === '') return '空';
  if (typeof value === 'string') return value;
  if (typeof value === 'number' || typeof value === 'boolean') return String(value);
  if (Array.isArray(value)) {
    return value.map((item) => formatPermissionValue(item)).join('、');
  }
  if (typeof value === 'object') {
    return Object.entries(value as Record<string, unknown>)
      .map(([key, nestedValue]) => `${FIELD_LABEL_MAP[key] || key}：${formatPermissionValue(nestedValue)}`)
      .join('；');
  }
  return String(value);
};

const getReadableSuggestions = (suggestions: PermissionUpdate[] = []) => {
  const destinationMap: Record<string, string> = {
    session: '仅本会话',
    projectSettings: '项目设置',
    userSettings: '用户设置',
    localSettings: '本地设置',
  };
  const behaviorMap: Record<string, string> = {
    allow: '允许',
    deny: '拒绝',
    ask: '继续询问',
  };

  return suggestions.map((suggestion, index) => {
    const destination = suggestion.destination
      ? destinationMap[suggestion.destination] || suggestion.destination
      : '当前会话';
    const behavior = suggestion.behavior
      ? behaviorMap[suggestion.behavior] || suggestion.behavior
      : '更新规则';

    return {
      index,
      label: behavior === '允许'
        ? `写入${destination}`
        : `${behavior}并写入${destination}`,
    };
  });
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

/** 获取工具输入的完整展示文本 */
const getPrimaryInputDetail = (input: any): { key: string; value: string } | null => {
  if (!input) return null;
  for (const key of PRIMARY_INPUT_KEYS) {
    const value = input[key];
    if (typeof value === 'string' && value) {
      return { key, value };
    }
  }
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
  const [selectedSuggestionIndex, setSelectedSuggestionIndex] = useState<number>(-1);
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
  const primaryInputDetail = useMemo(
    () => getPrimaryInputDetail(permission_request?.tool_input || tool_use.input),
    [permission_request?.tool_input, tool_use.input],
  );
  const readableSuggestions = useMemo(
    () => getReadableSuggestions(permission_request?.suggestions || []),
    [permission_request?.suggestions],
  );
  const readablePermissionFields = useMemo(() => {
    if (!permission_request?.tool_input) return [];

    return Object.entries(permission_request.tool_input)
      .filter(([key]) => key !== primaryInputDetail?.key)
      .map(([key, value]) => ({
        key,
        label: FIELD_LABEL_MAP[key] || key,
        value: formatPermissionValue(value),
      }));
  }, [permission_request?.tool_input, primaryInputDetail?.key]);
  const resultSummary = useMemo(() => {
    if (!tool_result) return null;
    return getResultSummary(tool_result.content);
  }, [tool_result]);
  const permissionFieldSummary = useMemo(() => {
    if (readablePermissionFields.length === 0) return null;
    return readablePermissionFields.map((field) => `${field.label}：${field.value}`).join(' · ');
  }, [readablePermissionFields]);

  // 最终状态
  const finalStatus = tool_result?.is_error ? 'error' : status;
  const hasResult = !!tool_result;
  const isRunning = finalStatus === 'running';
  const isSuccess = finalStatus === 'success';
  const isError = finalStatus === 'error';
  const isWaiting = finalStatus === 'waiting_permission';
  const waitingConfirmationText = permission_request?.expires_at
    ? `${new Date(permission_request.expires_at).toLocaleTimeString()} 前确认`
    : '确认后继续执行';

  useEffect(() => {
    setSelectedSuggestionIndex(-1);
  }, [permission_request?.request_id]);

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
            {isWaiting ? (
              <span className="shrink-0 text-[11px] text-slate-400">{waitingConfirmationText}</span>
            ) : durationText ? (
              <span className="shrink-0 text-[11px] text-slate-400">{durationText}</span>
            ) : null}
          </div>
          <div className="mt-0.5 min-w-0 text-[12px] text-slate-500">
            {isWaiting && permissionFieldSummary ? (
              <span className="block truncate">{permissionFieldSummary}</span>
            ) : hasResult && !isExpanded && resultSummary ? (
              <span className="block truncate">{resultSummary}</span>
            ) : inputSummary ? (
              <span className="block truncate">{inputSummary}</span>
            ) : (
              <span>{isWaiting ? '等待确认' : '处理中…'}</span>
            )}
          </div>
        </div>

        <div className="hidden flex-1 sm:block" />

        {isWaiting && permission_request ? (
          <div className="ml-auto flex shrink-0 flex-wrap items-center justify-end gap-2">
            <button
              onClick={() => permission_request.on_deny()}
              className="modal-btn-secondary rounded-lg px-3 py-1.5 text-xs font-medium text-slate-500 transition-colors hover:text-slate-800"
            >
              拒绝
            </button>
            <button
              onClick={() => {
                const selectedUpdate = selectedSuggestionIndex >= 0 && permission_request.suggestions
                  ? [permission_request.suggestions[selectedSuggestionIndex]]
                  : undefined;
                permission_request.on_allow(selectedUpdate);
              }}
              className="rounded-lg bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground shadow-[0_8px_20px_rgba(133,119,255,0.2)] transition-colors hover:bg-primary/90"
            >
              允许
            </button>
          </div>
        ) : null}

        {/* 复制按钮（有结果时） */}
        {hasResult && !isWaiting && (
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
        <div className="ml-7 mt-2 space-y-2">
          {primaryInputDetail ? (
            <div className="text-[13px] leading-7 text-slate-800">
              <pre className="whitespace-pre-wrap break-all">
                {primaryInputDetail.value}
              </pre>
            </div>
          ) : null}

          {readableSuggestions.length > 0 ? (
            <div className="flex flex-wrap items-center gap-2">
              <label
                className={cn(
                  "workspace-chip radius-shell-sm flex items-center gap-2 px-3 py-1.5 text-xs font-medium transition-all duration-200",
                  selectedSuggestionIndex === -1
                    ? "text-primary ring-1 ring-primary/25 shadow-[0_8px_18px_rgba(15,23,42,0.05)]"
                    : "text-slate-500 hover:text-slate-800",
                )}
              >
                <input
                  type="radio"
                  name={`permission-suggestion-${permission_request.request_id}`}
                  checked={selectedSuggestionIndex === -1}
                  onChange={() => setSelectedSuggestionIndex(-1)}
                  className="sr-only"
                />
                <span>仅这次</span>
              </label>
              {readableSuggestions.map((suggestion) => (
                <label
                  key={suggestion.index}
                  className={cn(
                    "workspace-chip radius-shell-sm flex items-center gap-2 px-3 py-1.5 text-xs font-medium transition-all duration-200",
                    selectedSuggestionIndex === suggestion.index
                      ? "text-primary ring-1 ring-primary/25 shadow-[0_8px_18px_rgba(15,23,42,0.05)]"
                      : "text-slate-500 hover:text-slate-800",
                  )}
                >
                  <input
                    type="radio"
                    name={`permission-suggestion-${permission_request.request_id}`}
                    checked={selectedSuggestionIndex === suggestion.index}
                    onChange={() => setSelectedSuggestionIndex(suggestion.index)}
                    className="sr-only"
                  />
                  <span>{suggestion.label}</span>
                </label>
              ))}
            </div>
          ) : null}
        </div>
      )}
    </div>
  );
}
