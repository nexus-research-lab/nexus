"use client";

import { memo } from "react";
import { Bot, Check, Loader2, Square, AlertTriangle } from "lucide-react";
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
      return <MessageLoadingDots size="sm" />;
    case "streaming":
      return <Loader2 className="h-3.5 w-3.5 animate-spin text-blue-500" />;
    case "done":
      return <Check className="h-3.5 w-3.5 text-emerald-500" />;
    case "error":
      return <AlertTriangle className="h-3.5 w-3.5 text-rose-500" />;
    case "cancelled":
      return <Square className="h-3 w-3 text-slate-400" />;
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
        "group flex w-full items-center gap-3 rounded-2xl border px-3 py-2.5 text-left transition duration-150 ease-out",
        is_active
          ? "border-sky-200/85 bg-sky-50/75 shadow-[0_10px_22px_rgb(59_130_246/0.08)]"
          : "border-[var(--card-default-border)] bg-[var(--card-default-background)] hover:bg-[var(--surface-interactive-hover-background)]",
      )}
    >
      {/* Agent 头像 */}
      <div
          className={cn(
            "flex h-8 w-8 shrink-0 items-center justify-center rounded-[10px] border",
            is_active
              ? "border-sky-200/85 bg-sky-50/85 text-blue-600"
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
          <span className="text-sm font-medium text-slate-800">{agent_name}</span>
          <StatusIndicator status={status} />
          {label ? (
            <span className="text-xs text-slate-400">{label}</span>
          ) : null}
        </div>
        {preview_text ? (
          <p className="mt-0.5 truncate text-xs text-slate-500">{preview_text}</p>
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
          className="flex h-6 w-6 shrink-0 items-center justify-center rounded-lg text-slate-400/90 opacity-0 transition duration-150 ease-out group-hover:opacity-100 hover:bg-slate-100/90 hover:text-slate-600/90"
        >
          <Square className="h-3 w-3 fill-current" />
        </button>
      ) : null}

      {/* 展开箭头提示 */}
      <svg
        className="h-4 w-4 shrink-0 text-slate-300 transition-colors duration-150 ease-out group-hover:text-slate-500"
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
