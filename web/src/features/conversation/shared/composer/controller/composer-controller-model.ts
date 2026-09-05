// INPUT: Composer input, capabilities, local UI state and runtime facts.
// OUTPUT: Semantic view state and action eligibility without CSS or layout tokens.
// POS: Composer controller projection; the panel resolves presentation from these facts.

import type { AgentConversationRuntimePhase } from "@/types/agent/agent-conversation";

import type { ComposerInputMode } from "../composer-model";
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
  copy: ComposerViewCopy;
  goalCreateBlockedReason: string | null;
  goalError: string | null;
  hasStopAction: boolean;
  historyIndex: number;
  historyItemCount: number;
  input: string;
  inputMode: ComposerInputMode;
  isActionMenuOpen: boolean;
  isGoalConfirming: boolean;
  isGoalCreating: boolean;
  isLoading: boolean;
  isLoopPickerOpen: boolean;
  isPreparingAttachments: boolean;
  isSessionSettingsSaving: boolean;
  queueItemCount: number;
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
  });
  const actionState = projectComposerActions({
    canCreateGoal: options.canCreateGoal,
    goalCreateBlockedReason: options.goalCreateBlockedReason,
    inputState,
    isGoalCreating: options.isGoalCreating,
    isGoalMode: modeState.isGoalMode,
    isPreparingAttachments: options.isPreparingAttachments,
    isSessionSettingsSaving: options.isSessionSettingsSaving,
    hasStopAction: options.hasStopAction,
    runtimeState,
  });

  return {
    activeError: modeState.activeError,
    canCreateGoal: options.canCreateGoal,
    canUseLoop: options.canUseLoop,
    charCount: inputState.charCount,
    historyIndex: options.historyIndex,
    input: options.input,
    inputHistoryLength: options.historyItemCount,
    isActionMenuOpen: options.isActionMenuOpen,
    isGoalConfirming: options.isGoalConfirming,
    isGoalCreating: options.isGoalCreating,
    isGoalMode: modeState.isGoalMode,
    isLoopPickerOpen: options.isLoopPickerOpen,
    isNearLimit: inputState.isNearLimit,
    isOverLimit: inputState.isOverLimit,
    isPreparingAttachments: options.isPreparingAttachments,
    isSendDisabled: actionState.isSendDisabled,
    isTextareaLocked: actionState.isTextareaLocked,
    resolvedPlaceholder: modeState.placeholder,
    runtimeActivity: runtimeState.activity,
    sendButtonLabel: modeState.sendButtonLabel,
    shouldShowStopButton: actionState.shouldShowStopButton,
  };
}
