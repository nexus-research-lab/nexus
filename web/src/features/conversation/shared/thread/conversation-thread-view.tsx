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
import {
  WORKSPACE_PANEL_HEADER_BUTTON_CLASS,
  WORKSPACE_PANEL_HEADER_HEIGHT_CLASS,
  WORKSPACE_PANEL_HEADER_ICON_CLASS,
  WORKSPACE_PANEL_HEADER_PADDING_CLASS,
} from "@/shared/ui/workspace/surface/workspace-header-layout";
import type { PermissionDecisionPayload } from "@/types/conversation/interaction/permission";
import type { UnresolvedToolStatus } from "@/features/conversation/shared/message/item/view/content/content-renderer-contract";

import type {
  ConversationThreadModel,
  ConversationThreadNavigationAction,
  ConversationThreadPresentation,
  ConversationThreadRoundModel,
} from "./conversation-thread-model";

export interface ConversationThreadMessageContext {
  agentAvatar: string | null;
  agentName: string;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  onStopMessage?: (msgId: string) => void;
  unresolvedToolStatus?: UnresolvedToolStatus;
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
        isMobile={model.isMobile}
        leadingAction={model.leadingAction}
        onClose={onClose}
        presentation={model.presentation}
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
  isMobile,
  leadingAction,
  onClose,
  presentation,
  subtitle,
  trailingAction,
}: {
  agentAvatar: string | null;
  agentName: string;
  headerAction: ReactNode;
  headerAvatar: ReactNode;
  isMobile: boolean;
  leadingAction: ConversationThreadNavigationAction;
  onClose: () => void;
  presentation: ConversationThreadPresentation;
  subtitle: ReactNode;
  trailingAction: ConversationThreadNavigationAction;
}) {
  return (
    <header
      className={cn(
        "flex shrink-0 items-center gap-2",
        isMobile ? "h-[52px]" : WORKSPACE_PANEL_HEADER_HEIGHT_CLASS,
        WORKSPACE_PANEL_HEADER_PADDING_CLASS,
        presentation === "transcript"
          && "border-b border-(--divider-subtle-color)",
      )}
    >
      <ThreadNavigationButton
        action={leadingAction}
        isMobile={isMobile}
        onClick={onClose}
      />
      {headerAvatar ?? (
        <ThreadAgentAvatar avatarUrl={agentAvatar} isMobile={isMobile} />
      )}
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-semibold text-(--text-strong)">
          {agentName}
        </p>
        <ThreadSubtitle>{subtitle}</ThreadSubtitle>
      </div>
      {headerAction}
      <ThreadNavigationButton
        action={trailingAction}
        isMobile={isMobile}
        onClick={onClose}
      />
    </header>
  );
}

function ThreadNavigationButton({
  action,
  isMobile,
  onClick,
}: {
  action: ConversationThreadNavigationAction;
  isMobile: boolean;
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
      className={cn(
        "flex shrink-0 items-center justify-center rounded-lg text-(--icon-default) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong)",
        isMobile ? "h-8 w-8" : WORKSPACE_PANEL_HEADER_BUTTON_CLASS,
      )}
      onClick={onClick}
      title={presentation.title}
      type="button"
    >
      <Icon className={WORKSPACE_PANEL_HEADER_ICON_CLASS} />
    </button>
  );
}

function ThreadAgentAvatar({
  avatarUrl,
  isMobile,
}: {
  avatarUrl: string | null;
  isMobile: boolean;
}) {
  return (
    <MessageAvatar
      avatarUrl={avatarUrl}
      className={cn(
        "shrink-0 radius-control-md",
        isMobile ? "h-8 w-8" : "h-7 w-7",
      )}
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
      className={cn(
        "soft-scrollbar min-h-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto overscroll-y-none px-4",
        model.presentation === "inspector" ? "pb-3 pt-0" : "py-3",
      )}
      style={{ overflowAnchor: "none", scrollbarGutter: "stable" }}
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
          presentation={model.presentation}
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
  presentation,
  rounds,
}: {
  emptyContent: ReactNode;
  messageContext: ConversationThreadMessageContext;
  presentation: ConversationThreadPresentation;
  rounds: ConversationThreadRoundModel[];
}) {
  if (rounds.length === 0) {
    return emptyContent;
  }
  return rounds.map((round) => (
    <ThreadRound
      key={round.roundId}
      emptyContent={emptyContent}
      messageContext={messageContext}
      presentation={presentation}
      round={round}
    />
  ));
}

function ThreadRound({
  emptyContent,
  messageContext,
  presentation,
  round,
}: {
  emptyContent: ReactNode;
  messageContext: ConversationThreadMessageContext;
  presentation: ConversationThreadPresentation;
  round: ConversationThreadRoundModel;
}) {
  const isInspector = presentation === "inspector";
  return (
    <>
      <MessageItem
        assistantContentMode={
          isInspector ? "room_thread_process" : "room_thread"
        }
        assistantEmptyState={isInspector ? emptyContent : undefined}
        className="max-w-full overflow-x-hidden"
        compact
        currentAgentAvatar={messageContext.agentAvatar}
        currentAgentName={messageContext.agentName}
        defaultProcessExpanded
        isLastRound={round.isLast}
        isLoading={round.isLoading}
        messages={round.messages}
        onOpenWorkspaceFile={messageContext.onOpenWorkspaceFile}
        onPermissionResponse={messageContext.onPermissionResponse}
        onStopMessage={messageContext.onStopMessage}
        pendingPermissions={round.pendingPermissions}
        roundId={round.roundId}
        showAssistantHeader={!isInspector}
        showUserMessages={!isInspector}
        unresolvedToolStatus={messageContext.unresolvedToolStatus}
        workspaceAgentId={messageContext.workspaceAgentId}
      />
      {round.showDivider && !isInspector ? (
        <hr aria-hidden="true" className="conversation-round-divider" />
      ) : null}
    </>
  );
}

function ThreadScrollToLatest({
  onClick,
  show,
}: {
  onClick: () => void;
  show: boolean;
}) {
  return (
    <ScrollToLatestButton
      onClick={onClick}
      visible={show}
    />
  );
}
