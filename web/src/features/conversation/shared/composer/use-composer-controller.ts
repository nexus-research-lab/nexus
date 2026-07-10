import {
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";
import type { KeyboardEvent } from "react";

import { useTextareaHeight } from "@/hooks/ui/use-textarea-height";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { LoopCatalogItem } from "@/types/capability/loop";

import { useComposerAttachments } from "./attachments/use-composer-attachments";
import {
  COMPOSITION_END_ENTER_GUARD_MS,
  MENTION_NAVIGATION_KEYS,
  type ComposerInputMode,
  type ComposerNativeKeyboardEvent,
  type ComposerPanelProps,
  getComposerInputRowPaddingClass,
  isCaretOnFirstLine,
  isCaretOnLastLine,
  isImeKeyboardEvent,
  resolveComposerDelivery,
} from "./composer-model";
import { useComposerHistory } from "./use-composer-history";
import { useComposerMention } from "./use-composer-mention";

const EMPTY_INPUT_QUEUE_ITEMS: NonNullable<
  ComposerPanelProps["inputQueueItems"]
> = [];
const EMPTY_ROOM_MEMBERS: NonNullable<ComposerPanelProps["roomMembers"]> = [];
const EMPTY_UNAVAILABLE_AGENT_IDS: NonNullable<
  ComposerPanelProps["mentionUnavailableAgentIds"]
> = [];

export function useComposerController({
  allowSendWhileLoading = false,
  compact,
  defaultDeliveryPolicy = "queue",
  disabled = false,
  enableLoops = false,
  goalCreateDisabledReason = null,
  initialDraft = null,
  inputQueueItems = EMPTY_INPUT_QUEUE_ITEMS,
  isLoading = false,
  maxLength = 10000,
  mentionUnavailableAgentIds = EMPTY_UNAVAILABLE_AGENT_IDS,
  onCreateGoal,
  onCreateLoopGoal,
  onEnqueueMessage,
  onPrepareAttachments,
  onSendMessage,
  onStop,
  placeholder,
  queueWhenSessionBusy = true,
  roomMembers = EMPTY_ROOM_MEMBERS,
  runtimePhase = null,
}: ComposerPanelProps) {
  const { t } = useI18n();
  const [inputMode, setInputMode] = useState<ComposerInputMode>("message");
  const [input, setInput] = useState("");
  const [isActionMenuOpen, setIsActionMenuOpen] = useState(false);
  const [isLoopPickerOpen, setIsLoopPickerOpen] = useState(false);
  const [isGoalCreating, setIsGoalCreating] = useState(false);
  const [goalError, setGoalError] = useState<string | null>(null);
  const isComposingRef = useRef(false);
  const ignoreNextEnterAfterCompositionRef = useRef(false);
  const lastCompositionEndAtRef = useRef(0);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const actionButtonRef = useRef<HTMLButtonElement>(null);
  const isGoalMode = inputMode === "goal";
  const {
    attachmentError,
    attachments,
    clearAttachmentError,
    clearAttachments,
    handleFileSelect,
    handlePaste,
    isPreparingAttachments,
    prepareAttachments,
    removeAttachment,
  } = useComposerAttachments({
    isGoalMode,
    onGoalAttachmentRejected: setGoalError,
    onPrepareAttachments,
  });
  const {
    closeMention,
    mentionActive,
    mentionFilter,
    mentionTargetItems,
    selectMentionItem,
    updateMentionForInput,
  } = useComposerMention({
    input,
    isGoalMode,
    mentionUnavailableAgentIds,
    roomMembers,
    setInput,
    textareaRef,
  });
  const {
    index: historyIndex,
    itemCount: inputHistoryLength,
    recallNext,
    recallPrevious,
    record: recordHistory,
  } = useComposerHistory({
    clearError: clearAttachmentError,
    input,
    setInput,
  });
  const isDispatching = isLoading && runtimePhase === "sending";
  const isInputLocked = disabled || (!allowSendWhileLoading && isLoading);
  const isTextareaLocked = isInputLocked || (isGoalMode && isGoalCreating);
  const canStopGeneration = isLoading && !isDispatching && Boolean(onStop);
  const canCreateGoal = Boolean(onCreateGoal);
  const canUseLoop = enableLoops && (Boolean(onCreateLoopGoal) || canCreateGoal);
  const goalCreateBlockedReason = goalCreateDisabledReason?.trim() || null;

  useTextareaHeight(textareaRef, input, {
    minHeight: 24,
    maxHeight: 200,
    lineHeight: 24,
    paddingY: 0,
  });

  const focusTextarea = useCallback(() => {
    requestAnimationFrame(() => textareaRef.current?.focus());
  }, []);

  const handleInputChange = useCallback((value: string) => {
    setInput(value);
    if (attachmentError) {
      clearAttachmentError();
    }
    if (goalError) {
      setGoalError(null);
    }
    updateMentionForInput(value);
  }, [
    attachmentError,
    clearAttachmentError,
    goalError,
    updateMentionForInput,
  ]);

  useEffect(() => {
    if (textareaRef.current && !isInputLocked) {
      textareaRef.current.focus();
    }
  }, [isInputLocked]);

  useEffect(() => {
    const normalizedDraft = initialDraft?.trim() ?? "";
    if (normalizedDraft) {
      setInput((current) => current || normalizedDraft);
    }
  }, [initialDraft]);

  const submitGoal = useCallback(async () => {
    const objective = input.trim();
    if (
      !objective
      || isInputLocked
      || isGoalCreating
      || !onCreateGoal
      || goalCreateBlockedReason
    ) {
      return;
    }
    setIsGoalCreating(true);
    setGoalError(null);
    try {
      await onCreateGoal(objective);
      setInput("");
      setInputMode("message");
    } catch (error) {
      setGoalError(
        error instanceof Error
          ? error.message
          : t("composer.goal_create_failed"),
      );
    } finally {
      setIsGoalCreating(false);
    }
  }, [
    goalCreateBlockedReason,
    input,
    isGoalCreating,
    isInputLocked,
    onCreateGoal,
    t,
  ]);

  const submitMessage = useCallback(async () => {
    const content = input.trim();
    if (
      (!content && attachments.length === 0)
      || isInputLocked
      || isPreparingAttachments
    ) {
      return;
    }
    const preparedAttachments = await prepareAttachments();
    if (!preparedAttachments) {
      return;
    }
    const busy = isLoading || inputQueueItems.length > 0;
    const delivery = resolveComposerDelivery(
      busy,
      queueWhenSessionBusy,
      defaultDeliveryPolicy,
    );
    const deliver = delivery.handler === "enqueue"
      ? onEnqueueMessage
      : onSendMessage;
    if (!deliver) {
      return;
    }
    try {
      await deliver(content, delivery.policy, preparedAttachments);
      recordHistory(content);
      setInput("");
      clearAttachments();
      clearAttachmentError();
      if (textareaRef.current) {
        textareaRef.current.style.height = "auto";
      }
    } catch (error) {
      console.error("发送消息失败:", error);
    }
  }, [
    attachments.length,
    clearAttachmentError,
    clearAttachments,
    defaultDeliveryPolicy,
    input,
    inputQueueItems.length,
    isInputLocked,
    isLoading,
    isPreparingAttachments,
    onEnqueueMessage,
    onSendMessage,
    prepareAttachments,
    queueWhenSessionBusy,
    recordHistory,
  ]);

  const handleSend = useCallback(async () => {
    if (isGoalMode) {
      await submitGoal();
      return;
    }
    await submitMessage();
  }, [isGoalMode, submitGoal, submitMessage]);

  const openAttachmentPicker = useCallback(() => {
    setIsActionMenuOpen(false);
    fileInputRef.current?.click();
  }, []);

  const startGoalInput = useCallback(() => {
    if (!canCreateGoal) {
      return;
    }
    setIsActionMenuOpen(false);
    setInputMode("goal");
    setGoalError(null);
    closeMention();
    focusTextarea();
  }, [canCreateGoal, closeMention, focusTextarea]);

  const cancelGoalInput = useCallback(() => {
    setInputMode("message");
    setGoalError(null);
    focusTextarea();
  }, [focusTextarea]);

  const toggleGoalInput = useCallback((checked: boolean) => {
    if (checked) {
      startGoalInput();
      return;
    }
    setIsActionMenuOpen(false);
    cancelGoalInput();
  }, [cancelGoalInput, startGoalInput]);

  const openLoopPicker = useCallback(() => {
    if (canUseLoop) {
      setIsActionMenuOpen(false);
      setIsLoopPickerOpen(true);
    }
  }, [canUseLoop]);

  const applyLoopPrompt = useCallback((loop: LoopCatalogItem) => {
    setInputMode("message");
    setGoalError(null);
    setInput(loop.kickoff_prompt);
    closeMention();
    focusTextarea();
  }, [closeMention, focusTextarea]);

  const applyLoopGoal = useCallback((loop: LoopCatalogItem) => {
    if (!canCreateGoal) {
      applyLoopPrompt(loop);
      return;
    }
    setInputMode("goal");
    setGoalError(null);
    setInput(loop.kickoff_prompt);
    closeMention();
    focusTextarea();
  }, [applyLoopPrompt, canCreateGoal, closeMention, focusTextarea]);

  const handleLoopSelect = useCallback(async (loop: LoopCatalogItem) => {
    if (!onCreateLoopGoal) {
      applyLoopGoal(loop);
      return;
    }
    setGoalError(null);
    closeMention();
    await onCreateLoopGoal(loop);
    setInputMode("message");
    setInput("");
  }, [applyLoopGoal, closeMention, onCreateLoopGoal]);

  const handleCompositionStart = useCallback(() => {
    isComposingRef.current = true;
    ignoreNextEnterAfterCompositionRef.current = false;
  }, []);

  const handleCompositionEnd = useCallback((timeStamp: number) => {
    isComposingRef.current = false;
    ignoreNextEnterAfterCompositionRef.current = true;
    lastCompositionEndAtRef.current = timeStamp;
  }, []);

  const handleKeyDown = useCallback((
    event: KeyboardEvent<HTMLTextAreaElement>,
  ) => {
    const nativeEvent = event.nativeEvent as ComposerNativeKeyboardEvent;
    const justFinishedComposition = lastCompositionEndAtRef.current > 0
      && event.timeStamp - lastCompositionEndAtRef.current
        <= COMPOSITION_END_ENTER_GUARD_MS;

    // Safari 可能在中文候选词确认后补发一个不带 composing 标记的 Enter。
    if (isComposingRef.current || isImeKeyboardEvent(nativeEvent)) {
      return;
    }
    if (ignoreNextEnterAfterCompositionRef.current && event.key !== "Enter") {
      ignoreNextEnterAfterCompositionRef.current = false;
    }
    if (mentionActive && MENTION_NAVIGATION_KEYS.has(event.key)) {
      return;
    }
    if (event.key === "Enter" && !event.shiftKey) {
      if (ignoreNextEnterAfterCompositionRef.current && justFinishedComposition) {
        ignoreNextEnterAfterCompositionRef.current = false;
        return;
      }
      event.preventDefault();
      void handleSend();
      return;
    }
    if (
      event.key === "ArrowUp"
      && inputHistoryLength > 0
      && (event.ctrlKey || isCaretOnFirstLine(event.currentTarget))
    ) {
      event.preventDefault();
      recallPrevious();
      return;
    }
    if (
      event.key === "ArrowDown"
      && historyIndex >= 0
      && (event.ctrlKey || isCaretOnLastLine(event.currentTarget))
    ) {
      event.preventDefault();
      recallNext();
      return;
    }
    if (event.key === "Escape" && isLoading && onStop) {
      event.preventDefault();
      onStop();
    }
  }, [
    handleSend,
    historyIndex,
    inputHistoryLength,
    isLoading,
    mentionActive,
    onStop,
    recallNext,
    recallPrevious,
  ]);

  const hasTextInput = input.trim().length > 0;
  const isInputEmpty = !hasTextInput && attachments.length === 0;
  const charCount = input.length;
  const isNearLimit = charCount > maxLength * 0.8;
  const isOverLimit = charCount > maxLength;
  const isSendDisabled = isGoalMode
    ? !hasTextInput
      || isInputLocked
      || isOverLimit
      || isGoalCreating
      || !onCreateGoal
      || Boolean(goalCreateBlockedReason)
    : isInputEmpty
      || isInputLocked
      || isOverLimit
      || isPreparingAttachments;
  const shouldShowStopButton = !isGoalMode
    && canStopGeneration
    && (!allowSendWhileLoading || isInputEmpty);
  const sessionBusy = isLoading || inputQueueItems.length > 0;
  const inlineEnterLabel = isGoalMode
    ? t("composer.goal_enter_start")
    : queueWhenSessionBusy && sessionBusy
      ? t("composer.enter_queue")
      : t("composer.enter_send");

  return {
    refs: {
      actionButtonRef,
      fileInputRef,
      textareaRef,
    },
    state: {
      activeError: isGoalMode
        ? goalError ?? goalCreateBlockedReason
        : attachmentError,
      canCreateGoal,
      canStopGeneration,
      canUseLoop,
      charCount,
      composerInputRowPaddingClass: getComposerInputRowPaddingClass(
        compact,
        inputQueueItems.length > 0,
        isGoalMode,
      ),
      historyIndex,
      input,
      inputHistoryLength,
      inlineEnterLabel,
      isActionMenuOpen,
      isDispatching,
      isGoalCreating,
      isGoalMode,
      isInputLocked,
      isLoopPickerOpen,
      isNearLimit,
      isOverLimit,
      isPreparingAttachments,
      isSendDisabled,
      isTextareaLocked,
      resolvedPlaceholder: isGoalMode
        ? t("composer.goal_placeholder")
        : placeholder ?? t("composer.default_placeholder"),
      sendButtonLabel: isGoalMode
        ? t("composer.goal_confirm")
        : t("composer.send_message"),
      shouldShowInlineShortcuts: !compact && input.length === 0,
      shouldShowStopButton,
    },
    attachments: {
      attachments,
      handleFileSelect,
      handlePaste,
      removeAttachment,
    },
    mention: {
      closeMention,
      mentionActive,
      mentionFilter,
      mentionTargetItems,
      selectMentionItem,
    },
    actions: {
      cancelGoalInput,
      handleCompositionEnd,
      handleCompositionStart,
      handleInputChange,
      handleKeyDown,
      handleLoopSelect,
      handleSend,
      openAttachmentPicker,
      openLoopPicker,
      setIsActionMenuOpen,
      setIsLoopPickerOpen,
      toggleGoalInput,
    },
  };
}
