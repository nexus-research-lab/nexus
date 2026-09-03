// INPUT: Composer 发送、Goal 创建、附件准备和停止执行状态。
// OUTPUT: 单一语义动作按钮及当前可访问名称和视觉状态。
// POS: Composer 提交动作投影；共享 UiButton 拥有基础 DOM 与视觉合同。

import type { ReactNode } from "react";
import { Send, StopCircle, Target } from "lucide-react";

import { UiIconButton } from "@/shared/ui/button/button";
import { LoadingOrb } from "@/shared/ui/feedback/loading-orb";

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
  tone: "danger" | "primary";
  variant: "solid" | "surface";
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
    <UiIconButton
      aria-label={projection.ariaLabel}
      className={projection.className}
      disabled={projection.disabled}
      onClick={
        projection.action === "stop"
          ? () => props.onStop?.()
          : () => void props.onSend()
      }
      size="md"
      tone={projection.tone}
      variant={projection.variant}
    >
      {content[projection.visual]}
    </UiIconButton>
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
      className: "nexus-chat-composer-submit shrink-0",
      disabled: props.isDisabled,
      tone: "primary",
      variant: "solid",
    },
    stop: {
      ariaLabel: props.stopLabel,
      className: "nexus-chat-composer-submit nexus-chat-composer-submit-stop shrink-0",
      disabled: false,
      tone: "danger",
      variant: "surface",
    },
  };
  return { action, ...behavior[action], visual };
}
