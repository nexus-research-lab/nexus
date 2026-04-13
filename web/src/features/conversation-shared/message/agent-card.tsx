"use client";

import { memo } from "react";
import { Bot, Check, Square, AlertTriangle } from "lucide-react";
import { cn } from "@/lib/utils";
import type { AgentRoundStatus } from "@/features/conversation-shared/utils";
import { MessageLoadingDots } from "./message-primitives";

interface AgentCardProps {
  agent_id: string;
  agent_name: string;
  status: AgentRoundStatus;
  preview_text: string;
  is_active?: boolean;
  on_click: () => void;
  on_stop?: () => void;
}

function StatusIndicator({ status }: { status: AgentRoundStatus }) {
  switch (status) {
    case "pending":
      return <MessageLoadingDots size="sm" name="braille" />;
    case "streaming":
      return <MessageLoadingDots size="sm" name="dna" class_name="text-blue-500" />;
    case "done":
      return <Check className="h-3.5 w-3.5 text-emerald-500" />;
    case "error":
      return <AlertTriangle className="h-3.5 w-3.5 text-rose-500" />;
    case "cancelled":
      return <Square className="h-3 w-3 text-[color:var(--icon-muted)]" />;
  }
}

function statusLabel(status: AgentRoundStatus): string {
  switch (status) {
    case "pending": return "等待中";
    case "streaming": return "回复中…";
    case "done": return "";
    case "error": return "出错";
    case "cancelled": return "已停止";
  }
}

export const AgentCard = memo(function AgentCard({
  agent_name,
  status,
  preview_text,
  is_active = false,
  on_click,
  on_stop,
}: AgentCardProps) {
  const is_busy = status === "pending" || status === "streaming";
  const label = statusLabel(status);

  return (
    <button
      type="button"
      onClick={on_click}
      className={cn(
        "group flex w-full items-center gap-3 rounded-2xl border px-3 py-2.5 text-left transition duration-[var(--motion-duration-fast)] ease-out",
        is_active
          ? "border-[var(--surface-interactive-active-border)] bg-[var(--surface-interactive-active-background)]"
          : "border-[var(--card-default-border)] bg-[var(--card-default-background)] hover:bg-[var(--surface-interactive-hover-background)]",
      )}
    >
      {/* Agent 头像 */}
      <div
        className={cn(
          "flex h-8 w-8 shrink-0 items-center justify-center rounded-[10px] border",
          is_active
            ? "border-[var(--surface-interactive-active-border)] bg-[var(--surface-interactive-active-background)] text-[color:var(--icon-default)]"
            : "",
        )}
        style={!is_active ? {
          background: "var(--surface-avatar-background)",
          borderColor: "var(--surface-avatar-border)",
          color: "var(--surface-avatar-foreground)",
          boxShadow: "var(--surface-avatar-shadow)",
        } : undefined}
      >
        <Bot className="h-3.5 w-3.5" />
      </div>

      {/* 内容区 */}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium text-[color:var(--text-strong)]">{agent_name}</span>
          <StatusIndicator status={status} />
          {label ? (
            <span className="text-xs text-[color:var(--text-soft)]">{label}</span>
          ) : null}
        </div>
        {preview_text ? (
          <p className="mt-0.5 truncate text-xs text-[color:var(--text-muted)]">{preview_text}</p>
        ) : null}
      </div>

      {/* 停止按钮（streaming 时可用） */}
      {is_busy && on_stop ? (
        <button
          type="button"
          aria-label="停止生成"
          onClick={(e) => {
            e.stopPropagation();
            on_stop();
          }}
          className="flex h-6 w-6 shrink-0 items-center justify-center rounded-lg text-[color:var(--icon-muted)] opacity-0 transition duration-[var(--motion-duration-fast)] ease-out group-hover:opacity-100 hover:bg-[var(--surface-interactive-hover-background)] hover:text-[color:var(--icon-default)]"
        >
          <Square className="h-3 w-3 fill-current" />
        </button>
      ) : null}

      {/* 展开箭头提示 */}
      <svg
        className="h-4 w-4 shrink-0 text-[color:var(--icon-muted)] transition-colors duration-[var(--motion-duration-fast)] ease-out group-hover:text-[color:var(--icon-default)]"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        strokeWidth={2}
      >
        <path d="M9 5l7 7-7 7" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
    </button>
  );
});
