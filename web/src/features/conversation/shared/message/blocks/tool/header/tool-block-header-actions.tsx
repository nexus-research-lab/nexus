import type { MouseEventHandler } from "react";
import {
  Check,
  ChevronDown,
  ChevronRight,
  Copy,
  type LucideIcon,
} from "lucide-react";

import { cn } from "@/shared/ui/class-name";

import type { ToolBlockHeaderProjection } from "./tool-block-header-model";

interface ToolBlockHeaderActionsProps {
  copied: boolean;
  interactionDisabled: boolean;
  interactionDisabledReason?: string;
  onAllow?: () => void;
  onCopyResult: () => void;
  onDeny?: () => void;
  projection: ToolBlockHeaderProjection;
}

interface PermissionAction {
  disabled: boolean;
  disabledReason?: string;
  onAllow: () => void;
  onDeny: () => void;
}

const COPY_ICON_BY_STATE: ReadonlyArray<LucideIcon> = [Copy, Check];
const EXPANSION_ICON_BY_STATE: Readonly<Record<
  ToolBlockHeaderProjection["expansionState"],
  LucideIcon
>> = {
  collapsed: ChevronRight,
  expanded: ChevronDown,
};

export function ToolBlockHeaderActions({
  copied,
  interactionDisabled,
  interactionDisabledReason,
  onAllow,
  onCopyResult,
  onDeny,
  projection,
}: ToolBlockHeaderActionsProps) {
  const permissionAction = buildPermissionAction({
    disabled: interactionDisabled,
    disabledReason: interactionDisabledReason,
    onAllow,
    onDeny,
    visible: projection.showPermissionActions,
  });
  return (
    <div className="ml-auto flex shrink-0 flex-wrap items-center justify-end gap-1.5">
      <PermissionActions action={permissionAction} />
      <CopyResultAction
        copied={copied}
        onCopyResult={onCopyResult}
        visible={projection.canCopyResult}
      />
      <ExpansionIndicator projection={projection} />
    </div>
  );
}

function buildPermissionAction({
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
}): PermissionAction | null {
  const rules = [
    {
      matches: [visible, Boolean(onAllow), Boolean(onDeny)].every(Boolean),
      value: {
        disabled,
        disabledReason,
        onAllow: onAllow!,
        onDeny: onDeny!,
      },
    },
    { matches: true, value: null },
  ];
  return rules.find((rule) => rule.matches)!.value;
}

function PermissionActions({ action }: { action: PermissionAction | null }) {
  if (!action) {
    return null;
  }
  const state = getPermissionButtonState(action.disabled, action.disabledReason);
  return (
    <>
      <button
        className={cn(
          "rounded-[7px] border border-(--divider-subtle-color) px-2 py-1 text-xs font-medium text-(--text-muted) transition-colors",
          state.denyClassName,
        )}
        disabled={action.disabled}
        onClick={stopPropagationAndRun(action.onDeny)}
        title={state.title}
        type="button"
      >
        拒绝
      </button>
      <button
        className={cn(
          "rounded-[7px] border px-2 py-1 text-xs font-medium transition-colors",
          state.allowClassName,
        )}
        disabled={action.disabled}
        onClick={stopPropagationAndRun(action.onAllow)}
        title={state.title}
        type="button"
      >
        允许
      </button>
    </>
  );
}

function getPermissionButtonState(
  disabled: boolean,
  disabledReason?: string,
) {
  const states = [
    {
      allowClassName: "border-primary/24 bg-primary/8 text-primary hover:bg-primary/12",
      denyClassName: "hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
      title: undefined,
    },
    {
      allowClassName: "cursor-not-allowed border-(--divider-subtle-color) bg-transparent text-(--text-soft)",
      denyClassName: "cursor-not-allowed opacity-(--disabled-opacity)",
      title: disabledReason,
    },
  ];
  return states[Number(disabled)];
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
  if (!visible) {
    return null;
  }
  const CopyIcon = COPY_ICON_BY_STATE[Number(copied)];
  const labels = ["复制结果", "已复制结果"];
  const styles = [
    "text-(--icon-muted) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
    "bg-[color:color-mix(in_srgb,var(--success)_10%,transparent)] text-(--success)",
  ];
  return (
    <button
      aria-label={labels[Number(copied)]}
      className={cn(
        "inline-flex h-6 w-6 items-center justify-center rounded-[6px] transition-colors",
        styles[Number(copied)],
      )}
      onClick={stopPropagationAndRun(onCopyResult)}
      title={labels[Number(copied)]}
      type="button"
    >
      <CopyIcon className="h-3.5 w-3.5" />
    </button>
  );
}

function ExpansionIndicator({
  projection,
}: {
  projection: ToolBlockHeaderProjection;
}) {
  if (!projection.canToggle) {
    return null;
  }
  const ExpansionIcon = EXPANSION_ICON_BY_STATE[projection.expansionState];
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
