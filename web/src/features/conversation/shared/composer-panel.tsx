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
  get_composer_shell_class_name,
  get_composer_shell_style,
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
  is_loading?: boolean;
  runtime_phase?: AgentConversationRuntimePhase | null;
  on_send_message: (
    content: string,
    deliveryPolicy: AgentConversationDeliveryPolicy,
    attachments?: PreparedComposerAttachment[],
  ) => void | Promise<void>;
  input_queue_items?: InputQueueItem[];
  on_enqueue_message?: (
    content: string,
    deliveryPolicy: AgentConversationDeliveryPolicy,
    attachments?: PreparedComposerAttachment[],
  ) => void | Promise<void>;
  on_delete_queued_message?: (itemId: string) => void | Promise<void>;
  on_guide_queued_message?: (itemId: string) => void | Promise<void>;
  on_reorder_queue_messages?: (orderedIds: string[]) => void | Promise<void>;
  on_stop?: () => void;
  default_delivery_policy?: AgentConversationDefaultDeliveryPolicy;
  initial_draft?: string | null;
  disabled?: boolean;
  allow_send_while_loading?: boolean;
  queue_when_session_busy?: boolean;
  placeholder?: string;
  max_length?: number;
  room_members?: Agent[];
  mention_unavailable_agent_ids?: string[];
  on_prepare_attachments?: (files: File[]) => Promise<PreparedComposerAttachment[]>;
  on_create_goal?: (objective: string) => Promise<void>;
  enable_loops?: boolean;
  on_create_loop_goal?: (loop: LoopCatalogItem) => Promise<void>;
  goal_create_disabled_reason?: string | null;
  goal_mode_extra?: ReactNode;
  goal_scope_label?: string;
  tour_anchor?: string;
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
  is_loading: isLoading = false,
  runtime_phase: runtimePhase = null,
  on_send_message: onSendMessage,
  input_queue_items: inputQueueItems = [],
  on_enqueue_message: onEnqueueMessage,
  on_delete_queued_message: onDeleteQueuedMessage,
  on_guide_queued_message: onGuideQueuedMessage,
  on_reorder_queue_messages: onReorderQueueMessages,
  on_stop: onStop,
  default_delivery_policy: defaultDeliveryPolicy = "queue",
  initial_draft: initialDraft = null,
  disabled = false,
  allow_send_while_loading: allowSendWhileLoading = false,
  queue_when_session_busy: queueWhenSessionBusy = true,
  placeholder,
  max_length: maxLength = 10000,
  room_members: roomMembers = [],
  mention_unavailable_agent_ids: mentionUnavailableAgentIds = [],
  on_prepare_attachments: onPrepareAttachments,
  on_create_goal: onCreateGoal,
  enable_loops: enableLoops = false,
  on_create_loop_goal: onCreateLoopGoal,
  goal_create_disabled_reason: goalCreateDisabledReason = null,
  goal_mode_extra: goalModeExtra = null,
  goal_scope_label: goalScopeLabel = "会话 Goal",
  tour_anchor: tourAnchor,
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
    attachment_error: attachmentError,
    attachments,
    clear_attachment_error: clearAttachmentError,
    clear_attachments: clearAttachments,
    handle_file_select: handleFileSelect,
    handle_paste: handlePaste,
    is_preparing_attachments: isPreparingAttachments,
    prepare_attachments: prepareAttachments,
    remove_attachment: removeAttachment,
  } = useComposerAttachments({
    is_goal_mode: isGoalMode,
    on_goal_attachment_rejected: setGoalError,
    on_prepare_attachments: onPrepareAttachments,
  });

  const isComposingRef = useRef(false);
  const ignoreNextEnterAfterCompositionRef = useRef(false);
  const lastCompositionEndAtRef = useRef(0);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const actionButtonRef = useRef<HTMLButtonElement>(null);
  const {
    close_mention: closeMention,
    mention_active: mentionActive,
    mention_filter: mentionFilter,
    mention_target_items: mentionTargetItems,
    select_mention_item: selectMentionItem,
    update_mention_for_input: updateMentionForInput,
  } = useComposerMention({
    input,
    is_goal_mode: isGoalMode,
    mention_unavailable_agent_ids: mentionUnavailableAgentIds,
    room_members: roomMembers,
    set_input: setInput,
    textarea_ref: textareaRef,
  });
  const isDispatching = isLoading && runtimePhase === "sending";
  const isInputLocked = disabled || (!allowSendWhileLoading && isLoading);
  const isTextareaLocked = isInputLocked || (isGoalMode && isGoalCreating);
  const canStopGeneration = isLoading && !isDispatching && Boolean(onStop);
  const canCreateGoal = Boolean(onCreateGoal);
  const canUseLoop = enableLoops && (Boolean(onCreateLoopGoal) || canCreateGoal);
  const goalCreateBlockedReason =
    goalCreateDisabledReason?.trim() || null;

  useTextareaHeight(textareaRef, input, { min_height: 24, max_height: 200, line_height: 24, padding_y: 0 });

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
          is_open={isLoopPickerOpen}
          on_close={() => setIsLoopPickerOpen(false)}
          on_select={handleLoopSelect}
        />
      ) : null}

      <div className={get_composer_shell_class_name(isInputLocked)} style={get_composer_shell_style(compact)}>
        <ComposerPendingQueue
          compact={compact}
          disabled={disabled}
          input_queue_items={inputQueueItems}
          on_delete_queued_message={onDeleteQueuedMessage}
          on_guide_queued_message={onGuideQueuedMessage}
          on_reorder_queue_messages={onReorderQueueMessages}
        />

        <ComposerAttachmentList
          attachments={attachments}
          on_remove={removeAttachment}
          remove_label={t("composer.remove_attachment")}
        />

        <div className={cn("flex items-end gap-2", composerInputRowPaddingClass)}>
          {mentionActive && mentionTargetItems.length > 0 ? (
            <MentionTargetPopover
              anchor_rect={textareaRef.current?.getBoundingClientRect() ?? null}
              filter={mentionFilter}
              items={mentionTargetItems}
              on_close={closeMention}
              on_select={selectMentionItem}
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
          action_button_ref={actionButtonRef}
          active_error={activeError}
          can_create_goal={canCreateGoal}
          can_use_loop={canUseLoop}
          can_stop_generation={canStopGeneration}
          char_count={charCount}
          goal_mode_extra={goalModeExtra}
          goal_scope_label={goalScopeLabel}
          history_index={historyIndex}
          input_history_length={inputHistory.length}
          is_action_menu_open={isActionMenuOpen}
          is_dispatching={isDispatching}
          is_goal_creating={isGoalCreating}
          is_goal_mode={isGoalMode}
          is_input_locked={isInputLocked}
          is_near_limit={isNearLimit}
          is_over_limit={isOverLimit}
          is_preparing_attachments={isPreparingAttachments}
          max_length={maxLength}
          on_action_menu_close={() => setIsActionMenuOpen(false)}
          on_action_menu_toggle={() => setIsActionMenuOpen((current) => !current)}
          on_attachment_select={openAttachmentPicker}
          on_cancel_goal={cancelGoalInput}
          on_goal_toggle={toggleGoalInput}
          on_loop_select={openLoopPicker}
        />
      </div>
    </section>
  );
});

ComposerPanelView.displayName = "ComposerPanelView";

export function ComposerPanel(props: ComposerPanelProps) {
  return <ComposerPanelView {...props} />;
}
