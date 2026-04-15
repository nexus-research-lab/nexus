"use client";

import { ChangeEvent, KeyboardEvent, memo, useCallback, useEffect, useRef, useState } from "react";
import { FileText, Paperclip, Send, StopCircle, X } from "lucide-react";

import { useTextareaHeight } from "@/hooks/use-textarea-height";
import { cn } from "@/lib/utils";
import { LoadingOrb } from "@/shared/ui/feedback/loading-orb";
import { Agent } from "@/types/agent";

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
import { MentionPopover } from "./mention-popover";

interface AttachmentFile {
  id: string;
  file: File;
}

interface ComposerPanelProps {
  compact: boolean;
  is_loading?: boolean;
  on_send_message: (content: string) => void | Promise<void>;
  on_stop?: () => void;
  initial_draft?: string | null;
  disabled?: boolean;
  placeholder?: string;
  max_length?: number;
  room_members?: Agent[];
  mention_unavailable_agent_ids?: string[];
  control_status_text?: string;
  on_prepare_attachments?: (files: File[]) => Promise<PreparedComposerAttachment[]>;
}

type ComposerNativeKeyboardEvent = globalThis.KeyboardEvent & {
  keyCode?: number;
  which?: number;
};

const IME_COMPOSITION_KEY_CODE = 229;
const COMPOSITION_END_ENTER_GUARD_MS = 80;

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

const ComposerPanelView = memo(({
  compact,
  is_loading = false,
  on_send_message,
  on_stop,
  initial_draft = null,
  disabled = false,
  placeholder = "继续描述目标、补充上下文，或直接开始协作…",
  max_length = 10000,
  room_members = [],
  mention_unavailable_agent_ids = [],
  control_status_text,
  on_prepare_attachments,
}: ComposerPanelProps) => {
  const [input, setInput] = useState("");
  const [input_history, setInputHistory] = useState<string[]>([]);
  const [history_index, setHistoryIndex] = useState(-1);
  const [history_draft, setHistoryDraft] = useState("");
  const [attachments, setAttachments] = useState<AttachmentFile[]>([]);
  const [attachment_error, setAttachmentError] = useState<string | null>(null);
  const [is_preparing_attachments, setIsPreparingAttachments] = useState(false);

  // 中文注释：共享 Composer 同时服务 DM 和 Room，这里统一在共享层过滤不可提及成员，
  // 避免再保留第二套几乎相同的输入区实现。
  const available_room_members = room_members.filter(
    (member) => !mention_unavailable_agent_ids.includes(member.agent_id),
  );

  // @mention 状态
  const [mention_active, set_mention_active] = useState(false);
  const [mention_filter, set_mention_filter] = useState("");
  const [mention_start_pos, set_mention_start_pos] = useState(-1);

  const is_composing_ref = useRef(false);
  const ignore_next_enter_after_composition_ref = useRef(false);
  const last_composition_end_at_ref = useRef(0);
  const textarea_ref = useRef<HTMLTextAreaElement>(null);
  const file_input_ref = useRef<HTMLInputElement>(null);

  useTextareaHeight(textarea_ref, input, { min_height: 24, max_height: 200, line_height: 24, padding_y: 0 });

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
    if (textarea_ref.current && !disabled) {
      textarea_ref.current.focus();
    }
  }, [disabled]);

  useEffect(() => {
    const normalized_draft = initial_draft?.trim() ?? "";
    if (!normalized_draft) {
      return;
    }
    setInput((current_value) => current_value || normalized_draft);
  }, [initial_draft]);

  const handle_send = useCallback(async () => {
    const trimmed_input = input.trim();
    if ((!trimmed_input && attachments.length === 0) || disabled || is_loading || is_preparing_attachments) {
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

    await on_send_message(next_message);
    setInput("");
    setAttachments([]);
    setAttachmentError(null);

    if (textarea_ref.current) {
      textarea_ref.current.style.height = "auto";
    }
  }, [
    attachments,
    disabled,
    input,
    is_loading,
    is_preparing_attachments,
    on_prepare_attachments,
    on_send_message,
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

    // 中文注释：Safari 在中文输入法确认候选词后，可能补发一个不带 composing 标记的 Enter。
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
    is_input_empty || disabled || is_over_limit || is_preparing_attachments;

  return (
    <section
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

      <div className={get_composer_shell_class_name(disabled)} style={get_composer_shell_style(compact)}>
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
            disabled={disabled || is_loading}
            onClick={() => file_input_ref.current?.click()}
            type="button"
          >
            <Paperclip size={16} />
          </button>

          {mention_active && available_room_members.length > 0 ? (
            <MentionPopover
              anchor_rect={textarea_ref.current?.getBoundingClientRect() ?? null}
              filter={mention_filter}
              members={available_room_members}
              on_close={handle_mention_close}
              on_select={handle_mention_select}
            />
          ) : null}

          <textarea
            ref={textarea_ref}
            className={cn(
              "multiline-cursor min-h-6 min-w-0 flex-1 max-h-[200px] resize-none overflow-y-auto bg-transparent text-[14px] leading-6 text-(--text-strong) outline-none shadow-none ring-0",
              "placeholder:text-(--text-soft)",
              "disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)",
              "focus:border-0 focus:bg-transparent focus:outline-none focus:ring-0 focus-visible:outline-none focus-visible:ring-0 focus-visible:shadow-none",
            )}
            disabled={disabled || is_loading}
            onChange={(event) => handle_input_change(event.target.value)}
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

          {is_loading && on_stop ? (
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
            ) : is_loading && on_stop ? (
              <span className="flex items-center gap-2 text-emerald-900/90">
                <LoadingOrb frames={["✽", "✻", "✶", "✢", "·"]} />
                <span className="animate-pulse">正在回复中…</span>
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
                  <span>发送</span>
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
