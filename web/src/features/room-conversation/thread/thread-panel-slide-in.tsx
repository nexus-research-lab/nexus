"use client";

import { cn } from "@/lib/utils";
import { Message } from "@/types/message";
import { PendingPermission, PermissionDecisionPayload } from "@/types/permission";
import { ThreadDetailPanel } from "../thread-detail-panel";

interface ThreadPanelSlideInProps {
  /** 是否打开 */
  is_open: boolean;
  round_id: string | null;
  agent_id: string | null;
  agent_name: string | null;
  /** 已过滤好的 Thread 消息 */
  messages: Message[];
  pending_permissions?: PendingPermission[];
  on_permission_response?: (payload: PermissionDecisionPayload) => boolean;
  can_respond_to_permissions?: boolean;
  permission_read_only_reason?: string;
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
  pending_permissions = [],
  on_permission_response,
  can_respond_to_permissions = true,
  permission_read_only_reason,
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
              pending_permissions={pending_permissions}
              on_permission_response={on_permission_response}
              can_respond_to_permissions={can_respond_to_permissions}
              permission_read_only_reason={permission_read_only_reason}
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
        "absolute right-0 top-0 z-30 h-full w-[420px] max-w-[85%] border-l shadow-xl",
        "transition-[transform,opacity] duration-300 ease-[cubic-bezier(0.22,1,0.36,1)]",
        is_open ? "translate-x-0 opacity-100" : "pointer-events-none translate-x-3 opacity-0",
      )}
      style={{
        background: "var(--surface-popover-background)",
        borderColor: "var(--surface-popover-border)",
      }}
    >
      {round_id && agent_id && agent_name ? (
        <ThreadDetailPanel
          round_id={round_id}
          agent_id={agent_id}
          agent_name={agent_name}
          messages={messages}
          pending_permissions={pending_permissions}
          on_permission_response={on_permission_response}
          can_respond_to_permissions={can_respond_to_permissions}
          permission_read_only_reason={permission_read_only_reason}
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
