/**
 * Tool Execution Block Component - 革命性时间线设计
 *
 * 视觉化工具执行：进度条融入卡片、悬浮预览、内联复制
 */

"use client";

import { useState, useCallback, useEffect, useMemo } from 'react';
import {
  Check,
  CheckCircle,
  ChevronDown,
  ChevronRight,
  Clock,
  Copy,
  Loader,
  Sparkles,
  XCircle,
} from 'lucide-react';
import { useScrollAnchoredState } from "@/hooks/conversation/use-scroll-anchored-state";
import { useCopyToClipboard } from "@/hooks/ui/use-copy-to-clipboard";
import { cn } from '@/lib/utils';
import { get_ui_choice_class_name } from "@/shared/ui/choice-styles";
import { CodeBlock } from './code-block';
import { ImageBlock } from "./image-block";
import { type ToolResultContent, type ToolUseContent } from '@/types/conversation/message';
import { type PermissionRiskLevel, type PermissionUpdate } from '@/types/conversation/permission';
import {
  FIELD_LABEL_MAP,
  TOOL_LABEL_STYLES,
  TOOL_TONE_STYLES,
  format_permission_value,
  get_input_summary,
  get_primary_input_detail,
  get_readable_suggestions,
  get_result_summary,
  get_tool_title,
  is_image_content,
} from "./tool-block-model";

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
  on_open_workspace_file?: (path: string) => void;
  workspace_agent_id?: string | null;
}

// ==================== 辅助函数 ====================

