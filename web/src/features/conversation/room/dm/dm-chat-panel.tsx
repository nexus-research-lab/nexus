"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { useAgentConversation } from "@/hooks/agent";
import { useProviderAvailability } from "@/hooks/capability/use-provider-availability";
import { useExtractTodos } from "@/hooks/conversation/use-extract-todos";
import { useFollowScroll } from "@/hooks/conversation/use-follow-scroll";
import { useSessionLoader } from "@/hooks/conversation/use-session-loader";
import { useSessionRoundIndex } from "@/hooks/conversation/use-session-round-index";
import { useDefaultChatDeliveryPolicy } from "@/hooks/settings/use-default-chat-delivery-policy";
import { createGoalApi } from "@/lib/api/goal-api";
import { useAuth } from "@/shared/auth/auth-context";
import {
  AgentConversationIdentity,
} from "@/types/agent/agent-conversation";
import { SessionSnapshotPayload } from "@/types/conversation/conversation";
import { TodoItem } from "@/types/conversation/todo";

import { ComposerPanel } from "@/features/conversation/shared/composer/composer-panel";
import {
  prepareWorkspaceAttachments,
} from "@/features/conversation/shared/composer/attachments/composer-attachments";
import { ConversationErrorBubble } from "@/features/conversation/shared/conversation-error-bubble";
import { ConversationFeed } from "@/features/conversation/shared/conversation-feed";
import {
  buildConversationScrollContentKey,
} from "@/features/conversation/shared/conversation-scroll-content-key";
import type {
  ConversationRoundScrollHandle,
} from "@/features/conversation/shared/conversation-round-scroll";
import { ConversationSessionNavigator } from "@/features/conversation/shared/session-navigator/conversation-session-navigator";
import { goalContinuationHoldForPermission } from "@/features/conversation/shared/goal-continuation-hold";
import { GoalPanel } from "@/features/conversation/shared/goal-panel";
import { ProviderUnavailableBanner } from "@/features/conversation/shared/provider-unavailable-banner";
import { ScrollToLatestButton } from "@/features/conversation/shared/scroll-to-latest-button";
import { useConversationTimeline } from "@/features/conversation/shared/use-conversation-timeline";
import { useConversationComposerHandlers } from "@/features/conversation/shared/composer/use-conversation-composer-handlers";
import { useConversationHistoryLoader } from "@/features/conversation/shared/use-conversation-history-loader";
import {
  useConversationSnapshotReporter,
  type ConversationSnapshotBuildInput,
} from "@/features/conversation/shared/use-conversation-snapshot-reporter";
import { useVisibleRoundWindowLoader } from "@/features/conversation/shared/use-visible-round-window-loader";
import { CONVERSATION_TOUR_ANCHORS } from "../room-tour";

export interface DmChatPanelProps {
  currentAgentName?: string | null;
  currentAgentAvatar?: string | null;
  currentAgentPermissionMode?: string | null;
  sessionIdentity: AgentConversationIdentity | null;
  layout?: "desktop" | "mobile";
  initialDraft?: string | null;
  onInitialDraftConsumed?: () => void;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  onTodosChange?: (todos: TodoItem[]) => void;
  onLoadingChange?: (isLoading: boolean) => void;
  onConversationSnapshotChange?: (snapshot: SessionSnapshotPayload) => void;
  onRoomEvent?: (
    eventType: string,
    data: import("@/types/agent/agent-conversation").RoomEventPayload,
  ) => void;
}

