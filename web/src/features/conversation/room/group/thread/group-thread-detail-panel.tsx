"use client";

import { useMemo } from "react";
import { ArrowLeft, Bot, X } from "lucide-react";
import { cn } from "@/lib/utils";
import { useFollowScroll } from "@/hooks/conversation/use-follow-scroll";
import { Message } from "@/types/conversation/message";
import {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/permission";
import { ScrollToLatestButton } from "@/features/conversation/shared/scroll-to-latest-button";
import { MessageItem } from "@/features/conversation/shared/message";
import { MessageAvatar } from "@/features/conversation/shared/message/ui/message-primitives";

interface GroupThreadDetailPanelProps {
  round_id: string;
  agent_id: string;
  agent_name: string;
  agent_avatar?: string | null;
  user_avatar?: string | null;
  /** 已过滤好的 Thread 消息。 */
  messages: Message[];
  pending_permissions?: PendingPermission[];
  on_permission_response?: (payload: PermissionDecisionPayload) => boolean;
  can_respond_to_permissions?: boolean;
  permission_read_only_reason?: string;
  on_close: () => void;
  on_stop_message?: (msgId: string) => void;
  on_open_workspace_file?: (path: string) => void;
  is_loading?: boolean;
  /** mobile 模式下使用全屏样式 */
  layout?: "desktop" | "mobile";
}

/**
 * Thread 详情面板 — 展示单个 Agent 在某轮中的完整回复内容。
 * 上游已经完成消息过滤，这里只负责展示。
 */
export function GroupThreadDetailPanel({
  round_id: roundId,
  agent_id: agentId,
  agent_name: agentName,
  agent_avatar: agentAvatar,
  user_avatar: userAvatar,
  messages,
  pending_permissions: pendingPermissions = [],
  on_permission_response: onPermissionResponse,
  can_respond_to_permissions: canRespondToPermissions = true,
  permission_read_only_reason: permissionReadOnlyReason,
  on_close: onClose,
  on_stop_message: onStopMessage,
  on_open_workspace_file: onOpenWorkspaceFile,
  is_loading: isLoading = false,
  layout = "desktop",
}: GroupThreadDetailPanelProps) {
  const isMobile = layout === "mobile";
  const threadSessionKey = useMemo(
    () => `${roundId}:${agentId}`,
    [agentId, roundId],
  );
  const {
    scroll_ref: scrollRef,
    feed_ref: feedRef,
    bottom_anchor_ref: bottomAnchorRef,
    on_scroll: onScroll,
    on_touch_end: onTouchEnd,
    on_touch_move: onTouchMove,
    on_touch_start: onTouchStart,
    on_wheel: onWheel,
    show_scroll_to_bottom: showScrollToBottom,
    scroll_to_bottom: scrollToBottom,
  } = useFollowScroll({
    // Thread 和 DM 实时态一样，需要在过程消息、权限确认和 loading 变化时持续跟随到底部。
    message_count: messages.length,
    auxiliary_block_count: pendingPermissions.length,
    is_loading: isLoading,
    session_key: threadSessionKey,
  });

  return (
    <div
      className={cn(
        "relative flex h-full min-w-0 w-full flex-1 flex-col overflow-hidden",
        isMobile ? "bg-(--surface-panel-background)" : "bg-transparent",
      )}
    >
      {/* ── 头部 ────────────────────────────────────────────── */}
      <div
        className="flex shrink-0 items-center gap-2 border-b px-3 py-3"
        style={{ borderColor: "var(--divider-subtle-color)" }}
      >
        {isMobile ? (
          <button
            type="button"
            onClick={onClose}
            aria-label="关闭 Thread"
            title="关闭 Thread"
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-(--icon-default) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong)"
          >
            <ArrowLeft className="h-4 w-4" />
          </button>
        ) : null}

        <MessageAvatar
          avatar_url={agentAvatar}
          class_name="h-8 w-8 shrink-0 rounded-xl"
          size="full"
        >
          {!agentAvatar && <Bot className="h-3.5 w-3.5" />}
        </MessageAvatar>
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-semibold text-(--text-strong)">
            {agentName}
          </p>
          <p className="text-xs text-(--text-soft)">Thread</p>
        </div>

        {!isMobile ? (
          <button
            type="button"
            onClick={onClose}
            aria-label="关闭 Thread"
            title="关闭 Thread"
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-(--icon-default) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong)"
          >
            <X className="h-4 w-4" />
          </button>
        ) : null}
      </div>

      {/* ── 内容区 ────────────────────────────────────────────── */}
      <div
        ref={scrollRef}
        className="soft-scrollbar min-h-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto px-4 py-3"
        onScroll={onScroll}
        onTouchEnd={onTouchEnd}
        onTouchMove={onTouchMove}
        onTouchStart={onTouchStart}
        onWheel={onWheel}
      >
        <div ref={feedRef}>
          <MessageItem
            compact
            current_agent_name={agentName}
            current_agent_avatar={agentAvatar ?? null}
            workspace_agent_id={agentId}
            current_user_avatar={userAvatar ?? null}
            round_id={roundId}
            messages={messages}
            pending_permissions={pendingPermissions}
            on_permission_response={onPermissionResponse}
            can_respond_to_permissions={canRespondToPermissions}
            permission_read_only_reason={permissionReadOnlyReason}
            assistant_content_mode="room_thread"
            is_last_round
            is_loading={isLoading}
            default_process_expanded
            on_open_workspace_file={onOpenWorkspaceFile}
            on_stop_message={onStopMessage}
            class_name="max-w-full overflow-x-hidden"
          />
          <div ref={bottomAnchorRef} className="h-px w-full" />
        </div>
      </div>

      {showScrollToBottom ? (
        <ScrollToLatestButton
          is_loading={isLoading}
          is_mobile_layout={isMobile}
          placement="panel"
          on_click={() => scrollToBottom("smooth")}
        />
      ) : null}
    </div>
  );
}
