import type { ReactNode } from "react";
import { Send, StopCircle, Target } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { LoadingOrb } from "@/shared/ui/feedback/loading-orb";

import {
  COMPOSER_DANGER_ACTION_BUTTON_CLASS_NAME,
  COMPOSER_PRIMARY_ACTION_BUTTON_CLASS_NAME,
} from "../composer-styles";

export interface ComposerSubmitButtonProps {
  enterLabel: string;
  isDisabled: boolean;
  isGoalCreating: boolean;
  isGoalMode: boolean;
  isPreparingAttachments: boolean;
  onSend: () => void | Promise<void>;
  onStop: () => void;
  sendLabel: string;
  shouldStop: boolean;
  stopLabel: string;
}

type ComposerSubmitVisual = "goal" | "loading" | "send" | "stop";

interface ComposerSubmitProjection {
  action: "send" | "stop";
  ariaLabel: string;
  className: string;
  disabled: boolean;
  inlineLabel: string | null;
  visual: ComposerSubmitVisual;
}

const SUBMIT_ACTION_BY_VISUAL: Record<
  ComposerSubmitVisual,
  ComposerSubmitProjection["action"]
> = {
  goal: "send",
  loading: "send",
  send: "send",
  stop: "stop",
};

export function ComposerSubmitButton(props: ComposerSubmitButtonProps) {
  const projection = projectComposerSubmitButton(props);
  const commands = {
    send: () => void props.onSend(),
    stop: props.onStop,
  };
  const content: Record<ComposerSubmitVisual, ReactNode> = {
    goal: <Target size={16} />,
    loading: <LoadingOrb frames={["·", "◦", "•", "◦"]} />,
    send: <Send size={16} />,
    stop: <StopCircle size={16} />,
  };
  return (
    <button
      aria-label={projection.ariaLabel}
      className={projection.className}
      disabled={projection.disabled}
      onClick={commands[projection.action]}
      type="button"
    >
      <ComposerSubmitInlineLabel label={projection.inlineLabel} />
      {content[projection.visual]}
    </button>
  );
}

function projectComposerSubmitButton(
  props: ComposerSubmitButtonProps,
): ComposerSubmitProjection {
  const visualCandidates: Array<{
    active: boolean;
    visual: ComposerSubmitVisual;
  }> = [
    { active: props.shouldStop, visual: "stop" },
    {
      active: [
        props.isPreparingAttachments,
        props.isGoalCreating,
      ].some(Boolean),
      visual: "loading",
    },
    { active: props.isGoalMode, visual: "goal" },
  ];
  const visual = visualCandidates.find((candidate) => candidate.active)?.visual
    ?? "send";
  const action = SUBMIT_ACTION_BY_VISUAL[visual];
  const behavior: Record<
    ComposerSubmitProjection["action"],
    Omit<ComposerSubmitProjection, "action" | "visual">
  > = {
    send: {
      ariaLabel: props.sendLabel,
      className: cn(
        COMPOSER_PRIMARY_ACTION_BUTTON_CLASS_NAME,
        "gap-1.5 min-[760px]:w-auto min-[760px]:px-3",
      ),
      disabled: props.isDisabled,
      inlineLabel: props.enterLabel,
    },
    stop: {
      ariaLabel: props.stopLabel,
      className: COMPOSER_DANGER_ACTION_BUTTON_CLASS_NAME,
      disabled: false,
      inlineLabel: null,
    },
  };
  return { action, ...behavior[action], visual };
}

function ComposerSubmitInlineLabel({ label }: { label: string | null }) {
  if (!label) {
    return null;
  }
  return (
    <span className="hidden text-[12px] font-semibold min-[760px]:inline">
      {label}
    </span>
  );
}
