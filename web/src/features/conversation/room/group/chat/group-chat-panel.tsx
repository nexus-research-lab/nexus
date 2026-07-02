"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { UserRound } from "lucide-react";

import { useAgentConversation } from "@/hooks/agent";
import { useProviderAvailability } from "@/hooks/capability/use-provider-availability";
import { useExtractTodos } from "@/hooks/conversation/use-extract-todos";
import { useFollowScroll } from "@/hooks/conversation/use-follow-scroll";
import { useSessionLoader } from "@/hooks/conversation/use-session-loader";
import { useDefaultChatDeliveryPolicy } from "@/hooks/settings/use-default-chat-delivery-policy";
import { create_goal_api } from "@/lib/api/goal-api";
import { build_room_shared_session_key } from "@/lib/conversation/session-key";
import { useAuth } from "@/shared/auth/auth-context";
import { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import { RoomConversationSnapshotPayload } from "@/types/conversation/conversation";
import { TodoItem } from "@/types/conversation/todo";
import { Agent } from "@/types/agent/agent";
import type { LoopCatalogItem } from "@/types/capability/loop";

import { ScrollToLatestButton } from "@/features/conversation/shared/scroll-to-latest-button";
import { ComposerPanel } from "@/features/conversation/shared/composer-panel";
import { prepare_room_conversation_attachments } from "@/features/conversation/shared/composer-attachments";
import { ConversationErrorBubble } from "@/features/conversation/shared/conversation-error-bubble";
import { is_provider_error } from "@/features/conversation/shared/conversation-error-utils";
import { ProviderUnavailableBanner } from "@/features/conversation/shared/provider-unavailable-banner";
import { ROOM_GOAL_SCOPE_LABEL } from "@/features/conversation/shared/goal-continuation-hold";
import { build_timeline_round_ids } from "@/features/conversation/shared/timeline-rounds";
import { useConversationComposerHandlers } from "@/features/conversation/shared/use-conversation-composer-handlers";
import { useConversationHistoryLoader } from "@/features/conversation/shared/use-conversation-history-loader";
import {
  useConversationSnapshotReporter,
  type ConversationSnapshotBuildInput,
} from "@/features/conversation/shared/use-conversation-snapshot-reporter";
import {
  group_room_pending_permissions_by_round,
  group_room_pending_slots_by_round,
  group_room_messages_by_round,
} from "@/features/conversation/shared/utils";
import { GroupConversationFeed } from "./group-conversation-feed";
import { useRoomThreadSource } from "./use-room-thread-panel-data";
import { GroupConversationEmptyState } from "./group-conversation-empty-state";
import { RoomGoalPanel } from "./room-goal-panel";
import {
  build_room_goal_metadata,
  build_room_loop_goal_metadata,
  build_room_loop_goal_objective,
  resolve_default_room_goal_lead,
} from "./room-goal-model";
import { CONVERSATION_TOUR_ANCHORS } from "../../room-tour";

export interface GroupChatPanelProps {
  agent_id: string | null;
  current_agent_name?: string | null;
  current_agent_avatar?: string | null;
  /** Room conversation id — used to derive the shared session_key */
  conversation_id: string | null;
  room_id?: string | null;
  room_members: Agent[];
  room_host_agent_id?: string | null;
  room_host_auto_reply_enabled?: boolean;
  layout?: "desktop" | "mobile";
  initial_draft?: string | null;
  on_initial_draft_consumed?: () => void;
  on_open_agent_contact?: (agentId: string) => void;
  on_open_workspace_file?: (path: string) => void;
  on_todos_change?: (todos: TodoItem[]) => void;
  on_loading_change?: (isLoading: boolean) => void;
  on_conversation_snapshot_change?: (
    snapshot: RoomConversationSnapshotPayload,
  ) => void;
  on_create_conversation?: (title?: string) => void | Promise<string | null>;
  on_room_event?: (
    eventType: string,
    data: import("@/types/agent/agent-conversation").RoomEventPayload,
  ) => void;
}

/**
 * GroupChatPanel — 必须在 GroupThreadContextProvider 内部使用。
 * Provider 由 RoomSurfaceLayout / RoomMobileSurface 提供。
 */
export function GroupChatPanel({
  agent_id: agentId,
  current_agent_name: currentAgentName,
  current_agent_avatar: currentAgentAvatar,
  conversation_id: conversationId,
  room_id: roomId = null,
  room_members: roomMembers,
  room_host_agent_id: roomHostAgentId = null,
  room_host_auto_reply_enabled: roomHostAutoReplyEnabled = false,
  layout = "desktop",
  initial_draft: initialDraft = null,
  on_initial_draft_consumed: onInitialDraftConsumed,
  on_open_agent_contact: onOpenAgentContact,
  on_open_workspace_file: onOpenWorkspaceFile,
  on_todos_change: onTodosChange,
  on_loading_change: onLoadingChange,
  on_conversation_snapshot_change: onConversationSnapshotChange,
  on_create_conversation: onCreateConversation,
  on_room_event: onRoomEvent,
}: GroupChatPanelProps) {
  const isMobileLayout = layout === "mobile";
  const { status: authStatus } = useAuth();
  const currentUserAvatar = authStatus?.avatar ?? null;

  const sessionKey = conversationId
    ? build_room_shared_session_key(conversationId)
    : null;
  const defaultDeliveryPolicy = useDefaultChatDeliveryPolicy();
  const [goalRefreshSeq, setGoalRefreshSeq] = useState(0);
  const refreshGoalPanel = useCallback(() => {
    setGoalRefreshSeq((value) => value + 1);
  }, []);
  const defaultRoomGoalLeadAgentId = useMemo(
    () => resolve_default_room_goal_lead(roomMembers, roomHostAgentId),
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
  const { has_available_provider: hasAvailableProvider, is_ready: providerReady } = useProviderAvailability();
  const showProviderWarning = providerReady && !hasAvailableProvider;
  const systemError = error && !is_provider_error(error) ? error : null;
  const {
    scroll_ref: scrollRef,
    feed_ref: feedRef,
    bottom_anchor_ref: bottomAnchorRef,
    show_scroll_to_bottom: showScrollToBottom,
    scroll_to_bottom: scrollToBottom,
    prepare_history_prepend_restore: prepareHistoryPrependRestore,
    cancel_history_prepend_restore: cancelHistoryPrependRestore,
    on_scroll: onScroll,
    on_wheel: onWheel,
    on_touch_start: onTouchStart,
    on_touch_move: onTouchMove,
    on_touch_end: onTouchEnd,
  } = useFollowScroll({
    message_count: messages.length,
    auxiliary_block_count:
      pendingAgentSlots.length + pendingPermissions.length,
    auxiliary_block_key: systemError,
    is_loading: isLoading,
    session_key: sessionKey,
    history_prepend_token: historyPrependToken,
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
    () => group_room_messages_by_round(messages),
    [messages],
  );
  const pendingSlotGroups = useMemo(
    () => group_room_pending_slots_by_round(pendingAgentSlots),
    [pendingAgentSlots],
  );
  const pendingPermissionGroups = useMemo(
    () => group_room_pending_permissions_by_round(pendingPermissions),
    [pendingPermissions],
  );
  const roundIds = useMemo(
    () =>
      build_timeline_round_ids(messageGroups, liveRoundIds, [
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
  const { handle_scroll: handleScroll } = useConversationHistoryLoader({
    scroll_ref: scrollRef,
    message_count: messages.length,
    has_more_history: hasMoreHistory,
    is_history_loading: isHistoryLoading,
    is_loading: isLoading,
    load_older_messages: loadOlderMessages,
    prepare_history_prepend_restore: prepareHistoryPrependRestore,
    cancel_history_prepend_restore: cancelHistoryPrependRestore,
    on_scroll: onScroll,
  });

  const handleStopMessage = useCallback(
    (msgId: string) => stopGeneration(msgId),
    [stopGeneration],
  );
  const prepareRoomAttachments = useCallback(async (files: File[]) => {
    if (!roomId || !conversationId) {
      throw new Error("当前 Room 会话尚未就绪，暂时无法附加文件。");
    }
    return prepare_room_conversation_attachments(roomId, conversationId, files);
  }, [conversationId, roomId]);
  const { handle_prepare_attachments: handlePrepareAttachments, handle_send_message: handleSendMessage } =
    useConversationComposerHandlers({
      can_send_initial_draft: canControlSession,
      initial_draft: initialDraft,
      initial_draft_log_label: "room",
      is_loading: isLoading,
      on_initial_draft_consumed: onInitialDraftConsumed,
      prepare_attachments: prepareRoomAttachments,
      scroll_to_bottom: scrollToBottom,
      send_message: sendMessage,
      session_key: sessionKey,
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
    await create_goal_api({
      session_key: sessionKey,
      objective,
      token_budget: null,
      metadata: build_room_goal_metadata(roomMembers, leadAgentId),
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
    await create_goal_api({
      session_key: sessionKey,
      objective: build_room_loop_goal_objective(loop),
      token_budget: null,
      metadata: build_room_loop_goal_metadata(roomMembers, leadAgentId, loop),
    });
    refreshGoalPanel();
  }, [
    refreshGoalPanel,
    roomGoalLeadAgentId,
    roomMembers,
    sessionKey,
  ]);
  useRoomThreadSource({
    agent_avatar_map: agentAvatarMap,
    agent_name_map: agentNameMap,
    can_control_session: canControlSession,
    conversation_id: conversationId,
    current_user_avatar: currentUserAvatar,
    message_groups: messageGroups,
    observer_read_only_reason: observerReadOnlyReason,
    on_open_workspace_file: onOpenWorkspaceFile,
    on_stop_message: handleStopMessage,
    pending_permission_groups: pendingPermissionGroups,
    pending_slot_groups: pendingSlotGroups,
    send_permission_response: sendPermissionResponse,
  });
  return (
    <div className="relative flex h-full min-w-0 flex-1 flex-col overflow-hidden bg-transparent">

      {!sessionKey ? (
        <GroupConversationEmptyState
          on_create_conversation={onCreateConversation ?? (() => {})}
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
              agent_name_map={agentNameMap}
              agent_avatar_map={agentAvatarMap}
              bottom_anchor_ref={bottomAnchorRef}
              feed_ref={feedRef}
              scroll_ref={scrollRef}
              current_agent_name={currentAgentName ?? null}
              current_agent_avatar={currentAgentAvatar ?? null}
              current_user_avatar={currentUserAvatar}
              is_last_round_pending_permissions={pendingPermissions}
              is_loading={isLoading}
              runtime_phase={runtimePhase}
              live_round_ids={liveRoundIds}
              is_mobile_layout={isMobileLayout}
              message_groups={messageGroups}
              pending_permission_groups={pendingPermissionGroups}
              pending_slot_groups={pendingSlotGroups}
              on_open_agent_contact={onOpenAgentContact}
              on_open_workspace_file={onOpenWorkspaceFile}
              on_permission_response={sendPermissionResponse}
              can_respond_to_permissions={canControlSession}
              permission_read_only_reason={observerReadOnlyReason}
              on_stop_message={
                canControlSession ? handleStopMessage : undefined
              }
              round_ids={roundIds}
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
              is_loading={isLoading}
              is_mobile_layout={isMobileLayout}
              on_click={() => scrollToBottom("smooth")}
            />
          ) : null}

          {showProviderWarning ? (
            <ProviderUnavailableBanner compact={isMobileLayout} />
          ) : null}

          <RoomGoalPanel
            activity_key={`${messages.length}:${isLoading ? "loading" : "idle"}:${goalRefreshSeq}`}
            can_control_session={canControlSession}
            is_loading={isLoading}
            is_mobile_layout={isMobileLayout}
            room_host_agent_id={roomHostAgentId}
            room_host_auto_reply_enabled={Boolean(roomHostAutoReplyEnabled)}
            room_members={roomMembers}
            session_key={sessionKey}
          />

          <ComposerPanel
            allow_send_while_loading
            compact={isMobileLayout}
            default_delivery_policy={defaultDeliveryPolicy}
            enable_loops
            goal_create_disabled_reason={roomGoalCreateDisabledReason}
            goal_mode_extra={roomGoalLeadControl}
            goal_scope_label={ROOM_GOAL_SCOPE_LABEL}
            input_queue_items={inputQueueItems}
            is_loading={isLoading}
            queue_when_session_busy={false}
            runtime_phase={runtimePhase}
            on_create_loop_goal={sessionKey && canControlSession ? handleCreateLoopGoal : undefined}
            on_create_goal={sessionKey && canControlSession ? handleCreateGoal : undefined}
            on_delete_queued_message={deleteInputQueueMessage}
            on_enqueue_message={enqueueInputQueueMessage}
            on_guide_queued_message={guideInputQueueMessage}
            on_prepare_attachments={handlePrepareAttachments}
            on_reorder_queue_messages={reorderInputQueueMessages}
            on_send_message={handleSendMessage}
            on_stop={canControlSession ? () => stopGeneration() : undefined}
            room_members={roomMembers}
            tour_anchor={CONVERSATION_TOUR_ANCHORS.composer}
            disabled={!canControlSession}
          />
        </>
      )}
    </div>
  );
}
