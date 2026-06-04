"use client";

import { ChangeEvent, ClipboardEvent, KeyboardEvent, memo, type ReactNode, useCallback, useEffect, useRef, useState } from "react";
import {
  ChevronDown,
  ChevronUp,
  CornerDownRight,
  File as FileIcon,
  FileText,
  GripVertical,
  Image as ImageIcon,
  Plus,
  Paperclip,
  Send,
  StopCircle,
  Target,
  Trash2,
  X,
} from "lucide-react";

import { useTextareaHeight } from "@/hooks/ui/use-textarea-height";
import { cn } from "@/lib/utils";
import { LoadingOrb } from "@/shared/ui/feedback/loading-orb";
import { GlassSwitch } from "@/shared/ui/liquid-glass";
import { UiActionMenu } from "@/shared/ui/action-menu";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  AgentConversationDefaultDeliveryPolicy,
  AgentConversationDeliveryPolicy,
  AgentConversationRuntimePhase,
  InputQueueItem,
} from "@/types/agent/agent-conversation";
import { Agent } from "@/types/agent/agent";

import {
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
  ComposerAttachmentKind,
  get_composer_attachment_kind,
  get_attachment_rejection_reason,
  PreparedComposerAttachment,
} from "./composer-attachments";
import { MentionTargetItem, MentionTargetPopover } from "./mention-popover";

interface AttachmentFile {
  id: string;
  file: File;
  kind: ComposerAttachmentKind;
}

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
const PENDING_QUEUE_AUTO_SCROLL_ZONE_PX = 28;
const PENDING_QUEUE_AUTO_SCROLL_MAX_DELTA_PX = 10;
const MAX_COMPOSER_ATTACHMENTS = 6;
type ComposerInputMode = "message" | "goal";

const CLIPBOARD_IMAGE_EXTENSION_BY_MIME: Record<string, string> = {
  "image/png": "png",
  "image/jpeg": "jpg",
  "image/webp": "webp",
  "image/gif": "gif",
  "image/bmp": "bmp",
  "image/svg+xml": "svg",
};

