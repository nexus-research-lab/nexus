"use client";

import {
  KeyboardEvent,
  memo,
  type ReactNode,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";
import {
  Send,
  StopCircle,
  Target,
} from "lucide-react";

import { useTextareaHeight } from "@/hooks/ui/use-textarea-height";
import { cn } from "@/lib/utils";
import { LoadingOrb } from "@/shared/ui/feedback/loading-orb";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  AgentConversationDefaultDeliveryPolicy,
  AgentConversationDeliveryPolicy,
  AgentConversationRuntimePhase,
  InputQueueItem,
} from "@/types/agent/agent-conversation";
import { Agent } from "@/types/agent/agent";

import {
  COMPOSER_DANGER_ACTION_BUTTON_CLASS_NAME,
  COMPOSER_PRIMARY_ACTION_BUTTON_CLASS_NAME,
  getComposerShellClassName,
  getComposerShellStyle,
} from "./composer-styles";
import {
  COMPOSER_ATTACHMENT_ACCEPT,
  PreparedComposerAttachment,
} from "./composer-attachments";
import {
  ComposerAttachmentList,
} from "./composer-local-attachments";
import { ComposerFooter } from "./composer-footer";
import { ComposerPendingQueue } from "./composer-pending-queue";
import { MentionTargetPopover } from "./mention-popover";
import { LoopPickerDialog } from "./loop-picker-dialog";
import { useComposerAttachments } from "./use-composer-attachments";
import { useComposerMention } from "./use-composer-mention";
import type { LoopCatalogItem } from "@/types/capability/loop";

interface ComposerPanelProps {
  compact: boolean;
  isLoading?: boolean;
  runtimePhase?: AgentConversationRuntimePhase | null;
  onSendMessage: (
    content: string,
    deliveryPolicy: AgentConversationDeliveryPolicy,
    attachments?: PreparedComposerAttachment[],
  ) => void | Promise<void>;
  inputQueueItems?: InputQueueItem[];
  onEnqueueMessage?: (
    content: string,
    deliveryPolicy: AgentConversationDeliveryPolicy,
    attachments?: PreparedComposerAttachment[],
  ) => void | Promise<void>;
  onDeleteQueuedMessage?: (itemId: string) => void | Promise<void>;
  onGuideQueuedMessage?: (itemId: string) => void | Promise<void>;
  onReorderQueueMessages?: (orderedIds: string[]) => void | Promise<void>;
  onStop?: () => void;
  defaultDeliveryPolicy?: AgentConversationDefaultDeliveryPolicy;
  initialDraft?: string | null;
  disabled?: boolean;
  allowSendWhileLoading?: boolean;
  queueWhenSessionBusy?: boolean;
  placeholder?: string;
  maxLength?: number;
  roomMembers?: Agent[];
  mentionUnavailableAgentIds?: string[];
  onPrepareAttachments?: (files: File[]) => Promise<PreparedComposerAttachment[]>;
  onCreateGoal?: (objective: string) => Promise<void>;
  enableLoops?: boolean;
  onCreateLoopGoal?: (loop: LoopCatalogItem) => Promise<void>;
  goalCreateDisabledReason?: string | null;
  goalModeExtra?: ReactNode;
  goalScopeLabel?: string;
  tourAnchor?: string;
}

type ComposerNativeKeyboardEvent = globalThis.KeyboardEvent & {
  keyCode?: number;
  which?: number;
};

const IME_COMPOSITION_KEY_CODE = 229;
const COMPOSITION_END_ENTER_GUARD_MS = 80;
type ComposerInputMode = "message" | "goal";
function isCaretOnFirstLine(target: HTMLTextAreaElement) {
  const selectionStart = target.selectionStart ?? 0;
  const selectionEnd = target.selectionEnd ?? 0;
  if (selectionStart !== selectionEnd) {
    return false;
  }
  return !target.value.slice(0, selectionStart).includes("\n");
}

function isCaretOnLastLine(target: HTMLTextAreaElement) {
  const selectionStart = target.selectionStart ?? 0;
  const selectionEnd = target.selectionEnd ?? 0;
  if (selectionStart !== selectionEnd) {
    return false;
  }
  return !target.value.slice(selectionEnd).includes("\n");
}

