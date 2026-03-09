"use client";

import { KeyboardEvent, memo, useCallback, useEffect, useRef, useState } from "react";
import { FileText, Image, Paperclip, Send, StopCircle, X, Zap } from "lucide-react";
import { cn } from "@/lib/utils";
import { LoadingOrb } from "@/components/header/loading";

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
    placeholder = "输入指令...",
    maxLength = 10000
  }: ChatInputProps) => {
  const [input, setInput] = useState("");
  const [inputHistory, setInputHistory] = useState<string[]>([]);
  const [historyIndex, setHistoryIndex] = useState(-1);
  const [attachments, setAttachments] = useState<AttachmentFile[]>([]);
  const [isComposing, setIsComposing] = useState(false);
  const [isFocused, setIsFocused] = useState(false);

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
    if (isComposing) return;

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
            prev.map(a => a.id === attachment.id ? {...a, preview: e.target?.result as string} : a)
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
    <div className=" w-full px-8 relative border-t border-primary/30 bg-background/95 backdrop-blur-md">
      {/* 扫描线效果 */}
      <div className="absolute inset-0 scanline opacity-20 pointer-events-none"/>

      {/* 顶部发光线 */}
      <div
        className="absolute top-0 left-0 right-0 h-px bg-gradient-to-r from-transparent via-primary/50 to-transparent"/>

      <div className="max-w-4xl mx-auto py-2 relative">
        {/* 附件预览区域 */}
        {attachments.length > 0 && (
          <div className="mb-3 flex flex-wrap gap-2 p-2 rounded border border-primary/20 bg-secondary/30">
            {attachments.map(attachment => (
              <div
                key={attachment.id}
                className="relative group flex items-center gap-2 px-3 py-2 rounded border border-primary/30 bg-background/50"
              >
                {attachment.type === 'image' ? (
                  attachment.preview ? (
                    <img
                      src={attachment.preview}
                      alt={attachment.file.name}
                      className="w-8 h-8 object-cover rounded"
                    />
                  ) : (
                    <Image size={16} className="text-primary"/>
                  )
                ) : (
                  <FileText size={16} className="text-accent"/>
                )}
                <span className="text-xs text-foreground/70 max-w-[120px] truncate">
                  {attachment.file.name}
                </span>
                <button
                  onClick={() => removeAttachment(attachment.id)}
                  className="ml-1 p-0.5 hover:bg-red-500/20 rounded text-red-400 opacity-60 hover:opacity-100 transition-opacity"
                >
                  <X size={12}/>
                </button>
              </div>
            ))}
          </div>
        )}

        {/* 主输入框容器 */}
        <div
          className={cn(
            "relative rounded border bg-secondary/50 backdrop-blur-sm transition-all duration-300",
            isFocused
              ? "border-primary/60 shadow-[0_0_20px_rgba(0,240,255,0.15),inset_0_0_20px_rgba(0,240,255,0.05)]"
              : "border-primary/30",
            disabled && "opacity-50 cursor-not-allowed"
          )}
        >
          {/* 角落装饰 */}
          <div className="absolute top-0 left-0 w-3 h-3 border-t border-l  border-primary/60"/>
          <div className="absolute top-0 right-0 w-3 h-3 border-t border-r border-primary/60"/>
          <div className="absolute bottom-0 left-0 w-3 h-3 border-b border-l border-primary/60"/>
          <div className="absolute bottom-0 right-0 w-3 h-3 border-b border-r border-primary/60"/>

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
              />
              <button
                type="button"
                onClick={() => fileInputRef.current?.click()}
                disabled={disabled || isLoading}
                className={cn(
                  "p-2 rounded border border-primary/30 transition-all duration-200",
                  "text-muted-foreground hover:text-primary hover:border-primary/50 hover:bg-primary/10",
                  "disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:bg-transparent disabled:hover:text-muted-foreground",
                  "hover:shadow-[0_0_10px_rgba(0,240,255,0.2)]"
                )}
                title="添加附件"
              >
                <Paperclip size={16}/>
              </button>
            </div>

            {/* 文本输入框 */}
            <div className="flex-1 relative">
              <textarea
                ref={textareaRef}
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={handleKeyDown}
                onCompositionStart={() => setIsComposing(true)}
                onCompositionEnd={() => setIsComposing(false)}
                onFocus={() => setIsFocused(true)}
                onBlur={() => setIsFocused(false)}
                placeholder={placeholder}
                disabled={disabled || isLoading}
                className={cn(
                  "w-full bg-transparent resize-none outline-none",
                  "text-sm leading-relaxed text-foreground",
                  "placeholder:text-muted-foreground/40",
                  "disabled:opacity-50 disabled:cursor-not-allowed",
                  "min-h-[24px] max-h-[160px]",
                  // 隐藏滚动条
                  "[&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]"
                )}
                rows={1}
                style={{
                  // @ts-ignore - fieldSizing is a newer CSS property
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
                    isOverLimit && "text-red-500",
                    isNearLimit && !isOverLimit && "text-yellow-500",
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
                  onClick={onStop}
                  className={cn(
                    "flex items-center justify-center p-2",
                    "bg-red-500/20 hover:bg-red-500/30 text-red-400",
                    "rounded border border-red-500/50 hover:border-red-500/70",
                    "transition-all duration-200",
                    "hover:shadow-[0_0_15px_rgba(239,68,68,0.3)]",
                    "relative overflow-hidden group"
                  )}
                  title="停止生成 (Esc)"
                >
                  <div className="absolute inset-0 bg-red-500/10 animate-pulse"/>
                  <StopCircle size={16} className="relative z-10"/>
                </button>
              ) : (
                <button
                  onClick={handleSend}
                  disabled={isInputEmpty || disabled || isOverLimit}
                  className={cn(
                    "flex items-center justify-center p-2",
                    "bg-primary/20 text-primary",
                    "rounded border border-primary/50 transition-all duration-200",
                    "hover:bg-primary/30 hover:border-primary/70",
                    "hover:shadow-[0_0_15px_rgba(0,240,255,0.3)]",
                    "disabled:opacity-30 disabled:cursor-not-allowed disabled:hover:shadow-none disabled:hover:bg-primary/20",
                    "relative overflow-hidden group"
                  )}
                  title="发送 (Enter)"
                >
                  {/* 悬停光效 */}
                  <div
                    className="absolute inset-0 bg-gradient-to-r from-transparent via-white/10 to-transparent translate-x-[-100%] group-hover:translate-x-[100%] transition-transform duration-700"/>
                  <Send size={16} className="relative z-10"/>
                </button>
              )}
            </div>
          </div>

          {/* 底部状态栏 */}
          <div className="flex items-center justify-between px-3 py-1.5 border-t border-primary/10 bg-secondary/30">
            <div className="flex items-center gap-3 text-[10px] text-muted-foreground/50">
              {isLoading ? (
                <span className="flex items-center gap-2 text-primary/70">
                  <LoadingOrb frames={["✽", "✻", "✶", "✢", "·"]}/>
                  <Zap size={10} className="animate-pulse"/>
                  <span className="animate-pulse">正在处理...</span>
                  <span className="text-muted-foreground/30">[ESC 停止]</span>
                </span>
              ) : (
                <>
                  <span className="flex items-center gap-1">
                    <kbd className="px-1 py-0.5 bg-primary/10 rounded text-[9px] border border-primary/20">Enter</kbd>
                    <span>发送</span>
                  </span>
                  <span className="flex items-center gap-1">
                    <kbd className="px-1 py-0.5 bg-primary/10 rounded text-[9px] border border-primary/20">Shift</kbd>
                    <span>+</span>
                    <kbd className="px-1 py-0.5 bg-primary/10 rounded text-[9px] border border-primary/20">Enter</kbd>
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