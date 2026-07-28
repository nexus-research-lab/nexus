"use client";

/**
 * INPUT: Thread 视图模型、消息上下文与滚动/触摸/pointer 处理器。
 * OUTPUT: 禁用浏览器锚点争抢的 Thread 标题、消息流和回到底部入口。
 * POS: Agent Thread 的纯展示与事件绑定层。
 */
import { ArrowLeft, Bot, X, type LucideIcon } from "lucide-react";
import type {
  PointerEventHandler,
  ReactNode,
  RefObject,
  TouchEventHandler,
  UIEventHandler,
  WheelEventHandler,
} from "react";

import { MessageItem } from "@/features/conversation/shared/message/item/message-item";
import { MessageAvatar } from "@/features/conversation/shared/message/ui/message-avatar";
import { ScrollToLatestButton } from "@/features/conversation/shared/scroll-to-latest-button";
import { cn } from "@/shared/ui/class-name";
import type { PermissionDecisionPayload } from "@/types/conversation/interaction/permission";

import type {
  ConversationThreadModel,
  ConversationThreadNavigationAction,
  ConversationThreadRoundModel,
} from "./conversation-thread-model";

export interface ConversationThreadMessageContext {
  agentAvatar: string | null;
  agentName: string;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  onStopMessage?: (msgId: string) => void;
  userAvatar: string | null;
  workspaceAgentId: string;
}

interface ConversationThreadViewProps {
  agentAvatar: string | null;
  agentName: string;
  bottomAnchorRef: RefObject<HTMLDivElement | null>;
  emptyContent: ReactNode;
  feedRef: RefObject<HTMLDivElement | null>;
  footer: ReactNode;
  headerAction: ReactNode;
  headerAvatar: ReactNode;
  isLoading: boolean;
  messageContext: ConversationThreadMessageContext;
  model: ConversationThreadModel;
  notice: ReactNode;
  onClose: () => void;
  onPointerDown: PointerEventHandler<HTMLDivElement>;
  onScroll: UIEventHandler<HTMLDivElement>;
  onScrollToLatest: () => void;
  onTouchEnd: TouchEventHandler<HTMLDivElement>;
  onTouchMove: TouchEventHandler<HTMLDivElement>;
  onTouchStart: TouchEventHandler<HTMLDivElement>;
  onWheel: WheelEventHandler<HTMLDivElement>;
  scrollRef: RefObject<HTMLDivElement | null>;
  showScrollToLatest: boolean;
  subtitle: ReactNode;
}

interface ThreadFeedProps {
  bottomAnchorRef: RefObject<HTMLDivElement | null>;
  emptyContent: ReactNode;
  feedRef: RefObject<HTMLDivElement | null>;
  messageContext: ConversationThreadMessageContext;
  model: ConversationThreadModel;
  onPointerDown: PointerEventHandler<HTMLDivElement>;
  onScroll: UIEventHandler<HTMLDivElement>;
  onTouchEnd: TouchEventHandler<HTMLDivElement>;
  onTouchMove: TouchEventHandler<HTMLDivElement>;
  onTouchStart: TouchEventHandler<HTMLDivElement>;
  onWheel: WheelEventHandler<HTMLDivElement>;
  scrollRef: RefObject<HTMLDivElement | null>;
}

interface ThreadNavigationPresentation {
  Icon: LucideIcon;
  ariaLabel: string;
  title: string;
}

type ThreadNavigationButtonAction = Exclude<
  ConversationThreadNavigationAction,
  null
>;

const THREAD_NAVIGATION_PRESENTATION: Record<
  ThreadNavigationButtonAction,
  ThreadNavigationPresentation
> = {
  back: { Icon: ArrowLeft, ariaLabel: "返回", title: "返回" },
  close: { Icon: X, ariaLabel: "关闭 Thread", title: "关闭 Thread" },
};

export function ConversationThreadView({
  agentAvatar,
  agentName,
  bottomAnchorRef,
  emptyContent,
  feedRef,
  footer,
  headerAction,
  headerAvatar,
  isLoading,
  messageContext,
  model,
  notice,
  onClose,
  onPointerDown,
  onScroll,
  onScrollToLatest,
  onTouchEnd,
  onTouchMove,
  onTouchStart,
  onWheel,
  scrollRef,
  showScrollToLatest,
  subtitle,
}: ConversationThreadViewProps) {
  return (
    <div
      className={cn(
        "relative flex h-full min-w-0 w-full flex-1 flex-col overflow-hidden",
        model.isMobile ? "bg-(--surface-panel-background)" : "bg-transparent",
      )}
    >
      <ThreadHeader
        agentAvatar={agentAvatar}
        agentName={agentName}
        headerAction={headerAction}
        headerAvatar={headerAvatar}
        leadingAction={model.leadingAction}
        onClose={onClose}
        subtitle={subtitle}
        trailingAction={model.trailingAction}
      />
      {notice}
      <ThreadFeed
        bottomAnchorRef={bottomAnchorRef}
        emptyContent={emptyContent}
        feedRef={feedRef}
        messageContext={messageContext}
        model={model}
        onPointerDown={onPointerDown}
        onScroll={onScroll}
        onTouchEnd={onTouchEnd}
        onTouchMove={onTouchMove}
        onTouchStart={onTouchStart}
        onWheel={onWheel}
        scrollRef={scrollRef}
      />
      <ThreadScrollToLatest
        isLoading={isLoading}
        show={showScrollToLatest}
        onClick={onScrollToLatest}
      />
      {footer}
    </div>
  );
}

