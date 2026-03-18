/**
 * 空状态提示组件
 *
 * 当没有会话时显示,引导用户创建新会话
 */

"use client";

import { FolderKanban, MessageSquarePlus, Sparkles } from "lucide-react";

interface EmptyStateProps {
  onNewSession: () => void;
}

export function EmptyState({onNewSession}: EmptyStateProps) {
  return (
    <div className="flex-1 flex items-center justify-center p-8">
      <div className="soft-ring radius-shell-xl panel-surface relative max-w-2xl w-full overflow-hidden p-10 text-center">
        <div className="pointer-events-none absolute left-12 top-12 h-28 w-28 rounded-full glow-lilac opacity-40" />
        <div className="pointer-events-none absolute bottom-10 right-12 h-28 w-28 rounded-full glow-green opacity-35" />
        {/* Icon */}
        <div className="flex justify-center">
          <div className="neo-pill radius-shell-md relative inline-flex h-24 w-24 items-center justify-center text-primary">
            <div className="absolute -right-2 -top-2 rounded-full bg-[linear-gradient(135deg,rgba(255,194,148,0.92),rgba(255,155,86,0.88))] p-2 text-[#8a4409] shadow-[0_14px_24px_rgba(255,157,86,0.24)]">
              <FolderKanban className="h-4 w-4" />
            </div>
            <Sparkles className="w-10 h-10"/>
          </div>
        </div>

        {/* Title */}
        <div className="mt-8 space-y-3">
          <h2 className="text-4xl font-extrabold tracking-[-0.05em] text-foreground">
            还没有会话
          </h2>
          <p className="mx-auto max-w-md text-sm leading-7 text-muted-foreground">
            先创建一个新会话，当前工作区就会进入参考图那种柔和、浮起、可聚焦的主交互状态。
          </p>
        </div>

        {/* Features */}
        <div className="mt-8 grid gap-3 text-sm text-muted-foreground md:grid-cols-3">
          <div className="neo-inset radius-shell-md px-4 py-4">
            <div className="mx-auto mb-2 h-2 w-2 rounded-full bg-primary"/>
            <span>按 Agent 隔离上下文</span>
          </div>
          <div className="neo-inset radius-shell-md px-4 py-4">
            <div className="mx-auto mb-2 h-2 w-2 rounded-full bg-primary"/>
            <span>历史对话自动保存</span>
          </div>
          <div className="neo-inset radius-shell-md px-4 py-4">
            <div className="mx-auto mb-2 h-2 w-2 rounded-full bg-primary"/>
            <span>工具使用需你授权</span>
          </div>
        </div>

        {/* CTA Button */}
        <button
          onClick={onNewSession}
          className="mt-8 inline-flex items-center gap-2 rounded-full bg-[linear-gradient(135deg,rgba(166,255,194,0.92),rgba(102,217,143,0.88))] px-7 py-3.5 text-sm font-bold text-[#18653a] shadow-[0_20px_34px_rgba(102,217,143,0.22)] transition-transform hover:-translate-y-0.5"
        >
          <MessageSquarePlus className="w-5 h-5"/>
          <span>创建新会话</span>
        </button>

        {/* Hint */}
        <p className="mt-5 text-xs text-muted-foreground/80">
          先切换到想要的 Agent，再创建会话
        </p>
      </div>
    </div>
  );
}
