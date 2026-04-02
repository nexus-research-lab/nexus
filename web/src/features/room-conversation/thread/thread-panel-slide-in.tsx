"use client";

import { cn } from "@/lib/utils";
import { Message } from "@/types/message";
import { ThreadDetailPanel } from "../thread-detail-panel";

interface ThreadPanelSlideInProps {
  /** 是否打开 */
  is_open: boolean;
  round_id: string | null;
  agent_id: string | null;
  agent_name: string | null;
  /** 已过滤好的 Thread 消息 */
  messages: Message[];
  on_close: () => void;
  on_stop_message?: (msg_id: string) => void;
  on_open_workspace_file?: (path: string) => void;
  is_loading?: boolean;
  layout?: "desktop" | "mobile";
}

/**
 * Thread 面板滑入容器 — 包裹 ThreadDetailPanel，提供滑入/滑出动画。
 * 桌面端：右侧覆盖面板；移动端：全屏覆盖。
 */
export function ThreadPanelSlideIn({
  is_open,
  round_id,
  agent_id,
  agent_name,
  messages,
  on_close,
  on_stop_message,
  on_open_workspace_file,
  is_loading = false,
  layout = "desktop",
}: ThreadPanelSlideInProps) {
  const is_mobile = layout === "mobile";

  if (is_mobile) {
    // 移动端：固定全屏覆盖
    return (
      <div
        className={cn(
          "fixed inset-0 z-50 transition-[transform,opacity] duration-300 ease-[cubic-bezier(0.22,1,0.36,1)]",
          is_open ? "translate-y-0 opacity-100" : "pointer-events-none translate-y-full opacity-0",
        )}
      >
        {/* 半透明背景 */}
        {is_open && (
          <div className="absolute inset-0 bg-black/20" onClick={on_close} />
        )}
        <div className="relative h-full">
          {round_id && agent_id && agent_name ? (
            <ThreadDetailPanel
              round_id={round_id}
              agent_id={agent_id}
              agent_name={agent_name}
              messages={messages}
              on_close={on_close}
              on_stop_message={on_stop_message}
              on_open_workspace_file={on_open_workspace_file}
              is_loading={is_loading}
              layout="mobile"
            />
          ) : null}
        </div>
      </div>
    );
  }

  // 桌面端：右侧覆盖面板
  return (
    <div
      className={cn(
        "absolute right-0 top-0 z-30 h-full w-[420px] max-w-[85%] border-l border-slate-200/60 bg-white shadow-xl",
        "transition-[transform,opacity] duration-300 ease-[cubic-bezier(0.22,1,0.36,1)]",
        is_open ? "translate-x-0 opacity-100" : "pointer-events-none translate-x-3 opacity-0",
      )}
    >
      {round_id && agent_id && agent_name ? (
        <ThreadDetailPanel
          round_id={round_id}
          agent_id={agent_id}
          agent_name={agent_name}
          messages={messages}
          on_close={on_close}
          on_stop_message={on_stop_message}
          on_open_workspace_file={on_open_workspace_file}
          is_loading={is_loading}
          layout="desktop"
        />
      ) : null}
    </div>
  );
}
