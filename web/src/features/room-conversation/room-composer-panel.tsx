"use client";

import { KeyboardEvent, memo, useCallback, useEffect, useRef, useState } from "react";
import { FileText, Image as ImageIcon, Paperclip, Send, StopCircle, X } from "lucide-react";

import { cn } from "@/lib/utils";
import { LoadingOrb } from "@/shared/ui/feedback/loading-orb";
import { useTextareaHeight } from "@/hooks/use-textarea-height";
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
  current_agent_name: string | null;
  is_loading: boolean;
  on_send_message: (content: string) => void | Promise<void>;
  on_stop: () => void;
  initial_draft?: string | null;
  disabled?: boolean;
  placeholder?: string;
  max_length?: number;
  room_members?: Agent[];
  mention_disabled?: boolean;
  status_hint?: string | null;
}

const RoomComposerPanelView = memo(({
  compact,
  current_agent_name,
  is_loading,
  on_send_message,
  on_stop,
  initial_draft = null,
  disabled = false,
  placeholder = "继续描述目标、补充上下文，或直接开始协作…",
  max_length = 10000,
  room_members = [],
  mention_disabled = false,
  status_hint = null,
}: RoomComposerPanelProps) => {
  const [input, setInput] = useState("");
  const [input_history, setInputHistory] = useState<string[]>([]);
  const [history_index, setHistoryIndex] = useState(-1);
  const [attachments, setAttachments] = useState<AttachmentFile[]>([]);
  const [is_focused, setIsFocused] = useState(false);

  // @mention 状态
  const [mention_active, set_mention_active] = useState(false);
  const [mention_filter, set_mention_filter] = useState("");
  const [mention_start_pos, set_mention_start_pos] = useState(-1);

  const is_composing_ref = useRef(false);
  const textarea_ref = useRef<HTMLTextAreaElement>(null);
  const file_input_ref = useRef<HTMLInputElement>(null);

  // Pretext-based auto-height: no scrollHeight reflow
  useTextareaHeight(textarea_ref, input, { minHeight: 24, maxHeight: 128, lineHeight: 24, paddingY: 0 });

  // @mention：检测 @ 字符并追踪过滤文本
  const handle_input_change = useCallback((value: string) => {
    setInput(value);

    if (room_members.length === 0 || mention_disabled) {
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
  }, [mention_disabled, room_members.length]);

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

    if (event.key === "Escape" && is_loading) {
      event.preventDefault();
      on_stop();
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
  const resolved_status_hint = status_hint ?? (
    current_agent_name ? `@${current_agent_name} 正在这个协作中` : "继续推进当前协作"
  );

  return (
    <div
      className={cn(
        "w-full border-t border-slate-200/80 bg-[#f8fafc]",
        compact ? "px-2 pb-2 pt-2" : "px-3 pb-3 pt-3 sm:px-5 xl:px-6",
      )}
    >
      <div className="relative mx-auto w-full max-w-[1020px]">
        {attachments.length > 0 ? (
          <div className="workspace-card radius-shell-md mb-3 flex flex-wrap gap-2 p-3">
            {attachments.map((attachment) => (
              <div
                key={attachment.id}
                className="workspace-chip radius-shell-sm group relative flex items-center gap-2 px-3 py-2"
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
                  className="ml-1 rounded p-0.5 text-red-400 opacity-60 transition-opacity hover:bg-red-500/10 hover:opacity-100 focus-visible:ring-2 focus-visible:ring-primary/50"
                  onClick={() => remove_attachment(attachment.id)}
                >
                  <X size={12} />
                </button>
              </div>
            ))}
          </div>
        ) : null}

        <div
          className={cn(
            "relative overflow-hidden rounded-[18px] border border-slate-300 bg-white transition-all duration-300",
            is_focused ? "shadow-[0_8px_18px_rgba(15,23,42,0.08)]" : "shadow-sm",
            disabled && "cursor-not-allowed opacity-50",
            compact && "shadow-none",
          )}
        >
          <div className={cn("border-b border-slate-200 px-4", compact ? "py-1.5" : "py-2")}>
            <div className="flex items-center justify-between gap-3 text-[11px] text-slate-500">
              <span className="font-semibold uppercase tracking-[0.14em]">Message</span>
              {!compact ? (
                <span className="truncate text-slate-400">
                  {resolved_status_hint}
                </span>
              ) : null}
            </div>
          </div>

          <div className={cn("flex items-end gap-2", compact ? "p-2.5" : "px-3 py-2.5")}>
            <div className="flex items-center gap-1 pb-1">
              <input
                ref={file_input_ref}
                accept="image/*,.pdf,.doc,.docx,.txt,.md,.ppt,.pptx,.xls,.xlsx"
                aria-label="选择附件文件"
                className="hidden"
                multiple
                onChange={handle_file_select}
                type="file"
              />
              <button
                aria-label="添加附件"
                className={cn(
                  "rounded-md p-2 transition-all duration-200",
                  "text-slate-500 hover:bg-slate-100 hover:text-slate-950",
                  "disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:bg-transparent disabled:hover:text-muted-foreground",
                  "focus-visible:ring-2 focus-visible:ring-primary/40",
                )}
                disabled={disabled}
                onClick={() => file_input_ref.current?.click()}
                type="button"
              >
                <Paperclip size={16} />
              </button>
            </div>

            <div className="relative flex-1">
              {mention_active && !mention_disabled && room_members.length > 0 ? (
                <MentionPopover
                  anchor_rect={textarea_ref.current?.getBoundingClientRect() ?? null}
                  filter={mention_filter}
                  members={room_members}
                  on_close={handle_mention_close}
                  on_select={handle_mention_select}
                />
              ) : null}
              <textarea
                ref={textarea_ref}
                className={cn(
                  "multiline-cursor min-h-6 max-h-32 w-full resize-none bg-transparent text-[15px] leading-6 text-slate-900 outline-none",
                  "placeholder:text-slate-400",
                  "disabled:cursor-not-allowed disabled:opacity-50",
                  "[&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]",
                )}
                disabled={disabled}
                onBlur={() => setIsFocused(false)}
                onChange={(event) => handle_input_change(event.target.value)}
                onCompositionEnd={() => {
                  setTimeout(() => {
                    is_composing_ref.current = false;
                  }, 0);
                }}
                onCompositionStart={() => {
                  is_composing_ref.current = true;
                }}
                onFocus={() => setIsFocused(true)}
                onKeyDown={handle_key_down}
                placeholder={placeholder}
                rows={1}
                value={input}
              />
            </div>

            <div className={cn("flex items-center gap-2 pb-1", compact && "gap-1.5")}>
              {char_count > 0 ? (
                <div className="text-[10px] tabular-nums">
                  <span
                    className={cn(
                      is_over_limit && "text-destructive",
                      is_near_limit && !is_over_limit && "text-warning",
                      !is_near_limit && "text-slate-700/40",
                    )}
                  >
                    {char_count}
                  </span>
                  <span className="text-slate-300">/{max_length}</span>
                </div>
              ) : null}

              {is_loading ? (
                <button
                  aria-label="停止生成"
                  className={cn(
                    "relative overflow-hidden rounded-2xl p-2",
                    "bg-[linear-gradient(135deg,rgba(255,224,224,0.98),rgba(247,173,173,0.92))] text-destructive",
                    "transition-all duration-200 focus-visible:ring-2 focus-visible:ring-primary/40",
                    "group hover:-translate-y-0.5 hover:shadow-[0_12px_20px_rgba(239,68,68,0.18)]",
                  )}
                  onClick={on_stop}
                  type="button"
                >
                  <div className="absolute inset-0 animate-pulse bg-destructive/10" />
                  <StopCircle size={16} className="relative z-10" />
                </button>
              ) : null}

              <button
                aria-label="发送消息"
                className={cn(
                  "relative overflow-hidden rounded-2xl p-2",
                  "bg-[linear-gradient(135deg,rgba(166,255,194,0.94),rgba(102,217,143,0.90))] text-[#18653a]",
                  "transition-all duration-200",
                  "hover:-translate-y-0.5 hover:shadow-[0_14px_22px_rgba(102,217,143,0.18)]",
                  "disabled:cursor-not-allowed disabled:opacity-30 disabled:hover:bg-primary/20 disabled:hover:shadow-none",
                  "focus-visible:ring-2 focus-visible:ring-primary/40",
                  "group",
                )}
                disabled={is_input_empty || disabled || is_over_limit}
                onClick={handle_send}
                type="button"
              >
                <div className="absolute inset-0 translate-x-full bg-linear-to-r from-transparent via-white/10 to-transparent transition-transform duration-700 group-hover:translate-x-full" />
                <Send size={16} className="relative z-10" />
              </button>
            </div>
          </div>

          <div className="flex items-center justify-between border-t border-slate-200 px-4 py-2">
            <div className="flex items-center gap-3 text-[10px] text-slate-400">
              {is_loading ? (
                <span className="flex items-center gap-2 text-emerald-700/72">
                  <LoadingOrb frames={["✽", "✻", "✶", "✢", "·"]} />
                  <span className="animate-pulse">协作进行中，仍可继续发送</span>
                  {mention_disabled ? (
                    <span className="text-slate-700/40">@ 暂不可用</span>
                  ) : null}
                  <span className="text-slate-700/28">[ESC 停止最近一轮]</span>
                </span>
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
            </div>

            {history_index >= 0 ? (
              <div className="text-[10px] text-sky-700/70">
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
