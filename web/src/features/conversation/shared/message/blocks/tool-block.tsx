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
import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { cn, formatTokens } from '@/lib/utils';
import { getUiChoiceClassName } from "@/shared/ui/choice-styles";
import { CodeBlock } from './code-block';
import { ImageBlock } from "./image-block";
import { type TaskProgressContent, type ToolResultContent, type ToolUseContent } from '@/types/conversation/message';
import { type PermissionRiskLevel, type PermissionUpdate } from '@/types/conversation/permission';
import {
  FIELD_LABEL_MAP,
  TOOL_LABEL_STYLES,
  TOOL_TONE_STYLES,
  formatPermissionValue,
  getInputSummary,
  getPrimaryInputDetail,
  getReadableSuggestions,
  getResultSummary,
  getToolTitle,
  isImageContent,
} from "./tool-block-model";

interface ToolPermissionRequest {
  request_id: string;
  tool_input: Record<string, any>;
  risk_level?: PermissionRiskLevel;
  risk_label?: string;
  summary?: string;
  suggestions?: PermissionUpdate[];
  expires_at?: string;
  on_allow: (updatedPermissions?: PermissionUpdate[]) => void;
  on_deny: (updatedPermissions?: PermissionUpdate[]) => void;
}

interface ToolBlockProps {
  toolUse: ToolUseContent;
  toolResult?: ToolResultContent;
  /** 子 Agent 运行中的实时进度（按 toolUseId 折叠进来），仅运行态展示。 */
  liveProgress?: TaskProgressContent | null;
  status?: "pending" | "running" | "success" | "error" | "waiting_permission";
  startTime?: number;
  endTime?: number;
  permissionRequest?: ToolPermissionRequest;
  interactionDisabled?: boolean;
  interactionDisabledReason?: string;
  onOpenWorkspaceFile?: (path: string) => void;
  workspaceAgentId?: string | null;
}

// ==================== 辅助函数 ====================

const getPermissionChoiceClassName = (selected: boolean) =>
  getUiChoiceClassName({ active: selected, size: "xs", variant: "surface" });

const TOOL_DETAIL_SCROLL_CLASS_NAME =
  "min-w-0 max-h-[18rem] overflow-auto overscroll-contain custom-scrollbar";

// ==================== 主组件 ====================

