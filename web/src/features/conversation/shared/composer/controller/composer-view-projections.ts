import type { AgentConversationRuntimePhase } from "@/types/agent/agent-conversation";

import {
  type ComposerInputMode,
  type ComposerRuntimeActivity,
  MAX_COMPOSER_INPUT_LENGTH,
} from "../composer-model";

export interface ComposerViewCopy {
  defaultPlaceholder: string;
  enterQueue: string;
  enterSend: string;
  goalConfirm: string;
  goalEnterStart: string;
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
  enterLabel: string;
  isGoalMode: boolean;
  placeholder: string;
  sendButtonLabel: string;
}

export interface ComposerActionProjection {
  isSendDisabled: boolean;
  isTextareaLocked: boolean;
  shouldShowInlineShortcuts: boolean;
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
  const isDispatching = [isLoading, runtimePhase === "sending"].every(Boolean);
  return {
    activity: isLoading
      ? RUNTIME_ACTIVITY_BY_PHASE[runtimePhase ?? "idle"] ?? "replying"
      : null,
    canStopGeneration: [isLoading, !isDispatching].every(Boolean),
    sessionBusy: [isLoading, queueItemCount > 0].some(Boolean),
  };
}

export function projectComposerMode({
  attachmentError,
  copy,
  goalCreateBlockedReason,
  goalError,
  inputMode,
  queueWhenSessionBusy,
  sessionBusy,
}: {
  attachmentError: string | null;
  copy: ComposerViewCopy;
  goalCreateBlockedReason: string | null;
  goalError: string | null;
  inputMode: ComposerInputMode;
  queueWhenSessionBusy: boolean;
  sessionBusy: boolean;
}): ComposerModeProjection {
  const isGoalMode = inputMode === "goal";
  const modeCopy = resolveModeCopy(
    isGoalMode,
    copy,
    queueWhenSessionBusy,
    sessionBusy,
  );
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
  queueWhenSessionBusy: boolean,
  sessionBusy: boolean,
) {
  if (isGoalMode) {
    return {
      enterLabel: copy.goalEnterStart,
      placeholder: copy.goalPlaceholder,
      sendButtonLabel: copy.goalConfirm,
    };
  }
  return {
    enterLabel: resolveMessageEnterLabel(
      copy,
      queueWhenSessionBusy,
      sessionBusy,
    ),
    placeholder: copy.defaultPlaceholder,
    sendButtonLabel: copy.sendMessage,
  };
}

function resolveMessageEnterLabel(
  copy: ComposerViewCopy,
  queueWhenSessionBusy: boolean,
  sessionBusy: boolean,
): string {
  const shouldQueue = [queueWhenSessionBusy, sessionBusy].every(Boolean);
  return shouldQueue ? copy.enterQueue : copy.enterSend;
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
  compact,
  goalCreateBlockedReason,
  input,
  inputState,
  isGoalCreating,
  isGoalMode,
  isPreparingAttachments,
  runtimeState,
}: {
  canCreateGoal: boolean;
  compact: boolean;
  goalCreateBlockedReason: string | null;
  input: string;
  inputState: ComposerInputProjection;
  isGoalCreating: boolean;
  isGoalMode: boolean;
  isPreparingAttachments: boolean;
  runtimeState: ComposerRuntimeProjection;
}): ComposerActionProjection {
  const goalSendDisabled = [
    !inputState.hasTextInput,
    inputState.isOverLimit,
    isGoalCreating,
    !canCreateGoal,
    Boolean(goalCreateBlockedReason),
  ].some(Boolean);
  const messageSendDisabled = [
    inputState.isInputEmpty,
    inputState.isOverLimit,
    isPreparingAttachments,
  ].some(Boolean);
  return {
    isSendDisabled: isGoalMode ? goalSendDisabled : messageSendDisabled,
    isTextareaLocked: [isGoalMode, isGoalCreating].every(Boolean),
    shouldShowInlineShortcuts: [!compact, input.length === 0].every(Boolean),
    shouldShowStopButton: [
      !isGoalMode,
      runtimeState.canStopGeneration,
      inputState.isInputEmpty,
    ].every(Boolean),
  };
}
