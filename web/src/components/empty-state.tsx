/**
 * 空状态提示组件
 *
 * 当没有会话时显示,引导用户创建新会话
 */

"use client";

import { MessageSquarePlus, Sparkles } from "lucide-react";

interface EmptyStateProps {
  onNewSession: () => void;
}

export function EmptyState({onNewSession}: EmptyStateProps) {
  return (
    <div className="flex-1 flex items-center justify-center p-8">
      <div className="max-w-md w-full text-center space-y-6">
        {/* Icon */}
        <div className="flex justify-center">
          <div className="relative">
            <div className="absolute inset-0 bg-primary/20 blur-3xl rounded-full"/>
            <div
              className="relative bg-gradient-to-br from-primary/10 to-primary/5 p-6 rounded-2xl border border-primary/20">
              <Sparkles className="w-16 h-16 text-primary"/>
            </div>
          </div>
        </div>

        {/* Title */}
        <div className="space-y-2">
          <h2 className="text-2xl font-bold text-foreground">
            欢迎使用 Nexus Core
          </h2>
          <p className="text-muted-foreground">
            开始你的第一次对话,体验强大的AI助手
          </p>
        </div>

        {/* Features */}
        <div className="grid gap-3 text-sm text-muted-foreground">
          <div className="flex items-center gap-2 justify-center">
            <div className="w-1.5 h-1.5 rounded-full bg-primary"/>
            <span>实时流式响应</span>
          </div>
          <div className="flex items-center gap-2 justify-center">
            <div className="w-1.5 h-1.5 rounded-full bg-primary"/>
            <span>会话历史自动保存</span>
          </div>
          <div className="flex items-center gap-2 justify-center">
            <div className="w-1.5 h-1.5 rounded-full bg-primary"/>
            <span>支持工具调用和代码执行</span>
          </div>
        </div>

        {/* CTA Button */}
        <button
          onClick={onNewSession}
          className="group relative inline-flex items-center gap-2 px-6 py-3 bg-primary text-primary-foreground rounded-lg font-medium transition-all hover:scale-105 hover:shadow-lg hover:shadow-primary/25"
        >
          <MessageSquarePlus className="w-5 h-5"/>
          <span>创建新会话</span>
          <div
            className="absolute inset-0 bg-gradient-to-r from-transparent via-white/10 to-transparent opacity-0 group-hover:opacity-100 transition-opacity rounded-lg"/>
        </button>

        {/* Hint */}
        <p className="text-xs text-muted-foreground/60">
          提示:你也可以点击左侧的 "New Agent" 按钮
        </p>
      </div>
    </div>
  );
}
