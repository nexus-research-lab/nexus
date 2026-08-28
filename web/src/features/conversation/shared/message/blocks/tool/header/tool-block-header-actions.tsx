import type { MouseEventHandler } from "react";
import {
  Check,
  ChevronDown,
  ChevronRight,
  Copy,
} from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";

interface ToolBlockHeaderActionsProps {
  canCopyResult: boolean;
  canToggle: boolean;
  copied: boolean;
  expanded: boolean;
  interactionDisabled: boolean;
  interactionDisabledReason?: string;
  onAllow?: () => void;
  onCopyResult: () => void;
  onDeny?: () => void;
  showPermissionActions: boolean;
}

export function ToolBlockHeaderActions({
  canCopyResult,
  canToggle,
  copied,
  expanded,
  interactionDisabled,
  interactionDisabledReason,
  onAllow,
  onCopyResult,
  onDeny,
  showPermissionActions,
}: ToolBlockHeaderActionsProps) {
  return (
    <div className="ml-auto flex shrink-0 flex-wrap items-center justify-end gap-1.5">
      <PermissionActions
        disabled={interactionDisabled}
        disabledReason={interactionDisabledReason}
        onAllow={onAllow}
        onDeny={onDeny}
        visible={showPermissionActions}
      />
      <CopyResultAction
        copied={copied}
        onCopyResult={onCopyResult}
        visible={canCopyResult && expanded}
      />
      <ExpansionIndicator canToggle={canToggle} expanded={expanded} />
    </div>
  );
}

function PermissionActions({
  disabled,
  disabledReason,
  onAllow,
  onDeny,
  visible,
}: {
  disabled: boolean;
  disabledReason?: string;
  onAllow?: () => void;
  onDeny?: () => void;
  visible: boolean;
}) {
  const { t } = useI18n();
  if (!visible || !onAllow || !onDeny) {
    return null;
  }
  const state = getPermissionButtonState(disabled, disabledReason);
  return (
    <>
      <button
        className={cn(
          "radius-control-sm border border-(--divider-subtle-color) px-2 py-1 text-xs font-medium text-(--text-muted) transition-colors",
          state.denyClassName,
        )}
        disabled={disabled}
        onClick={stopPropagationAndRun(onDeny)}
        title={state.title}
        type="button"
      >
        {t("room.permission_deny")}
      </button>
      <button
        className={cn(
          "radius-control-sm border px-2 py-1 text-xs font-medium transition-colors",
          state.allowClassName,
        )}
        disabled={disabled}
        onClick={stopPropagationAndRun(onAllow)}
        title={state.title}
        type="button"
      >
        {t("room.permission_allow")}
      </button>
    </>
  );
}

function getPermissionButtonState(
  disabled: boolean,
  disabledReason?: string,
) {
  return disabled
    ? {
      allowClassName: "cursor-not-allowed border-(--divider-subtle-color) bg-transparent text-(--text-soft)",
      denyClassName: "cursor-not-allowed opacity-(--disabled-opacity)",
      title: disabledReason,
    }
    : {
      allowClassName: "border-primary/24 bg-primary/8 text-primary hover:bg-primary/12",
      denyClassName: "hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
      title: undefined,
    };
}

function CopyResultAction({
  copied,
  onCopyResult,
  visible,
}: {
  copied: boolean;
  onCopyResult: () => void;
  visible: boolean;
}) {
  const { t } = useI18n();
  if (!visible) {
    return null;
  }
  const CopyIcon = copied ? Check : Copy;
  const label = t(copied
    ? "message.tool_copied_result"
    : "message.tool_copy_result");
  return (
    <button
      aria-label={label}
      className={cn(
        "inline-flex h-6 w-6 items-center justify-center rounded-[6px] transition-colors",
        copied
          ? "bg-[color:color-mix(in_srgb,var(--success)_10%,transparent)] text-(--success)"
          : "text-(--icon-muted) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
      )}
      onClick={stopPropagationAndRun(onCopyResult)}
      title={label}
      type="button"
    >
      <CopyIcon className="h-3.5 w-3.5" />
    </button>
  );
}

function ExpansionIndicator({
  canToggle,
  expanded,
}: {
  canToggle: boolean;
  expanded: boolean;
}) {
  if (!canToggle) {
    return null;
  }
  const ExpansionIcon = expanded ? ChevronDown : ChevronRight;
  return (
    <div className="shrink-0 text-(--icon-muted)">
      <ExpansionIcon className="h-3.5 w-3.5" />
    </div>
  );
}

function stopPropagationAndRun(action: () => void): MouseEventHandler<HTMLButtonElement> {
  return (event) => {
    event.stopPropagation();
    action();
  };
}
