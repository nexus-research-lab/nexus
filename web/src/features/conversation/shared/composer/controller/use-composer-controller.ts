import { useCallback, useEffect, useRef } from "react";

import { useTextareaHeight } from "@/hooks/ui/use-textarea-height";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { Agent } from "@/types/agent/agent";

import { useComposerAttachments } from "../attachments/use-composer-attachments";
import type { ComposerPanelProps } from "../composer-model";
import { useComposerHistory } from "../use-composer-history";
import { useComposerMention } from "../use-composer-mention";
import { buildComposerViewState } from "./composer-controller-model";
import { useComposerDraft } from "./use-composer-draft";
import { useComposerGoalActions } from "./use-composer-goal-actions";
import { useComposerKeyboard } from "./use-composer-keyboard";
import { useComposerMessageSubmit } from "./use-composer-message-submit";

const EMPTY_ROOM_MEMBERS: Agent[] = [];

export function useComposerController({
  compact,
  defaultDeliveryPolicy,
  enableLoops = false,
  goalCreateDisabledReason = null,
  inputQueueItems,
  isLoading,
  onCreateGoal,
  onCreateLoopGoal,
  onEnqueueMessage,
  onPrepareAttachments,
  onSendMessage,
  onStop,
  queueWhenSessionBusy = true,
  roomMembers = EMPTY_ROOM_MEMBERS,
  runtimePhase,
}: ComposerPanelProps) {
  const { t } = useI18n();
  const draft = useComposerDraft();
  const {
    setActionMenuOpen,
    setGoalError,
    setInput,
    setLoopPickerOpen,
    state: draftState,
  } = draft;
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const actionButtonRef = useRef<HTMLButtonElement>(null);
  const isGoalMode = draftState.inputMode === "goal";
  const attachments = useComposerAttachments({
    isGoalMode,
    onGoalAttachmentRejected: setGoalError,
    onPrepareAttachments,
  });
  const {
    attachmentError,
    clearAttachmentError,
  } = attachments;
  const mention = useComposerMention({
    input: draftState.input,
    isGoalMode,
    roomMembers,
    setInput,
    textareaRef,
  });
  const { updateMentionForInput } = mention;
  const history = useComposerHistory({
    clearError: clearAttachmentError,
    input: draftState.input,
    setInput,
  });

  useTextareaHeight(textareaRef, draftState.input, {
    minHeight: 24,
    maxHeight: 200,
    lineHeight: 24,
    paddingY: 0,
  });

  const focusTextarea = useCallback(() => {
    requestAnimationFrame(() => textareaRef.current?.focus());
  }, []);
  useEffect(() => {
    textareaRef.current?.focus();
  }, []);

  const resetTextareaHeight = useCallback(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = "auto";
    }
  }, []);
  const resetInput = useCallback(() => setInput(""), [setInput]);
  const submitMessage = useComposerMessageSubmit({
    attachmentCount: attachments.attachments.length,
    clearAttachmentError,
    clearAttachments: attachments.clearAttachments,
    defaultDeliveryPolicy,
    input: draftState.input,
    isLoading,
    isPreparingAttachments: attachments.isPreparingAttachments,
    onEnqueueMessage,
    onSendMessage,
    prepareAttachments: attachments.prepareAttachments,
    queueItemCount: inputQueueItems.length,
    queueWhenSessionBusy,
    recordHistory: history.record,
    resetInput,
    resetTextareaHeight,
    runtimePhase,
    targetAgentIDs: mention.selectedTargetIDs,
    clearSelectedTargetIDs: mention.clearSelectedTargetIDs,
  });
  const goal = useComposerGoalActions({
    closeMention: mention.closeMention,
    draft,
    enableLoops,
    fallbackErrorMessage: t("composer.goal_create_failed"),
    focusTextarea,
    goalCreateDisabledReason,
    onCreateGoal,
    onCreateLoopGoal,
  });
  const { submitGoal } = goal;
  const handleSend = useCallback(async () => {
    if (isGoalMode) {
      await submitGoal();
    } else {
      await submitMessage();
    }
  }, [isGoalMode, submitGoal, submitMessage]);
  const keyboard = useComposerKeyboard({
    historyIndex: history.index,
    historyItemCount: history.itemCount,
    isLoading,
    mentionActive: mention.mentionActive,
    onSend: handleSend,
    onStop,
    recallNext: history.recallNext,
    recallPrevious: history.recallPrevious,
  });

  const handleInputChange = useCallback((value: string) => {
    setInput(value);
    if (attachmentError) {
      clearAttachmentError();
    }
    if (draftState.goalError) {
      setGoalError(null);
    }
    updateMentionForInput(value);
  }, [
    attachmentError,
    clearAttachmentError,
    draftState.goalError,
    setGoalError,
    setInput,
    updateMentionForInput,
  ]);
  const openAttachmentPicker = useCallback(() => {
    setActionMenuOpen(false);
    fileInputRef.current?.click();
  }, [setActionMenuOpen]);

  const state = buildComposerViewState({
    attachmentCount: attachments.attachments.length,
    attachmentError,
    canCreateGoal: goal.canCreateGoal,
    canUseLoop: goal.canUseLoop,
    compact,
    copy: {
      defaultPlaceholder: t("composer.default_placeholder"),
      enterQueue: t("composer.enter_queue"),
      enterSend: t("composer.enter_send"),
      goalConfirm: t("composer.goal_confirm"),
      goalEnterStart: t("composer.goal_enter_start"),
      goalPlaceholder: t("composer.goal_placeholder"),
      sendMessage: t("composer.send_message"),
    },
    goalCreateBlockedReason: goal.blockedReason,
    goalError: draftState.goalError,
    historyIndex: history.index,
    historyItemCount: history.itemCount,
    input: draftState.input,
    inputMode: draftState.inputMode,
    isActionMenuOpen: draftState.isActionMenuOpen,
    isGoalCreating: draftState.isGoalCreating,
    isLoading,
    isLoopPickerOpen: draftState.isLoopPickerOpen,
    isPreparingAttachments: attachments.isPreparingAttachments,
    queueItemCount: inputQueueItems.length,
    queueWhenSessionBusy,
    runtimePhase,
  });

  return {
    refs: { actionButtonRef, fileInputRef, textareaRef },
    state,
    attachments: {
      attachments: attachments.attachments,
      handleFileSelect: attachments.handleFileSelect,
      handlePaste: attachments.handlePaste,
      removeAttachment: attachments.removeAttachment,
    },
    mention: {
      closeMention: mention.closeMention,
      mentionActive: mention.mentionActive,
      mentionFilter: mention.mentionFilter,
      mentionTargetItems: mention.mentionTargetItems,
      selectMentionItem: mention.selectMentionItem,
    },
    actions: {
      cancelGoalInput: goal.cancelGoalInput,
      handleCompositionEnd: keyboard.handleCompositionEnd,
      handleCompositionStart: keyboard.handleCompositionStart,
      handleInputChange,
      handleKeyDown: keyboard.handleKeyDown,
      handleLoopSelect: goal.handleLoopSelect,
      handleSend,
      openAttachmentPicker,
      openLoopPicker: goal.openLoopPicker,
      setIsActionMenuOpen: setActionMenuOpen,
      setIsLoopPickerOpen: setLoopPickerOpen,
      toggleGoalInput: goal.toggleGoalInput,
    },
  };
}
