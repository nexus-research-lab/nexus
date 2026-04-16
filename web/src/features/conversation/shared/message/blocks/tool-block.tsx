/**
 * Tool Execution Block Component - 革命性时间线设计
 *
 * 视觉化工具执行：进度条融入卡片、悬浮预览、内联复制
 */

"use client";

import { useState, useCallback, useEffect, useMemo } from 'react';
import { CheckCircle, ChevronDown, ChevronRight, Clock, Loader, Sparkles, XCircle } from 'lucide-react';
import { useScrollAnchoredState } from "@/hooks/conversation/use-scroll-anchored-state";
import { cn } from '@/lib/utils';
import { CodeBlock } from './code-block';
import { ToolResultContent, ToolUseContent } from '@/types/conversation/message';
import { PermissionRiskLevel, PermissionUpdate } from '@/types/conversation/permission';

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
  interaction_disabled?: boolean;
  interaction_disabled_reason?: string;
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

const get_tool_title = (tool_name: string): string => {
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

const format_permission_value = (value: unknown): string => {
  if (value == null || value === '') return '空';
  if (typeof value === 'string') return value;
  if (typeof value === 'number' || typeof value === 'boolean') return String(value);
  if (Array.isArray(value)) {
    return value.map((item) => format_permission_value(item)).join('、');
  }
  if (typeof value === 'object') {
    return Object.entries(value as Record<string, unknown>)
      .map(([key, nestedValue]) => `${FIELD_LABEL_MAP[key] || key}：${format_permission_value(nestedValue)}`)
      .join('；');
  }
  return String(value);
};

const get_readable_suggestions = (suggestions: PermissionUpdate[] = []) => {
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
const get_input_summary = (input: any): string | null => {
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
const get_primary_input_detail = (input: any): { key: string; value: string } | null => {
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
const get_result_summary = (content: any): string => {
  if (typeof content === 'string') {
    return content.slice(0, 80) + (content.length > 80 ? '...' : '');
  }
  return 'JSON 数据';
};

const TOOL_TONE_STYLES: Record<string, string> = {
  default: 'text-(--icon-muted)',
  error: 'text-(--destructive)',
  running: 'text-(--primary)',
  success: 'text-(--success)',
  waiting: 'text-(--warning)',
};

const TOOL_LABEL_STYLES: Record<string, string> = {
  default: 'text-(--text-default)',
  error: 'text-(--destructive)',
  running: 'text-(--primary)',
  success: 'text-(--success)',
  waiting: 'text-(--warning)',
};

const get_permission_choice_class_name = (selected: boolean) =>
  cn(
    "rounded-full border px-2.5 py-0.5 text-[10px] font-medium transition-colors",
    selected
      ? "border-white/12 bg-primary/8 text-primary"
      : "border-white/12 bg-white/6 text-(--text-muted) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
  );

const TOOL_DETAIL_SCROLL_CLASS_NAME =
  "min-w-0 max-h-[18rem] overflow-auto overscroll-contain custom-scrollbar";

// ==================== 主组件 ====================

export function ToolBlock({
  tool_use,
  tool_result,
  status = 'success',
  start_time,
  end_time,
  permission_request,
  interaction_disabled = false,
  interaction_disabled_reason,
}: ToolBlockProps) {
  const {
    is_open: isExpanded,
    toggle: toggleExpanded,
    anchor_ref: toolAnchorRef,
  } = useScrollAnchoredState(false);
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
  const inputSummary = useMemo(() => get_input_summary(tool_use.input), [tool_use.input]);
  const toolTitle = useMemo(() => get_tool_title(tool_use.name), [tool_use.name]);
  const primaryInputDetail = useMemo(
    () => get_primary_input_detail(permission_request?.tool_input || tool_use.input),
    [permission_request?.tool_input, tool_use.input],
  );
  const readableSuggestions = useMemo(
    () => get_readable_suggestions(permission_request?.suggestions || []),
    [permission_request?.suggestions],
  );
  const readablePermissionFields = useMemo(() => {
    if (!permission_request?.tool_input) return [];

    return Object.entries(permission_request.tool_input)
      .filter(([key]) => key !== primaryInputDetail?.key)
      .map(([key, value]) => ({
        key,
        label: FIELD_LABEL_MAP[key] || key,
        value: format_permission_value(value),
      }));
  }, [permission_request?.tool_input, primaryInputDetail?.key]);
  const resultSummary = useMemo(() => {
    if (!tool_result) return null;
    return get_result_summary(tool_result.content);
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
  const statusTone = isSuccess
    ? 'success'
    : isError
      ? 'error'
      : isRunning
        ? 'running'
        : isWaiting
          ? 'waiting'
          : 'default';
  const waitingConfirmationText = permission_request?.expires_at
    ? `${new Date(permission_request.expires_at).toLocaleTimeString()} 前确认`
    : '确认后继续执行';
  const waitingActionHint = interaction_disabled
    ? interaction_disabled_reason || '当前窗口是观察视图'
    : waitingConfirmationText;

  useEffect(() => {
    setSelectedSuggestionIndex(-1);
  }, [permission_request?.request_id]);

  return (
    <div
      ref={toolAnchorRef as React.RefObject<HTMLDivElement>}
      className="border-l-2 pl-4"
      style={{ borderColor: "color-mix(in srgb, var(--foreground) 18%, transparent)" }}
    >
      <div
        className={cn(
          "flex min-w-0 flex-wrap cursor-pointer select-none items-center gap-x-2 gap-y-1 py-1 text-xs transition-colors sm:flex-nowrap",
          isRunning && "animate-pulse"
        )}
        onClick={() => hasResult && toggleExpanded()}
      >
        {/* 工具图标 */}
        <div className={cn("flex h-5 w-5 items-center justify-center rounded-full", TOOL_TONE_STYLES[statusTone])}>
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
            <span className={cn("shrink-0 text-[11px] font-medium", TOOL_LABEL_STYLES[statusTone])}>
              {toolTitle}
            </span>
            {isWaiting ? (
              <span className="shrink-0 text-[11px] text-(--text-soft)">{waitingActionHint}</span>
            ) : durationText ? (
              <span className="shrink-0 text-[11px] text-(--text-soft)">{durationText}</span>
            ) : null}
          </div>
          <div className="mt-0.5 min-w-0 text-[12px] text-(--text-muted)">
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
              type="button"
              onClick={(event) => {
                event.stopPropagation();
                permission_request.on_deny();
              }}
              disabled={interaction_disabled}
              title={interaction_disabled ? interaction_disabled_reason : undefined}
              className={cn(
                "dialog-button-secondary rounded-lg px-3 py-1.5 text-xs font-medium text-(--text-muted) transition-colors",
                interaction_disabled
                  ? "cursor-not-allowed opacity-(--disabled-opacity)"
                  : "hover:text-(--text-strong)",
              )}
            >
              拒绝
            </button>
            <button
              type="button"
              onClick={(event) => {
                event.stopPropagation();
                const selectedUpdate = selectedSuggestionIndex >= 0 && permission_request.suggestions
                  ? [permission_request.suggestions[selectedSuggestionIndex]]
                  : undefined;
                permission_request.on_allow(selectedUpdate);
              }}
              disabled={interaction_disabled}
              title={interaction_disabled ? interaction_disabled_reason : undefined}
              className={cn(
                "rounded-lg px-3 py-1.5 text-xs font-medium text-primary-foreground transition-colors",
                interaction_disabled
                  ? "cursor-not-allowed bg-(--muted)"
                  : "bg-primary hover:bg-primary/90",
              )}
            >
              允许
            </button>
          </div>
        ) : null}

        {/* 复制按钮（有结果时） */}
        {hasResult && !isWaiting ? (
          <button
            onClick={handleCopyResult}
            className={cn(
              "ml-auto sm:ml-0",
              "rounded px-1.5 py-0.5 text-[10px] transition-all",
              copied
                ? "bg-[color:color-mix(in_srgb,var(--success)_10%,transparent)] text-(--success)"
                : "text-(--icon-muted) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
            )}
          >
            {copied ? '✓' : '复制'}
          </button>
        ) : null}

        {/* 展开指示器 */}
        {hasResult && (
          <div className="shrink-0 text-(--icon-muted)">
            {isExpanded ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
          </div>
        )}
      </div>

      {isRunning && (
        <div
          className="ml-7 h-px"
          style={{ backgroundColor: "color-mix(in srgb, var(--foreground) 12%, transparent)" }}
        />
      )}

      {!hasResult && isRunning && (
        <div className="ml-7 mt-2 flex items-center gap-2 text-xs text-(--text-muted)">
          <div className="flex gap-1">
            <div className="w-1.5 h-1.5 bg-primary rounded-full animate-[pulse_1s_ease-in-out_infinite]" />
            <div className="w-1.5 h-1.5 bg-primary rounded-full animate-[pulse_1s_ease-in-out_0.2s_infinite]" />
            <div className="w-1.5 h-1.5 bg-primary rounded-full animate-[pulse_1s_ease-in-out_0.4s_infinite]" />
          </div>
          <span className="text-[11px] text-(--text-soft)">处理中</span>
        </div>
      )}

      {hasResult && isExpanded && (
        <div className="ml-7 mt-2 min-w-0">
          <div className={TOOL_DETAIL_SCROLL_CLASS_NAME}>
            {typeof tool_result.content === 'string' ? (
              <pre
                className="rounded-2xl border p-3 text-xs whitespace-pre-wrap break-all text-(--text-strong)"
                style={{
                  background: "var(--surface-panel-subtle-background)",
                  borderColor: "var(--surface-panel-subtle-border)",
                }}
              >
                {tool_result.content}
              </pre>
            ) : (
              <CodeBlock language="json" value={JSON.stringify(tool_result.content, null, 2)} />
            )}
          </div>
        </div>
      )}

      {permission_request && isWaiting && (
        <div className="ml-7 mt-1.5 space-y-1.5">
          {primaryInputDetail?.value.trim() ? (
            <div
              className="rounded-xl border px-2.5 py-1.5 text-[12px] leading-5 text-(--text-default)"
              style={{
                background: "var(--surface-panel-subtle-background)",
                borderColor: "var(--surface-panel-subtle-border)",
              }}
            >
              <div className={TOOL_DETAIL_SCROLL_CLASS_NAME}>
                <pre className="whitespace-pre-wrap break-all">
                  {primaryInputDetail.value}
                </pre>
              </div>
            </div>
          ) : null}

          {readableSuggestions.length > 0 ? (
            <div className="space-y-1">
              <div className="text-[10px] font-medium text-(--text-soft)">权限范围</div>
              <div className="flex flex-wrap items-center gap-1.5">
                <label
                  className={get_permission_choice_class_name(selectedSuggestionIndex === -1)}
                >
                  <input
                    type="radio"
                    name={`permission-suggestion-${permission_request.request_id}`}
                    checked={selectedSuggestionIndex === -1}
                    disabled={interaction_disabled}
                    onChange={() => setSelectedSuggestionIndex(-1)}
                    className="sr-only"
                  />
                  <span>仅这次</span>
                </label>
                {readableSuggestions.map((suggestion) => (
                  <label
                    key={suggestion.index}
                    className={get_permission_choice_class_name(selectedSuggestionIndex === suggestion.index)}
                  >
                    <input
                      type="radio"
                      name={`permission-suggestion-${permission_request.request_id}`}
                      checked={selectedSuggestionIndex === suggestion.index}
                      disabled={interaction_disabled}
                      onChange={() => setSelectedSuggestionIndex(suggestion.index)}
                      className="sr-only"
                    />
                    <span>{suggestion.label}</span>
                  </label>
                ))}
              </div>
            </div>
          ) : null}
          {interaction_disabled && interaction_disabled_reason ? (
            <div className="text-[11px] text-(--text-soft)">
              {interaction_disabled_reason}
            </div>
          ) : null}
        </div>
      )}
    </div>
  );
}
