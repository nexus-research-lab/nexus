import type { AgentConversationRuntimePhase } from "@/types/agent/agent-conversation";

import {
  type ComposerInputMode,
  MAX_COMPOSER_INPUT_LENGTH,
  getComposerInputRowPaddingClass,
} from "../composer-model";

interface ComposerViewCopy {
  defaultPlaceholder: string;
  enterQueue: string;
  enterSend: string;
  goalConfirm: string;
  goalEnterStart: string;
  goalPlaceholder: string;
  sendMessage: string;
}

interface ComposerViewStateOptions {
  attachmentCount: number;
  attachmentError: string | null;
  canCreateGoal: boolean;
  canUseLoop: boolean;
  compact: boolean;
  copy: ComposerViewCopy;
  goalCreateBlockedReason: string | null;
  goalError: string | null;
  historyIndex: number;
  historyItemCount: number;
  input: string;
  inputMode: ComposerInputMode;
  isActionMenuOpen: boolean;
  isGoalCreating: boolean;
  isLoading: boolean;
  isLoopPickerOpen: boolean;
  isPreparingAttachments: boolean;
  hasStopHandler: boolean;
  queueItemCount: number;
  queueWhenSessionBusy: boolean;
  runtimePhase: AgentConversationRuntimePhase | null;
}

export function buildComposerViewState({
  attachmentCount,
  attachmentError,
  canCreateGoal,
  canUseLoop,
  compact,
  copy,
  goalCreateBlockedReason,
  goalError,
  historyIndex,
  historyItemCount,
  input,
  inputMode,
  isActionMenuOpen,
  isGoalCreating,
  isLoading,
  isLoopPickerOpen,
  isPreparingAttachments,
  hasStopHandler,
  queueItemCount,
  queueWhenSessionBusy,
  runtimePhase,
}: ComposerViewStateOptions) {
  const isGoalMode = inputMode === "goal";
  const hasTextInput = input.trim().length > 0;
  const isInputEmpty = !hasTextInput && attachmentCount === 0;
  const charCount = input.length;
  const isNearLimit = charCount > MAX_COMPOSER_INPUT_LENGTH * 0.8;
  const isOverLimit = charCount > MAX_COMPOSER_INPUT_LENGTH;
  const isDispatching = isLoading && runtimePhase === "sending";
  const canStopGeneration = isLoading && !isDispatching && hasStopHandler;
  const sessionBusy = isLoading || queueItemCount > 0;
  const modeCopy = isGoalMode
    ? {
        enterLabel: copy.goalEnterStart,
        placeholder: copy.goalPlaceholder,
        sendButtonLabel: copy.goalConfirm,
      }
    : {
        enterLabel: queueWhenSessionBusy && sessionBusy
          ? copy.enterQueue
          : copy.enterSend,
        placeholder: copy.defaultPlaceholder,
        sendButtonLabel: copy.sendMessage,
      };

  return {
    activeError: isGoalMode
      ? goalError ?? goalCreateBlockedReason
      : attachmentError,
    canCreateGoal,
    canStopGeneration,
    canUseLoop,
    charCount,
    composerInputRowPaddingClass: getComposerInputRowPaddingClass(
      compact,
      queueItemCount > 0,
      isGoalMode,
    ),
    historyIndex,
    input,
    inputHistoryLength: historyItemCount,
    inlineEnterLabel: modeCopy.enterLabel,
    isActionMenuOpen,
    isDispatching,
    isGoalCreating,
    isGoalMode,
    isLoopPickerOpen,
    isNearLimit,
    isOverLimit,
    isPreparingAttachments,
    isSendDisabled: isGoalMode
      ? !hasTextInput
        || isOverLimit
        || isGoalCreating
        || !canCreateGoal
        || Boolean(goalCreateBlockedReason)
      : isInputEmpty || isOverLimit || isPreparingAttachments,
    isTextareaLocked: isGoalMode && isGoalCreating,
    resolvedPlaceholder: modeCopy.placeholder,
    sendButtonLabel: modeCopy.sendButtonLabel,
    shouldShowInlineShortcuts: !compact && input.length === 0,
    shouldShowStopButton: !isGoalMode && canStopGeneration && isInputEmpty,
  };
}
