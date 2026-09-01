/**
 * INPUT: ToolBlock 单一视图模型、展开状态与可用操作。
 * OUTPUT: 收起时含摘要的单行工具头，展开时仅保留工具身份、状态与操作。
 * POS: 普通 ToolBlock 的稳定头部，不渲染展开明细。
 */
import type { HTMLAttributes, KeyboardEventHandler } from "react";

import { cn } from "@/shared/ui/class-name";

import type {
  ToolBlockViewModel,
  ToolStatusTone,
} from "../tool-block-types";
import { ToolActivityIcon } from "../../../ui/activity-icon";
import { ToolBlockHeaderActions } from "./tool-block-header-actions";

const TOOL_LABEL_STYLES: Readonly<Record<ToolStatusTone, string>> = {
  default: "text-(--text-soft)",
  error: "text-(--destructive)",
  running: "text-(--primary)",
  success: "text-(--text-soft)",
  waiting: "text-(--text-soft)",
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
  const canToggle = model.hasResult;
  const waitingForPermission = model.status === "waiting_permission";
  const toggleProps = buildToggleProps(canToggle, onToggle);
  return (
    <div
      {...toggleProps}
      className={cn(
        "grid min-h-7 min-w-0 grid-cols-[20px_minmax(0,1fr)_auto] items-center gap-1.5 radius-control-sm px-1.5 py-0.5 text-sm font-normal leading-5 text-(--text-soft) transition-colors",
        canToggle
          ? "cursor-pointer hover:bg-(--surface-interactive-hover-background)"
          : "cursor-default",
      )}
      data-activity-row="tool"
      data-message-detail-sticky-header={isExpanded || undefined}
      data-tool-block-layout={isExpanded ? "expanded" : "collapsed"}
      data-tool-block-status={model.status}
    >
      <ToolSemanticIcon model={model} />
      <ToolBlockHeaderContent
        detailText={isExpanded ? null : model.collapsedDetailText}
        liveStatusText={model.status === "running" ? model.liveStatusText : null}
        metaText={waitingForPermission
          ? model.waitingActionHint
          : model.durationText}
        model={model}
      />
      <ToolBlockHeaderActions
        canCopyResult={model.hasResult && !waitingForPermission}
        canToggle={canToggle}
        copied={copied}
        expanded={isExpanded}
        interactionDisabled={interactionDisabled}
        interactionDisabledReason={interactionDisabledReason}
        onAllow={onAllow}
        onCopyResult={onCopyResult}
        onDeny={onDeny}
        showPermissionActions={waitingForPermission}
      />
    </div>
  );
}

function buildToggleProps(
  enabled: boolean,
  onToggle: () => void,
): HTMLAttributes<HTMLDivElement> {
  return enabled ? {
    onClick: onToggle,
    onKeyDown: createToggleKeyHandler(onToggle),
    role: "button",
    tabIndex: 0,
  } : {};
}

function createToggleKeyHandler(
  onToggle: () => void,
): KeyboardEventHandler<HTMLDivElement> {
  return (event) => {
    if (event.key !== "Enter" && event.key !== " ") {
      return;
    }
    event.preventDefault();
    onToggle();
  };
}

function ToolSemanticIcon({ model }: { model: ToolBlockViewModel }) {
  return (
    <div
      className={cn(
        "flex h-5 w-5 items-center justify-center text-(--icon-muted)",
        model.status === "running" && "text-(--primary)",
      )}
      data-tool-block-icon={model.toolVisualKind}
      data-timeline-anchor
      data-timeline-anchor-mode="box"
      title={model.toolTitle}
    >
      <ToolActivityIcon className="h-3.5 w-3.5" kind={model.toolVisualKind} />
    </div>
  );
}

function ToolBlockHeaderContent({
  detailText,
  liveStatusText,
  metaText,
  model,
}: {
  detailText: string | null;
  liveStatusText: string | null;
  metaText: string | null;
  model: ToolBlockViewModel;
}) {
  return (
    <div
      className={cn(
        "flex min-w-0 items-baseline gap-1.5",
        model.status === "running" && "nexus-live-tool-text",
      )}
      data-live-tool-text={model.status === "running" || undefined}
    >
      <span className={cn(
        "shrink-0",
        TOOL_LABEL_STYLES[model.statusTone],
      )}>
        {model.toolTitle}
      </span>
      <ToolStatusText model={model} />
      <ToolDetailText text={detailText} />
      <OptionalLiveStatus text={liveStatusText} />
      <OptionalMetaText text={metaText} />
    </div>
  );
}

function ToolStatusText({ model }: { model: ToolBlockViewModel }) {
  const assistiveOnly = model.status === "success" || model.status === "error";
  return (
    <span
      className={cn(
        assistiveOnly ? "sr-only" : "shrink-0 text-xs",
        !assistiveOnly && TOOL_LABEL_STYLES[model.statusTone],
      )}
      data-tool-block-status-visibility={assistiveOnly ? "assistive" : "visible"}
    >
      {model.statusText}
    </span>
  );
}

function ToolDetailText({
  text,
}: {
  text: string | null;
}) {
  if (!text) {
    return null;
  }
  return (
    <span
      className="message-cjk-font min-w-0 flex-1 truncate text-(--text-soft)"
      data-tool-block-detail="inline"
    >
      {text}
    </span>
  );
}

function OptionalMetaText({ text }: { text: string | null }) {
  if (!text) {
    return null;
  }
  return (
    <span className="shrink-0 text-xs text-(--text-soft)">
      {text}
    </span>
  );
}

function OptionalLiveStatus({ text }: { text: string | null }) {
  if (!text) {
    return null;
  }
  return (
    <span className="min-w-0 truncate text-xs text-(--text-soft)">
      {text}
    </span>
  );
}