const ComposerPanelView = memo(({
  compact,
  isLoading: isLoading = false,
  runtimePhase: runtimePhase = null,
  onSendMessage: onSendMessage,
  inputQueueItems: inputQueueItems = [],
  onEnqueueMessage: onEnqueueMessage,
  onDeleteQueuedMessage: onDeleteQueuedMessage,
  onGuideQueuedMessage: onGuideQueuedMessage,
  onReorderQueueMessages: onReorderQueueMessages,
  onStop: onStop,
  defaultDeliveryPolicy: defaultDeliveryPolicy = "queue",
  initialDraft: initialDraft = null,
  disabled = false,
  allowSendWhileLoading: allowSendWhileLoading = false,
  queueWhenSessionBusy: queueWhenSessionBusy = true,
  placeholder,
  maxLength: maxLength = 10000,
  roomMembers: roomMembers = [],
  mentionUnavailableAgentIds: mentionUnavailableAgentIds = [],
  onPrepareAttachments: onPrepareAttachments,
  onCreateGoal: onCreateGoal,
  enableLoops: enableLoops = false,
  onCreateLoopGoal: onCreateLoopGoal,
  goalCreateDisabledReason: goalCreateDisabledReason = null,
  goalModeExtra: goalModeExtra = null,
  goalScopeLabel: goalScopeLabel = "会话 Goal",
  tourAnchor: tourAnchor,
}: ComposerPanelProps) => {
  const { t } = useI18n();
  const [inputMode, setInputMode] = useState<ComposerInputMode>("message");
  const isGoalMode = inputMode === "goal";
  const resolvedPlaceholder = isGoalMode
    ? t("composer.goal_placeholder")
    : placeholder ?? t("composer.default_placeholder");
  const [input, setInput] = useState("");
  const [inputHistory, setInputHistory] = useState<string[]>([]);
  const [historyIndex, setHistoryIndex] = useState(-1);
  const [historyDraft, setHistoryDraft] = useState("");
  const [isActionMenuOpen, setIsActionMenuOpen] = useState(false);
  const [isLoopPickerOpen, setIsLoopPickerOpen] = useState(false);
  const [isGoalCreating, setIsGoalCreating] = useState(false);
  const [goalError, setGoalError] = useState<string | null>(null);
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

  const isComposingRef = useRef(false);
  const ignoreNextEnterAfterCompositionRef = useRef(false);
  const lastCompositionEndAtRef = useRef(0);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const actionButtonRef = useRef<HTMLButtonElement>(null);
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
  const isDispatching = isLoading && runtimePhase === "sending";
  const isInputLocked = disabled || (!allowSendWhileLoading && isLoading);
  const isTextareaLocked = isInputLocked || (isGoalMode && isGoalCreating);
  const canStopGeneration = isLoading && !isDispatching && Boolean(onStop);
  const canCreateGoal = Boolean(onCreateGoal);
  const canUseLoop = enableLoops && (Boolean(onCreateLoopGoal) || canCreateGoal);
  const goalCreateBlockedReason =
    goalCreateDisabledReason?.trim() || null;

  useTextareaHeight(textareaRef, input, { minHeight: 24, maxHeight: 200, lineHeight: 24, paddingY: 0 });

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
    if (!normalizedDraft) {
      return;
    }
    setInput((currentValue) => currentValue || normalizedDraft);
  }, [initialDraft]);

  const dispatchMessage = useCallback(async (
    content: string,
    policy: AgentConversationDeliveryPolicy,
    preparedAttachments: PreparedComposerAttachment[],
  ) => {
    await onSendMessage(content, policy, preparedAttachments);
  }, [onSendMessage]);

  const handleSend = useCallback(async () => {
    const trimmedInput = input.trim();
    if (isGoalMode) {
      if (
        !trimmedInput ||
        isInputLocked ||
        isGoalCreating ||
        !onCreateGoal ||
        goalCreateBlockedReason
      ) {
        return;
      }
      setIsGoalCreating(true);
      setGoalError(null);
      try {
        await onCreateGoal(trimmedInput);
        setInput("");
        setInputMode("message");
      } catch (error) {
        setGoalError(error instanceof Error ? error.message : t("composer.goal_create_failed"));
      } finally {
        setIsGoalCreating(false);
      }
      return;
    }

    if (
      (!trimmedInput && attachments.length === 0) ||
      isInputLocked ||
      isPreparingAttachments
    ) {
      return;
    }

    const preparedAttachments = await prepareAttachments();
    if (!preparedAttachments) {
      return;
    }

    if (trimmedInput) {
      setInputHistory((prev) => [trimmedInput, ...prev.slice(0, 49)]);
    }
    setHistoryIndex(-1);
    setHistoryDraft("");

    try {
      const shouldEnqueueMessage = queueWhenSessionBusy && (isLoading || inputQueueItems.length > 0);
      if (shouldEnqueueMessage) {
        if (!onEnqueueMessage) {
          return;
        }
        await onEnqueueMessage(trimmedInput, defaultDeliveryPolicy, preparedAttachments);
      } else {
        const deliveryPolicy = isLoading || inputQueueItems.length > 0
          ? defaultDeliveryPolicy
          : "queue";
        await dispatchMessage(trimmedInput, deliveryPolicy, preparedAttachments);
      }
      setInput("");
      clearAttachments();
      clearAttachmentError();
    } catch (error) {
      console.error("发送消息失败:", error);
      return;
    }

    if (textareaRef.current) {
      textareaRef.current.style.height = "auto";
    }
  }, [
    attachments.length,
    clearAttachmentError,
    clearAttachments,
    defaultDeliveryPolicy,
    dispatchMessage,
    goalCreateBlockedReason,
    inputQueueItems.length,
    input,
    isGoalCreating,
    isGoalMode,
    isInputLocked,
    isLoading,
    isPreparingAttachments,
    onEnqueueMessage,
    onCreateGoal,
    prepareAttachments,
    queueWhenSessionBusy,
    t,
  ]);

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
    requestAnimationFrame(() => textareaRef.current?.focus());
  }, [canCreateGoal, closeMention]);

  const cancelGoalInput = useCallback(() => {
    setInputMode("message");
    setGoalError(null);
    requestAnimationFrame(() => textareaRef.current?.focus());
  }, []);

  const toggleGoalInput = useCallback((checked: boolean) => {
    if (checked) {
      startGoalInput();
      return;
    }
    setIsActionMenuOpen(false);
    cancelGoalInput();
  }, [cancelGoalInput, startGoalInput]);

  const openLoopPicker = useCallback(() => {
    if (!canUseLoop) {
      return;
    }
    setIsActionMenuOpen(false);
    setIsLoopPickerOpen(true);
  }, [canUseLoop]);

  const applyLoopPrompt = useCallback((loop: LoopCatalogItem) => {
    setInputMode("message");
    setGoalError(null);
    setInput(loop.kickoff_prompt);
    closeMention();
    requestAnimationFrame(() => textareaRef.current?.focus());
  }, [closeMention]);

  const applyLoopGoal = useCallback((loop: LoopCatalogItem) => {
    if (!canCreateGoal) {
      applyLoopPrompt(loop);
      return;
    }
    setInputMode("goal");
    setGoalError(null);
    setInput(loop.kickoff_prompt);
    closeMention();
    requestAnimationFrame(() => textareaRef.current?.focus());
  }, [applyLoopPrompt, canCreateGoal, closeMention]);

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

  const recallPreviousHistory = useCallback(() => {
    if (inputHistory.length === 0) {
      return;
    }
    if (historyIndex < 0) {
      setHistoryDraft(input);
    }
    const nextIndex = Math.min(historyIndex + 1, inputHistory.length - 1);
    setHistoryIndex(nextIndex);
    setInput(inputHistory[nextIndex] ?? "");
    clearAttachmentError();
  }, [clearAttachmentError, historyIndex, input, inputHistory]);

  const recallNextHistory = useCallback(() => {
    if (historyIndex > 0) {
      const nextIndex = historyIndex - 1;
      setHistoryIndex(nextIndex);
      setInput(inputHistory[nextIndex] ?? "");
      return;
    }

    if (historyIndex === 0) {
      setHistoryIndex(-1);
      setInput(historyDraft);
      setHistoryDraft("");
    }
  }, [historyDraft, historyIndex, inputHistory]);

  const handleKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    const nativeEvent = event.nativeEvent as ComposerNativeKeyboardEvent;
    const justFinishedComposition =
      lastCompositionEndAtRef.current > 0 &&
      event.timeStamp - lastCompositionEndAtRef.current <= COMPOSITION_END_ENTER_GUARD_MS;

    // Safari 在中文输入法确认候选词后，可能补发一个不带 composing 标记的 Enter。
    // 这里同时拦截 IME 的 229/Process 信号，并且只吞掉紧跟 compositionend 的下一次 Enter，
    // 避免候选词确认被误判成发送消息。
    if (
      isComposingRef.current ||
      nativeEvent.isComposing ||
      nativeEvent.key === "Process" ||
      nativeEvent.keyCode === IME_COMPOSITION_KEY_CODE ||
      nativeEvent.which === IME_COMPOSITION_KEY_CODE
    ) {
      return;
    }

    if (ignoreNextEnterAfterCompositionRef.current && event.key !== "Enter") {
      ignoreNextEnterAfterCompositionRef.current = false;
    }

    if (mentionActive && ["ArrowDown", "ArrowUp", "Enter", "Tab", "Escape"].includes(event.key)) {
      return;
    }

    if (event.key === "Enter" && !event.shiftKey) {
      if (ignoreNextEnterAfterCompositionRef.current && justFinishedComposition) {
        ignoreNextEnterAfterCompositionRef.current = false;
        return;
      }

      event.preventDefault();
      handleSend();
      return;
    }

    const shouldOpenPreviousHistory =
      event.key === "ArrowUp" &&
      inputHistory.length > 0 &&
      (event.ctrlKey || isCaretOnFirstLine(event.currentTarget));
    if (shouldOpenPreviousHistory) {
      event.preventDefault();
      recallPreviousHistory();
      return;
    }

    const shouldOpenNextHistory =
      event.key === "ArrowDown" &&
      historyIndex >= 0 &&
      (event.ctrlKey || isCaretOnLastLine(event.currentTarget));
    if (shouldOpenNextHistory) {
      event.preventDefault();
      recallNextHistory();
      return;
    }

    if (event.key === "Escape" && isLoading && onStop) {
      event.preventDefault();
      onStop();
    }
  };

  const hasTextInput = input.trim().length > 0;
  const isInputEmpty = !hasTextInput && attachments.length === 0;
  const charCount = input.length;
  const isNearLimit = charCount > maxLength * 0.8;
  const isOverLimit = charCount > maxLength;
  const isSendDisabled = isGoalMode
    ? !hasTextInput || isInputLocked || isOverLimit || isGoalCreating || !onCreateGoal || Boolean(goalCreateBlockedReason)
    : isInputEmpty || isInputLocked || isOverLimit || isPreparingAttachments;
  const shouldShowStopButton =
    !isGoalMode && canStopGeneration && (!allowSendWhileLoading || isInputEmpty);
  const hasPendingQueue = inputQueueItems.length > 0;
  const activeError = isGoalMode
    ? goalError ?? goalCreateBlockedReason
    : attachmentError;
  const sendButtonLabel = isGoalMode ? t("composer.goal_confirm") : t("composer.send_message");
  const inlineEnterLabel = isGoalMode
    ? t("composer.goal_enter_start")
    : queueWhenSessionBusy && (isLoading || inputQueueItems.length > 0)
      ? t("composer.enter_queue")
      : t("composer.enter_send");
  const shouldShowInlineShortcuts = !compact && input.length === 0;
  let composerInputRowPaddingClass = compact ? "px-2 py-2" : "px-3 py-3";
  if (hasPendingQueue) {
    composerInputRowPaddingClass = compact ? "px-2 pb-2 pt-1" : "px-3 pb-3 pt-1.5";
  }
  if (isGoalMode) {
    composerInputRowPaddingClass = compact ? "px-2 pb-2 pt-1.5" : "px-3 pb-3 pt-2";
  }

  return (
    <section
      data-tour-anchor={tourAnchor}
      className={cn(
        "mx-auto w-full max-w-[1020px] border-t border-(--surface-canvas-border) bg-transparent",
        compact ? "px-2 pb-2 pt-2" : "px-3 pb-3 pt-3 sm:px-5 xl:px-6",
      )}
    >
      <input
        ref={fileInputRef}
        accept={COMPOSER_ATTACHMENT_ACCEPT}
        aria-label={t("composer.choose_attachment_file")}
        className="hidden"
        multiple
        onChange={handleFileSelect}
        type="file"
      />
      {canUseLoop ? (
        <LoopPickerDialog
          isOpen={isLoopPickerOpen}
          onClose={() => setIsLoopPickerOpen(false)}
          onSelect={handleLoopSelect}
        />
      ) : null}

      <div className={getComposerShellClassName(isInputLocked)} style={getComposerShellStyle(compact)}>
        <ComposerPendingQueue
          compact={compact}
          disabled={disabled}
          inputQueueItems={inputQueueItems}
          onDeleteQueuedMessage={onDeleteQueuedMessage}
          onGuideQueuedMessage={onGuideQueuedMessage}
          onReorderQueueMessages={onReorderQueueMessages}
        />

        <ComposerAttachmentList
          attachments={attachments}
          onRemove={removeAttachment}
          removeLabel={t("composer.remove_attachment")}
        />

        <div className={cn("flex items-end gap-2", composerInputRowPaddingClass)}>
          {mentionActive && mentionTargetItems.length > 0 ? (
            <MentionTargetPopover
              anchorRect={textareaRef.current?.getBoundingClientRect() ?? null}
              filter={mentionFilter}
              items={mentionTargetItems}
              onClose={closeMention}
              onSelect={selectMentionItem}
              placement="above"
            />
          ) : null}

          <div className="relative min-w-0 flex-1">
            <textarea
              aria-label={t("composer.default_placeholder")}
              ref={textareaRef}
              className={cn(
                "multiline-cursor soft-scrollbar min-h-6 w-full min-w-0 max-h-[200px] resize-none overflow-y-auto overscroll-contain bg-transparent text-[14px] leading-6 text-(--text-strong) outline-none shadow-none ring-0",
                "placeholder:text-(--text-soft)",
                "disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)",
                "focus:border-0 focus:bg-transparent focus:outline-none focus:ring-0 focus-visible:outline-none focus-visible:ring-0 focus-visible:shadow-none",
                shouldShowInlineShortcuts && "min-[760px]:pr-[210px]",
              )}
              disabled={isTextareaLocked}
              onChange={(event) => handleInputChange(event.target.value)}
              onWheel={(event) => {
                const target = event.currentTarget;
                if (target.scrollHeight > target.clientHeight) {
                  event.stopPropagation();
                }
              }}
              onCompositionEnd={(event) => {
                isComposingRef.current = false;
                ignoreNextEnterAfterCompositionRef.current = true;
                lastCompositionEndAtRef.current = event.timeStamp;
              }}
              onCompositionStart={() => {
                isComposingRef.current = true;
                ignoreNextEnterAfterCompositionRef.current = false;
              }}
              onKeyDown={handleKeyDown}
              onPaste={handlePaste}
              placeholder={resolvedPlaceholder}
              rows={1}
              value={input}
            />
            {shouldShowInlineShortcuts ? (
              <div className="pointer-events-none absolute right-0 top-1/2 hidden -translate-y-1/2 items-center gap-2 text-[10px] text-(--text-soft) min-[760px]:flex">
                <span className="flex items-center gap-1">
                  <kbd>Enter</kbd>
                  <span>{inlineEnterLabel}</span>
                </span>
                <span className="flex items-center gap-1">
                  <kbd>Shift</kbd>
                  <span>+</span>
                  <kbd>Enter</kbd>
                  <span>{t("composer.shift_enter_newline")}</span>
                </span>
              </div>
            ) : null}
          </div>

          {shouldShowStopButton ? (
            <button
              aria-label={t("composer.stop_generation")}
              className={COMPOSER_DANGER_ACTION_BUTTON_CLASS_NAME}
              onClick={onStop}
              type="button"
            >
              <StopCircle size={16} />
            </button>
          ) : (
            <button
              aria-label={sendButtonLabel}
              className={COMPOSER_PRIMARY_ACTION_BUTTON_CLASS_NAME}
              disabled={isSendDisabled}
              onClick={() => {
                void handleSend();
              }}
              type="button"
            >
              {isPreparingAttachments || isGoalCreating ? (
                <LoadingOrb frames={["·", "◦", "•", "◦"]} />
              ) : isGoalMode ? (
                <Target size={16} />
              ) : (
                <Send size={16} />
              )}
            </button>
          )}
        </div>

        <ComposerFooter
          actionButtonRef={actionButtonRef}
          activeError={activeError}
          canCreateGoal={canCreateGoal}
          canUseLoop={canUseLoop}
          canStopGeneration={canStopGeneration}
          charCount={charCount}
          goalModeExtra={goalModeExtra}
          goalScopeLabel={goalScopeLabel}
          historyIndex={historyIndex}
          inputHistoryLength={inputHistory.length}
          isActionMenuOpen={isActionMenuOpen}
          isDispatching={isDispatching}
          isGoalCreating={isGoalCreating}
          isGoalMode={isGoalMode}
          isInputLocked={isInputLocked}
          isNearLimit={isNearLimit}
          isOverLimit={isOverLimit}
          isPreparingAttachments={isPreparingAttachments}
          maxLength={maxLength}
          onActionMenuClose={() => setIsActionMenuOpen(false)}
          onActionMenuToggle={() => setIsActionMenuOpen((current) => !current)}
          onAttachmentSelect={openAttachmentPicker}
          onCancelGoal={cancelGoalInput}
          onGoalToggle={toggleGoalInput}
          onLoopSelect={openLoopPicker}
        />
      </div>
    </section>
  );
});

ComposerPanelView.displayName = "ComposerPanelView";

export function ComposerPanel(props: ComposerPanelProps) {
  return <ComposerPanelView {...props} />;
}