const get_permission_choice_class_name = (selected: boolean) =>
  get_ui_choice_class_name({ active: selected, size: "xs", variant: "surface" });

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
  on_open_workspace_file,
  workspace_agent_id,
}: ToolBlockProps) {
  const {
    is_open: isExpanded,
    toggle: toggleExpanded,
    anchor_ref: toolAnchorRef,
  } = useScrollAnchoredState(false);
  const [selectedSuggestionIndex, setSelectedSuggestionIndex] = useState<number>(-1);
  const { copied, copy } = useCopyToClipboard();

  // 复制工具执行结果
  const handleCopyResult = useCallback(async (e: React.MouseEvent) => {
    e.stopPropagation();
    if (!tool_result) return;
    const contentToCopy = typeof tool_result.content === 'string'
      ? tool_result.content
      : JSON.stringify(tool_result.content, null, 2);
    await copy(contentToCopy);
  }, [copy, tool_result]);

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
  const expandedInputDetail = useMemo(
    () => get_primary_input_detail(tool_use.input),
    [tool_use.input],
  );
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
  const statusText = isWaiting
    ? '待确认'
    : isRunning
      ? '执行中'
      : isError
        ? '失败'
        : isSuccess
          ? '完成'
          : '待处理';
  const statusBadgeClassName = isSuccess
    ? "bg-[color:color-mix(in_srgb,var(--success)_10%,transparent)] text-(--success)"
    : isError
      ? "bg-[color:color-mix(in_srgb,var(--destructive)_10%,transparent)] text-(--destructive)"
      : isWaiting
        ? "bg-[color:color-mix(in_srgb,var(--warning)_12%,transparent)] text-(--warning)"
        : "bg-primary/10 text-primary";
  const waitingConfirmationText = permission_request?.expires_at
    ? `${new Date(permission_request.expires_at).toLocaleTimeString()} 前确认`
    : '确认后继续执行';
  const waitingActionHint = interaction_disabled
    ? interaction_disabled_reason || '当前暂不可操作'
    : waitingConfirmationText;
  const collapsedDetailText = useMemo(() => {
    if (isWaiting && permissionFieldSummary) {
      return permissionFieldSummary;
    }
    if (inputSummary) {
      return inputSummary;
    }
    if (hasResult) {
      return resultSummary;
    }
    return null;
  }, [hasResult, inputSummary, isWaiting, permissionFieldSummary, resultSummary]);
  const expandedDetailText = useMemo(() => {
    if (isWaiting && permissionFieldSummary) {
      return permissionFieldSummary;
    }
    return expandedInputDetail?.value.trim() || inputSummary || resultSummary || null;
  }, [expandedInputDetail, inputSummary, isWaiting, permissionFieldSummary, resultSummary]);
  const headerDetailText = isExpanded ? expandedDetailText : collapsedDetailText;

  useEffect(() => {
    setSelectedSuggestionIndex(-1);
  }, [permission_request?.request_id]);

  return (
    <div
      ref={toolAnchorRef as React.RefObject<HTMLDivElement>}
      className="message-cjk-font group/tool-block min-w-0"
    >
      <div
        className={cn(
          "grid min-w-0 grid-cols-[20px_minmax(0,1fr)_auto] items-center gap-2 rounded-[7px] px-1.5 py-1 text-xs transition-colors",
          hasResult
            ? "cursor-pointer hover:bg-(--surface-interactive-hover-background)"
            : "cursor-default",
          isRunning && "bg-primary/5",
          isWaiting && "bg-[color:color-mix(in_srgb,var(--warning)_7%,transparent)]",
        )}
        onClick={() => hasResult && toggleExpanded()}
      >
        {/* 工具图标 */}
        <div
          data-timeline-anchor
          data-timeline-anchor-mode="box"
          className={cn(
            "flex h-5 w-5 items-center justify-center rounded-full",
            TOOL_TONE_STYLES[statusTone],
          )}
        >
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
          <div className="flex min-w-0 items-center gap-1.5">
            <span className={cn("shrink-0 text-[11px] font-medium", TOOL_LABEL_STYLES[statusTone])}>
              {toolTitle}
            </span>
            <span
              className={cn(
                "shrink-0 rounded-[6px] px-1.5 py-0.5 text-[10px] font-semibold",
                statusBadgeClassName,
              )}
            >
              {statusText}
            </span>
            {isWaiting ? (
              <span className="shrink-0 text-[11px] text-(--text-soft)">{waitingActionHint}</span>
            ) : durationText ? (
              <span className="shrink-0 text-[11px] text-(--text-soft)">{durationText}</span>
            ) : null}
          </div>
          <div className="mt-0.5 min-w-0 text-[12px] text-(--text-muted)">
            {headerDetailText ? (
              <span
                className={cn(
                  "block",
                  isExpanded
                    ? "whitespace-pre-wrap break-all"
                    : "truncate",
                  "message-cjk-font",
                )}
              >
                {headerDetailText}
              </span>
            ) : (
              <span>{isWaiting ? '等待确认' : '处理中…'}</span>
            )}
          </div>
        </div>

        <div className="ml-auto flex shrink-0 flex-wrap items-center justify-end gap-1.5">
          {isWaiting && permission_request ? (
            <>
              <button
                type="button"
                onClick={(event) => {
                  event.stopPropagation();
                  permission_request.on_deny();
                }}
                disabled={interaction_disabled}
                title={interaction_disabled ? interaction_disabled_reason : undefined}
                className={cn(
                  "rounded-[7px] border border-(--divider-subtle-color) px-2 py-1 text-xs font-medium text-(--text-muted) transition-colors",
                  interaction_disabled
                    ? "cursor-not-allowed opacity-(--disabled-opacity)"
                    : "hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
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
                  "rounded-[7px] border px-2 py-1 text-xs font-medium transition-colors",
                  interaction_disabled
                    ? "cursor-not-allowed border-(--divider-subtle-color) bg-transparent text-(--text-soft)"
                    : "border-primary/24 bg-primary/8 text-primary hover:bg-primary/12",
                )}
              >
                允许
              </button>
            </>
          ) : null}

          {hasResult && !isWaiting ? (
            <button
              type="button"
              aria-label={copied ? '已复制结果' : '复制结果'}
              title={copied ? '已复制结果' : '复制结果'}
              onClick={handleCopyResult}
              className={cn(
                "inline-flex h-6 w-6 items-center justify-center rounded-[6px] transition-colors",
                copied
                  ? "bg-[color:color-mix(in_srgb,var(--success)_10%,transparent)] text-(--success)"
                  : "text-(--icon-muted) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
              )}
            >
              {copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
            </button>
          ) : null}

          {hasResult ? (
            <div className="shrink-0 text-(--icon-muted)">
              {isExpanded ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
            </div>
          ) : null}
        </div>
      </div>

      {!hasResult && isRunning ? (
        <div className="ml-7 mt-1 h-px overflow-hidden rounded-full bg-primary/15">
          <div className="h-full w-1/3 animate-pulse rounded-full bg-primary/60" />
        </div>
      ) : null}

      {hasResult && isExpanded && (
        <div className="message-cjk-font ml-7 mt-2 min-w-0">
          <div className={TOOL_DETAIL_SCROLL_CLASS_NAME}>
            {typeof tool_result.content === 'string' ? (
              <pre className="message-cjk-font px-0 py-0 text-xs whitespace-pre-wrap break-all text-(--text-strong)">
                {tool_result.content}
              </pre>
            ) : Array.isArray(tool_result.content) && tool_result.content.some(is_image_content) ? (
              <div className="space-y-2">
                {tool_result.content.map((item, index) => (
                  is_image_content(item) ? (
                    <ImageBlock
                      key={`tool-result-image-${index}`}
                      block={item}
                      on_open_workspace_file={on_open_workspace_file}
                      workspace_agent_id={workspace_agent_id}
                    />
                  ) : (
                    <CodeBlock
                      key={`tool-result-json-${index}`}
                      language="json"
                      value={JSON.stringify(item, null, 2)}
                    />
                  )
                ))}
              </div>
            ) : (
              <CodeBlock language="json" value={JSON.stringify(tool_result.content, null, 2)} />
            )}
          </div>
        </div>
      )}

      {permission_request && isWaiting && (
        <div className="message-cjk-font ml-7 mt-2 space-y-2 border-t border-(--divider-subtle-color) pt-2">
          {primaryInputDetail?.value.trim() ? (
            <div className="space-y-1 px-0 py-0 text-[12px] leading-5 text-(--text-default)">
              <div className="text-[10px] font-semibold uppercase tracking-[0.14em] text-(--text-soft)">
                {FIELD_LABEL_MAP[primaryInputDetail.key] || primaryInputDetail.key}
              </div>
              <div className={TOOL_DETAIL_SCROLL_CLASS_NAME}>
                <pre className="message-cjk-font whitespace-pre-wrap break-all text-[12px] leading-5 text-(--text-default)">
                  {primaryInputDetail.value}
                </pre>
              </div>
            </div>
          ) : null}

          {readableSuggestions.length > 0 ? (
            <div className="space-y-1">
              <div className="text-[10px] font-semibold uppercase tracking-[0.14em] text-(--text-soft)">权限范围</div>
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