export function ToolBlock({
  toolUse: toolUse,
  toolResult: toolResult,
  liveProgress: liveProgress,
  status = 'success',
  startTime: startTime,
  endTime: endTime,
  permissionRequest: permissionRequest,
  interactionDisabled: interactionDisabled = false,
  interactionDisabledReason: interactionDisabledReason,
  onOpenWorkspaceFile: onOpenWorkspaceFile,
  workspaceAgentId: workspaceAgentId,
}: ToolBlockProps) {
  const {
    isOpen: isExpanded,
    toggle: toggleExpanded,
    anchorRef: toolAnchorRef,
  } = useScrollAnchoredState(false);
  const [selectedSuggestionIndex, setSelectedSuggestionIndex] = useResettableState<number>(
    -1,
    permissionRequest?.request_id ?? null,
  );
  const { copied, copy } = useCopyToClipboard();

  // 复制工具执行结果
  const handleCopyResult = useCallback(async (e: React.MouseEvent) => {
    e.stopPropagation();
    if (!toolResult) return;
    const contentToCopy = typeof toolResult.content === 'string'
      ? toolResult.content
      : JSON.stringify(toolResult.content, null, 2);
    await copy(contentToCopy);
  }, [copy, toolResult]);

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
  const inputSummary = useMemo(() => getInputSummary(toolUse.input), [toolUse.input]);
  const toolTitle = useMemo(() => getToolTitle(toolUse.name), [toolUse.name]);
  const primaryInputDetail = useMemo(
    () => getPrimaryInputDetail(permissionRequest?.tool_input || toolUse.input),
    [permissionRequest?.tool_input, toolUse.input],
  );
  const readableSuggestions = useMemo(
    () => getReadableSuggestions(permissionRequest?.suggestions || []),
    [permissionRequest?.suggestions],
  );
  const readablePermissionFields = useMemo(() => {
    if (!permissionRequest?.tool_input) return [];

    return Object.entries(permissionRequest.tool_input)
      .filter(([key]) => key !== primaryInputDetail?.key)
      .map(([key, value]) => ({
        key,
        label: FIELD_LABEL_MAP[key] || key,
        value: formatPermissionValue(value),
      }));
  }, [permissionRequest?.tool_input, primaryInputDetail?.key]);
  const resultSummary = useMemo(() => {
    if (!toolResult) return null;
    return getResultSummary(toolResult.content);
  }, [toolResult]);
  const expandedInputDetail = useMemo(
    () => getPrimaryInputDetail(toolUse.input),
    [toolUse.input],
  );
  const permissionFieldSummary = useMemo(() => {
    if (readablePermissionFields.length === 0) return null;
    return readablePermissionFields.map((field) => `${field.label}：${field.value}`).join(' · ');
  }, [readablePermissionFields]);

  // 子 Agent 实时进度行（仅运行态）：当前子工具 · token 数
  const liveStatusText = useMemo(() => {
    if (!liveProgress) return null;
    const totalTokens = liveProgress.usage?.total_tokens;
    const parts = [
      liveProgress.last_tool_name ? `当前 ${liveProgress.last_tool_name}` : null,
      typeof totalTokens === "number" && totalTokens > 0 ? formatTokens(totalTokens) : null,
    ].filter(Boolean);
    return parts.length ? parts.join(" · ") : null;
  }, [liveProgress]);

  // 最终状态
  const finalStatus = toolResult?.is_error ? 'error' : status;
  const hasResult = !!toolResult;
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
  const waitingConfirmationText = permissionRequest?.expires_at
    ? `${new Date(permissionRequest.expires_at).toLocaleTimeString()} 前确认`
    : '确认后继续执行';
  const waitingActionHint = interactionDisabled
    ? interactionDisabledReason || '当前暂不可操作'
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
        onKeyDown={hasResult ? (event) => {
          if (event.key === "Enter" || event.key === " ") {
            event.preventDefault();
            toggleExpanded();
          }
        } : undefined}
        role={hasResult ? "button" : undefined}
        tabIndex={hasResult ? 0 : undefined}
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
          {isRunning && liveStatusText ? (
            <div className="mt-0.5 truncate text-[11px] text-(--text-soft)">
              {liveStatusText}
            </div>
          ) : null}
        </div>

        <div className="ml-auto flex shrink-0 flex-wrap items-center justify-end gap-1.5">
          {isWaiting && permissionRequest ? (
            <>
              <button
                type="button"
                onClick={(event) => {
                  event.stopPropagation();
                  permissionRequest.on_deny();
                }}
                disabled={interactionDisabled}
                title={interactionDisabled ? interactionDisabledReason : undefined}
                className={cn(
                  "rounded-[7px] border border-(--divider-subtle-color) px-2 py-1 text-xs font-medium text-(--text-muted) transition-colors",
                  interactionDisabled
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
                  const selectedUpdate = selectedSuggestionIndex >= 0 && permissionRequest.suggestions
                    ? [permissionRequest.suggestions[selectedSuggestionIndex]]
                    : undefined;
                  permissionRequest.on_allow(selectedUpdate);
                }}
                disabled={interactionDisabled}
                title={interactionDisabled ? interactionDisabledReason : undefined}
                className={cn(
                  "rounded-[7px] border px-2 py-1 text-xs font-medium transition-colors",
                  interactionDisabled
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
            {typeof toolResult.content === 'string' ? (
              <pre className="message-cjk-font px-0 py-0 text-xs whitespace-pre-wrap break-all text-(--text-strong)">
                {toolResult.content}
              </pre>
            ) : Array.isArray(toolResult.content) && toolResult.content.some(isImageContent) ? (
              <div className="space-y-2">
                {toolResult.content.map((item) => (
                  isImageContent(item) ? (
                    <ImageBlock
                      key={getToolResultContentKey(item)}
                      block={item}
                      onOpenWorkspaceFile={onOpenWorkspaceFile}
                      workspaceAgentId={workspaceAgentId}
                    />
                  ) : (
                    <CodeBlock
                      key={getToolResultContentKey(item)}
                      language="json"
                      value={JSON.stringify(item, null, 2)}
                    />
                  )
                ))}
              </div>
            ) : (
              <CodeBlock language="json" value={JSON.stringify(toolResult.content, null, 2)} />
            )}
          </div>
        </div>
      )}

      {permissionRequest && isWaiting && (
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
                  className={getPermissionChoiceClassName(selectedSuggestionIndex === -1)}
                >
                  <input
                    type="radio"
                    name={`permission-suggestion-${permissionRequest.request_id}`}
                    checked={selectedSuggestionIndex === -1}
                    disabled={interactionDisabled}
                    onChange={() => setSelectedSuggestionIndex(-1)}
                    className="sr-only"
                  />
                  <span>仅这次</span>
                </label>
                {readableSuggestions.map((suggestion) => (
                  <label
                    key={suggestion.index}
                    className={getPermissionChoiceClassName(selectedSuggestionIndex === suggestion.index)}
                  >
                    <input
                      type="radio"
                      name={`permission-suggestion-${permissionRequest.request_id}`}
                      checked={selectedSuggestionIndex === suggestion.index}
                      disabled={interactionDisabled}
                      onChange={() => setSelectedSuggestionIndex(suggestion.index)}
                      className="sr-only"
                    />
                    <span>{suggestion.label}</span>
                  </label>
                ))}
              </div>
            </div>
          ) : null}
          {interactionDisabled && interactionDisabledReason ? (
            <div className="text-[11px] text-(--text-soft)">
              {interactionDisabledReason}
            </div>
          ) : null}
        </div>
      )}
    </div>
  );
}

function getToolResultContentKey(item: unknown): string {
  return `tool-result-${JSON.stringify(item)}`;
}
