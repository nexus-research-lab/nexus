"use client";

import { KeyboardEvent, memo, useCallback, useEffect, useRef, useState } from "react";
import { FileText, Image as ImageIcon, Paperclip, Send, X } from "lucide-react";

import { cn } from "@/lib/utils";
import {
  COMPOSER_ATTACHMENT_CLASS_NAME,
  COMPOSER_ATTACHMENT_REMOVE_CLASS_NAME,
  COMPOSER_FOOTER_CLASS_NAME,
  getComposerShellClassName,
  getComposerShellStyle,
} from "@/features/conversation-shared/composer-styles";
import { useTextareaHeight } from "@/hooks/use-textarea-height";
import { WorkspacePillButton } from "@/shared/ui/workspace/workspace-pill-button";
import { Agent } from "@/types/agent";

import { MentionPopover } from "@/features/conversation-shared/mention-popover";

interface AttachmentFile {
  id: string;
  file: File;
  preview?: string;
  type: "image" | "document";
}

interface RoomComposerPanelProps {
  compact: boolean;
  on_send_message: (content: string) => void | Promise<void>;
  initial_draft?: string | null;
  disabled?: boolean;
  placeholder?: string;
  max_length?: number;
  room_members?: Agent[];
  mention_unavailable_agent_ids?: string[];
  control_status_text?: string;
}

