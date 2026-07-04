"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { UserRound } from "lucide-react";

import { useAgentConversation } from "@/hooks/agent";
import { useProviderAvailability } from "@/hooks/capability/use-provider-availability";
import { useExtractTodos } from "@/hooks/conversation/use-extract-todos";
import { useFollowScroll } from "@/hooks/conversation/use-follow-scroll";
import { useSessionLoader } from "@/hooks/conversation/use-session-loader";
import { useSessionRoundIndex } from "@/hooks/conversation/use-session-round-index";
import { useDefaultChatDeliveryPolicy } from "@/hooks/settings/use-default-chat-delivery-policy";
import { createGoalApi } from "@/lib/api/goal-api";
import { buildRoomSharedSessionKey } from "@/lib/conversation/session-key";
import { useAuth } from "@/shared/auth/auth-context";
import { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import { RoomConversationSnapshotPayload } from "@/types/conversation/conversation";
import { TodoItem } from "@/types/conversation/todo";
import { Agent } from "@/types/agent/agent";
import type { LoopCatalogItem } from "@/types/capability/loop";

import { ScrollToLatestButton } from "@/features/conversation/shared/scroll-to-latest-button";
import { ComposerPanel } from "@/features/conversation/shared/composer-panel";
import { prepareRoomConversationAttachments } from "@/features/conversation/shared/composer-attachments";
import { ConversationErrorBubble } from "@/features/conversation/shared/conversation-error-bubble";
import type {
  ConversationRoundScrollHandle,
} from "@/features/conversation/shared/conversation-round-scroll";
import { ConversationSessionNavigator } from "@/features/conversation/shared/conversation-session-navigator";
import { ProviderUnavailableBanner } from "@/features/conversation/shared/provider-unavailable-banner";
import { ROOM_GOAL_SCOPE_LABEL } from "@/features/conversation/shared/goal-continuation-hold";
import {
  buildIndexedTimelineRoundIds,
  buildTimelineRoundIds,
} from "@/features/conversation/shared/timeline-rounds";
import { useConversationComposerHandlers } from "@/features/conversation/shared/use-conversation-composer-handlers";
import { useConversationHistoryLoader } from "@/features/conversation/shared/use-conversation-history-loader";
import {
  useConversationSnapshotReporter,
  type ConversationSnapshotBuildInput,
} from "@/features/conversation/shared/use-conversation-snapshot-reporter";
import {
  groupRoomPendingPermissionsByRound,
  groupRoomPendingSlotsByRound,
  groupRoomMessagesByRound,
} from "@/features/conversation/shared/utils";
import { useVisibleRoundWindowLoader } from "@/features/conversation/shared/use-visible-round-window-loader";
import { GroupConversationFeed } from "./group-conversation-feed";
import { useRoomThreadSource } from "./use-room-thread-panel-data";
import { GroupConversationEmptyState } from "./group-conversation-empty-state";
import { RoomGoalPanel } from "./room-goal-panel";
import {
  buildRoomGoalMetadata,
  buildRoomLoopGoalMetadata,
  buildRoomLoopGoalObjective,
  resolveDefaultRoomGoalLead,
} from "./room-goal-model";
import { CONVERSATION_TOUR_ANCHORS } from "../../room-tour";

export interface GroupChatPanelProps {
  agentId: string | null;
  currentAgentName?: string | null;
  currentAgentAvatar?: string | null;
  /** Room conversation id — used to derive the shared sessionKey */
  conversationId: string | null;
  roomId?: string | null;
  roomMembers: Agent[];
  roomHostAgentId?: string | null;
  roomHostAutoReplyEnabled?: boolean;
  layout?: "desktop" | "mobile";
  initialDraft?: string | null;
  onInitialDraftConsumed?: () => void;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  onTodosChange?: (todos: TodoItem[]) => void;
  onLoadingChange?: (isLoading: boolean) => void;
  onConversationSnapshotChange?: (
    snapshot: RoomConversationSnapshotPayload,
  ) => void;
  onCreateConversation?: (title?: string) => void | Promise<string | null>;
  onRoomEvent?: (
    eventType: string,
    data: import("@/types/agent/agent-conversation").RoomEventPayload,
  ) => void;
}

/**
 * GroupChatPanel — 必须在 GroupThreadContextProvider 内部使用。
 * Provider 由 RoomSurfaceLayout / RoomMobileSurface 提供。
 */
export function GroupChatPanel({
  agentId: agentId,
  currentAgentName: currentAgentName,
  currentAgentAvatar: currentAgentAvatar,
  conversationId: conversationId,
  roomId: roomId = null,
  roomMembers: roomMembers,
  roomHostAgentId: roomHostAgentId = null,
  roomHostAutoReplyEnabled: roomHostAutoReplyEnabled = false,
  layout = "desktop",
  initialDraft: initialDraft = null,
  onInitialDraftConsumed: onInitialDraftConsumed,
  onOpenAgentContact: onOpenAgentContact,
  onOpenWorkspaceFile: onOpenWorkspaceFile,
  onTodosChange: onTodosChange,
  onLoadingChange: onLoadingChange,
  onConversationSnapshotChange: onConversationSnapshotChange,
  onCreateConversation: onCreateConversation,
  onRoomEvent: onRoomEvent,
}: GroupChatPanelProps) {
  const isMobileLayout = layout === "mobile";
  const { status: authStatus } = useAuth();
  const currentUserAvatar = authStatus?.avatar ?? null;

  const sessionKey = conversationId
    ? buildRoomSharedSessionKey(conversationId)
    : null;
  const roundScrollRef = useRef<ConversationRoundScrollHandle | null>(null);
  const defaultDeliveryPolicy = useDefaultChatDeliveryPolicy();
  const [goalRefreshSeq, setGoalRefreshSeq] = useState(0);
  const refreshGoalPanel = useCallback(() => {
    setGoalRefreshSeq((value) => value + 1);
  }, []);
  const defaultRoomGoalLeadAgentId = useMemo(
    () => resolveDefaultRoomGoalLead(roomMembers, roomHostAgentId),
    [roomHostAgentId, roomMembers],
  );
  const [roomGoalLeadAgentId, setRoomGoalLeadAgentId] = useState(
    defaultRoomGoalLeadAgentId,
  );
  useEffect(() => {
    setRoomGoalLeadAgentId((current) => {
      if (current && roomMembers.some((agent) => agent.agent_id === current)) {
        return current;
      }
      return defaultRoomGoalLeadAgentId;
    });
  }, [defaultRoomGoalLeadAgentId, roomMembers]);
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
  const sessionIdentity = useMemo<AgentConversationIdentity | null>(() => {
    if (!conversationId) {
      return null;
    }

    return {
      session_key: sessionKey,
      agent_id: agentId,
      room_id: roomId,
      conversation_id: conversationId,
      chat_type: "group",
    };
  }, [agentId, conversationId, roomId, sessionKey]);

  const agentNameMap = useMemo(() => {
    if (roomMembers.length === 0) return undefined;
    const map: Record<string, string> = {};
    for (const member of roomMembers) {
      map[member.agent_id] = member.name;
    }
    return map;
  }, [roomMembers]);

  const agentAvatarMap = useMemo(() => {
    if (roomMembers.length === 0) return undefined;
    const map: Record<string, string | null> = {};
    for (const member of roomMembers) {
      map[member.agent_id] = member.avatar ?? null;
    }
    return map;
  }, [roomMembers]);

  const {
    error,
    messages,
    is_loading: isLoading,
    is_history_loading: isHistoryLoading,
    has_more_history: hasMoreHistory,
    history_prepend_token: historyPrependToken,
    pending_agent_slots: pendingAgentSlots,
    pending_permissions: pendingPermissions,
    send_message: sendMessage,
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
      console.error("Room conversation error:", err);
    },
    on_room_event: handleConversationEvent,
  });

  const todos = useExtractTodos(messages, sessionKey);
  const { hasAvailableProvider, isReady: providerReady } = useProviderAvailability();
  const showProviderWarning = providerReady && !hasAvailableProvider;
  const systemError = error;
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
    auxiliaryBlockCount:
      pendingAgentSlots.length + pendingPermissions.length,
    auxiliaryBlockKey: systemError,
    isLoading,
    sessionKey,
    historyPrependToken,
  });
  const canControlSession = true;
  const observerReadOnlyReason = "";

  const buildRoomSnapshot = useCallback(
    (input: ConversationSnapshotBuildInput): RoomConversationSnapshotPayload => {
      const {
        scope_key: scopeKey,
        last_message: lastMessage,
        latest_reply_timestamp: latestReplyTimestamp,
        should_report_last_activity: shouldReportLastActivity,
      } = input;

      return {
        conversation_id: scopeKey,
        ...(shouldReportLastActivity && latestReplyTimestamp !== null
          ? { last_activity_at: latestReplyTimestamp }
          : {}),
        session_id: lastMessage.session_id ?? null,
      };
    },
    [],
  );

  useEffect(() => {
    onTodosChange?.(todos);
  }, [onTodosChange, todos]);
  useEffect(() => {
    onLoadingChange?.(isLoading);
  }, [isLoading, onLoadingChange]);

  useConversationSnapshotReporter({
    scope_key: conversationId,
    messages,
    build_snapshot: buildRoomSnapshot,
    on_snapshot_change: onConversationSnapshotChange,
  });

  useSessionLoader({
    session_key: sessionKey,
    load_session: loadSession,
    debug_name: "GroupChatPanel",
  });

  const messageGroups = useMemo(
    () => groupRoomMessagesByRound(messages),
    [messages],
  );
  const pendingSlotGroups = useMemo(
    () => groupRoomPendingSlotsByRound(pendingAgentSlots),
    [pendingAgentSlots],
  );
  const pendingPermissionGroups = useMemo(
    () => groupRoomPendingPermissionsByRound(pendingPermissions),
    [pendingPermissions],
  );
  const loadedRoundIds = useMemo(
    () =>
      buildTimelineRoundIds(messageGroups, liveRoundIds, [
        ...pendingSlotGroups.keys(),
        ...pendingPermissionGroups.keys(),
      ]),
    [
      liveRoundIds,
      messageGroups,
      pendingPermissionGroups,
      pendingSlotGroups,
    ],
  );
  const roundIndexItems = useSessionRoundIndex(sessionKey);
  const feedRoundIds = useMemo(
    () => buildIndexedTimelineRoundIds(roundIndexItems, loadedRoundIds),
    [loadedRoundIds, roundIndexItems],
  );
  const useIndexedTimeline = roundIndexItems.length > 0;
  const visibleRoundLoaderRevision = `${feedRoundIds.length}:${messages.length}:${pendingAgentSlots.length}:${pendingPermissions.length}:${liveRoundIds.length}`;
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

  const handleStopMessage = useCallback(
    (msgId: string) => stopGeneration(msgId),
    [stopGeneration],
  );
  const prepareRoomAttachments = useCallback(async (files: File[]) => {
    if (!roomId || !conversationId) {
      throw new Error("当前 Room 会话尚未就绪，暂时无法附加文件。");
    }
    return prepareRoomConversationAttachments(roomId, conversationId, files);
  }, [conversationId, roomId]);
  const { handlePrepareAttachments, handleSendMessage } =
    useConversationComposerHandlers({
      canSendInitialDraft: canControlSession,
      initialDraft,
      initialDraftLogLabel: "room",
      isLoading,
      onInitialDraftConsumed,
      prepareAttachments: prepareRoomAttachments,
      scrollToBottom,
      sendMessage,
      sessionKey,
    });
  const roomGoalCreateDisabledReason =
    roomMembers.length === 0
      ? "房间还没有可指派的 Agent"
      : roomGoalLeadAgentId.trim() === ""
        ? "请选择 Room Goal 负责人"
        : null;
  const roomGoalLeadControl = (
    <label
      className="pointer-events-auto inline-flex h-5 min-w-0 max-w-[190px] items-center gap-1 rounded-[7px] border border-(--surface-canvas-border) bg-(--surface-elevated-background) px-1.5 text-[10px] font-medium text-(--text-muted)"
      title="选择 Room Goal 负责人"
    >
      <UserRound className="h-3 w-3 shrink-0" />
      <select
        className="min-w-0 flex-1 bg-transparent text-[10px] font-semibold text-(--text-default) outline-none disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)"
        disabled={!canControlSession || isLoading || roomMembers.length === 0}
        value={roomGoalLeadAgentId}
        onChange={(event) => setRoomGoalLeadAgentId(event.target.value)}
      >
        <option value="">负责人</option>
        {roomMembers.map((agent) => (
          <option key={agent.agent_id} value={agent.agent_id}>
            {agent.name}
          </option>
        ))}
      </select>
    </label>
  );
  const handleCreateGoal = useCallback(async (objective: string) => {
    if (!sessionKey) {
      throw new Error("当前房间会话尚未准备好，暂时无法启动 Goal。");
    }
    const leadAgentId = roomGoalLeadAgentId.trim();
    if (!leadAgentId) {
      throw new Error("请选择 Room Goal 负责人。");
    }
    await createGoalApi({
      session_key: sessionKey,
      objective,
      token_budget: null,
      metadata: buildRoomGoalMetadata(roomMembers, leadAgentId),
    });
    refreshGoalPanel();
  }, [
    refreshGoalPanel,
    roomGoalLeadAgentId,
    roomMembers,
    sessionKey,
  ]);
  const handleCreateLoopGoal = useCallback(async (loop: LoopCatalogItem) => {
    if (!sessionKey) {
      throw new Error("当前房间会话尚未准备好，暂时无法启动 Loop。");
    }
    const leadAgentId = roomGoalLeadAgentId.trim();
    if (!leadAgentId) {
      throw new Error("请选择 Room Goal 负责人。");
    }
    await createGoalApi({
      session_key: sessionKey,
      objective: buildRoomLoopGoalObjective(loop),
      token_budget: null,
      metadata: buildRoomLoopGoalMetadata(roomMembers, leadAgentId, loop),
    });
    refreshGoalPanel();
  }, [
    refreshGoalPanel,
    roomGoalLeadAgentId,
    roomMembers,
    sessionKey,
  ]);
  useRoomThreadSource({
    agentAvatarMap,
    agentNameMap,
    canControlSession,
    conversationId,
    currentUserAvatar,
    messageGroups,
    observerReadOnlyReason,
    onOpenWorkspaceFile,
    onStopMessage: handleStopMessage,
    pendingPermissionGroups,
    pendingSlotGroups,
    sendPermissionResponse,
  });
  return (
    <div className="relative flex h-full min-w-0 flex-1 flex-col overflow-hidden bg-transparent">
      {!isMobileLayout && sessionKey ? (
        <ConversationSessionNavigator
          className="absolute bottom-[156px] left-3 top-7 z-20"
          liveRoundIds={liveRoundIds}
          messageGroups={messageGroups}
          onLoadRoundWindow={loadRoundWindow}
          onNavigateStart={pauseFollowLatest}
          roundScrollRef={roundScrollRef}
          roundIndexItems={roundIndexItems}
          scrollRef={scrollRef}
        />
      ) : null}

      {!sessionKey ? (
        <GroupConversationEmptyState
          onCreateConversation={onCreateConversation ?? (() => {})}
        />
      ) : (
        <>
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
            <GroupConversationFeed
              agentNameMap={agentNameMap}
              agentAvatarMap={agentAvatarMap}
              bottomAnchorRef={bottomAnchorRef}
              feedRef={feedRef}
              scrollRef={scrollRef}
              currentAgentName={currentAgentName ?? null}
              currentAgentAvatar={currentAgentAvatar ?? null}
              currentUserAvatar={currentUserAvatar}
              isLastRoundPendingPermissions={pendingPermissions}
              isLoading={isLoading}
              runtimePhase={runtimePhase}
              liveRoundIds={liveRoundIds}
              isMobileLayout={isMobileLayout}
              messageGroups={messageGroups}
              pendingPermissionGroups={pendingPermissionGroups}
              pendingSlotGroups={pendingSlotGroups}
              onOpenAgentContact={onOpenAgentContact}
              onOpenWorkspaceFile={onOpenWorkspaceFile}
              onPermissionResponse={sendPermissionResponse}
              canRespondToPermissions={canControlSession}
              permissionReadOnlyReason={observerReadOnlyReason}
              onStopMessage={
                canControlSession ? handleStopMessage : undefined
              }
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

          <RoomGoalPanel
            activityKey={`${messages.length}:${isLoading ? "loading" : "idle"}:${goalRefreshSeq}`}
            canControlSession={canControlSession}
            isLoading={isLoading}
            isMobileLayout={isMobileLayout}
            roomHostAgentId={roomHostAgentId}
            roomHostAutoReplyEnabled={Boolean(roomHostAutoReplyEnabled)}
            roomMembers={roomMembers}
            sessionKey={sessionKey}
          />

          <ComposerPanel
            allowSendWhileLoading
            compact={isMobileLayout}
            defaultDeliveryPolicy={defaultDeliveryPolicy}
            enableLoops
            goalCreateDisabledReason={roomGoalCreateDisabledReason}
            goalModeExtra={roomGoalLeadControl}
            goalScopeLabel={ROOM_GOAL_SCOPE_LABEL}
            inputQueueItems={inputQueueItems}
            isLoading={isLoading}
            queueWhenSessionBusy={false}
            runtimePhase={runtimePhase}
            onCreateLoopGoal={sessionKey && canControlSession ? handleCreateLoopGoal : undefined}
            onCreateGoal={sessionKey && canControlSession ? handleCreateGoal : undefined}
            onDeleteQueuedMessage={deleteInputQueueMessage}
            onEnqueueMessage={enqueueInputQueueMessage}
            onGuideQueuedMessage={guideInputQueueMessage}
            onPrepareAttachments={handlePrepareAttachments}
            onReorderQueueMessages={reorderInputQueueMessages}
            onSendMessage={handleSendMessage}
            onStop={canControlSession ? () => stopGeneration() : undefined}
            roomMembers={roomMembers}
            tourAnchor={CONVERSATION_TOUR_ANCHORS.composer}
            disabled={!canControlSession}
          />
        </>
      )}
    </div>
  );
}
