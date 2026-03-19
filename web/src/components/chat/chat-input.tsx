"use client";

import NextImage from "next/image";
import { KeyboardEvent, memo, useCallback, useEffect, useRef, useState } from "react";
import { FileText, Image as ImageIcon, Paperclip, Send, StopCircle, X, Zap } from "lucide-react";
import { cn } from "@/lib/utils";
import { LoadingOrb } from "@/components/loading";

interface AttachmentFile {
  id: string;
  file: File;
  preview?: string;
  type: 'image' | 'document';
}

interface ChatInputProps {
  isLoading: boolean;
  onSendMessage: (message: string, attachments?: AttachmentFile[]) => void;
  onStop: () => void;
  disabled?: boolean;
  placeholder?: string;
  maxLength?: number;
}

const ChatInput = memo((
  {
    isLoading,
    onSendMessage,
    onStop,
    disabled = false,
    placeholder = "输入消息...",
    maxLength = 10000
  }: ChatInputProps) => {
  const [input, setInput] = useState("");
  const [inputHistory, setInputHistory] = useState<string[]>([]);
  const [historyIndex, setHistoryIndex] = useState(-1);
  const [attachments, setAttachments] = useState<AttachmentFile[]>([]);
  const [isFocused, setIsFocused] = useState(false);

  // 使用 ref 跟踪输入法状态，避免 React 状态更新延迟
  const isComposingRef = useRef(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // 自动调整textarea高度
  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto';
      textareaRef.current.style.height = `${Math.min(textareaRef.current.scrollHeight, 160)}px`;
    }
  }, [input]);

  // 聚焦输入框
  useEffect(() => {
    if (textareaRef.current && !disabled) {
      textareaRef.current.focus();
    }
  }, [disabled]);

  const handleSend = useCallback(() => {
    const trimmedInput = input.trim();
    if ((!trimmedInput && attachments.length === 0) || disabled || isLoading) return;

    // 添加到历史记录（保留最近50条）
    if (trimmedInput) {
      setInputHistory(prev => [trimmedInput, ...prev.slice(0, 49)]);
    }
    setHistoryIndex(-1);

    onSendMessage(trimmedInput, attachments.length > 0 ? attachments : undefined);
    setInput("");
    setAttachments([]);

    // 重置高度
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto';
    }
  }, [input, attachments, disabled, isLoading, onSendMessage]);

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    // 输入法正在组合中，不处理 Enter
    // 使用 ref 和原生事件双重检查，解决 Safari 中文输入法问题
    if (isComposingRef.current || e.nativeEvent.isComposing) return;

    // Enter发送（不按Shift）
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
      return;
    }

    // 向上翻历史
    if (e.key === 'ArrowUp' && e.ctrlKey && inputHistory.length > 0) {
      e.preventDefault();
      const newIndex = Math.min(historyIndex + 1, inputHistory.length - 1);
      setHistoryIndex(newIndex);
      setInput(inputHistory[newIndex]);
      return;
    }

    // 向下翻历史
    if (e.key === 'ArrowDown' && e.ctrlKey) {
      e.preventDefault();
      if (historyIndex > 0) {
        const newIndex = historyIndex - 1;
        setHistoryIndex(newIndex);
        setInput(inputHistory[newIndex]);
      } else if (historyIndex === 0) {
        setHistoryIndex(-1);
        setInput("");
      }
      return;
    }

    // Esc停止生成
    if (e.key === 'Escape' && isLoading) {
      e.preventDefault();
      onStop();
      return;
    }
  };

  // 处理文件选择
  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files;
    if (!files) return;

    const newAttachments: AttachmentFile[] = [];

    Array.from(files).forEach(file => {
      const isImage = file.type.startsWith('image/');
      const attachment: AttachmentFile = {
        id: `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
        file,
        type: isImage ? 'image' : 'document',
      };

      // 为图片生成预览
      if (isImage) {
        const reader = new FileReader();
        reader.onload = (e) => {
          setAttachments(prev =>
            prev.map(a => a.id === attachment.id ? { ...a, preview: e.target?.result as string } : a)
          );
        };
        reader.readAsDataURL(file);
      }

      newAttachments.push(attachment);
    });

    setAttachments(prev => [...prev, ...newAttachments]);

    // 重置 input 以允许选择相同文件
    if (fileInputRef.current) {
      fileInputRef.current.value = '';
    }
  };

  // 移除附件
  const removeAttachment = (id: string) => {
    setAttachments(prev => prev.filter(a => a.id !== id));
  };

  const isInputEmpty = input.trim().length === 0 && attachments.length === 0;
  const charCount = input.length;
  const isNearLimit = charCount > maxLength * 0.8;
  const isOverLimit = charCount > maxLength;

  return (
    <div className="w-full border-t border-white/55 bg-transparent px-8 pb-6 pt-3">
      <div className="relative mx-auto w-full py-2">
        {/* 附件预览区域 */}
        {attachments.length > 0 && (
          <div className="neo-card-flat radius-shell-md mb-3 flex flex-wrap gap-2 p-3">
            {attachments.map(attachment => (
              <div
                key={attachment.id}
                className="neo-pill radius-shell-sm relative group flex items-center gap-2 px-3 py-2"
              >
                {attachment.type === 'image' ? (
                  attachment.preview ? (
                    <NextImage
                      src={attachment.preview}
                      alt={attachment.file.name}
                      className="h-8 w-8 rounded object-cover"
                      height={32}
                      unoptimized
                      width={32}
                    />
                  ) : (
                    <ImageIcon size={16} className="text-primary" />
                  )
                ) : (
                  <FileText size={16} className="text-accent" />
                )}
                <span className="text-xs text-foreground/70 max-w-[120px] truncate">
                  {attachment.file.name}
                </span>
                <button
                  aria-label="移除附件"
                  onClick={() => removeAttachment(attachment.id)}
                  className="ml-1 rounded p-0.5 text-red-400 opacity-60 transition-opacity hover:bg-red-500/10 hover:opacity-100 focus-visible:ring-2 focus-visible:ring-primary/50"
                >
                  <X size={12} />
                </button>
              </div>
            ))}
          </div>
        )}

        {/* 主输入框容器 */}
        <div
          className={cn(
            "panel-surface soft-ring radius-shell-lg relative transition-all duration-300",
            isFocused
              ? "shadow-[0_28px_54px_rgba(133,119,255,0.18)]"
              : "",
            disabled && "opacity-50 cursor-not-allowed"
          )}
        >
          {/* 输入区域 */}
          <div className="flex items-end gap-2 p-3">
            {/* 左侧工具栏 */}
            <div className="flex items-center gap-1 pb-1">
              {/* 附件按钮 */}
              <input
                ref={fileInputRef}
                type="file"
                multiple
                accept="image/*,.pdf,.doc,.docx,.txt,.md,.ppt,.pptx,.xls,.xlsx"
                onChange={handleFileSelect}
                className="hidden"
                aria-label="选择附件文件"
              />
              <button
                type="button"
                onClick={() => fileInputRef.current?.click()}
                disabled={disabled || isLoading}
                aria-label="添加附件"
                className={cn(
                  "neo-pill rounded-2xl p-2 transition-all duration-200",
                  "text-muted-foreground hover:-translate-y-0.5 hover:text-primary",
                  "disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:bg-transparent disabled:hover:text-muted-foreground",
                  "focus-visible:ring-2 focus-visible:ring-primary/50",
                  "hover:shadow-[0_12px_22px_rgba(133,119,255,0.12)]"
                )}
              >
                <Paperclip size={16} />
              </button>
            </div>

            {/* 文本输入框 */}
            <div className="flex-1 relative">
              <textarea
                ref={textareaRef}
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={handleKeyDown}
                onCompositionStart={() => {
                  isComposingRef.current = true;
                }}
                onCompositionEnd={() => {
                  // Safari 时序问题：延迟更新状态，确保 keydown 事件先处理
                  setTimeout(() => {
                    isComposingRef.current = false;
                  }, 0);
                }}
                onFocus={() => setIsFocused(true)}
                onBlur={() => setIsFocused(false)}
                placeholder={placeholder}
                disabled={disabled || isLoading}
                className={cn(
                  "w-full bg-transparent resize-none outline-none",
                  "text-sm leading-relaxed text-foreground multiline-cursor",
                  "placeholder:text-muted-foreground/40",
                  "disabled:opacity-50 disabled:cursor-not-allowed",
                  "min-h-[24px] max-h-[160px]",
                  // 隐藏滚动条
                  "[&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]"
                )}
                rows={1}
                style={{
                  fieldSizing: 'content',
                }}
              />
            </div>

            {/* 右侧操作区域 */}
            <div className="flex items-center gap-2 pb-1">
              {/* 字符计数 */}
              {charCount > 0 && (
                <div className="text-[10px] tabular-nums">
                  <span className={cn(
                    isOverLimit && "text-destructive",
                    isNearLimit && !isOverLimit && "text-warning",
                    !isNearLimit && "text-muted-foreground/40"
                  )}>
                    {charCount}
                  </span>
                  <span className="text-muted-foreground/30">/{maxLength}</span>
                </div>
              )}

              {/* 发送/停止按钮 */}
              {isLoading ? (
                <button
                  aria-label="停止生成"
                  onClick={onStop}
                  className={cn(
                    "flex items-center justify-center rounded-2xl p-2",
                    "bg-[linear-gradient(135deg,rgba(255,219,219,0.96),rgba(245,163,163,0.9))] text-destructive",
                    "transition-all duration-200 focus-visible:ring-2 focus-visible:ring-primary/50",
                    "hover:-translate-y-0.5 hover:shadow-[0_14px_24px_rgba(239,68,68,0.22)]",
                    "relative overflow-hidden group"
                  )}
                >
                  <div className="absolute inset-0 bg-destructive/10 animate-pulse" />
                  <StopCircle size={16} className="relative z-10" />
                </button>
              ) : (
                <button
                  aria-label="发送消息"
                  onClick={handleSend}
                  disabled={isInputEmpty || disabled || isOverLimit}
                  className={cn(
                    "flex items-center justify-center rounded-2xl p-2",
                    "bg-[linear-gradient(135deg,rgba(174,163,255,0.92),rgba(207,200,255,0.92))] text-primary",
                    "transition-all duration-200",
                    "hover:-translate-y-0.5 hover:shadow-[0_16px_26px_rgba(133,119,255,0.22)]",
                    "disabled:opacity-30 disabled:cursor-not-allowed disabled:hover:shadow-none disabled:hover:bg-primary/20",
                    "focus-visible:ring-2 focus-visible:ring-primary/50",
                    "relative overflow-hidden group"
                  )}
                >
                  {/* 悬停光效 */}
                  <div
                    className="absolute inset-0 bg-gradient-to-r from-transparent via-white/10 to-transparent translate-x-[-100%] group-hover:translate-x-[100%] transition-transform duration-700" />
                  <Send size={16} className="relative z-10" />
                </button>
              )}
            </div>
          </div>

          {/* 底部状态栏 */}
          <div className="flex items-center justify-between border-t border-white/55 px-4 py-2">
            <div className="flex items-center gap-3 text-[10px] text-muted-foreground/50">
              {isLoading ? (
                <span className="flex items-center gap-2 text-primary/70">
                  <LoadingOrb frames={["✽", "✻", "✶", "✢", "·"]} />
                  <Zap size={10} className="animate-pulse" />
                  <span className="animate-pulse">Agent 正在思考...</span>
                  <span className="text-muted-foreground/30">[ESC 停止]</span>
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

            {/* 历史记录提示 */}
            {historyIndex >= 0 && (
              <div className="text-[10px] text-accent/70 flex items-center gap-1">
                <span>历史</span>
                <span className="tabular-nums">{historyIndex + 1}/{inputHistory.length}</span>
                <span className="text-muted-foreground/30">[Ctrl+↑/↓]</span>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
});

ChatInput.displayName = "ChatInput";

export default ChatInput;
