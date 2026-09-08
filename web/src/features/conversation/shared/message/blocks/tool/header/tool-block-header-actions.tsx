/**
 * INPUT: ToolBlock 展开、复制、权限资格与交互禁用状态。
 * OUTPUT: 复用共享微型 Button/IconButton 的权限与结果动作，以及只读展开指示。
 * POS: ToolBlock Header 的唯一动作组；不拥有整行展开事件或权限状态推导。
 */
import type { MouseEventHandler } from "react";
import {
  Check,
  ChevronDown,
  ChevronRight,
  Copy,
} from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";

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
  const title = disabled ? disabledReason : undefined;
  return (
    <>
      <UiButton
        disabled={disabled}
        onClick={stopPropagationAndRun(onDeny)}
        size="2xs"
        title={title}
        variant="surface"
      >
        {t("room.permission_deny")}
      </UiButton>
      <UiButton
        disabled={disabled}
        onClick={stopPropagationAndRun(onAllow)}
        size="2xs"
        title={title}
        tone="primary"
        variant="surface"
      >
        {t("room.permission_allow")}
      </UiButton>
    </>
  );
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
    <UiIconButton
      aria-label={label}
      onClick={stopPropagationAndRun(onCopyResult)}
      size="xs"
      tone={copied ? "success" : "default"}
      tooltip={label}
      variant="ghost"
    >
      <CopyIcon className="h-3.5 w-3.5" />
    </UiIconButton>
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
