import type { HTMLAttributes, KeyboardEventHandler } from "react";
import {
  CheckCircle,
  Clock,
  Loader,
  Sparkles,
  Square,
  XCircle,
  type LucideIcon,
} from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";

import type {
  ToolBlockStatus,
  ToolBlockViewModel,
  ToolStatusTone,
} from "../tool-block-types";
import { ToolBlockHeaderActions } from "./tool-block-header-actions";
import {
  buildToolBlockHeaderProjection,
  type ToolBlockHeaderProjection,
} from "./tool-block-header-model";

const TOOL_STATUS_ICON_MAP: Readonly<Record<
  ToolBlockStatus,
  { className: string; icon: LucideIcon }
>> = {
  error: { className: "", icon: XCircle },
  pending: { className: "", icon: Sparkles },
  running: { className: "animate-spin", icon: Loader },
  rejected: { className: "", icon: XCircle },
  superseded: { className: "", icon: Square },
  stopped: { className: "", icon: Square },
  success: { className: "", icon: CheckCircle },
  waiting_permission: { className: "", icon: Clock },
};

const TOOL_TONE_STYLES: Readonly<Record<ToolStatusTone, string>> = {
  default: "text-(--icon-muted)",
  error: "text-(--destructive)",
  running: "text-(--primary)",
  success: "text-(--success)",
  waiting: "text-(--icon-muted)",
};

const TOOL_LABEL_STYLES: Readonly<Record<ToolStatusTone, string>> = {
  default: "text-(--text-default)",
  error: "text-(--destructive)",
  running: "text-(--primary)",
  success: "text-(--success)",
  waiting: "text-(--text-muted)",
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
  const { t } = useI18n();
  const projection = buildToolBlockHeaderProjection(model, isExpanded, t);
  const toggleProps = buildToggleProps(projection.canToggle, onToggle);
  return (
    <div
      {...toggleProps}
      className={cn(
        "grid min-w-0 grid-cols-[20px_minmax(0,1fr)_auto] items-center gap-2 radius-control-sm px-1.5 py-1 text-xs transition-colors",
        projection.canToggle
          ? "cursor-pointer hover:bg-(--surface-interactive-hover-background)"
          : "cursor-default",
        projection.stateClassName,
      )}
    >
      <ToolStatusIcon model={model} />
      <ToolBlockHeaderContent model={model} projection={projection} />
      <ToolBlockHeaderActions
        copied={copied}
        interactionDisabled={interactionDisabled}
        interactionDisabledReason={interactionDisabledReason}
        onAllow={onAllow}
        onCopyResult={onCopyResult}
        onDeny={onDeny}
        projection={projection}
      />
    </div>
  );
}

function buildToggleProps(
  enabled: boolean,
  onToggle: () => void,
): HTMLAttributes<HTMLDivElement> {
  const rules = [
    {
      matches: enabled,
      value: {
        onClick: onToggle,
        onKeyDown: createToggleKeyHandler(onToggle),
        role: "button",
        tabIndex: 0,
      },
    },
    { matches: true, value: {} },
  ];
  return rules.find((rule) => rule.matches)!.value;
}

function createToggleKeyHandler(
  onToggle: () => void,
): KeyboardEventHandler<HTMLDivElement> {
  const activationKeys = new Set(["Enter", " "]);
  return (event) => {
    if (!activationKeys.has(event.key)) {
      return;
    }
    event.preventDefault();
    onToggle();
  };
}

function ToolStatusIcon({ model }: { model: ToolBlockViewModel }) {
  const statusIcon = TOOL_STATUS_ICON_MAP[model.status];
  const StatusIcon = statusIcon.icon;
  return (
    <div
      className={cn(
        "flex h-5 w-5 items-center justify-center rounded-full",
        TOOL_TONE_STYLES[model.statusTone],
      )}
      data-timeline-anchor
      data-timeline-anchor-mode="box"
    >
      <StatusIcon className={cn("h-3.5 w-3.5", statusIcon.className)} />
    </div>
  );
}

function ToolBlockHeaderContent({
  model,
  projection,
}: {
  model: ToolBlockViewModel;
  projection: ToolBlockHeaderProjection;
}) {
  return (
    <div className="min-w-0 flex-1">
      <div className="flex min-w-0 items-center gap-1.5">
        <span className={cn(
          "shrink-0 text-xs font-medium",
          TOOL_LABEL_STYLES[model.statusTone],
        )}>
          {model.toolTitle}
        </span>
        <span className={cn(
          "shrink-0 rounded-[6px] px-1.5 py-0.5 text-2xs font-semibold",
          model.statusBadgeClassName,
        )}>
          {model.statusText}
        </span>
        <OptionalMetaText text={projection.metaText} />
      </div>
      <div className="mt-0.5 min-w-0 text-compact text-(--text-muted)">
        <span className={cn(
          "message-cjk-font block",
          projection.detailClassName,
        )}>
          {projection.detailText}
        </span>
      </div>
      <OptionalLiveStatus text={projection.liveStatusText} />
    </div>
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
    <div className="mt-0.5 truncate text-xs text-(--text-soft)">
      {text}
    </div>
  );
}
