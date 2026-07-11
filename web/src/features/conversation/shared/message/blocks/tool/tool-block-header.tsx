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
  type LucideIcon,
} from "lucide-react";

import { cn } from "@/shared/ui/class-name";

import type {
  ToolBlockStatus,
  ToolBlockViewModel,
  ToolStatusTone,
} from "./tool-block-types";

const TOOL_STATUS_ICON_MAP: Record<
  ToolBlockStatus,
  { className: string; icon: LucideIcon }
> = {
  pending: { className: "", icon: Sparkles },
  running: { className: "animate-spin", icon: Loader },
  success: { className: "", icon: CheckCircle },
  error: { className: "", icon: XCircle },
  waiting_permission: { className: "animate-pulse", icon: Clock },
};

const TOOL_TONE_STYLES: Record<ToolStatusTone, string> = {
  default: "text-(--icon-muted)",
  error: "text-(--destructive)",
  running: "text-(--primary)",
  success: "text-(--success)",
  waiting: "text-(--warning)",
};

const TOOL_LABEL_STYLES: Record<ToolStatusTone, string> = {
  default: "text-(--text-default)",
  error: "text-(--destructive)",
  running: "text-(--primary)",
  success: "text-(--success)",
  waiting: "text-(--warning)",
};

interface ToolBlockHeaderProps {
  copied: boolean;
  interactionDisabled: boolean;
  interactionDisabledReason?: string;
  isExpanded: boolean;
  model: ToolBlockViewModel;
  onAllow?: () => void;
  onCopyResult: () => void;
  onDeny?: () => void;
  onToggle: () => void;
}

export function ToolBlockHeader({
  copied,
  interactionDisabled,
  interactionDisabledReason,
  isExpanded,
  model,
  onAllow,
  onCopyResult,
  onDeny,
  onToggle,
}: ToolBlockHeaderProps) {
  const detailText = isExpanded
    ? model.expandedDetailText
    : model.collapsedDetailText;

  return (
    <div
      className={cn(
        "grid min-w-0 grid-cols-[20px_minmax(0,1fr)_auto] items-center gap-2 rounded-[7px] px-1.5 py-1 text-xs transition-colors",
        model.hasResult
          ? "cursor-pointer hover:bg-(--surface-interactive-hover-background)"
          : "cursor-default",
        model.isRunning && "bg-primary/5",
        model.isWaiting && "bg-[color:color-mix(in_srgb,var(--warning)_7%,transparent)]",
      )}
      onClick={model.hasResult ? onToggle : undefined}
      onKeyDown={model.hasResult ? (event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          onToggle();
        }
      } : undefined}
      role={model.hasResult ? "button" : undefined}
      tabIndex={model.hasResult ? 0 : undefined}
    >
      <div
        data-timeline-anchor
        data-timeline-anchor-mode="box"
        className={cn(
          "flex h-5 w-5 items-center justify-center rounded-full",
          TOOL_TONE_STYLES[model.statusTone],
        )}
      >
        <ToolStatusIcon model={model} />
      </div>

      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-1.5">
          <span className={cn(
            "shrink-0 text-[11px] font-medium",
            TOOL_LABEL_STYLES[model.statusTone],
          )}>
            {model.toolTitle}
          </span>
          <span className={cn(
            "shrink-0 rounded-[6px] px-1.5 py-0.5 text-[10px] font-semibold",
            model.statusBadgeClassName,
          )}>
            {model.statusText}
          </span>
          {model.isWaiting ? (
            <span className="shrink-0 text-[11px] text-(--text-soft)">
              {model.waitingActionHint}
            </span>
          ) : model.durationText ? (
            <span className="shrink-0 text-[11px] text-(--text-soft)">
              {model.durationText}
            </span>
          ) : null}
        </div>
        <div className="mt-0.5 min-w-0 text-[12px] text-(--text-muted)">
          {detailText ? (
            <span className={cn(
              "message-cjk-font block",
              isExpanded ? "whitespace-pre-wrap break-all" : "truncate",
            )}>
              {detailText}
            </span>
          ) : (
            <span>{model.isWaiting ? "等待确认" : "处理中…"}</span>
          )}
        </div>
        {model.isRunning && model.liveStatusText ? (
          <div className="mt-0.5 truncate text-[11px] text-(--text-soft)">
            {model.liveStatusText}
          </div>
        ) : null}
      </div>

      <div className="ml-auto flex shrink-0 flex-wrap items-center justify-end gap-1.5">
        {model.isWaiting && onAllow && onDeny ? (
          <PermissionActions
            disabled={interactionDisabled}
            disabledReason={interactionDisabledReason}
            onAllow={onAllow}
            onDeny={onDeny}
          />
        ) : null}
        {model.hasResult && !model.isWaiting ? (
          <button
            type="button"
            aria-label={copied ? "已复制结果" : "复制结果"}
            title={copied ? "已复制结果" : "复制结果"}
            onClick={(event) => {
              event.stopPropagation();
              onCopyResult();
            }}
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
        {model.hasResult ? (
          <div className="shrink-0 text-(--icon-muted)">
            {isExpanded
              ? <ChevronDown className="h-3.5 w-3.5" />
              : <ChevronRight className="h-3.5 w-3.5" />}
          </div>
        ) : null}
      </div>
    </div>
  );
}

function ToolStatusIcon({ model }: { model: ToolBlockViewModel }) {
  const statusIcon = TOOL_STATUS_ICON_MAP[model.status];
  const StatusIcon = statusIcon.icon;
  return <StatusIcon className={cn("h-3.5 w-3.5", statusIcon.className)} />;
}

function PermissionActions({
  disabled,
  disabledReason,
  onAllow,
  onDeny,
}: {
  disabled: boolean;
  disabledReason?: string;
  onAllow: () => void;
  onDeny: () => void;
}) {
  return (
    <>
      <button
        type="button"
        onClick={(event) => {
          event.stopPropagation();
          onDeny();
        }}
        disabled={disabled}
        title={disabled ? disabledReason : undefined}
        className={cn(
          "rounded-[7px] border border-(--divider-subtle-color) px-2 py-1 text-xs font-medium text-(--text-muted) transition-colors",
          disabled
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
          onAllow();
        }}
        disabled={disabled}
        title={disabled ? disabledReason : undefined}
        className={cn(
          "rounded-[7px] border px-2 py-1 text-xs font-medium transition-colors",
          disabled
            ? "cursor-not-allowed border-(--divider-subtle-color) bg-transparent text-(--text-soft)"
            : "border-primary/24 bg-primary/8 text-primary hover:bg-primary/12",
        )}
      >
        允许
      </button>
    </>
  );
}
