"use client";

import { ChangeEvent, KeyboardEvent, memo, useCallback, useEffect, useRef, useState } from "react";
import {
  ChevronDown,
  ChevronUp,
  CornerDownRight,
  FileText,
  GripVertical,
  Paperclip,
  Send,
  StopCircle,
  Trash2,
  X,
} from "lucide-react";

import { useTextareaHeight } from "@/hooks/ui/use-textarea-height";
import { cn } from "@/lib/utils";
import { LoadingOrb } from "@/shared/ui/feedback/loading-orb";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  AgentConversationDeliveryPolicy,
  AgentConversationRuntimePhase,
  InputQueueItem,
} from "@/types/agent/agent-conversation";
import { Agent } from "@/types/agent/agent";

import {
  COMPOSER_ACTION_BUTTON_CLASS_NAME,
  COMPOSER_ATTACHMENT_CLASS_NAME,
  COMPOSER_ATTACHMENT_REMOVE_CLASS_NAME,
  COMPOSER_ATTACHMENT_ROW_CLASS_NAME,
  COMPOSER_DANGER_ACTION_BUTTON_CLASS_NAME,
  COMPOSER_FOOTER_CLASS_NAME,
  COMPOSER_PRIMARY_ACTION_BUTTON_CLASS_NAME,
  get_composer_shell_class_name,
  get_composer_shell_style,
} from "./composer-styles";
import {
  COMPOSER_ATTACHMENT_ACCEPT,
  get_attachment_rejection_reason,
  PreparedComposerAttachment,
} from "./composer-attachments";
import { MentionTargetItem, MentionTargetPopover } from "./mention-popover";

interface AttachmentFile {
  id: string;
  file: File;
}

interface ComposerPanelProps {
  compact: boolean;
  is_loading?: boolean;
  runtime_phase?: AgentConversationRuntimePhase | null;
  on_send_message: (
    content: string,
    delivery_policy: AgentConversationDeliveryPolicy,
  ) => void | Promise<void>;
  input_queue_items?: InputQueueItem[];
  on_enqueue_message?: (
    content: string,
    delivery_policy: AgentConversationDeliveryPolicy,
  ) => void | Promise<void>;
  on_delete_queued_message?: (item_id: string) => void | Promise<void>;
  on_guide_queued_message?: (item_id: string) => void | Promise<void>;
  on_reorder_queue_messages?: (ordered_ids: string[]) => void | Promise<void>;
  on_stop?: () => void;
  default_delivery_policy?: AgentConversationDeliveryPolicy;
  initial_draft?: string | null;
  disabled?: boolean;
  allow_send_while_loading?: boolean;
  placeholder?: string;
  max_length?: number;
  room_members?: Agent[];
  mention_unavailable_agent_ids?: string[];
  control_status_text?: string;
  on_prepare_attachments?: (files: File[]) => Promise<PreparedComposerAttachment[]>;
  tour_anchor?: string;
}

type ComposerNativeKeyboardEvent = globalThis.KeyboardEvent & {
  keyCode?: number;
  which?: number;
};

const IME_COMPOSITION_KEY_CODE = 229;
const COMPOSITION_END_ENTER_GUARD_MS = 80;
const PENDING_QUEUE_AUTO_SCROLL_ZONE_PX = 28;
const PENDING_QUEUE_AUTO_SCROLL_MAX_DELTA_PX = 10;

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

function build_message_with_attachments(
  content: string,
  attachments: PreparedComposerAttachment[],
) {
  if (attachments.length === 0) {
    return content.trim();
  }

  const attachment_manifest = attachments
    .map((attachment) => `- ${attachment.file_name}（工作区文件：${attachment.workspace_path}）`)
    .join("\n");
  const attachment_blocks = attachments.map((attachment) => [
    `文件《${attachment.file_name}》内容摘录：`,
    "```text",
    attachment.excerpt,
    "```",
    attachment.truncated ? "注：消息里只附带前 12000 个字符，完整内容已写入工作区文件。" : null,
  ].filter(Boolean).join("\n"));

  return [
    content.trim(),
    "已附加文本文件：",
    attachment_manifest,
    ...attachment_blocks,
  ].filter(Boolean).join("\n\n");
}

