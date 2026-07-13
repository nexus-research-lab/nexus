import type { ReactNode, RefObject } from "react";

import type { ComposerRuntimeActivity } from "../../composer-model";
import type { AgentRuntimeKind } from "@/types/settings/preferences";

export interface ComposerFooterProps {
  actionButtonRef: RefObject<HTMLButtonElement | null>;
  activeError: string | null;
  canCreateGoal: boolean;
  canUseLoop: boolean;
  charCount: number;
  goalModeExtra: ReactNode;
  goalScopeLabel: string;
  historyIndex: number;
  inputHistoryLength: number;
  isActionMenuOpen: boolean;
  isGoalCreating: boolean;
  isGoalMode: boolean;
  isNearLimit: boolean;
  isOverLimit: boolean;
  isPreparingAttachments: boolean;
  maxLength: number;
  onActionMenuClose: () => void;
  onActionMenuToggle: () => void;
  onAttachmentSelect: () => void;
  onCancelGoal: () => void;
  onGoalToggle: (checked: boolean) => void;
  onLoopSelect: () => void;
  runtimeActivity: ComposerRuntimeActivity;
  runtimeKind: AgentRuntimeKind;
}

export interface ComposerFooterStatusCopy {
  compacting: string;
  goalCreating: string;
  preparingAttachments: string;
  replying: string;
  sending: string;
  stopHint: string;
}

export interface ComposerFooterStatusProjection {
  className: string;
  frames: string[] | null;
  hint: string | null;
  message: string;
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
  isPreparingAttachments,
  runtimeActivity,
}: {
  activeError: string | null;
  copy: ComposerFooterStatusCopy;
  isGoalCreating: boolean;
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
        className: "text-(--primary)",
        frames: PREPARING_FRAMES,
        hint: null,
        message: copy.goalCreating,
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
    className: "text-(--success)",
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
