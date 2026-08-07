import type { AgentConversationRuntimePhase } from "@/types/agent/agent-conversation";

import {
  type ComposerInputMode,
  type ComposerRuntimeActivity,
  MAX_COMPOSER_INPUT_LENGTH,
} from "../composer-model";

export interface ComposerViewCopy {
  defaultPlaceholder: string;
  goalConfirm: string;
  goalPlaceholder: string;
  sendMessage: string;
}

export interface ComposerInputProjection {
  charCount: number;
  hasTextInput: boolean;
  isInputEmpty: boolean;
  isNearLimit: boolean;
  isOverLimit: boolean;
}

export interface ComposerRuntimeProjection {
  canStopGeneration: boolean;
  activity: ComposerRuntimeActivity;
  isAwaitingPermission: boolean;
  sessionBusy: boolean;
}

const RUNTIME_ACTIVITY_BY_PHASE: Partial<Record<
  AgentConversationRuntimePhase,
  Exclude<ComposerRuntimeActivity, null>
>> = {
  compacting: "compacting",
  sending: "sending",
};

export interface ComposerModeProjection {
  activeError: string | null;
  isGoalMode: boolean;
  placeholder: string;
  sendButtonLabel: string;
}

export interface ComposerActionProjection {
  isSendDisabled: boolean;
  isTextareaLocked: boolean;
  shouldShowStopButton: boolean;
}

export function projectComposerInput(
  input: string,
  attachmentCount: number,
): ComposerInputProjection {
  const charCount = input.length;
  const hasTextInput = input.trim().length > 0;
  return {
    charCount,
    hasTextInput,
    isInputEmpty: [!hasTextInput, attachmentCount === 0].every(Boolean),
    isNearLimit: charCount > MAX_COMPOSER_INPUT_LENGTH * 0.8,
    isOverLimit: charCount > MAX_COMPOSER_INPUT_LENGTH,
  };
}

export function projectComposerRuntime({
  isLoading,
  queueItemCount,
  runtimePhase,
}: {
  isLoading: boolean;
  queueItemCount: number;
  runtimePhase: AgentConversationRuntimePhase | null;
}): ComposerRuntimeProjection {
  // 压缩由独立 runtime_status 事件驱动，可能先于 is_loading 投影到前端。
  // 这里让显式 phase 优先，避免压缩状态在 loading 标志同步前被吞掉。
  const isCompacting = runtimePhase === "compacting";
  const isRuntimeActive = isLoading || isCompacting;
  const isDispatching = [isLoading, runtimePhase === "sending"].every(
    Boolean,
  );
  return {
    activity: isCompacting
      ? "compacting"
      : isLoading
        ? RUNTIME_ACTIVITY_BY_PHASE[runtimePhase ?? "idle"] ?? "replying"
        : null,
    canStopGeneration: [isRuntimeActive, !isDispatching].every(Boolean),
    isAwaitingPermission: runtimePhase === "awaiting_permission",
    sessionBusy: [isRuntimeActive, queueItemCount > 0].some(Boolean),
  };
}

export function projectComposerMode({
  attachmentError,
  copy,
  goalCreateBlockedReason,
  goalError,
  inputMode,
}: {
  attachmentError: string | null;
  copy: ComposerViewCopy;
  goalCreateBlockedReason: string | null;
  goalError: string | null;
  inputMode: ComposerInputMode;
}): ComposerModeProjection {
  const isGoalMode = inputMode === "goal";
  const modeCopy = resolveModeCopy(isGoalMode, copy);
  return {
    activeError: resolveActiveError(
      isGoalMode,
      attachmentError,
      goalError,
      goalCreateBlockedReason,
    ),
    ...modeCopy,
    isGoalMode,
  };
}

function resolveModeCopy(
  isGoalMode: boolean,
  copy: ComposerViewCopy,
) {
  if (isGoalMode) {
    return {
      placeholder: copy.goalPlaceholder,
      sendButtonLabel: copy.goalConfirm,
    };
  }
  return {
    placeholder: copy.defaultPlaceholder,
    sendButtonLabel: copy.sendMessage,
  };
}

function resolveActiveError(
  isGoalMode: boolean,
  attachmentError: string | null,
  goalError: string | null,
  goalCreateBlockedReason: string | null,
): string | null {
  const activeGoalError = goalError ?? goalCreateBlockedReason;
  return isGoalMode ? activeGoalError : attachmentError;
}

export function projectComposerActions({
  canCreateGoal,
  goalCreateBlockedReason,
  hasStopAction,
  inputState,
  isGoalCreating,
  isGoalMode,
  isPreparingAttachments,
  isSessionSettingsSaving,
  runtimeState,
}: {
  canCreateGoal: boolean;
  goalCreateBlockedReason: string | null;
  hasStopAction: boolean;
  inputState: ComposerInputProjection;
  isGoalCreating: boolean;
  isGoalMode: boolean;
  isPreparingAttachments: boolean;
  isSessionSettingsSaving: boolean;
  runtimeState: ComposerRuntimeProjection;
}): ComposerActionProjection {
  const goalSendDisabled = [
    !inputState.hasTextInput,
    inputState.isOverLimit,
    isGoalCreating,
    isSessionSettingsSaving,
    !canCreateGoal,
    Boolean(goalCreateBlockedReason),
  ].some(Boolean);
  const messageSendDisabled = [
    inputState.isInputEmpty,
    inputState.isOverLimit,
    isPreparingAttachments,
    isSessionSettingsSaving,
    runtimeState.isAwaitingPermission,
  ].some(Boolean);
  return {
    isSendDisabled: isGoalMode ? goalSendDisabled : messageSendDisabled,
    isTextareaLocked: [isGoalMode, isGoalCreating].every(Boolean),
    shouldShowStopButton: [
      !isGoalMode,
      hasStopAction,
      runtimeState.canStopGeneration,
      inputState.isInputEmpty,
    ].every(Boolean),
  };
}