export function DmChatPanel({
  currentAgentName: currentAgentName,
  currentAgentAvatar: currentAgentAvatar,
  currentAgentPermissionMode: currentAgentPermissionMode,
  sessionIdentity: sessionIdentity,
  layout = "desktop",
  initialDraft: initialDraft = null,
  onInitialDraftConsumed: onInitialDraftConsumed,
  onOpenAgentContact: onOpenAgentContact,
  onOpenWorkspaceFile: onOpenWorkspaceFile,
  onTodosChange: onTodosChange,
  onLoadingChange: onLoadingChange,
  onConversationSnapshotChange: onConversationSnapshotChange,
  onRoomEvent: onRoomEvent,
}: DmChatPanelProps) {
  const isMobileLayout = layout === "mobile";
  const sessionKey = sessionIdentity?.session_key ?? null;
  const roundScrollRef = useRef<ConversationRoundScrollHandle | null>(null);
  const defaultDeliveryPolicy = useDefaultChatDeliveryPolicy();
  const { status: authStatus } = useAuth();
  const currentUserAvatar = authStatus?.avatar ?? null;
  const [goalRefreshSeq, setGoalRefreshSeq] = useState(0);
  const refreshGoalPanel = useCallback(() => {
    setGoalRefreshSeq((value) => value + 1);
  }, []);
  const goalContinuationHold = useMemo(
    () =>
      goalContinuationHoldForPermission(
        currentAgentName,
        currentAgentPermissionMode,
      ),
    [currentAgentName, currentAgentPermissionMode],
  );
  const canControlSession = true;
  const handleConversationEvent = useCallback(
    (
      eventType: string,
      data: import("@/types/agent/agent-conversation").RoomEventPayload,
    ) => {
      if (eventType.startsWith("goal_")) {
        refreshGoalPanel();
      }
      onRoomEvent?.(eventType, data);
    },
    [onRoomEvent, refreshGoalPanel],
  );

  const {
    error,
    messages,
    is_loading: isLoading,
    is_history_loading: isHistoryLoading,
    has_more_history: hasMoreHistory,
    history_prepend_token: historyPrependToken,
    pending_permissions: pendingPermissions,
    send_message: sendMessage,
    rewrite_last_user_message: rewriteLastUserMessage,
    stop_generation: stopGeneration,
    load_session: loadSession,
    load_older_messages: loadOlderMessages,
    load_round_window: loadRoundWindow,
    send_permission_response: sendPermissionResponse,
    runtime_phase: runtimePhase,
    live_round_ids: liveRoundIds,
    input_queue_items: inputQueueItems,
    enqueue_input_queue_message: enqueueInputQueueMessage,
    delete_input_queue_message: deleteInputQueueMessage,
    guide_input_queue_message: guideInputQueueMessage,
    reorder_input_queue_messages: reorderInputQueueMessages,
  } = useAgentConversation({
    identity: sessionIdentity,
    on_error: (err) => {
      console.error("DM conversation error:", err);
    },
    on_room_event: handleConversationEvent,
  });

  const todos = useExtractTodos(messages, sessionKey);
  const { hasAvailableProvider, isReady: providerReady } = useProviderAvailability();
  const showProviderWarning = providerReady && !hasAvailableProvider;
  const systemError = error;
  const scrollContentKey = useMemo(
    () => buildConversationScrollContentKey(sessionKey, messages),
    [messages, sessionKey],
  );
  const {
    scrollRef: scrollRef,
    feedRef: feedRef,
    bottomAnchorRef: bottomAnchorRef,
    showScrollToBottom: showScrollToBottom,
    scrollToBottom: scrollToBottom,
    pauseFollowLatest: pauseFollowLatest,
    prepareHistoryPrependRestore: prepareHistoryPrependRestore,
    cancelHistoryPrependRestore: cancelHistoryPrependRestore,
    onScroll: onScroll,
    onWheel: onWheel,
    onTouchStart: onTouchStart,
    onTouchMove: onTouchMove,
    onTouchEnd: onTouchEnd,
  } = useFollowScroll({
    messageCount: messages.length,
    auxiliaryBlockCount: pendingPermissions.length,
    auxiliaryBlockKey: systemError,
    contentKey: scrollContentKey,
    isLoading,
    sessionKey,
    historyPrependToken,
  });
  const prepareDmAttachments = useCallback(async (files: File[]) => {
    const targetAgentId = sessionIdentity?.agent_id;
    if (!targetAgentId) {
      throw new Error("当前会话尚未准备好，暂时无法附加文件。");
    }
    return prepareWorkspaceAttachments(targetAgentId, files);
  }, [sessionIdentity?.agent_id]);
  const { handlePrepareAttachments, handleSendMessage } =
    useConversationComposerHandlers({
      initialDraft,
      initialDraftLogLabel: "DM",
      isLoading,
      onInitialDraftConsumed,
      prepareAttachments: prepareDmAttachments,
      scrollToBottom,
      sendMessage,
      sessionKey,
    });

  const buildDmSnapshot = useCallback(
    (input: ConversationSnapshotBuildInput): SessionSnapshotPayload => {
      const {
        scope_key: scopeKey,
        last_message: lastMessage,
        latest_reply_timestamp: latestReplyTimestamp,
        should_report_last_activity: shouldReportLastActivity,
      } = input;

      return {
        session_key: scopeKey,
        agent_id: sessionIdentity?.agent_id ?? null,
        room_id: sessionIdentity?.room_id ?? null,
        conversation_id: sessionIdentity?.conversation_id ?? null,
        room_session_id: sessionIdentity?.room_session_id ?? null,
        ...(shouldReportLastActivity && latestReplyTimestamp !== null
          ? { last_activity_at: latestReplyTimestamp }
          : {}),
        session_id: lastMessage.session_id ?? null,
      };
    },
    [sessionIdentity],
  );

  useEffect(() => {
    onTodosChange?.(todos);
  }, [onTodosChange, todos]);
  useEffect(() => {
    onLoadingChange?.(isLoading);
  }, [isLoading, onLoadingChange]);

  useConversationSnapshotReporter({
    scope_key: sessionKey,
    messages,
    build_snapshot: buildDmSnapshot,
    on_snapshot_change: onConversationSnapshotChange,
  });

  useSessionLoader({
    session_key: sessionKey,
    load_session: loadSession,
    debug_name: "DmChatPanel",
  });

  const roundIndexItems = useSessionRoundIndex(sessionKey);
  const timeline = useConversationTimeline({
    chat_type: "dm",
    messages,
    live_round_ids: liveRoundIds,
    round_index_items: roundIndexItems,
  });
  const messageGroups = timeline.message_groups;
  const feedRoundIds = timeline.feed_round_ids;
  const useIndexedTimeline = roundIndexItems.length > 0;
  const visibleRoundLoaderRevision = `${feedRoundIds.length}:${messages.length}:${liveRoundIds.length}`;
  useVisibleRoundWindowLoader({
    enabled: useIndexedTimeline,
    loadRoundWindow,
    revision: visibleRoundLoaderRevision,
    scopeKey: sessionKey,
    scrollRef,
  });

  const { handleScroll } = useConversationHistoryLoader({
    autoFillViewport: !useIndexedTimeline,
    scrollRef,
    messageCount: messages.length,
    hasMoreHistory,
    isHistoryLoading,
    isLoading,
    loadOlderMessages,
    prepareHistoryPrependRestore,
    cancelHistoryPrependRestore,
    onScroll,
  });

  const handleStop = () => stopGeneration();

  const handleEditLastUserMessage = useCallback(
    (messageId: string, newContent: string) => {
      void rewriteLastUserMessage(messageId, newContent);
    },
    [rewriteLastUserMessage],
  );

  const handleCreateGoal = useCallback(async (objective: string) => {
    if (!sessionKey) {
      throw new Error("当前会话尚未准备好，暂时无法启动 Goal。");
    }
    await createGoalApi({
      session_key: sessionKey,
      objective,
      token_budget: null,
    });
    refreshGoalPanel();
  }, [refreshGoalPanel, sessionKey]);

  return (
    <div className="relative flex h-full min-w-0 flex-1 flex-col overflow-hidden bg-transparent">
      {!isMobileLayout ? (
        <ConversationSessionNavigator
          className="absolute bottom-[156px] left-3 top-7 z-20"
          timeline={timeline}
          onLoadRoundWindow={loadRoundWindow}
          onNavigateStart={pauseFollowLatest}
          roundScrollRef={roundScrollRef}
          scrollRef={scrollRef}
        />
      ) : null}

      <div
        data-tour-anchor={CONVERSATION_TOUR_ANCHORS.feed}
        ref={scrollRef}
        className={
          isMobileLayout
            ? "soft-scrollbar relative z-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto px-1 py-2"
            : "soft-scrollbar relative z-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto px-4 py-5 sm:px-6 sm:py-6 xl:px-8 xl:py-7"
        }
        style={{ overflowAnchor: "none" }}
        onScroll={handleScroll}
        onTouchEnd={onTouchEnd}
        onTouchMove={onTouchMove}
        onTouchStart={onTouchStart}
        onWheel={onWheel}
      >
        {isHistoryLoading ? (
          <div className="mx-auto mb-3 flex w-full max-w-[980px] items-center justify-center text-xs text-muted-foreground">
            正在加载更早消息...
          </div>
        ) : null}
        <ConversationFeed
          bottomAnchorRef={bottomAnchorRef}
          feedRef={feedRef}
          scrollRef={scrollRef}
          currentAgentName={currentAgentName ?? null}
          currentAgentAvatar={currentAgentAvatar ?? null}
          workspaceAgentId={sessionIdentity?.agent_id ?? null}
          currentUserAvatar={currentUserAvatar}
          isLastRoundPendingPermissions={pendingPermissions}
          isLoading={isLoading}
          runtimePhase={runtimePhase}
          liveRoundIds={liveRoundIds}
          isMobileLayout={isMobileLayout}
          messageGroups={messageGroups}
          onOpenAgentContact={onOpenAgentContact}
          onOpenWorkspaceFile={onOpenWorkspaceFile}
          onEditLastUserMessage={handleEditLastUserMessage}
          onPermissionResponse={sendPermissionResponse}
          roundScrollRef={roundScrollRef}
          roundIndexItems={roundIndexItems}
          roundIds={feedRoundIds}
        />
        {systemError ? (
          <div className={isMobileLayout ? "mt-4" : "mx-auto mt-2 w-full max-w-[980px]"}>
            <ConversationErrorBubble
              error={systemError}
              compact={isMobileLayout}
            />
          </div>
        ) : null}
      </div>

      {showScrollToBottom ? (
        <ScrollToLatestButton
          isLoading={isLoading}
          isMobileLayout={isMobileLayout}
          onClick={() => scrollToBottom("smooth")}
        />
      ) : null}

      {showProviderWarning ? (
        <ProviderUnavailableBanner compact={isMobileLayout} />
      ) : null}

      <GoalPanel
        activityKey={`${messages.length}:${isLoading ? "loading" : "idle"}:${goalRefreshSeq}`}
        compact={isMobileLayout}
        continuationHold={goalContinuationHold}
        disabled={!canControlSession}
        isGenerating={isLoading}
        sessionKey={sessionKey}
        scopeLabel="会话 Goal"
      />

      <ComposerPanel
        allowSendWhileLoading
        compact={isMobileLayout}
        defaultDeliveryPolicy={defaultDeliveryPolicy}
        inputQueueItems={inputQueueItems}
        isLoading={isLoading}
        goalScopeLabel="会话 Goal"
        runtimePhase={runtimePhase}
        onDeleteQueuedMessage={deleteInputQueueMessage}
        onEnqueueMessage={enqueueInputQueueMessage}
        onCreateGoal={sessionKey && canControlSession ? handleCreateGoal : undefined}
        onGuideQueuedMessage={guideInputQueueMessage}
        onPrepareAttachments={handlePrepareAttachments}
        onReorderQueueMessages={reorderInputQueueMessages}
        onSendMessage={handleSendMessage}
        onStop={handleStop}
        tourAnchor={CONVERSATION_TOUR_ANCHORS.composer}
      />
    </div>
  );
}
