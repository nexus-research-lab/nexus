import type { AgentConversationRuntimePhase } from "@/types/agent/agent-conversation";

import {
  type ComposerInputMode,
  getComposerInputRowPaddingClass,
} from "../composer-model";
import {
  type ComposerViewCopy,
  projectComposerActions,
  projectComposerInput,
  projectComposerMode,
  projectComposerRuntime,
} from "./composer-view-projections";

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
  queueItemCount: number;
  queueWhenSessionBusy: boolean;
  runtimePhase: AgentConversationRuntimePhase | null;
}

export function buildComposerViewState(
  options: ComposerViewStateOptions,
) {
  const inputState = projectComposerInput(
    options.input,
    options.attachmentCount,
  );
  const runtimeState = projectComposerRuntime({
    isLoading: options.isLoading,
    queueItemCount: options.queueItemCount,
    runtimePhase: options.runtimePhase,
  });
  const modeState = projectComposerMode({
    attachmentError: options.attachmentError,
    copy: options.copy,
    goalCreateBlockedReason: options.goalCreateBlockedReason,
    goalError: options.goalError,
    inputMode: options.inputMode,
    queueWhenSessionBusy: options.queueWhenSessionBusy,
    sessionBusy: runtimeState.sessionBusy,
  });
  const actionState = projectComposerActions({
    canCreateGoal: options.canCreateGoal,
    compact: options.compact,
    goalCreateBlockedReason: options.goalCreateBlockedReason,
    input: options.input,
    inputState,
    isGoalCreating: options.isGoalCreating,
    isGoalMode: modeState.isGoalMode,
    isPreparingAttachments: options.isPreparingAttachments,
    runtimeState,
  });

  return {
    activeError: modeState.activeError,
    canCreateGoal: options.canCreateGoal,
    canStopGeneration: runtimeState.canStopGeneration,
    canUseLoop: options.canUseLoop,
    charCount: inputState.charCount,
    composerInputRowPaddingClass: getComposerInputRowPaddingClass(
      options.compact,
      options.queueItemCount > 0,
      modeState.isGoalMode,
    ),
    historyIndex: options.historyIndex,
    input: options.input,
    inputHistoryLength: options.historyItemCount,
    inlineEnterLabel: modeState.enterLabel,
    isActionMenuOpen: options.isActionMenuOpen,
    isDispatching: runtimeState.isDispatching,
    isGoalCreating: options.isGoalCreating,
    isGoalMode: modeState.isGoalMode,
    isLoopPickerOpen: options.isLoopPickerOpen,
    isNearLimit: inputState.isNearLimit,
    isOverLimit: inputState.isOverLimit,
    isPreparingAttachments: options.isPreparingAttachments,
    isSendDisabled: actionState.isSendDisabled,
    isTextareaLocked: actionState.isTextareaLocked,
    resolvedPlaceholder: modeState.placeholder,
    sendButtonLabel: modeState.sendButtonLabel,
    shouldShowInlineShortcuts: actionState.shouldShowInlineShortcuts,
    shouldShowStopButton: actionState.shouldShowStopButton,
  };
}
