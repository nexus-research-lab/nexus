"use client";

import { useMemo, type ReactNode } from "react";
import { ArrowLeft, Bot, X } from "lucide-react";

import { useFollowScroll } from "@/hooks/conversation/use-follow-scroll";
import { cn } from "@/lib/utils";
import { MessageItem } from "@/features/conversation/shared/message";
import {
  buildConversationScrollContentKey,
} from "@/features/conversation/shared/timeline/scroll-content-key";
import { ScrollToLatestButton } from "@/features/conversation/shared/scroll-to-latest-button";
import { MessageAvatar } from "@/features/conversation/shared/message/ui/message-primitives";
import { Message } from "@/types/conversation/message";
import {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/permission";

export interface GroupThreadRound {
  roundId: string;
  messages: Message[];
}

interface GroupThreadDetailPanelProps {
  roundId: string;
  agentId: string;
  agentName: string;
  agentAvatar?: string | null;
  userAvatar?: string | null;
  /** 已过滤好的 Thread 消息。 */
  messages: Message[];
  /** 子智能体可在同一个 Thread 中连续产生多轮消息。 */
  rounds?: GroupThreadRound[];
  pendingPermissions?: PendingPermission[];
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  onClose: () => void;
  onStopMessage?: (msgId: string) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  isLoading?: boolean;
  /** mobile 模式下使用全屏样式。 */
  layout?: "desktop" | "mobile";
  /** 默认按布局选择返回或关闭，也可由复用方显式指定。 */
  navigation?: "auto" | "back" | "close";
  /** 覆盖默认头像，用于没有持久化头像的稳定视觉身份。 */
  headerAvatar?: ReactNode;
  /** undefined 使用 Thread，null 隐藏副标题。 */
  headerSubtitle?: ReactNode;
  headerAction?: ReactNode;
  notice?: ReactNode;
  footer?: ReactNode;
  emptyContent?: ReactNode;
  sessionKey?: string;
  workspaceAgentId?: string | null;
}

/**
 * Thread 详情面板：群聊回复与子智能体都复用同一个完整过程渲染器。
 * 上游只负责提供消息轮次和能力动作，这里统一处理思考、工具、文件与滚动状态。
 */
export function GroupThreadDetailPanel({
  roundId,
  agentId,
  agentName,
  agentAvatar,
  userAvatar,
  messages,
  rounds,
  pendingPermissions = [],
  onPermissionResponse,
  onClose,
  onStopMessage,
  onOpenWorkspaceFile,
  isLoading = false,
  layout = "desktop",
  navigation = "auto",
  headerAvatar,
  headerSubtitle,
  headerAction,
  notice,
  footer,
  emptyContent,
  sessionKey,
  workspaceAgentId,
}: GroupThreadDetailPanelProps) {
  const isMobile = layout === "mobile";
  const resolvedRounds = useMemo<GroupThreadRound[]>(
    () => rounds ?? [{ roundId, messages }],
    [messages, roundId, rounds],
  );
  const allMessages = useMemo(
    () => resolvedRounds.flatMap((round) => round.messages),
    [resolvedRounds],
  );
  const threadSessionKey = useMemo(
    () => sessionKey ?? `${roundId}:${agentId}`,
    [agentId, roundId, sessionKey],
  );
  const scrollContentKey = useMemo(
    () => buildConversationScrollContentKey(threadSessionKey, allMessages),
    [allMessages, threadSessionKey],
  );
  const {
    scrollRef,
    feedRef,
    bottomAnchorRef,
    onScroll,
    onTouchEnd,
    onTouchMove,
    onTouchStart,
    onWheel,
    showScrollToBottom,
    scrollToBottom,
  } = useFollowScroll({
    // Thread 和 DM 实时态一样，需要在过程消息、权限确认和 loading 变化时持续跟随到底部。
    messageCount: allMessages.length,
    auxiliaryBlockCount: pendingPermissions.length,
    contentKey: scrollContentKey,
    isLoading,
    sessionKey: threadSessionKey,
  });
  const showBack = navigation === "back" || (navigation === "auto" && isMobile);
  const showClose = navigation === "close" || (navigation === "auto" && !isMobile);
  const subtitle = headerSubtitle === undefined ? "Thread" : headerSubtitle;

  return (
    <div
      className={cn(
        "relative flex h-full min-w-0 w-full flex-1 flex-col overflow-hidden",
        isMobile ? "bg-(--surface-panel-background)" : "bg-transparent",
      )}
    >
      <header className="flex shrink-0 items-center gap-2 border-b border-(--divider-subtle-color) px-3 py-3">
        {showBack ? (
          <button
            aria-label="返回"
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-(--icon-default) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong)"
            onClick={onClose}
            title="返回"
            type="button"
          >
            <ArrowLeft className="h-4 w-4" />
          </button>
        ) : null}

        {headerAvatar ?? (
          <MessageAvatar
            avatarUrl={agentAvatar}
            className="h-8 w-8 shrink-0 rounded-xl"
            size="full"
          >
            {!agentAvatar && <Bot className="h-3.5 w-3.5" />}
          </MessageAvatar>
        )}
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-semibold text-(--text-strong)">
            {agentName}
          </p>
          {subtitle ? <div className="text-xs text-(--text-soft)">{subtitle}</div> : null}
        </div>

        {headerAction}
        {showClose ? (
          <button
            aria-label="关闭 Thread"
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-(--icon-default) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong)"
            onClick={onClose}
            title="关闭 Thread"
            type="button"
          >
            <X className="h-4 w-4" />
          </button>
        ) : null}
      </header>

      {notice}

      <div
        className="soft-scrollbar min-h-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto px-4 py-3"
        onScroll={onScroll}
        onTouchEnd={onTouchEnd}
        onTouchMove={onTouchMove}
        onTouchStart={onTouchStart}
        onWheel={onWheel}
        ref={scrollRef}
      >
        <div ref={feedRef}>
          {resolvedRounds.length === 0 ? emptyContent : resolvedRounds.map((round, index) => {
            const isLastRound = index === resolvedRounds.length - 1;
            return (
              <MessageItem
                assistantContentMode="room_thread"
                className={cn(
                  "max-w-full overflow-x-hidden",
                  !isLastRound && "border-b border-(--divider-subtle-color)",
                )}
                compact
                currentAgentAvatar={agentAvatar ?? null}
                currentAgentName={agentName}
                currentUserAvatar={userAvatar ?? null}
                defaultProcessExpanded
                isLastRound={isLastRound}
                isLoading={isLastRound && isLoading}
                key={round.roundId}
                messages={round.messages}
                onOpenWorkspaceFile={onOpenWorkspaceFile}
                onPermissionResponse={onPermissionResponse}
                onStopMessage={onStopMessage}
                pendingPermissions={isLastRound ? pendingPermissions : []}
                roundId={round.roundId}
                workspaceAgentId={workspaceAgentId ?? agentId}
              />
            );
          })}
          <div className="h-px w-full" ref={bottomAnchorRef} />
        </div>
      </div>

      {showScrollToBottom ? (
        <ScrollToLatestButton
          isLoading={isLoading}
          isMobileLayout={isMobile}
          onClick={() => scrollToBottom("smooth")}
          placement="panel"
        />
      ) : null}

      {footer}
    </div>
  );
}
