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
    delivery_policy: AgentConversationDeliveryPolicy,
    attachments?: PreparedComposerAttachment[],
  ) => void | Promise<void>;
  input_queue_items?: InputQueueItem[];
  on_enqueue_message?: (
    content: string,
    delivery_policy: AgentConversationDeliveryPolicy,
    attachments?: PreparedComposerAttachment[],
  ) => void | Promise<void>;
  on_delete_queued_message?: (item_id: string) => void | Promise<void>;
  on_guide_queued_message?: (item_id: string) => void | Promise<void>;
  on_reorder_queue_messages?: (ordered_ids: string[]) => void | Promise<void>;
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
function is_caret_on_first_line(target: HTMLTextAreaElement) {
  const selection_start = target.selectionStart ?? 0;
  const selection_end = target.selectionEnd ?? 0;
  if (selection_start !== selection_end) {
    return false;
  }
  return !target.value.slice(0, selection_start).includes("\n");
}

function is_caret_on_last_line(target: HTMLTextAreaElement) {
  const selection_start = target.selectionStart ?? 0;
  const selection_end = target.selectionEnd ?? 0;
  if (selection_start !== selection_end) {
    return false;
  }
  return !target.value.slice(selection_end).includes("\n");
}

const ComposerPanelView = memo(({
  compact,
  is_loading = false,
  runtime_phase = null,
  on_send_message,
  input_queue_items = [],
  on_enqueue_message,
  on_delete_queued_message,
  on_guide_queued_message,
  on_reorder_queue_messages,
  on_stop,
  default_delivery_policy = "queue",
  initial_draft = null,
  disabled = false,
  allow_send_while_loading = false,
  queue_when_session_busy = true,
  placeholder,
  max_length = 10000,
  room_members = [],
  mention_unavailable_agent_ids = [],
  on_prepare_attachments,
  on_create_goal,
  enable_loops = false,
  on_create_loop_goal,
  goal_create_disabled_reason = null,
  goal_mode_extra = null,
  goal_scope_label = "会话 Goal",
  tour_anchor,
}: ComposerPanelProps) => {
  const { t } = useI18n();
  const [input_mode, set_input_mode] = useState<ComposerInputMode>("message");
  const is_goal_mode = input_mode === "goal";
  const resolved_placeholder = is_goal_mode
    ? t("composer.goal_placeholder")
    : placeholder ?? t("composer.default_placeholder");
  const [input, setInput] = useState("");
  const [input_history, setInputHistory] = useState<string[]>([]);
  const [history_index, setHistoryIndex] = useState(-1);
  const [history_draft, setHistoryDraft] = useState("");
  const [is_action_menu_open, set_is_action_menu_open] = useState(false);
  const [is_loop_picker_open, set_is_loop_picker_open] = useState(false);
  const [is_goal_creating, set_is_goal_creating] = useState(false);
  const [goal_error, set_goal_error] = useState<string | null>(null);
  const {
    attachment_error,
    attachments,
    clear_attachment_error,
    clear_attachments,
    handle_file_select,
    handle_paste,
    is_preparing_attachments,
    prepare_attachments,
    remove_attachment,
  } = useComposerAttachments({
    is_goal_mode,
    on_goal_attachment_rejected: set_goal_error,
    on_prepare_attachments,
  });

  const is_composing_ref = useRef(false);
  const ignore_next_enter_after_composition_ref = useRef(false);
  const last_composition_end_at_ref = useRef(0);
  const textarea_ref = useRef<HTMLTextAreaElement>(null);
  const file_input_ref = useRef<HTMLInputElement>(null);
  const action_button_ref = useRef<HTMLButtonElement>(null);
  const {
    close_mention,
    mention_active,
    mention_filter,
    mention_target_items,
    select_mention_item,
    update_mention_for_input,
  } = useComposerMention({
    input,
    is_goal_mode,
    mention_unavailable_agent_ids,
    room_members,
    set_input: setInput,
    textarea_ref,
  });
  const is_dispatching = is_loading && runtime_phase === "sending";
  const is_input_locked = disabled || (!allow_send_while_loading && is_loading);
  const is_textarea_locked = is_input_locked || (is_goal_mode && is_goal_creating);
  const can_stop_generation = is_loading && !is_dispatching && Boolean(on_stop);
  const can_create_goal = Boolean(on_create_goal);
  const can_use_loop = enable_loops && (Boolean(on_create_loop_goal) || can_create_goal);
  const goal_create_blocked_reason =
    goal_create_disabled_reason?.trim() || null;

  useTextareaHeight(textarea_ref, input, { min_height: 24, max_height: 200, line_height: 24, padding_y: 0 });

  const handle_input_change = useCallback((value: string) => {
    setInput(value);
    if (attachment_error) {
      clear_attachment_error();
    }
    if (goal_error) {
      set_goal_error(null);
    }

    update_mention_for_input(value);
  }, [
    attachment_error,
    clear_attachment_error,
    goal_error,
    update_mention_for_input,
  ]);

  useEffect(() => {
    if (textarea_ref.current && !is_input_locked) {
      textarea_ref.current.focus();
    }
  }, [is_input_locked]);

  useEffect(() => {
    const normalized_draft = initial_draft?.trim() ?? "";
    if (!normalized_draft) {
      return;
    }
    setInput((current_value) => current_value || normalized_draft);
  }, [initial_draft]);

  const dispatch_message = useCallback(async (
    content: string,
    policy: AgentConversationDeliveryPolicy,
    prepared_attachments: PreparedComposerAttachment[],
  ) => {
    await on_send_message(content, policy, prepared_attachments);
  }, [on_send_message]);

  const handle_send = useCallback(async () => {
    const trimmed_input = input.trim();
    if (is_goal_mode) {
      if (
        !trimmed_input ||
        is_input_locked ||
        is_goal_creating ||
        !on_create_goal ||
        goal_create_blocked_reason
      ) {
        return;
      }
      set_is_goal_creating(true);
      set_goal_error(null);
      try {
        await on_create_goal(trimmed_input);
        setInput("");
        set_input_mode("message");
      } catch (error) {
        set_goal_error(error instanceof Error ? error.message : t("composer.goal_create_failed"));
      } finally {
        set_is_goal_creating(false);
      }
      return;
    }

    if (
      (!trimmed_input && attachments.length === 0) ||
      is_input_locked ||
      is_preparing_attachments
    ) {
      return;
    }

    const prepared_attachments = await prepare_attachments();
    if (!prepared_attachments) {
      return;
    }

    if (trimmed_input) {
      setInputHistory((prev) => [trimmed_input, ...prev.slice(0, 49)]);
    }
    setHistoryIndex(-1);
    setHistoryDraft("");

    try {
      const should_enqueue_message = queue_when_session_busy && (is_loading || input_queue_items.length > 0);
      if (should_enqueue_message) {
        if (!on_enqueue_message) {
          return;
        }
        await on_enqueue_message(trimmed_input, default_delivery_policy, prepared_attachments);
      } else {
        const delivery_policy = is_loading || input_queue_items.length > 0
          ? default_delivery_policy
          : "queue";
        await dispatch_message(trimmed_input, delivery_policy, prepared_attachments);
      }
      setInput("");
      clear_attachments();
      clear_attachment_error();
    } catch (error) {
      console.error("发送消息失败:", error);
      return;
    }

    if (textarea_ref.current) {
      textarea_ref.current.style.height = "auto";
    }
  }, [
    attachments.length,
    clear_attachment_error,
    clear_attachments,
    default_delivery_policy,
    dispatch_message,
    goal_create_blocked_reason,
    input_queue_items.length,
    input,
    is_goal_creating,
    is_goal_mode,
    is_input_locked,
    is_loading,
    is_preparing_attachments,
    on_enqueue_message,
    on_create_goal,
    prepare_attachments,
    queue_when_session_busy,
    t,
  ]);

  const open_attachment_picker = useCallback(() => {
    set_is_action_menu_open(false);
    file_input_ref.current?.click();
  }, []);

  const start_goal_input = useCallback(() => {
    if (!can_create_goal) {
      return;
    }
    set_is_action_menu_open(false);
    set_input_mode("goal");
    set_goal_error(null);
    close_mention();
    requestAnimationFrame(() => textarea_ref.current?.focus());
  }, [can_create_goal, close_mention]);

  const cancel_goal_input = useCallback(() => {
    set_input_mode("message");
    set_goal_error(null);
    requestAnimationFrame(() => textarea_ref.current?.focus());
  }, []);

  const toggle_goal_input = useCallback((checked: boolean) => {
    if (checked) {
      start_goal_input();
      return;
    }
    set_is_action_menu_open(false);
    cancel_goal_input();
  }, [cancel_goal_input, start_goal_input]);

  const open_loop_picker = useCallback(() => {
    if (!can_use_loop) {
      return;
    }
    set_is_action_menu_open(false);
    set_is_loop_picker_open(true);
  }, [can_use_loop]);

  const apply_loop_prompt = useCallback((loop: LoopCatalogItem) => {
    set_input_mode("message");
    set_goal_error(null);
    setInput(loop.kickoff_prompt);
    close_mention();
    requestAnimationFrame(() => textarea_ref.current?.focus());
  }, [close_mention]);

  const apply_loop_goal = useCallback((loop: LoopCatalogItem) => {
    if (!can_create_goal) {
      apply_loop_prompt(loop);
      return;
    }
    set_input_mode("goal");
    set_goal_error(null);
    setInput(loop.kickoff_prompt);
    close_mention();
    requestAnimationFrame(() => textarea_ref.current?.focus());
  }, [apply_loop_prompt, can_create_goal, close_mention]);

  const handle_loop_select = useCallback(async (loop: LoopCatalogItem) => {
    if (!on_create_loop_goal) {
      apply_loop_goal(loop);
      return;
    }
    set_goal_error(null);
    close_mention();
    await on_create_loop_goal(loop);
    set_input_mode("message");
    setInput("");
  }, [apply_loop_goal, close_mention, on_create_loop_goal]);

  const recall_previous_history = useCallback(() => {
    if (input_history.length === 0) {
      return;
    }
    if (history_index < 0) {
      setHistoryDraft(input);
    }
    const next_index = Math.min(history_index + 1, input_history.length - 1);
    setHistoryIndex(next_index);
    setInput(input_history[next_index] ?? "");
    clear_attachment_error();
  }, [clear_attachment_error, history_index, input, input_history]);

  const recall_next_history = useCallback(() => {
    if (history_index > 0) {
      const next_index = history_index - 1;
      setHistoryIndex(next_index);
      setInput(input_history[next_index] ?? "");
      return;
    }

    if (history_index === 0) {
      setHistoryIndex(-1);
      setInput(history_draft);
      setHistoryDraft("");
    }
  }, [history_draft, history_index, input_history]);

  const handle_key_down = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    const native_event = event.nativeEvent as ComposerNativeKeyboardEvent;
    const just_finished_composition =
      last_composition_end_at_ref.current > 0 &&
      Date.now() - last_composition_end_at_ref.current <= COMPOSITION_END_ENTER_GUARD_MS;

    // Safari 在中文输入法确认候选词后，可能补发一个不带 composing 标记的 Enter。
    // 这里同时拦截 IME 的 229/Process 信号，并且只吞掉紧跟 compositionend 的下一次 Enter，
    // 避免候选词确认被误判成发送消息。
    if (
      is_composing_ref.current ||
      native_event.isComposing ||
      native_event.key === "Process" ||
      native_event.keyCode === IME_COMPOSITION_KEY_CODE ||
      native_event.which === IME_COMPOSITION_KEY_CODE
    ) {
      return;
    }

    if (ignore_next_enter_after_composition_ref.current && event.key !== "Enter") {
      ignore_next_enter_after_composition_ref.current = false;
    }

    if (mention_active && ["ArrowDown", "ArrowUp", "Enter", "Tab", "Escape"].includes(event.key)) {
      return;
    }

    if (event.key === "Enter" && !event.shiftKey) {
      if (ignore_next_enter_after_composition_ref.current && just_finished_composition) {
        ignore_next_enter_after_composition_ref.current = false;
        return;
      }

      event.preventDefault();
      handle_send();
      return;
    }

    const should_open_previous_history =
      event.key === "ArrowUp" &&
      input_history.length > 0 &&
      (event.ctrlKey || is_caret_on_first_line(event.currentTarget));
    if (should_open_previous_history) {
      event.preventDefault();
      recall_previous_history();
      return;
    }

    const should_open_next_history =
      event.key === "ArrowDown" &&
      history_index >= 0 &&
      (event.ctrlKey || is_caret_on_last_line(event.currentTarget));
    if (should_open_next_history) {
      event.preventDefault();
      recall_next_history();
      return;
    }

    if (event.key === "Escape" && is_loading && on_stop) {
      event.preventDefault();
      on_stop();
    }
  };

  const has_text_input = input.trim().length > 0;
  const is_input_empty = !has_text_input && attachments.length === 0;
  const char_count = input.length;
  const is_near_limit = char_count > max_length * 0.8;
  const is_over_limit = char_count > max_length;
  const is_send_disabled = is_goal_mode
    ? !has_text_input || is_input_locked || is_over_limit || is_goal_creating || !on_create_goal || Boolean(goal_create_blocked_reason)
    : is_input_empty || is_input_locked || is_over_limit || is_preparing_attachments;
  const should_show_stop_button =
    !is_goal_mode && can_stop_generation && (!allow_send_while_loading || is_input_empty);
  const has_pending_queue = input_queue_items.length > 0;
  const active_error = is_goal_mode
    ? goal_error ?? goal_create_blocked_reason
    : attachment_error;
  const send_button_label = is_goal_mode ? t("composer.goal_confirm") : t("composer.send_message");
  const inline_enter_label = is_goal_mode
    ? t("composer.goal_enter_start")
    : queue_when_session_busy && (is_loading || input_queue_items.length > 0)
      ? t("composer.enter_queue")
      : t("composer.enter_send");
  const should_show_inline_shortcuts = !compact && input.length === 0;
  let composer_input_row_padding_class = compact ? "px-2 py-2" : "px-3 py-3";
  if (has_pending_queue) {
    composer_input_row_padding_class = compact ? "px-2 pb-2 pt-1" : "px-3 pb-3 pt-1.5";
  }
  if (is_goal_mode) {
    composer_input_row_padding_class = compact ? "px-2 pb-2 pt-1.5" : "px-3 pb-3 pt-2";
  }

  return (
    <section
      data-tour-anchor={tour_anchor}
      className={cn(
        "mx-auto w-full max-w-[1020px] border-t border-(--surface-canvas-border) bg-transparent",
        compact ? "px-2 pb-2 pt-2" : "px-3 pb-3 pt-3 sm:px-5 xl:px-6",
      )}
    >
      <input
        ref={file_input_ref}
        accept={COMPOSER_ATTACHMENT_ACCEPT}
        aria-label={t("composer.choose_attachment_file")}
        className="hidden"
        multiple
        onChange={handle_file_select}
        type="file"
      />
      {can_use_loop ? (
        <LoopPickerDialog
          is_open={is_loop_picker_open}
          on_close={() => set_is_loop_picker_open(false)}
          on_select={handle_loop_select}
        />
      ) : null}

      <div className={get_composer_shell_class_name(is_input_locked)} style={get_composer_shell_style(compact)}>
        <ComposerPendingQueue
          compact={compact}
          disabled={disabled}
          input_queue_items={input_queue_items}
          on_delete_queued_message={on_delete_queued_message}
          on_guide_queued_message={on_guide_queued_message}
          on_reorder_queue_messages={on_reorder_queue_messages}
        />

        <ComposerAttachmentList
          attachments={attachments}
          on_remove={remove_attachment}
          remove_label={t("composer.remove_attachment")}
        />

        <div className={cn("flex items-end gap-2", composer_input_row_padding_class)}>
          {mention_active && mention_target_items.length > 0 ? (
            <MentionTargetPopover
              anchor_rect={textarea_ref.current?.getBoundingClientRect() ?? null}
              filter={mention_filter}
              items={mention_target_items}
              on_close={close_mention}
              on_select={select_mention_item}
              placement="above"
            />
          ) : null}

          <div className="relative min-w-0 flex-1">
            <textarea
              ref={textarea_ref}
              className={cn(
                "multiline-cursor soft-scrollbar min-h-6 w-full min-w-0 max-h-[200px] resize-none overflow-y-auto overscroll-contain bg-transparent text-[14px] leading-6 text-(--text-strong) outline-none shadow-none ring-0",
                "placeholder:text-(--text-soft)",
                "disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)",
                "focus:border-0 focus:bg-transparent focus:outline-none focus:ring-0 focus-visible:outline-none focus-visible:ring-0 focus-visible:shadow-none",
                should_show_inline_shortcuts && "min-[760px]:pr-[210px]",
              )}
              disabled={is_textarea_locked}
              onChange={(event) => handle_input_change(event.target.value)}
              onWheel={(event) => {
                const target = event.currentTarget;
                if (target.scrollHeight > target.clientHeight) {
                  event.stopPropagation();
                }
              }}
              onCompositionEnd={() => {
                is_composing_ref.current = false;
                ignore_next_enter_after_composition_ref.current = true;
                last_composition_end_at_ref.current = Date.now();
              }}
              onCompositionStart={() => {
                is_composing_ref.current = true;
                ignore_next_enter_after_composition_ref.current = false;
              }}
              onKeyDown={handle_key_down}
              onPaste={handle_paste}
              placeholder={resolved_placeholder}
              rows={1}
              value={input}
            />
            {should_show_inline_shortcuts ? (
              <div className="pointer-events-none absolute right-0 top-1/2 hidden -translate-y-1/2 items-center gap-2 text-[10px] text-(--text-soft) min-[760px]:flex">
                <span className="flex items-center gap-1">
                  <kbd>Enter</kbd>
                  <span>{inline_enter_label}</span>
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

          {should_show_stop_button ? (
            <button
              aria-label={t("composer.stop_generation")}
              className={COMPOSER_DANGER_ACTION_BUTTON_CLASS_NAME}
              onClick={on_stop}
              type="button"
            >
              <StopCircle size={16} />
            </button>
          ) : (
            <button
              aria-label={send_button_label}
              className={COMPOSER_PRIMARY_ACTION_BUTTON_CLASS_NAME}
              disabled={is_send_disabled}
              onClick={() => {
                void handle_send();
              }}
              type="button"
            >
              {is_preparing_attachments || is_goal_creating ? (
                <LoadingOrb frames={["·", "◦", "•", "◦"]} />
              ) : is_goal_mode ? (
                <Target size={16} />
              ) : (
                <Send size={16} />
              )}
            </button>
          )}
        </div>

        <ComposerFooter
          action_button_ref={action_button_ref}
          active_error={active_error}
          can_create_goal={can_create_goal}
          can_use_loop={can_use_loop}
          can_stop_generation={can_stop_generation}
          char_count={char_count}
          goal_mode_extra={goal_mode_extra}
          goal_scope_label={goal_scope_label}
          history_index={history_index}
          input_history_length={input_history.length}
          is_action_menu_open={is_action_menu_open}
          is_dispatching={is_dispatching}
          is_goal_creating={is_goal_creating}
          is_goal_mode={is_goal_mode}
          is_input_locked={is_input_locked}
          is_near_limit={is_near_limit}
          is_over_limit={is_over_limit}
          is_preparing_attachments={is_preparing_attachments}
          max_length={max_length}
          on_action_menu_close={() => set_is_action_menu_open(false)}
          on_action_menu_toggle={() => set_is_action_menu_open((current) => !current)}
          on_attachment_select={open_attachment_picker}
          on_cancel_goal={cancel_goal_input}
          on_goal_toggle={toggle_goal_input}
          on_loop_select={open_loop_picker}
        />
      </div>
    </section>
  );
});

ComposerPanelView.displayName = "ComposerPanelView";

export function ComposerPanel(props: ComposerPanelProps) {
  return <ComposerPanelView {...props} />;
}