function ThreadHeader({
  agentAvatar,
  agentName,
  headerAction,
  headerAvatar,
  leadingAction,
  onClose,
  subtitle,
  trailingAction,
}: {
  agentAvatar: string | null;
  agentName: string;
  headerAction: ReactNode;
  headerAvatar: ReactNode;
  leadingAction: ConversationThreadNavigationAction;
  onClose: () => void;
  subtitle: ReactNode;
  trailingAction: ConversationThreadNavigationAction;
}) {
  return (
    <header className="flex shrink-0 items-center gap-2 border-b border-(--divider-subtle-color) px-3 py-3">
      <ThreadNavigationButton action={leadingAction} onClick={onClose} />
      {headerAvatar ?? <ThreadAgentAvatar avatarUrl={agentAvatar} />}
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-semibold text-(--text-strong)">
          {agentName}
        </p>
        <ThreadSubtitle>{subtitle}</ThreadSubtitle>
      </div>
      {headerAction}
      <ThreadNavigationButton action={trailingAction} onClick={onClose} />
    </header>
  );
}

function ThreadNavigationButton({
  action,
  onClick,
}: {
  action: ConversationThreadNavigationAction;
  onClick: () => void;
}) {
  if (!action) {
    return null;
  }
  const presentation = THREAD_NAVIGATION_PRESENTATION[action];
  const { Icon } = presentation;
  return (
    <button
      aria-label={presentation.ariaLabel}
      className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-(--icon-default) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong)"
      onClick={onClick}
      title={presentation.title}
      type="button"
    >
      <Icon className="h-4 w-4" />
    </button>
  );
}

function ThreadAgentAvatar({ avatarUrl }: { avatarUrl: string | null }) {
  return (
    <MessageAvatar
      avatarUrl={avatarUrl}
      className="h-8 w-8 shrink-0 radius-control-md"
      size="full"
    >
      {avatarUrl ? null : <Bot className="h-3.5 w-3.5" />}
    </MessageAvatar>
  );
}

function ThreadSubtitle({ children }: { children: ReactNode }) {
  if (!children) {
    return null;
  }
  return <div className="text-xs text-(--text-soft)">{children}</div>;
}

function ThreadFeed({
  bottomAnchorRef,
  emptyContent,
  feedRef,
  messageContext,
  model,
  onPointerDown,
  onScroll,
  onTouchEnd,
  onTouchMove,
  onTouchStart,
  onWheel,
  scrollRef,
}: ThreadFeedProps) {
  return (
    <div
      className="soft-scrollbar min-h-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto overscroll-y-none px-4 py-3"
      style={{ overflowAnchor: "none" }}
      tabIndex={-1}
      onPointerDown={onPointerDown}
      onScroll={onScroll}
      onTouchEnd={onTouchEnd}
      onTouchMove={onTouchMove}
      onTouchStart={onTouchStart}
      onWheel={onWheel}
      ref={scrollRef}
    >
      <div ref={feedRef}>
        <ThreadRounds
          emptyContent={emptyContent}
          messageContext={messageContext}
          rounds={model.rounds}
        />
        <div className="h-px w-full" ref={bottomAnchorRef} />
      </div>
    </div>
  );
}

function ThreadRounds({
  emptyContent,
  messageContext,
  rounds,
}: {
  emptyContent: ReactNode;
  messageContext: ConversationThreadMessageContext;
  rounds: ConversationThreadRoundModel[];
}) {
  if (rounds.length === 0) {
    return emptyContent;
  }
  return rounds.map((round) => (
    <ThreadRound
      key={round.roundId}
      messageContext={messageContext}
      round={round}
    />
  ));
}

function ThreadRound({
  messageContext,
  round,
}: {
  messageContext: ConversationThreadMessageContext;
  round: ConversationThreadRoundModel;
}) {
  return (
    <>
      <MessageItem
        assistantContentMode="room_thread"
        className="max-w-full overflow-x-hidden"
        compact
        currentAgentAvatar={messageContext.agentAvatar}
        currentAgentName={messageContext.agentName}
        currentUserAvatar={messageContext.userAvatar}
        defaultProcessExpanded
        isLastRound={round.isLast}
        isLoading={round.isLoading}
        messages={round.messages}
        onOpenWorkspaceFile={messageContext.onOpenWorkspaceFile}
        onPermissionResponse={messageContext.onPermissionResponse}
        onStopMessage={messageContext.onStopMessage}
        pendingPermissions={round.pendingPermissions}
        roundId={round.roundId}
        workspaceAgentId={messageContext.workspaceAgentId}
      />
      {round.showDivider ? (
        <hr aria-hidden="true" className="conversation-round-divider" />
      ) : null}
    </>
  );
}

function ThreadScrollToLatest({
  isLoading,
  onClick,
  show,
}: {
  isLoading: boolean;
  onClick: () => void;
  show: boolean;
}) {
  return (
    <ScrollToLatestButton
      isLoading={isLoading}
      onClick={onClick}
      visible={show}
    />
  );
}
