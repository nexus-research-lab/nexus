// INPUT: Composer runtime/Goal/附件/错误状态、字符限制与本地化文案。
// OUTPUT: Footer 状态优先级、语义 tone、加载阶段与字符计数风险等级。
// POS: Composer Footer 纯业务投影；不返回 class、Tailwind、颜色、阴影、帧字符或动效名称。

import type { ReactNode, RefObject } from "react";

import type { ContextUsageData } from "@/types/generated/protocol";

import type {
  ComposerContextUsageItem,
  ComposerRuntimeActivity,
} from "../../composer-model";
import type {
  ComposerSessionSettingsController,
} from "../../controller/use-composer-session-settings";
import type { ComposerLocalDirectoriesController } from "../../controller/use-composer-local-directories";
import type { ComposerSubmitButtonProps } from "../composer-submit-button";

export interface ComposerFooterProps {
  actionButtonRef: RefObject<HTMLButtonElement | null>;
  activeError: string | null;
  canCreateGoal: boolean;
  canUseLoop: boolean;
  canUseWorkGraphDistillations: boolean;
  charCount: number;
  contextUsage: ContextUsageData | null;
  contextUsageItems?: readonly ComposerContextUsageItem[];
  goalModeExtra: ReactNode;
  goalScopeLabel: string;
  historyIndex: number;
  inputHistoryLength: number;
  isActionMenuOpen: boolean;
  isGoalConfirming: boolean;
  isGoalCreating: boolean;
  isGoalMode: boolean;
  isNearLimit: boolean;
  isOverLimit: boolean;
  isPreparingAttachments: boolean;
  localDirectoriesController: ComposerLocalDirectoriesController;
  maxLength: number;
  onActionMenuClose: () => void;
  onActionMenuToggle: () => void;
  onAttachmentSelect: () => void;
  onCancelGoal: () => void;
  onGoalToggle: (checked: boolean) => void;
  onLoopSelect: () => void;
  onWorkGraphDistillationsSelect: () => void;
  onLocalDirectorySelect: () => void;
  runtimeActivity: ComposerRuntimeActivity;
  sessionSettingsController: ComposerSessionSettingsController;
  sessionSettingsDisabled: boolean;
  showActionMenu: boolean;
  showPoweredByNexus: boolean;
  submit: ComposerSubmitButtonProps;
}

export interface ComposerFooterStatusCopy {
  compacting: string;
  goalCreating: string;
  goalConfirming: string;
  preparingAttachments: string;
  replying: string;
  sending: string;
  stopHint: string;
}

export interface ComposerFooterStatusProjection {
  hint: string | null;
  indicator: "active" | "preparing" | null;
  kind: "activity" | "error" | "goal" | "preparing" | "replying";
  message: string | null;
  tone: "brand" | "danger" | "default" | "soft";
}

const RUNTIME_STATUS_DEFINITIONS: Record<
  Exclude<ComposerRuntimeActivity, null>,
  {copyKey: "compacting" | "replying" | "sending"; showStopHint: boolean}
> = {
  compacting: {copyKey: "compacting", showStopHint: true},
  replying: {copyKey: "replying", showStopHint: true},
  sending: {copyKey: "sending", showStopHint: false},
};

export function projectComposerFooterStatus({
  activeError,
  copy,
  isGoalCreating,
  isGoalConfirming,
  isPreparingAttachments,
  runtimeActivity,
}: {
  activeError: string | null;
  copy: ComposerFooterStatusCopy;
  isGoalCreating: boolean;
  isGoalConfirming: boolean;
  isPreparingAttachments: boolean;
  runtimeActivity: ComposerRuntimeActivity;
}): ComposerFooterStatusProjection | null {
  const candidates: Array<ComposerFooterStatusProjection | null> = [
    projectRuntimeActivityStatus(runtimeActivity, copy),
    isPreparingAttachments
      ? {
        hint: null,
        indicator: "preparing",
        kind: "preparing",
        message: copy.preparingAttachments,
        tone: "default",
      }
      : null,
    isGoalCreating
      ? {
        hint: null,
        indicator: "preparing",
        kind: "goal",
        message: isGoalConfirming
          ? copy.goalConfirming
          : copy.goalCreating,
        tone: "brand",
      }
      : null,
    activeError
      ? {
        hint: null,
        indicator: null,
        kind: "error",
        message: activeError,
        tone: "danger",
      }
      : null,
  ];
  return candidates.find((candidate) => candidate !== null) ?? null;
}

function projectRuntimeActivityStatus(
  activity: ComposerRuntimeActivity,
  copy: ComposerFooterStatusCopy,
): ComposerFooterStatusProjection | null {
  if (!activity) {
    return null;
  }
  if (activity === "replying") {
    return {
      hint: copy.stopHint,
      indicator: null,
      kind: "replying",
      message: null,
      tone: "soft",
    };
  }
  const definition = RUNTIME_STATUS_DEFINITIONS[activity];
  return buildActiveStatus(
    `${copy[definition.copyKey]}${activity === "sending" ? "" : "…"}`,
    definition.showStopHint ? copy.stopHint : null,
  );
}

function buildActiveStatus(
  message: string,
  hint: string | null,
): ComposerFooterStatusProjection {
  return {
    hint,
    indicator: "active",
    kind: "activity",
    message,
    tone: "brand",
  };
}

export function getCharacterCountTone({
  isNearLimit,
  isOverLimit,
}: {
  isNearLimit: boolean;
  isOverLimit: boolean;
}): "danger" | "soft" | "warning" {
  const candidates = [
    { active: isOverLimit, tone: "danger" as const },
    { active: isNearLimit, tone: "warning" as const },
  ];
  return candidates.find((candidate) => candidate.active)?.tone ?? "soft";
}
