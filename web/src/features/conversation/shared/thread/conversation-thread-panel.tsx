"use client";

/**
 * INPUT: Thread 消息、运行态、交互请求与关闭动作。
 * OUTPUT: 带独立跟随滚动和消息上下文的 Thread 面板。
 * POS: Room Agent Thread 的状态装配入口。
 */
import { type ReactNode, useMemo } from "react";

import {
  buildConversationAtomicLayoutKey,
  buildConversationScrollContentKey,
  buildConversationScrollTopologyKey,
} from "@/features/conversation/shared/timeline/scroll/follow-scroll-model";
import { useFollowScroll } from "@/features/conversation/shared/timeline/scroll/use-follow-scroll";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/interaction/permission";
import type { Message } from "@/types/conversation/message/entity";

import {
  buildConversationThreadModel,
  type ConversationThreadLayout,
  type ConversationThreadNavigation,
  type ConversationThreadPresentation,
  type ConversationThreadRound,
} from "./conversation-thread-model";
import {
  ConversationThreadView,
  type ConversationThreadMessageContext,
} from "./conversation-thread-view";

interface ConversationThreadPanelProps {
  roundId: string;
  agentId: string;
  agentName: string;
  agentAvatar?: string | null;
  /** 已过滤好的 Thread 消息。 */
  messages: Message[];
  /** 子智能体可在同一个 Thread 中连续产生多轮消息。 */
  rounds?: ConversationThreadRound[];
  pendingPermissions?: PendingPermission[];
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  onClose: () => void;
  onStopMessage?: (msgId: string) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  isLoading?: boolean;
  /** mobile 模式下使用全屏样式。 */
  layout?: ConversationThreadLayout;
  /** 默认按布局选择返回或关闭，也可由复用方显式指定。 */
  navigation?: ConversationThreadNavigation;
  /** transcript 展示完整记录；inspector 只保留执行过程。 */
  presentation?: ConversationThreadPresentation;
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

const EMPTY_PENDING_PERMISSIONS: PendingPermission[] = [];

/**
 * Thread 面板只编排共享消息轨道；会话来源负责提供已过滤数据和领域动作。
 */
export function ConversationThreadPanel({
  roundId,
  agentId,
  agentName,
  agentAvatar,
  messages,
  rounds,
  pendingPermissions,
  onPermissionResponse,
  onClose,
  onStopMessage,
  onOpenWorkspaceFile,
  isLoading,
  layout,
  navigation,
  presentation,
  headerAvatar,
  headerSubtitle,
  headerAction,
  notice,
  footer,
  emptyContent,
  sessionKey,
  workspaceAgentId,
}: ConversationThreadPanelProps) {
  const resolvedAgentAvatar = valueOrDefault(agentAvatar, null);
  const resolvedPendingPermissions = valueOrDefault(
    pendingPermissions,
    EMPTY_PENDING_PERMISSIONS,
  );
  const resolvedIsLoading = valueOrDefault(isLoading, false);
  const resolvedLayout = valueOrDefault(layout, "desktop");
  const resolvedNavigation = valueOrDefault(navigation, "auto");
  const resolvedPresentation = valueOrDefault(presentation, "transcript");
  const model = useMemo(
    () =>
      buildConversationThreadModel({
        agentId,
        isLoading: resolvedIsLoading,
        layout: resolvedLayout,
        messages,
        navigation: resolvedNavigation,
        pendingPermissions: resolvedPendingPermissions,
        presentation: resolvedPresentation,
        roundId,
        rounds,
        sessionKey,
        workspaceAgentId,
      }),
    [
      agentId,
      messages,
      resolvedIsLoading,
      resolvedLayout,
      resolvedNavigation,
      resolvedPendingPermissions,
      resolvedPresentation,
      roundId,
      rounds,
      sessionKey,
      workspaceAgentId,
    ],
  );
  const scrollContentKey = useMemo(
    () => buildConversationScrollContentKey(model.sessionKey, model.allMessages),
    [model.allMessages, model.sessionKey],
  );
  const scrollTopologyKey = useMemo(
    () => buildConversationScrollTopologyKey(
      model.sessionKey,
      model.allMessages,
    ),
    [model.allMessages, model.sessionKey],
  );
  const atomicLayoutKey = useMemo(
    () => buildConversationAtomicLayoutKey(
      model.sessionKey,
      model.allMessages,
      resolvedPendingPermissions,
    ),
    [model.allMessages, model.sessionKey, resolvedPendingPermissions],
  );
  const followScroll = useFollowScroll({
    atomicLayoutKey,
    messageCount: model.allMessages.length,
    contentKey: scrollContentKey,
    sessionKey: model.sessionKey,
    topologyKey: scrollTopologyKey,
  });
  const messageContext: ConversationThreadMessageContext = {
    agentAvatar: resolvedAgentAvatar,
    agentName,
    onOpenWorkspaceFile,
    onPermissionResponse,
    onStopMessage,
    workspaceAgentId: model.workspaceAgentId,
  };

  return (
    <ConversationThreadView
      agentAvatar={resolvedAgentAvatar}
      agentName={agentName}
      bottomAnchorRef={followScroll.bottomAnchorRef}
      emptyContent={valueOrDefault(emptyContent, null)}
      feedRef={followScroll.feedRef}
      footer={footer}
      headerAction={headerAction}
      headerAvatar={headerAvatar}
      messageContext={messageContext}
      model={model}
      notice={notice}
      onClose={onClose}
      onPointerDown={followScroll.onPointerDown}
      onScroll={followScroll.onScroll}
      onScrollToLatest={() => followScroll.scrollToBottom("smooth")}
      onTouchEnd={followScroll.onTouchEnd}
      onTouchMove={followScroll.onTouchMove}
      onTouchStart={followScroll.onTouchStart}
      onWheel={followScroll.onWheel}
      scrollRef={followScroll.scrollRef}
      showScrollToLatest={followScroll.showScrollToBottom}
      subtitle={resolveThreadSubtitle(headerSubtitle)}
    />
  );
}

function valueOrDefault<T>(value: T | undefined, fallback: T): T {
  return value === undefined ? fallback : value;
}

function resolveThreadSubtitle(subtitle: ReactNode | undefined): ReactNode {
  return subtitle === undefined ? "Thread" : subtitle;
}
