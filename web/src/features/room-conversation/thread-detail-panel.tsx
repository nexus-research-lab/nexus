"use client";

import { useRef } from "react";
import { ArrowLeft, Bot, X } from "lucide-react";
import { cn } from "@/lib/utils";
import { Message } from "@/types/message";
import { MessageItem } from "@/features/conversation-shared/message";

interface ThreadDetailPanelProps {
  round_id: string;
  agent_id: string;
  agent_name: string;
  /** 已过滤好的 Thread 消息。 */
  messages: Message[];
  on_close: () => void;
  on_stop_message?: (msg_id: string) => void;
  on_open_workspace_file?: (path: string) => void;
  is_loading?: boolean;
  /** mobile 模式下使用全屏样式 */
  layout?: "desktop" | "mobile";
}

/**
 * Thread 详情面板 — 展示单个 Agent 在某轮中的完整回复内容。
 * 上游已经完成消息过滤，这里只负责展示。
 */
export function ThreadDetailPanel({
  round_id,
  agent_id,
  agent_name,
  messages,
  on_close,
  on_stop_message,
  on_open_workspace_file,
  is_loading = false,
  layout = "desktop",
}: ThreadDetailPanelProps) {
  const scroll_ref = useRef<HTMLDivElement>(null);
  const is_mobile = layout === "mobile";

  return (
    <div className={cn(
      "flex h-full min-w-0 flex-col overflow-hidden",
      is_mobile ? "bg-background" : "bg-white",
    )}>
      {/* ── 头部 ────────────────────────────────────────────── */}
      <div className="flex shrink-0 items-center gap-2 border-b border-slate-200/60 px-3 py-2.5">
        {is_mobile ? (
          <button
            type="button"
            onClick={on_close}
            aria-label="关闭 Thread"
            title="关闭 Thread"
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-slate-200 bg-white text-slate-500 transition-colors hover:bg-slate-50 hover:text-slate-700"
          >
            <ArrowLeft className="h-4 w-4" />
          </button>
        ) : null}

        <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md border border-slate-200 bg-slate-50 text-slate-500">
          <Bot className="h-3.5 w-3.5" />
        </div>
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-semibold text-slate-800">{agent_name}</p>
          <p className="text-xs text-slate-400">Thread</p>
        </div>

        {!is_mobile ? (
          <button
            type="button"
            onClick={on_close}
            aria-label="关闭 Thread"
            title="关闭 Thread"
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-slate-200 bg-white text-slate-500 transition-colors hover:bg-slate-50 hover:text-slate-700"
          >
            <X className="h-4 w-4" />
          </button>
        ) : null}
      </div>

      {/* ── 内容区 ────────────────────────────────────────────── */}
      <div
        ref={scroll_ref}
        className="soft-scrollbar min-h-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto px-2 py-3"
      >
        <MessageItem
          compact
          current_agent_name={agent_name}
          round_id={round_id}
          messages={messages}
          is_last_round
          is_loading={is_loading}
          default_process_expanded
          on_open_workspace_file={on_open_workspace_file}
          on_stop_message={on_stop_message}
          class_name="max-w-full overflow-x-hidden"
        />
      </div>
    </div>
  );
}
