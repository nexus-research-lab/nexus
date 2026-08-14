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
  className: string;
  frames: string[] | null;
  hint: string | null;
  message: string | null;
  messageClassName: string;
}

const ACTIVE_FRAMES = ["✽", "✻", "✶", "✢", "·"];
const PREPARING_FRAMES = ["·", "◦", "•", "◦"];

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
        className: "text-(--text-default)",
        frames: PREPARING_FRAMES,
        hint: null,
        message: copy.preparingAttachments,
        messageClassName: "",
      }
      : null,
    isGoalCreating
      ? {
        className: "text-(--brand-action)",
        frames: PREPARING_FRAMES,
        hint: null,
        message: isGoalConfirming
          ? copy.goalConfirming
          : copy.goalCreating,
        messageClassName: "animate-pulse",
      }
      : null,
    activeError
      ? {
        className: "text-(--destructive)",
        frames: null,
        hint: null,
        message: activeError,
        messageClassName: "",
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
      className: "text-(--text-soft)",
      frames: null,
      hint: copy.stopHint,
      message: null,
      messageClassName: "",
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
    className: "text-(--brand-action)",
    frames: ACTIVE_FRAMES,
    hint,
    message,
    messageClassName: "animate-pulse",
  };
}

export function getCharacterCountClassName({
  isNearLimit,
  isOverLimit,
}: {
  isNearLimit: boolean;
  isOverLimit: boolean;
}): string {
  const candidates = [
    { active: isOverLimit, className: "text-destructive" },
    { active: isNearLimit, className: "text-warning" },
  ];
  return candidates.find((candidate) => candidate.active)?.className
    ?? "text-(--text-soft)";
}
