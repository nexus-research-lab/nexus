"use client";

import { useMemo } from "react";
import { ArrowLeft, Bot, X } from "lucide-react";
import { cn } from "@/lib/utils";
import { useFollowScroll } from "@/hooks/use-follow-scroll";
import { Message } from "@/types/message";
import { PendingPermission, PermissionDecisionPayload } from "@/types/permission";
import { MessageItem } from "@/features/conversation-shared/message";

interface ThreadDetailPanelProps {
  round_id: string;
  agent_id: string;
  agent_name: string;
  /** 已过滤好的 Thread 消息。 */
  messages: Message[];
  pending_permissions?: PendingPermission[];
  on_permission_response?: (payload: PermissionDecisionPayload) => boolean;
  can_respond_to_permissions?: boolean;
  permission_read_only_reason?: string;
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
  pending_permissions = [],
  on_permission_response,
  can_respond_to_permissions = true,
  permission_read_only_reason,
  on_close,
  on_stop_message,
  on_open_workspace_file,
  is_loading = false,
  layout = "desktop",
}: ThreadDetailPanelProps) {
  const is_mobile = layout === "mobile";
  const thread_session_key = useMemo(
    () => `${round_id}:${agent_id}`,
    [agent_id, round_id],
  );
  const {
    scroll_ref,
    feed_ref,
    bottom_anchor_ref,
    on_scroll,
    on_touch_end,
    on_touch_move,
    on_touch_start,
    on_wheel,
  } = useFollowScroll({
    // 中文注释：Thread 和 DM 实时态一样，需要在过程消息、权限确认和 loading 变化时持续跟随到底部。
    trigger_deps: [messages, is_loading, pending_permissions] as const,
    session_key: thread_session_key,
  });

  return (
    <div className={cn(
      "flex h-full min-w-0 w-full flex-1 flex-col overflow-hidden",
      is_mobile ? "bg-background" : "bg-[var(--surface-popover-background)]",
    )}>
      {/* ── 头部 ────────────────────────────────────────────── */}
      <div className="flex shrink-0 items-center gap-2 border-b px-3 py-2.5" style={{ borderColor: "var(--divider-subtle-color)" }}>
        {is_mobile ? (
          <button
            type="button"
            onClick={on_close}
            aria-label="关闭 Thread"
            title="关闭 Thread"
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border text-[color:var(--icon-default)] transition-colors hover:bg-[var(--surface-interactive-hover-background)] hover:text-[color:var(--icon-strong)]"
            style={{
              background: "var(--surface-panel-subtle-background)",
              borderColor: "var(--surface-panel-subtle-border)",
            }}
          >
            <ArrowLeft className="h-4 w-4" />
          </button>
        ) : null}

        <div
          className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md border text-[color:var(--icon-default)]"
          style={{
            background: "var(--surface-avatar-background)",
            borderColor: "var(--surface-avatar-border)",
            boxShadow: "var(--surface-avatar-shadow)",
          }}
        >
          <Bot className="h-3.5 w-3.5" />
        </div>
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-semibold text-[color:var(--text-strong)]">{agent_name}</p>
          <p className="text-xs text-[color:var(--text-soft)]">Thread</p>
        </div>

        {!is_mobile ? (
          <button
            type="button"
            onClick={on_close}
            aria-label="关闭 Thread"
            title="关闭 Thread"
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border text-[color:var(--icon-default)] transition-colors hover:bg-[var(--surface-interactive-hover-background)] hover:text-[color:var(--icon-strong)]"
            style={{
              background: "var(--surface-panel-subtle-background)",
              borderColor: "var(--surface-panel-subtle-border)",
            }}
          >
            <X className="h-4 w-4" />
          </button>
        ) : null}
      </div>

      {/* ── 内容区 ────────────────────────────────────────────── */}
      <div
        ref={scroll_ref}
        className="soft-scrollbar min-h-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto px-4 py-3"
        onScroll={on_scroll}
        onTouchEnd={on_touch_end}
        onTouchMove={on_touch_move}
        onTouchStart={on_touch_start}
        onWheel={on_wheel}
      >
        <div ref={feed_ref}>
          <MessageItem
            compact
            current_agent_name={agent_name}
            round_id={round_id}
            messages={messages}
            pending_permissions={pending_permissions}
            on_permission_response={on_permission_response}
            can_respond_to_permissions={can_respond_to_permissions}
            permission_read_only_reason={permission_read_only_reason}
            assistant_content_mode="room_thread"
            is_last_round
            is_loading={is_loading}
            default_process_expanded
            on_open_workspace_file={on_open_workspace_file}
            on_stop_message={on_stop_message}
            class_name="max-w-full overflow-x-hidden"
          />
          <div ref={bottom_anchor_ref} className="h-px w-full" />
        </div>
      </div>
    </div>
  );
}