function reorder_pending_messages(
  messages: InputQueueItem[],
  source_id: string,
  target_id: string,
): InputQueueItem[] {
  const source_index = messages.findIndex((item) => item.id === source_id);
  const target_index = messages.findIndex((item) => item.id === target_id);
  if (source_index < 0 || target_index < 0 || source_index === target_index) {
    return messages;
  }
  const next = [...messages];
  const [source] = next.splice(source_index, 1);
  next.splice(target_index, 0, source);
  return next;
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
  placeholder = "继续描述目标、补充上下文，或直接开始协作…",
  max_length = 10000,
  room_members = [],
  mention_unavailable_agent_ids = [],
  control_status_text,
  on_prepare_attachments,
  tour_anchor,
}: ComposerPanelProps) => {
  const { t } = useI18n();
  const [input, setInput] = useState("");
  const [input_history, setInputHistory] = useState<string[]>([]);
  const [history_index, setHistoryIndex] = useState(-1);
  const [history_draft, setHistoryDraft] = useState("");
  const [attachments, setAttachments] = useState<AttachmentFile[]>([]);
  const [attachment_error, setAttachmentError] = useState<string | null>(null);
  const [is_preparing_attachments, setIsPreparingAttachments] = useState(false);
  const [dragging_message_id, set_dragging_message_id] = useState<string | null>(null);
  const [drag_over_message_id, set_drag_over_message_id] = useState<string | null>(null);
  const [is_pending_queue_collapsed, set_is_pending_queue_collapsed] = useState(false);
  const [is_queue_action_running, set_is_queue_action_running] = useState(false);

  // 共享 Composer 同时服务 DM 和 Room，这里统一在共享层过滤不可提及成员，
  // 避免再保留第二套几乎相同的输入区实现。
  const available_room_members = room_members.filter(
    (member) => !mention_unavailable_agent_ids.includes(member.agent_id),
  );
  const mention_target_items = available_room_members.map<MentionTargetItem>((member) => ({
    id: member.agent_id,
    label: member.name,
    subtitle: null,
    kind: "agent",
  }));

  // @mention 状态
  const [mention_active, set_mention_active] = useState(false);
  const [mention_filter, set_mention_filter] = useState("");
  const [mention_start_pos, set_mention_start_pos] = useState(-1);

  const is_composing_ref = useRef(false);
  const ignore_next_enter_after_composition_ref = useRef(false);
  const last_composition_end_at_ref = useRef(0);
  const textarea_ref = useRef<HTMLTextAreaElement>(null);
  const file_input_ref = useRef<HTMLInputElement>(null);
  const pending_queue_scroll_ref = useRef<HTMLDivElement>(null);
  const pending_queue_drag_y_ref = useRef<number | null>(null);
  const pending_queue_scroll_frame_ref = useRef<number | null>(null);
  const dragging_message_id_ref = useRef<string | null>(null);
  const is_dispatching = is_loading && runtime_phase === "sending";
  const is_input_locked = disabled || (!allow_send_while_loading && is_loading);
  const can_stop_generation = is_loading && !is_dispatching && Boolean(on_stop);

  useTextareaHeight(textarea_ref, input, { min_height: 24, max_height: 200, line_height: 24, padding_y: 0 });

  const stop_pending_queue_auto_scroll = useCallback(() => {
    if (pending_queue_scroll_frame_ref.current !== null) {
      cancelAnimationFrame(pending_queue_scroll_frame_ref.current);
      pending_queue_scroll_frame_ref.current = null;
    }
    pending_queue_drag_y_ref.current = null;
  }, []);

  const run_pending_queue_auto_scroll = useCallback(() => {
    const container = pending_queue_scroll_ref.current;
    const pointer_y = pending_queue_drag_y_ref.current;
    if (!container || pointer_y === null || !dragging_message_id_ref.current) {
      pending_queue_scroll_frame_ref.current = null;
      return;
    }

    const rect = container.getBoundingClientRect();
    const distance_to_top = pointer_y - rect.top;
    const distance_to_bottom = rect.bottom - pointer_y;
    let delta = 0;

    if (distance_to_top < PENDING_QUEUE_AUTO_SCROLL_ZONE_PX) {
      const ratio = (PENDING_QUEUE_AUTO_SCROLL_ZONE_PX - Math.max(distance_to_top, 0)) / PENDING_QUEUE_AUTO_SCROLL_ZONE_PX;
      delta = -Math.ceil(ratio * PENDING_QUEUE_AUTO_SCROLL_MAX_DELTA_PX);
    } else if (distance_to_bottom < PENDING_QUEUE_AUTO_SCROLL_ZONE_PX) {
      const ratio = (PENDING_QUEUE_AUTO_SCROLL_ZONE_PX - Math.max(distance_to_bottom, 0)) / PENDING_QUEUE_AUTO_SCROLL_ZONE_PX;
      delta = Math.ceil(ratio * PENDING_QUEUE_AUTO_SCROLL_MAX_DELTA_PX);
    }

    if (delta !== 0) {
      container.scrollTop += delta;
    }
    pending_queue_scroll_frame_ref.current = requestAnimationFrame(run_pending_queue_auto_scroll);
  }, []);

  const start_pending_queue_auto_scroll = useCallback((client_y: number) => {
    pending_queue_drag_y_ref.current = client_y;
    if (pending_queue_scroll_frame_ref.current === null) {
      pending_queue_scroll_frame_ref.current = requestAnimationFrame(run_pending_queue_auto_scroll);
    }
  }, [run_pending_queue_auto_scroll]);

  useEffect(() => stop_pending_queue_auto_scroll, [stop_pending_queue_auto_scroll]);

  const handle_input_change = useCallback((value: string) => {
    setInput(value);
    if (attachment_error) {
      setAttachmentError(null);
    }

    if (available_room_members.length === 0) {
      set_mention_active(false);
      return;
    }

    const cursor_pos = textarea_ref.current?.selectionStart ?? value.length;
    const before_cursor = value.slice(0, cursor_pos);
    const at_index = before_cursor.lastIndexOf("@");

    if (at_index >= 0) {
      const char_before_at = at_index > 0 ? before_cursor[at_index - 1] : " ";
      if (char_before_at === " " || char_before_at === "\n" || at_index === 0) {
        const filter_text = before_cursor.slice(at_index + 1);
        if (!filter_text.includes(" ")) {
          set_mention_active(true);
          set_mention_filter(filter_text);
          set_mention_start_pos(at_index);
          return;
        }
      }
    }

    set_mention_active(false);
  }, [attachment_error, available_room_members.length]);

  const handle_mention_select = useCallback((agent: Agent) => {
    const before = input.slice(0, mention_start_pos);
    const cursor_pos = textarea_ref.current?.selectionStart ?? input.length;
    const after = input.slice(cursor_pos);
    const next_input = `${before}@${agent.name} ${after}`;
    setInput(next_input);
    set_mention_active(false);

    requestAnimationFrame(() => {
      const new_cursor = mention_start_pos + agent.name.length + 2;
      textarea_ref.current?.setSelectionRange(new_cursor, new_cursor);
      textarea_ref.current?.focus();
    });
  }, [input, mention_start_pos]);

  const handle_mention_close = useCallback(() => {
    set_mention_active(false);
  }, []);

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
  ) => {
    await on_send_message(content, policy);
  }, [on_send_message]);

  const handle_send = useCallback(async () => {
    const trimmed_input = input.trim();
    if (
      (!trimmed_input && attachments.length === 0) ||
      is_input_locked ||
      is_preparing_attachments
    ) {
      return;
    }

    let next_message = trimmed_input;
    if (attachments.length > 0) {
      if (!on_prepare_attachments) {
        setAttachmentError("当前会话暂不支持附件。");
        return;
      }

      setIsPreparingAttachments(true);
      setAttachmentError(null);
      try {
        const prepared_attachments = await on_prepare_attachments(attachments.map((attachment) => attachment.file));
        next_message = build_message_with_attachments(trimmed_input, prepared_attachments);
      } catch (error) {
        setAttachmentError(error instanceof Error ? error.message : "附件整理失败，请稍后重试。");
        return;
      } finally {
        setIsPreparingAttachments(false);
      }
    }

    if (trimmed_input) {
      setInputHistory((prev) => [trimmed_input, ...prev.slice(0, 49)]);
    }
    setHistoryIndex(-1);
    setHistoryDraft("");

    try {
      if (is_loading || input_queue_items.length > 0) {
        if (!on_enqueue_message) {
          return;
        }
        await on_enqueue_message?.(next_message, default_delivery_policy);
      } else {
        await dispatch_message(next_message, default_delivery_policy);
      }
      setInput("");
      setAttachments([]);
      setAttachmentError(null);
    } catch (error) {
      console.error("发送消息失败:", error);
      return;
    }

    if (textarea_ref.current) {
      textarea_ref.current.style.height = "auto";
    }
  }, [
    attachments,
    default_delivery_policy,
    dispatch_message,
    input_queue_items.length,
    input,
    is_input_locked,
    is_loading,
    is_preparing_attachments,
    on_enqueue_message,
    on_prepare_attachments,
  ]);

  const remove_pending_message = useCallback(async (id: string) => {
    await on_delete_queued_message?.(id);
  }, [on_delete_queued_message]);

  const guide_pending_message = useCallback(async (message: InputQueueItem) => {
    if (disabled || is_queue_action_running) {
      return;
    }
    try {
      set_is_queue_action_running(true);
      await on_guide_queued_message?.(message.id);
    } catch (error) {
      console.error("引导队列消息失败:", error);
    } finally {
      set_is_queue_action_running(false);
    }
  }, [
    disabled,
    is_queue_action_running,
    on_guide_queued_message,
  ]);

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
    setAttachmentError(null);
  }, [history_index, input, input_history]);

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

  const handle_file_select = (event: ChangeEvent<HTMLInputElement>) => {
    const files = event.target.files;
    if (!files) {
      return;
    }

    const next_attachments: AttachmentFile[] = [];
    const rejected_files: string[] = [];

    Array.from(files).forEach((file) => {
      const rejection_reason = get_attachment_rejection_reason(file);
      if (rejection_reason) {
        rejected_files.push(rejection_reason);
        return;
      }

      next_attachments.push({
        id: `${Date.now()}-${Math.random().toString(36).slice(2, 11)}`,
        file,
      });
    });

    if (rejected_files.length > 0) {
      setAttachmentError(rejected_files[0] ?? "附件格式不受支持。");
    } else {
      setAttachmentError(null);
    }

    if (next_attachments.length > 0) {
      setAttachments((prev) => [...prev, ...next_attachments].slice(0, 6));
    }

    if (file_input_ref.current) {
      file_input_ref.current.value = "";
    }
  };

  const remove_attachment = (id: string) => {
    setAttachments((prev) => prev.filter((item) => item.id !== id));
  };

  const is_input_empty = input.trim().length === 0 && attachments.length === 0;
  const char_count = input.length;
  const is_near_limit = char_count > max_length * 0.8;
  const is_over_limit = char_count > max_length;
  const is_send_disabled =
    is_input_empty || is_input_locked || is_over_limit || is_preparing_attachments;
  const should_show_stop_button =
    can_stop_generation && (!allow_send_while_loading || is_input_empty);

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
        aria-label="选择附件文件"
        className="hidden"
        multiple
        onChange={handle_file_select}
        type="file"
      />

      <div className={get_composer_shell_class_name(is_input_locked)} style={get_composer_shell_style(compact)}>
        {input_queue_items.length > 0 ? (
          <div className={cn("border-b border-(--surface-canvas-border)", compact ? "px-2 py-1" : "px-3 py-1.5")}>
            <div className="flex items-center justify-between gap-2 text-[10px] font-medium text-(--text-soft)">
              <span className="inline-flex items-center gap-1.5">
                待发送队列
                <span className="tabular-nums">{input_queue_items.length}</span>
              </span>
              <button
                aria-label={is_pending_queue_collapsed ? "展开待发送队列" : "收起待发送队列"}
                className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-md text-(--text-soft) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
                onClick={() => set_is_pending_queue_collapsed((current) => !current)}
                type="button"
              >
                {is_pending_queue_collapsed ? (
                  <ChevronDown className="h-3 w-3" />
                ) : (
                  <ChevronUp className="h-3 w-3" />
                )}
              </button>
            </div>
            <div className={cn(
              "soft-scrollbar flex max-h-[112px] flex-col divide-y divide-(--divider-subtle-color) overflow-y-auto pr-1",
              is_pending_queue_collapsed ? "hidden" : "mt-0.5",
            )}
              onDragOver={(event) => {
                event.preventDefault();
                start_pending_queue_auto_scroll(event.clientY);
              }}
              ref={pending_queue_scroll_ref}
            >
              {input_queue_items.map((message) => {
                const is_dragging = dragging_message_id === message.id;
                const is_drag_target = Boolean(
                  dragging_message_id
                    && dragging_message_id !== message.id
                    && drag_over_message_id === message.id,
                );
                return (
                  <div
                    key={message.id}
                    draggable
                    className={cn(
                      "group -mx-1 flex min-h-8 items-center gap-2 px-1 py-1 text-(--text-default) transition-[background,box-shadow,opacity]",
                      is_dragging && "opacity-60",
                      is_drag_target && "bg-(--surface-interactive-hover-background) shadow-[inset_3px_0_0_var(--primary)]",
                    )}
                    onDragOver={(event) => {
                      event.preventDefault();
                      start_pending_queue_auto_scroll(event.clientY);
                      if (drag_over_message_id !== message.id) {
                        set_drag_over_message_id(message.id);
                      }
                    }}
                    onDragStart={() => {
                      dragging_message_id_ref.current = message.id;
                      set_dragging_message_id(message.id);
                    }}
                    onDragEnd={() => {
                      dragging_message_id_ref.current = null;
                      stop_pending_queue_auto_scroll();
                      set_dragging_message_id(null);
                      set_drag_over_message_id(null);
                    }}
                    onDrop={(event) => {
                      event.preventDefault();
                      if (!dragging_message_id) {
                        return;
                      }
                      const next_items = reorder_pending_messages(
                        input_queue_items,
                        dragging_message_id,
                        message.id,
                      );
                      void on_reorder_queue_messages?.(next_items.map((item) => item.id));
                      dragging_message_id_ref.current = null;
                      stop_pending_queue_auto_scroll();
                      set_dragging_message_id(null);
                      set_drag_over_message_id(null);
                    }}
                  >
                    <span
                      aria-label="拖动调整顺序"
                      className="inline-flex h-5 w-3.5 shrink-0 cursor-grab items-center justify-center text-(--text-soft) active:cursor-grabbing"
                    >
                      <GripVertical className="h-3.5 w-3.5" />
                    </span>
                    <p className="line-clamp-1 min-w-0 flex-1 text-[12px] leading-5 text-(--text-strong)">
                      {message.content}
                    </p>
                    <button
                      aria-label="引导当前 round"
                      className="inline-flex h-6 shrink-0 items-center justify-center gap-1 px-1 text-[11px] font-semibold text-(--text-soft) transition-colors hover:text-(--text-strong) disabled:pointer-events-none disabled:opacity-(--disabled-opacity)"
                      disabled={disabled || is_queue_action_running}
                      onClick={() => {
                        void guide_pending_message(message);
                      }}
                      type="button"
                    >
                      <CornerDownRight className="h-3 w-3" />
                      引导
                    </button>
                    <button
                      aria-label="删除待发送消息"
                      className="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-(--text-soft) transition-colors hover:text-(--destructive)"
                      onClick={() => {
                        void remove_pending_message(message.id);
                      }}
                      type="button"
                    >
                      <Trash2 className="h-3 w-3" />
                    </button>
                  </div>
                );
              })}
            </div>
          </div>
        ) : null}

        {attachments.length > 0 ? (
          <div className={COMPOSER_ATTACHMENT_ROW_CLASS_NAME}>
            {attachments.map((attachment) => (
              <div key={attachment.id} className={COMPOSER_ATTACHMENT_CLASS_NAME}>
                <FileText size={16} className="text-accent" />
                <span className="max-w-[120px] truncate text-xs text-foreground/70">
                  {attachment.file.name}
                </span>
                <button
                  aria-label="移除附件"
                  className={COMPOSER_ATTACHMENT_REMOVE_CLASS_NAME}
                  onClick={() => remove_attachment(attachment.id)}
                >
                  <X size={12} />
                </button>
              </div>
            ))}
          </div>
        ) : null}

        <div className={cn("flex items-end gap-2", compact ? "px-2 py-2" : "px-3 py-3")}>
          <button
            aria-label="添加附件"
            className={COMPOSER_ACTION_BUTTON_CLASS_NAME}
            disabled={is_input_locked || is_preparing_attachments}
            onClick={() => file_input_ref.current?.click()}
            type="button"
          >
            <Paperclip size={16} />
          </button>

          {mention_active && mention_target_items.length > 0 ? (
            <MentionTargetPopover
              anchor_rect={textarea_ref.current?.getBoundingClientRect() ?? null}
              filter={mention_filter}
              items={mention_target_items}
              on_close={handle_mention_close}
              on_select={(item) => {
                const selected_member = available_room_members.find((member) => member.agent_id === item.id);
                if (selected_member) {
                  handle_mention_select(selected_member);
                }
              }}
              placement="above"
            />
          ) : null}

          <textarea
            ref={textarea_ref}
            className={cn(
              "multiline-cursor soft-scrollbar min-h-6 min-w-0 flex-1 max-h-[200px] resize-none overflow-y-auto overscroll-contain bg-transparent text-[14px] leading-6 text-(--text-strong) outline-none shadow-none ring-0",
              "placeholder:text-(--text-soft)",
              "disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)",
              "focus:border-0 focus:bg-transparent focus:outline-none focus:ring-0 focus-visible:outline-none focus-visible:ring-0 focus-visible:shadow-none",
            )}
            disabled={is_input_locked}
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
            placeholder={placeholder}
            rows={1}
            value={input}
          />

          {should_show_stop_button ? (
            <button
              aria-label="停止生成"
              className={COMPOSER_DANGER_ACTION_BUTTON_CLASS_NAME}
              onClick={on_stop}
              type="button"
            >
              <StopCircle size={16} />
            </button>
          ) : (
            <button
              aria-label="发送消息"
              className={COMPOSER_PRIMARY_ACTION_BUTTON_CLASS_NAME}
              disabled={is_send_disabled}
              onClick={() => {
                void handle_send();
              }}
              type="button"
            >
              {is_preparing_attachments ? (
                <LoadingOrb frames={["·", "◦", "•", "◦"]} />
              ) : (
                <Send size={16} />
              )}
            </button>
          )}
        </div>

        <div className={COMPOSER_FOOTER_CLASS_NAME}>
          <div className="flex items-center gap-3 text-[10px] text-(--text-soft)">
            {disabled && control_status_text ? (
              <span className="text-(--text-default)">{control_status_text}</span>
            ) : is_dispatching ? (
              <span className="flex items-center gap-2 text-emerald-900/90">
                <LoadingOrb frames={["✽", "✻", "✶", "✢", "·"]} />
                <span className="animate-pulse">{t("status.sending")}</span>
              </span>
            ) : can_stop_generation ? (
              <span className="flex items-center gap-2 text-emerald-900/90">
                <LoadingOrb frames={["✽", "✻", "✶", "✢", "·"]} />
                <span className="animate-pulse">{t("status.replying")}…</span>
                <span className="text-(--text-soft)">[ESC 停止]</span>
              </span>
            ) : is_preparing_attachments ? (
              <span className="flex items-center gap-2 text-(--text-default)">
                <LoadingOrb frames={["·", "◦", "•", "◦"]} />
                <span>正在整理附件并同步到工作区…</span>
              </span>
            ) : attachment_error ? (
              <span className="text-(--destructive)">{attachment_error}</span>
            ) : (
              <>
                <span className="flex items-center gap-1">
                  <kbd>Enter</kbd>
                  <span>{is_loading || input_queue_items.length > 0 ? "入队" : "发送"}</span>
                </span>
                <span className="flex items-center gap-1">
                  <kbd>Shift</kbd>
                  <span>+</span>
                  <kbd>Enter</kbd>
                  <span>换行</span>
                </span>
                <span className="hidden sm:inline text-(--text-soft)">
                  附件仅支持文本文件
                </span>
              </>
            )}
            {!disabled && control_status_text ? (
              <span className="text-(--text-default)">{control_status_text}</span>
            ) : null}
          </div>

          <div className="flex items-center gap-3 text-[10px] tabular-nums">
            {char_count > 0 ? (
              <div>
                <span
                  className={cn(
                    is_over_limit && "text-destructive",
                    is_near_limit && !is_over_limit && "text-warning",
                    !is_near_limit && "text-(--text-soft)",
                  )}
                >
                  {char_count}
                </span>
                <span className="text-(--text-soft)">/{max_length}</span>
              </div>
            ) : null}
            {history_index >= 0 ? (
              <div className="text-[10px] text-(--text-default)">
                历史 {history_index + 1}/{input_history.length}
              </div>
            ) : null}
          </div>
        </div>
      </div>
    </section>
  );
});

ComposerPanelView.displayName = "ComposerPanelView";

export function ComposerPanel(props: ComposerPanelProps) {
  return <ComposerPanelView {...props} />;
}
