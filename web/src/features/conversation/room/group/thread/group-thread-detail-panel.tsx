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
import {
  buildConversationScrollContentKey,
} from "@/features/conversation/shared/conversation-scroll-content-key";
import { ScrollToLatestButton } from "@/features/conversation/shared/scroll-to-latest-button";
import { MessageItem } from "@/features/conversation/shared/message";
import { MessageAvatar } from "@/features/conversation/shared/message/ui/message-primitives";

interface GroupThreadDetailPanelProps {
  roundId: string;
  agentId: string;
  agentName: string;
  agentAvatar?: string | null;
  userAvatar?: string | null;
  /** 已过滤好的 Thread 消息。 */
  messages: Message[];
  pendingPermissions?: PendingPermission[];
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  canRespondToPermissions?: boolean;
  permissionReadOnlyReason?: string;
  onClose: () => void;
  onStopMessage?: (msgId: string) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  isLoading?: boolean;
  /** mobile 模式下使用全屏样式 */
  layout?: "desktop" | "mobile";
}

/**
 * Thread 详情面板 — 展示单个 Agent 在某轮中的完整回复内容。
 * 上游已经完成消息过滤，这里只负责展示。
 */
export function GroupThreadDetailPanel({
  roundId: roundId,
  agentId: agentId,
  agentName: agentName,
  agentAvatar: agentAvatar,
  userAvatar: userAvatar,
  messages,
  pendingPermissions: pendingPermissions = [],
  onPermissionResponse: onPermissionResponse,
  canRespondToPermissions: canRespondToPermissions = true,
  permissionReadOnlyReason: permissionReadOnlyReason,
  onClose: onClose,
  onStopMessage: onStopMessage,
  onOpenWorkspaceFile: onOpenWorkspaceFile,
  isLoading: isLoading = false,
  layout = "desktop",
}: GroupThreadDetailPanelProps) {
  const isMobile = layout === "mobile";
  const threadSessionKey = useMemo(
    () => `${roundId}:${agentId}`,
    [agentId, roundId],
  );
  const scrollContentKey = useMemo(
    () => buildConversationScrollContentKey(threadSessionKey, messages),
    [messages, threadSessionKey],
  );
  const {
    scrollRef: scrollRef,
    feedRef: feedRef,
    bottomAnchorRef: bottomAnchorRef,
    onScroll: onScroll,
    onTouchEnd: onTouchEnd,
    onTouchMove: onTouchMove,
    onTouchStart: onTouchStart,
    onWheel: onWheel,
    showScrollToBottom: showScrollToBottom,
    scrollToBottom: scrollToBottom,
  } = useFollowScroll({
    // Thread 和 DM 实时态一样，需要在过程消息、权限确认和 loading 变化时持续跟随到底部。
    messageCount: messages.length,
    auxiliaryBlockCount: pendingPermissions.length,
    contentKey: scrollContentKey,
    isLoading,
    sessionKey: threadSessionKey,
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
          avatarUrl={agentAvatar}
          className="h-8 w-8 shrink-0 rounded-xl"
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
            currentAgentName={agentName}
            currentAgentAvatar={agentAvatar ?? null}
            workspaceAgentId={agentId}
            currentUserAvatar={userAvatar ?? null}
            roundId={roundId}
            messages={messages}
            pendingPermissions={pendingPermissions}
            onPermissionResponse={onPermissionResponse}
            canRespondToPermissions={canRespondToPermissions}
            permissionReadOnlyReason={permissionReadOnlyReason}
            assistantContentMode="room_thread"
            isLastRound
            isLoading={isLoading}
            defaultProcessExpanded
            onOpenWorkspaceFile={onOpenWorkspaceFile}
            onStopMessage={onStopMessage}
            className="max-w-full overflow-x-hidden"
          />
          <div ref={bottomAnchorRef} className="h-px w-full" />
        </div>
      </div>

      {showScrollToBottom ? (
        <ScrollToLatestButton
          isLoading={isLoading}
          isMobileLayout={isMobile}
          placement="panel"
          onClick={() => scrollToBottom("smooth")}
        />
      ) : null}
    </div>
  );
}