const RoomComposerPanelView = memo(({
  compact,
  on_send_message,
  initial_draft = null,
  disabled = false,
  placeholder = "继续描述目标、补充上下文，或直接开始协作…",
  max_length = 10000,
  room_members = [],
  mention_unavailable_agent_ids = [],
  control_status_text,
}: RoomComposerPanelProps) => {
  const [input, setInput] = useState("");
  const [input_history, setInputHistory] = useState<string[]>([]);
  const [history_index, setHistoryIndex] = useState(-1);
  const [attachments, setAttachments] = useState<AttachmentFile[]>([]);

  // @mention 状态
  const [mention_active, set_mention_active] = useState(false);
  const [mention_filter, set_mention_filter] = useState("");
  const [mention_start_pos, set_mention_start_pos] = useState(-1);

  const is_composing_ref = useRef(false);
  const textarea_ref = useRef<HTMLTextAreaElement>(null);
  const file_input_ref = useRef<HTMLInputElement>(null);
  const available_room_members = room_members.filter(
    (member) => !mention_unavailable_agent_ids.includes(member.agent_id),
  );

  // Pretext-based auto-height: no scrollHeight reflow
  useTextareaHeight(textarea_ref, input, { minHeight: 24, maxHeight: 128, lineHeight: 24, paddingY: 0 });

  // @mention：检测 @ 字符并追踪过滤文本
  const handle_input_change = useCallback((value: string) => {
    setInput(value);

    if (available_room_members.length === 0) {
      set_mention_active(false);
      return;
    }

    const cursor_pos = textarea_ref.current?.selectionStart ?? value.length;
    // 从光标位置往前找最近的 @
    const before_cursor = value.slice(0, cursor_pos);
    const at_index = before_cursor.lastIndexOf("@");

    if (at_index >= 0) {
      // @ 前面必须是空格、换行或行首
      const char_before_at = at_index > 0 ? before_cursor[at_index - 1] : " ";
      if (char_before_at === " " || char_before_at === "\n" || at_index === 0) {
        const filter_text = before_cursor.slice(at_index + 1);
        // 不包含空格意味着还在输入名字
        if (!filter_text.includes(" ")) {
          set_mention_active(true);
          set_mention_filter(filter_text);
          set_mention_start_pos(at_index);
          return;
        }
      }
    }

    set_mention_active(false);
  }, [available_room_members.length]);

  const handle_mention_select = useCallback((agent: Agent) => {
    // 把 @filter 替换为 @AgentName + 空格
    const before = input.slice(0, mention_start_pos);
    const cursor_pos = textarea_ref.current?.selectionStart ?? input.length;
    const after = input.slice(cursor_pos);
    const next_input = `${before}@${agent.name} ${after}`;
    setInput(next_input);
    set_mention_active(false);

    // 恢复光标到插入点之后
    requestAnimationFrame(() => {
      const new_cursor = mention_start_pos + agent.name.length + 2; // @name + space
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

  const handle_send = useCallback(() => {
    const trimmed_input = input.trim();
    if ((!trimmed_input && attachments.length === 0) || disabled) {
      return;
    }

    if (trimmed_input) {
      setInputHistory((prev) => [trimmed_input, ...prev.slice(0, 49)]);
    }
    setHistoryIndex(-1);

    on_send_message(trimmed_input);
    setInput("");
    setAttachments([]);

    if (textarea_ref.current) {
      textarea_ref.current.style.height = "auto";
    }
  }, [attachments.length, disabled, input, on_send_message]);

  const handle_key_down = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (is_composing_ref.current || event.nativeEvent.isComposing) {
      return;
    }

    // @mention popover 激活时，让 popover 处理方向键/Enter/Esc
    if (mention_active) {
      if (["ArrowDown", "ArrowUp", "Enter", "Tab", "Escape"].includes(event.key)) {
        return; // popover 的 document keydown handler 会处理
      }
    }

    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      handle_send();
      return;
    }

    if (event.key === "ArrowUp" && event.ctrlKey && input_history.length > 0) {
      event.preventDefault();
      const next_index = Math.min(history_index + 1, input_history.length - 1);
      setHistoryIndex(next_index);
      setInput(input_history[next_index]);
      return;
    }

    if (event.key === "ArrowDown" && event.ctrlKey) {
      event.preventDefault();
      if (history_index > 0) {
        const next_index = history_index - 1;
        setHistoryIndex(next_index);
        setInput(input_history[next_index]);
      } else if (history_index === 0) {
        setHistoryIndex(-1);
        setInput("");
      }
      return;
    }
  };

  const handle_file_select = (event: React.ChangeEvent<HTMLInputElement>) => {
    const files = event.target.files;
    if (!files) {
      return;
    }

    const next_attachments: AttachmentFile[] = [];

    Array.from(files).forEach((file) => {
      const is_image = file.type.startsWith("image/");
      const attachment: AttachmentFile = {
        id: `${Date.now()}-${Math.random().toString(36).slice(2, 11)}`,
        file,
        type: is_image ? "image" : "document",
      };

      if (is_image) {
        const reader = new FileReader();
        reader.onload = (load_event) => {
          setAttachments((prev) =>
            prev.map((item) => (
              item.id === attachment.id
                ? { ...item, preview: load_event.target?.result as string }
                : item
            )),
          );
        };
        reader.readAsDataURL(file);
      }

      next_attachments.push(attachment);
    });

    setAttachments((prev) => [...prev, ...next_attachments]);

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

  return (
    <div
      className={cn(
        "w-full border-t border-[var(--surface-canvas-border)] bg-[var(--surface-canvas-background)]",
        compact ? "px-2 pb-2 pt-2" : "px-3 pb-3 pt-3 sm:px-5 xl:px-6",
      )}
    >
      <div className="relative mx-auto w-full max-w-[1020px]">
        {attachments.length > 0 ? (
          <div className="glass-card radius-shell-md mb-3 flex flex-wrap gap-2 p-3">
            {attachments.map((attachment) => (
              <div
                key={attachment.id}
                className={COMPOSER_ATTACHMENT_CLASS_NAME}
              >
                {attachment.type === "image" ? (
                  attachment.preview ? (
                    <img
                      alt={attachment.file.name}
                      className="h-8 w-8 rounded object-cover"
                      height={32}
                      loading="lazy"
                      src={attachment.preview}
                      width={32}
                    />
                  ) : (
                    <ImageIcon size={16} className="text-primary" />
                  )
                ) : (
                  <FileText size={16} className="text-accent" />
                )}
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

        <div
          className={getComposerShellClassName(disabled)}
          style={getComposerShellStyle(compact)}
        >
          <div className={cn("flex items-end gap-2", compact ? "p-1.5" : "px-2 py-1.5")}>
            <div className="flex items-center gap-1 pb-0.5">
              <input
                ref={file_input_ref}
                accept="image/*,.pdf,.doc,.docx,.txt,.md,.ppt,.pptx,.xls,.xlsx"
                aria-label="选择附件文件"
                className="hidden"
                multiple
                onChange={handle_file_select}
                type="file"
              />
              <WorkspacePillButton
                aria-label="添加附件"
                density={compact ? "compact" : "default"}
                disabled={disabled}
                onClick={() => file_input_ref.current?.click()}
                size="icon"
                variant="icon"
              >
                <Paperclip size={16} />
              </WorkspacePillButton>
            </div>

            <div className="relative flex-1">
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
                  "multiline-cursor min-h-6 max-h-24 w-full resize-none bg-transparent text-[14px] leading-6 text-[color:var(--text-strong)] outline-none shadow-none ring-0",
                  "placeholder:text-[color:var(--text-soft)]",
                  "disabled:cursor-not-allowed disabled:opacity-50",
                  "focus:border-0 focus:bg-transparent focus:outline-none focus:ring-0 focus-visible:outline-none focus-visible:ring-0 focus-visible:shadow-none",
                  "[&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]",
                )}
                disabled={disabled}
                onChange={(event) => handle_input_change(event.target.value)}
                onCompositionEnd={() => {
                  setTimeout(() => {
                    is_composing_ref.current = false;
                  }, 0);
                }}
                onCompositionStart={() => {
                  is_composing_ref.current = true;
                }}
                onKeyDown={handle_key_down}
                placeholder={placeholder}
                rows={1}
                value={input}
              />
            </div>

            <div className={cn("flex items-center gap-2 pb-0.5", compact && "gap-1.5")}>
              {char_count > 0 ? (
                <div className="text-[10px] tabular-nums">
                  <span
                    className={cn(
                      is_over_limit && "text-destructive",
                      is_near_limit && !is_over_limit && "text-warning",
                      !is_near_limit && "text-[color:var(--text-soft)]",
                    )}
                  >
                    {char_count}
                  </span>
                  <span className="text-[color:var(--text-soft)]">/{max_length}</span>
                </div>
              ) : null}

              <WorkspacePillButton
                aria-label="发送消息"
                density={compact ? "compact" : "default"}
                disabled={is_input_empty || disabled || is_over_limit}
                onClick={handle_send}
                size="icon"
                variant="primary"
              >
                <Send size={16} />
              </WorkspacePillButton>
            </div>
          </div>

          <div className={COMPOSER_FOOTER_CLASS_NAME}>
            <div className="flex items-center gap-3 text-[10px] text-[color:var(--text-soft)]">
              {disabled && control_status_text ? (
                <span className="text-[color:var(--text-default)]">{control_status_text}</span>
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
                </>
              )}
              {!disabled && control_status_text ? (
                <span className="text-[color:var(--text-default)]">{control_status_text}</span>
              ) : null}
            </div>

            {history_index >= 0 ? (
              <div className="text-[10px] text-[color:var(--text-default)]">
                历史 {history_index + 1}/{input_history.length}
              </div>
            ) : null}
          </div>
        </div>
      </div>
    </div>
  );
});

RoomComposerPanelView.displayName = "RoomComposerPanelView";

export function RoomComposerPanel(props: RoomComposerPanelProps) {
  return <RoomComposerPanelView {...props} />;
}
