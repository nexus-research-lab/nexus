import type { ReactNode } from "react";
import { Send, StopCircle, Target } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { LoadingOrb } from "@/shared/ui/feedback/loading-orb";

import {
  COMPOSER_DANGER_ACTION_BUTTON_CLASS_NAME,
  COMPOSER_PRIMARY_ACTION_BUTTON_CLASS_NAME,
} from "../composer-styles";

export interface ComposerSubmitButtonProps {
  isDisabled: boolean;
  isGoalCreating: boolean;
  isGoalMode: boolean;
  isPreparingAttachments: boolean;
  onSend: () => void | Promise<void>;
  onStop?: () => void;
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
  const projection = projectComposerSubmitButton({
    ...props,
    shouldStop: props.shouldStop && Boolean(props.onStop),
  });
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
      onClick={
        projection.action === "stop"
          ? () => props.onStop?.()
          : () => void props.onSend()
      }
      type="button"
    >
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
        props.isGoalMode && props.isGoalCreating,
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
        "nexus-chat-composer-submit gap-1.5",
      ),
      disabled: props.isDisabled,
    },
    stop: {
      ariaLabel: props.stopLabel,
      className: cn(
        COMPOSER_DANGER_ACTION_BUTTON_CLASS_NAME,
        "nexus-chat-composer-submit nexus-chat-composer-submit-stop gap-1.5",
      ),
      disabled: false,
    },
  };
  return { action, ...behavior[action], visual };
}
