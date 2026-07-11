import type { ReactNode, RefObject } from "react";

export interface ComposerFooterProps {
  actionButtonRef: RefObject<HTMLButtonElement | null>;
  activeError: string | null;
  canCreateGoal: boolean;
  canUseLoop: boolean;
  canStopGeneration: boolean;
  charCount: number;
  goalModeExtra: ReactNode;
  goalScopeLabel: string;
  historyIndex: number;
  inputHistoryLength: number;
  isActionMenuOpen: boolean;
  isDispatching: boolean;
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
}

export interface ComposerFooterStatusCopy {
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

interface FooterStatusCandidate {
  active: boolean;
  status: ComposerFooterStatusProjection;
}

const ACTIVE_FRAMES = ["✽", "✻", "✶", "✢", "·"];
const PREPARING_FRAMES = ["·", "◦", "•", "◦"];

export function projectComposerFooterStatus({
  activeError,
  canStopGeneration,
  copy,
  isDispatching,
  isGoalCreating,
  isPreparingAttachments,
}: {
  activeError: string | null;
  canStopGeneration: boolean;
  copy: ComposerFooterStatusCopy;
  isDispatching: boolean;
  isGoalCreating: boolean;
  isPreparingAttachments: boolean;
}): ComposerFooterStatusProjection | null {
  const candidates: FooterStatusCandidate[] = [
    {
      active: isDispatching,
      status: buildActiveStatus(copy.sending, null),
    },
    {
      active: canStopGeneration,
      status: buildActiveStatus(`${copy.replying}…`, copy.stopHint),
    },
    {
      active: isPreparingAttachments,
      status: {
        className: "text-(--text-default)",
        frames: PREPARING_FRAMES,
        hint: null,
        message: copy.preparingAttachments,
        messageClassName: "",
      },
    },
    {
      active: isGoalCreating,
      status: {
        className: "text-(--primary)",
        frames: PREPARING_FRAMES,
        hint: null,
        message: copy.goalCreating,
        messageClassName: "animate-pulse",
      },
    },
    {
      active: Boolean(activeError),
      status: {
        className: "text-(--destructive)",
        frames: null,
        hint: null,
        message: activeError ?? "",
        messageClassName: "",
      },
    },
  ];
  return candidates.find((candidate) => candidate.active)?.status ?? null;
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
