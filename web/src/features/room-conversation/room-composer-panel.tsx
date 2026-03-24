"use client";

import { KeyboardEvent, memo, useCallback, useEffect, useRef, useState } from "react";
import { FileText, Image as ImageIcon, Paperclip, Send, StopCircle, X, Zap } from "lucide-react";

import { cn } from "@/lib/utils";
import { LoadingOrb } from "@/shared/ui/loading-orb";
import { AttachmentFile, RoomComposerPanelProps } from "@/types/room-conversation";

const RoomComposerPanelView = memo(({
  compact,
  current_agent_name,
  is_loading,
  on_send_message,
  on_stop,
  disabled = false,
  placeholder = "继续描述目标、补充上下文，或直接开始协作…",
  max_length = 10000,
}: RoomComposerPanelProps) => {
  const [mode, setMode] = useState<"agent" | "room" | "app">("room");
  const [input, setInput] = useState("");
  const [input_history, setInputHistory] = useState<string[]>([]);
  const [history_index, setHistoryIndex] = useState(-1);
  const [attachments, setAttachments] = useState<AttachmentFile[]>([]);
  const [is_focused, setIsFocused] = useState(false);

  const is_composing_ref = useRef(false);
  const textarea_ref = useRef<HTMLTextAreaElement>(null);
  const file_input_ref = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (textarea_ref.current) {
      textarea_ref.current.style.height = "auto";
      textarea_ref.current.style.height = `${Math.min(textarea_ref.current.scrollHeight, 160)}px`;
    }
  }, [input]);

  useEffect(() => {
    if (textarea_ref.current && !disabled) {
      textarea_ref.current.focus();
    }
  }, [disabled]);

  const handle_send = useCallback(() => {
    const trimmed_input = input.trim();
    if ((!trimmed_input && attachments.length === 0) || disabled || is_loading) {
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
  }, [attachments.length, disabled, input, is_loading, on_send_message]);

  const handle_key_down = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (is_composing_ref.current || event.nativeEvent.isComposing) {
      return;
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
  const effective_placeholder =
    mode === "app"
      ? "让 App Agent 帮你建 room、拉成员，或调整协作网络…"
      : mode === "room"
        ? "继续这个 room 的任务，@ 成员、补充上下文或推进下一步…"
        : placeholder;

  return (
    <div
      className={cn(
        "w-full border-t workspace-divider bg-transparent",
        compact ? "px-2 pb-2 pt-2" : "px-2 pb-2 pt-2 sm:px-6 xl:px-8",
      )}
    >
      <div className="relative mx-auto w-full max-w-[980px] py-2">
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
            "workspace-input-shell radius-shell-lg relative overflow-hidden transition-all duration-300",
            is_focused ? "shadow-[0_30px_58px_rgba(111,126,162,0.18)]" : "",
            disabled && "cursor-not-allowed opacity-50",
            compact && "radius-shell-md shadow-none",
          )}
        >
          <div className="pointer-events-none absolute inset-0 home-glass-grid opacity-14" />
          <div className={cn("flex items-center gap-2 border-b workspace-divider px-3", compact ? "py-2" : "py-2.5")}>
            <button
              className={cn(
                "rounded-full px-3 py-1.5 text-xs font-semibold transition-all",
                mode === "agent" ? "workspace-chip text-slate-950" : "text-slate-700/54 hover:text-slate-950",
              )}
              onClick={() => setMode("agent")}
              type="button"
            >
              Agent
            </button>
            <button
              className={cn(
                "rounded-full px-3 py-1.5 text-xs font-semibold transition-all",
                mode === "room" ? "workspace-chip text-slate-950" : "text-slate-700/54 hover:text-slate-950",
              )}
              onClick={() => setMode("room")}
              type="button"
            >
              Room
            </button>
            <button
              className={cn(
                "rounded-full px-3 py-1.5 text-xs font-semibold transition-all",
                mode === "app" ? "workspace-chip text-slate-950" : "text-slate-700/54 hover:text-slate-950",
              )}
              onClick={() => setMode("app")}
              type="button"
            >
              Ask App
            </button>
            {!compact ? (
              <div className="ml-auto truncate text-[11px] text-slate-700/48">
                {mode === "agent"
                  ? `当前协作 Agent：${current_agent_name ?? "未指定"}`
                  : mode === "room"
                    ? "面向当前 room 的协作输入"
                    : "系统级组织与编排动作"}
              </div>
            ) : null}
          </div>

          <div className={cn("flex items-end gap-2", compact ? "p-2.5" : "p-3")}>
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
                  "workspace-chip rounded-2xl p-2 transition-all duration-200",
                  "text-slate-700/62 hover:-translate-y-0.5 hover:text-slate-950",
                  "disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:bg-transparent disabled:hover:text-muted-foreground",
                  "focus-visible:ring-2 focus-visible:ring-primary/40",
                  "hover:shadow-[0_12px_22px_rgba(111,126,162,0.14)]",
                )}
                disabled={disabled || is_loading}
                onClick={() => file_input_ref.current?.click()}
                type="button"
              >
                <Paperclip size={16} />
              </button>
            </div>

            <div className="relative flex-1">
              <textarea
                ref={textarea_ref}
                className={cn(
                  "multiline-cursor min-h-[24px] max-h-[160px] w-full resize-none bg-transparent text-sm leading-relaxed text-slate-900/82 outline-none",
                  "placeholder:text-slate-700/42",
                  "disabled:cursor-not-allowed disabled:opacity-50",
                  "[&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]",
                )}
                disabled={disabled || is_loading}
                onBlur={() => setIsFocused(false)}
                onChange={(event) => setInput(event.target.value)}
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
                placeholder={effective_placeholder}
                rows={1}
                style={{ fieldSizing: "content" }}
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
                  <span className="text-slate-700/28">/{max_length}</span>
                </div>
              ) : null}

              {is_loading ? (
                <button
                  aria-label="停止生成"
                  className={cn(
                    "relative overflow-hidden rounded-2xl p-2",
                    "bg-[linear-gradient(135deg,rgba(255,224,224,0.98),rgba(247,173,173,0.92))] text-destructive",
                    "transition-all duration-200 focus-visible:ring-2 focus-visible:ring-primary/40",
                    "group hover:-translate-y-0.5 hover:shadow-[0_14px_24px_rgba(239,68,68,0.22)]",
                  )}
                  onClick={on_stop}
                >
                  <div className="absolute inset-0 animate-pulse bg-destructive/10" />
                  <StopCircle size={16} className="relative z-10" />
                </button>
              ) : (
                <button
                  aria-label="发送消息"
                  className={cn(
                    "relative overflow-hidden rounded-2xl p-2",
                    "bg-[linear-gradient(135deg,rgba(166,255,194,0.94),rgba(102,217,143,0.90))] text-[#18653a]",
                    "transition-all duration-200",
                    "hover:-translate-y-0.5 hover:shadow-[0_18px_28px_rgba(102,217,143,0.22)]",
                    "disabled:cursor-not-allowed disabled:opacity-30 disabled:hover:bg-primary/20 disabled:hover:shadow-none",
                    "focus-visible:ring-2 focus-visible:ring-primary/40",
                    "group",
                  )}
                  disabled={is_input_empty || disabled || is_over_limit}
                  onClick={handle_send}
                >
                  <div className="absolute inset-0 translate-x-[-100%] bg-gradient-to-r from-transparent via-white/10 to-transparent transition-transform duration-700 group-hover:translate-x-[100%]" />
                  <Send size={16} className="relative z-10" />
                </button>
              )}
            </div>
          </div>

          <div className="flex items-center justify-between border-t workspace-divider px-4 py-2.5">
            <div className="flex items-center gap-3 text-[10px] text-slate-700/48">
              {is_loading ? (
                <span className="flex items-center gap-2 text-emerald-700/72">
                  <LoadingOrb frames={["✽", "✻", "✶", "✢", "·"]} />
                  <Zap size={10} className="animate-pulse" />
                  <span className="animate-pulse">Agent 正在思考...</span>
                  <span className="text-slate-700/28">[ESC 停止]</span>
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
              <div className="flex items-center gap-1 text-[10px] text-sky-700/70">
                <span>历史</span>
                <span className="tabular-nums">{history_index + 1}/{input_history.length}</span>
                <span className="text-slate-700/28">[Ctrl+↑/↓]</span>
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