function create_attachment_id() {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 11)}`;
}

function get_attachment_kind_label(kind: ComposerAttachmentKind) {
  if (kind === "image") {
    return "图片";
  }
  if (kind === "text") {
    return "文本文件";
  }
  return "工作文件";
}

function get_attachment_icon(kind: ComposerAttachmentKind) {
  if (kind === "image") {
    return ImageIcon;
  }
  if (kind === "text") {
    return FileText;
  }
  return FileIcon;
}

function build_pasted_image_file(file: File, index: number): File {
  if (!file.type.startsWith("image/")) {
    return file;
  }

  const extension = CLIPBOARD_IMAGE_EXTENSION_BY_MIME[file.type] ?? "png";
  const timestamp = new Date().toISOString().replace(/[:.]/g, "-");
  return new File(
    [file],
    `pasted-image-${timestamp}-${index + 1}.${extension}`,
    {
      lastModified: Date.now(),
      type: file.type,
    },
  );
}

function get_clipboard_files(clipboard_data: DataTransfer): File[] {
  const files_from_items = Array.from(clipboard_data.items)
    .filter((item) => item.kind === "file")
    .map((item) => item.getAsFile())
    .filter((file): file is File => Boolean(file))
    .map(build_pasted_image_file);

  if (files_from_items.length > 0) {
    return files_from_items;
  }

  return Array.from(clipboard_data.files).map(build_pasted_image_file);
}

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
  queue_when_session_busy = true,
  placeholder,
  max_length = 10000,
  room_members = [],
  mention_unavailable_agent_ids = [],
  on_prepare_attachments,
  on_create_goal,
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
  const [attachments, setAttachments] = useState<AttachmentFile[]>([]);
  const [attachment_error, setAttachmentError] = useState<string | null>(null);
  const [is_preparing_attachments, setIsPreparingAttachments] = useState(false);
  const [dragging_message_id, set_dragging_message_id] = useState<string | null>(null);
  const [drag_over_message_id, set_drag_over_message_id] = useState<string | null>(null);
  const [is_pending_queue_collapsed, set_is_pending_queue_collapsed] = useState(false);
  const [is_queue_action_running, set_is_queue_action_running] = useState(false);
  const [is_action_menu_open, set_is_action_menu_open] = useState(false);
  const [is_goal_creating, set_is_goal_creating] = useState(false);
  const [goal_error, set_goal_error] = useState<string | null>(null);

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
  const action_button_ref = useRef<HTMLButtonElement>(null);
  const pending_queue_scroll_ref = useRef<HTMLDivElement>(null);
  const pending_queue_drag_y_ref = useRef<number | null>(null);
  const pending_queue_scroll_frame_ref = useRef<number | null>(null);
  const dragging_message_id_ref = useRef<string | null>(null);
  const is_dispatching = is_loading && runtime_phase === "sending";
  const is_input_locked = disabled || (!allow_send_while_loading && is_loading);
  const is_textarea_locked = is_input_locked || (is_goal_mode && is_goal_creating);
  const can_stop_generation = is_loading && !is_dispatching && Boolean(on_stop);
  const can_create_goal = Boolean(on_create_goal);
  const goal_create_blocked_reason =
    goal_create_disabled_reason?.trim() || null;

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
    if (goal_error) {
      set_goal_error(null);
    }

    if (is_goal_mode) {
      set_mention_active(false);
      return;
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
  }, [attachment_error, available_room_members.length, goal_error, is_goal_mode]);

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

    let prepared_attachments: PreparedComposerAttachment[] = [];
    if (attachments.length > 0) {
      if (!on_prepare_attachments) {
        setAttachmentError(t("composer.unsupported_attachment"));
        return;
      }

      setIsPreparingAttachments(true);
      setAttachmentError(null);
      try {
        prepared_attachments = await on_prepare_attachments(attachments.map((attachment) => attachment.file));
      } catch (error) {
        setAttachmentError(error instanceof Error ? error.message : t("composer.attachment_failed"));
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
    on_prepare_attachments,
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
    set_mention_active(false);
    requestAnimationFrame(() => textarea_ref.current?.focus());
  }, [can_create_goal]);

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

  const append_attachment_files = useCallback((files: File[]) => {
    if (files.length === 0) {
      return;
    }

    const next_attachments: AttachmentFile[] = [];
    const rejected_files: string[] = [];

    files.forEach((file) => {
      const rejection_reason = get_attachment_rejection_reason(file);
      if (rejection_reason) {
        rejected_files.push(rejection_reason);
        return;
      }

      const kind = get_composer_attachment_kind(file);
      if (!kind) {
        rejected_files.push(t("composer.attachment_format_unsupported"));
        return;
      }

      next_attachments.push({
        id: create_attachment_id(),
        file,
        kind,
      });
    });

    if (rejected_files.length > 0) {
      setAttachmentError(rejected_files[0] ?? t("composer.attachment_format_unsupported"));
    } else {
      setAttachmentError(null);
    }

    if (next_attachments.length > 0) {
      setAttachments((prev) => [...prev, ...next_attachments].slice(0, MAX_COMPOSER_ATTACHMENTS));
    }
  }, [t]);

  const handle_file_select = (event: ChangeEvent<HTMLInputElement>) => {
    const files = event.target.files;
    if (!files) {
      return;
    }

    append_attachment_files(Array.from(files));

    if (file_input_ref.current) {
      file_input_ref.current.value = "";
    }
  };

  const handle_paste = useCallback((event: ClipboardEvent<HTMLTextAreaElement>) => {
    const pasted_files = get_clipboard_files(event.clipboardData);
    if (pasted_files.length === 0) {
      return;
    }

    event.preventDefault();
    if (is_goal_mode) {
      set_goal_error(t("composer.goal_attachment_unsupported"));
      return;
    }
    append_attachment_files(pasted_files);
  }, [append_attachment_files, is_goal_mode, t]);

  const remove_attachment = (id: string) => {
    setAttachments((prev) => prev.filter((item) => item.id !== id));
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

      <div className={get_composer_shell_class_name(is_input_locked)} style={get_composer_shell_style(compact)}>
        {has_pending_queue ? (
          <div
            className={cn(
              "border-b border-(--surface-canvas-border)",
              compact ? "px-2 pb-0.5 pt-1" : "px-3 pb-1 pt-1",
            )}
          >
            <div className="flex items-center justify-between gap-2 text-[10px] font-medium text-(--text-soft)">
              <span className="inline-flex items-center gap-1.5">
                {t("composer.pending_queue")}
                <span className="tabular-nums">{input_queue_items.length}</span>
              </span>
              <button
                aria-label={is_pending_queue_collapsed ? t("composer.expand_pending_queue") : t("composer.collapse_pending_queue")}
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
                const is_guidance_waiting = message.delivery_policy === "guide";
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
                      "group -mx-1 flex min-h-7 items-center gap-2 px-1 py-0.5 text-(--text-default) transition-[background,box-shadow,opacity]",
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
                      aria-label={t("composer.drag_to_reorder")}
                      className="inline-flex h-5 w-3.5 shrink-0 cursor-grab items-center justify-center text-(--text-soft) active:cursor-grabbing"
                    >
                      <GripVertical className="h-3.5 w-3.5" />
                    </span>
                    <p className="line-clamp-1 min-w-0 flex-1 text-[12px] leading-5 text-(--text-strong)">
                      {message.content.trim() ? (
                        message.content
                      ) : message.attachments && message.attachments.length > 0 ? (
                        <span className="inline-flex items-center gap-1 text-(--text-muted)">
                          <Paperclip className="h-3 w-3 shrink-0" />
                          {message.attachments.map((attachment) => attachment.file_name || attachment.workspace_path).join("、")}
                        </span>
                      ) : null}
                    </p>
                    <button
                      aria-label={is_guidance_waiting ? t("composer.cancel_guidance") : t("composer.mark_guidance")}
                      className="inline-flex h-6 shrink-0 items-center justify-center gap-1 px-1 text-[11px] font-semibold text-(--text-soft) transition-colors hover:text-(--text-strong) disabled:pointer-events-none disabled:opacity-(--disabled-opacity)"
                      disabled={disabled || is_queue_action_running}
                      onClick={() => {
                        void guide_pending_message(message);
                      }}
                      type="button"
                    >
                      <CornerDownRight className="h-3 w-3" />
                      {is_guidance_waiting ? t("composer.cancel_guide_action") : t("composer.guide_action")}
                    </button>
                    <button
                      aria-label={t("composer.delete_pending")}
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
              <div
                key={attachment.id}
                className={COMPOSER_ATTACHMENT_CLASS_NAME}
                title={`${get_attachment_kind_label(attachment.kind)}：${attachment.file.name}`}
              >
                {(() => {
                  const AttachmentIcon = get_attachment_icon(attachment.kind);
                  return <AttachmentIcon size={16} className="text-accent" />;
                })()}
                <span className="max-w-[120px] truncate text-xs text-foreground/70">
                  {attachment.file.name}
                </span>
                <button
                  aria-label={t("composer.remove_attachment")}
                  className={COMPOSER_ATTACHMENT_REMOVE_CLASS_NAME}
                  onClick={() => remove_attachment(attachment.id)}
                >
                  <X size={12} />
                </button>
              </div>
            ))}
          </div>
        ) : null}

        <div className={cn("flex items-end gap-2", composer_input_row_padding_class)}>
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

        <div className={COMPOSER_FOOTER_CLASS_NAME}>
          <div className="flex min-w-0 items-center gap-2 text-[10px] text-(--text-soft)">
            <div className="shrink-0">
              <button
                ref={action_button_ref}
                aria-expanded={is_action_menu_open}
                aria-haspopup="menu"
                aria-label={t("composer.open_actions")}
                className="inline-flex h-6 w-6 items-center justify-center rounded-[8px] text-(--icon-default) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) disabled:pointer-events-none disabled:opacity-(--disabled-opacity)"
                disabled={is_input_locked}
                onClick={() => set_is_action_menu_open((current) => !current)}
                type="button"
              >
                <Plus className="h-4 w-4" />
              </button>
              <UiActionMenu
                anchor_ref={action_button_ref}
                aria_label={t("composer.open_actions")}
                is_open={is_action_menu_open}
                items={[
                  {
                    value: "attachment",
                    label: t("composer.add_attachment"),
                    icon: <Paperclip className="h-4 w-4 text-(--icon-muted)" />,
                    disabled: is_input_locked || is_preparing_attachments || is_goal_mode,
                  },
                  {
                    value: "goal",
                    label: t("composer.start_goal"),
                    icon: <Target className="h-4 w-4 text-(--primary)" />,
                    trailing: (
                      <span
                        onClick={(event) => event.stopPropagation()}
                        onKeyDown={(event) => event.stopPropagation()}
                      >
                        <GlassSwitch
                          checked={is_goal_mode}
                          disabled={!can_create_goal || is_input_locked || is_goal_creating}
                          on_change={toggle_goal_input}
                          size="xs"
                        />
                      </span>
                    ),
                    active: is_goal_mode,
                    disabled: !can_create_goal || is_input_locked || is_goal_creating,
                    tone: "primary",
                  },
                ]}
                placement="top"
                on_close={() => set_is_action_menu_open(false)}
                on_select={(value) => {
                  if (value === "attachment") {
                    open_attachment_picker();
                    return;
                  }
                  if (value === "goal") {
                    toggle_goal_input(!is_goal_mode);
                  }
                }}
              />
            </div>

            {is_goal_mode ? (
              <span className="inline-flex min-w-0 items-center gap-1.5 font-semibold text-(--primary)">
                <Target className="h-3.5 w-3.5 shrink-0" />
                <span>{t("composer.goal_mode")}</span>
                <span className="truncate font-medium text-(--text-muted)">{goal_scope_label}</span>
                {goal_mode_extra}
                <button
                  aria-label={t("composer.cancel_goal_mode")}
                  className="pointer-events-auto inline-flex h-4 w-4 shrink-0 items-center justify-center rounded-[5px] text-(--text-soft) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
                  disabled={is_goal_creating}
                  onClick={cancel_goal_input}
                  type="button"
                >
                  <X className="h-3 w-3" />
                </button>
              </span>
            ) : null}

            {is_dispatching ? (
              <span className="flex items-center gap-2 text-emerald-900/90">
                <LoadingOrb frames={["✽", "✻", "✶", "✢", "·"]} />
                <span className="animate-pulse">{t("status.sending")}</span>
              </span>
            ) : can_stop_generation ? (
              <span className="flex items-center gap-2 text-emerald-900/90">
                <LoadingOrb frames={["✽", "✻", "✶", "✢", "·"]} />
                <span className="animate-pulse">{t("status.replying")}…</span>
                <span className="text-(--text-soft)">[{t("composer.esc_stop")}]</span>
              </span>
            ) : is_preparing_attachments ? (
              <span className="flex items-center gap-2 text-(--text-default)">
                <LoadingOrb frames={["·", "◦", "•", "◦"]} />
                <span>{t("composer.preparing_attachments")}</span>
              </span>
            ) : is_goal_creating ? (
              <span className="flex items-center gap-2 text-(--primary)">
                <LoadingOrb frames={["·", "◦", "•", "◦"]} />
                <span className="animate-pulse">{t("composer.goal_normalizing")}</span>
              </span>
            ) : active_error ? (
              <span className="text-(--destructive)">{active_error}</span>
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
                {t("composer.history_position", {
                  current: history_index + 1,
                  total: input_history.length,
                })}
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
