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
      <div className="max-w-2xl w-full rounded-[28px] border border-dashed border-border/80 bg-white/70 p-10 text-center shadow-sm">
        {/* Icon */}
        <div className="flex justify-center">
          <div className="relative inline-flex h-20 w-20 items-center justify-center rounded-[24px] bg-primary/10 text-primary">
            <div className="absolute -right-2 -top-2 rounded-full bg-white p-2 text-accent shadow-sm">
              <FolderKanban className="h-4 w-4" />
            </div>
            <Sparkles className="w-10 h-10"/>
          </div>
        </div>

        {/* Title */}
        <div className="mt-8 space-y-3">
          <h2 className="text-3xl font-semibold text-foreground">
            在当前 Agent 中开启一个新的 Session
          </h2>
          <p className="mx-auto max-w-xl text-sm leading-6 text-muted-foreground">
            这个区域是 Agent Space 的执行主工作区。创建 Session 之后，这里会承载消息时间线、工具调用结果、权限交互和运行状态。
          </p>
        </div>

        {/* Features */}
        <div className="mt-8 grid gap-3 text-sm text-muted-foreground md:grid-cols-3">
          <div className="rounded-2xl bg-secondary/80 px-4 py-4">
            <div className="mx-auto mb-2 h-2 w-2 rounded-full bg-primary"/>
            <span>按 Agent 隔离会话和上下文</span>
          </div>
          <div className="rounded-2xl bg-secondary/80 px-4 py-4">
            <div className="mx-auto mb-2 h-2 w-2 rounded-full bg-primary"/>
            <span>保留运行时间线与历史记录</span>
          </div>
          <div className="rounded-2xl bg-secondary/80 px-4 py-4">
            <div className="mx-auto mb-2 h-2 w-2 rounded-full bg-primary"/>
            <span>承接后续的权限与审计面板</span>
          </div>
        </div>

        {/* CTA Button */}
        <button
          onClick={onNewSession}
          className="mt-8 inline-flex items-center gap-2 rounded-2xl bg-primary px-6 py-3 text-sm font-medium text-primary-foreground shadow-sm transition-transform hover:-translate-y-0.5"
        >
          <MessageSquarePlus className="w-5 h-5"/>
          <span>创建新会话</span>
        </button>

        {/* Hint */}
        <p className="mt-5 text-xs text-muted-foreground/80">
          你也可以先返回 Agent Directory，再切换到其他 Agent 空间。
        </p>
      </div>
    </div>
  );
}
