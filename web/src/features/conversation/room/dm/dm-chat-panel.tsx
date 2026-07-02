"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { useAgentConversation } from "@/hooks/agent";
import { useProviderAvailability } from "@/hooks/capability/use-provider-availability";
import { useExtractTodos } from "@/hooks/conversation/use-extract-todos";
import { useFollowScroll } from "@/hooks/conversation/use-follow-scroll";
import { useSessionLoader } from "@/hooks/conversation/use-session-loader";
import { useDefaultChatDeliveryPolicy } from "@/hooks/settings/use-default-chat-delivery-policy";
import { create_goal_api } from "@/lib/api/goal-api";
import { useAuth } from "@/shared/auth/auth-context";
import {
  AgentConversationIdentity,
} from "@/types/agent/agent-conversation";
import { SessionSnapshotPayload } from "@/types/conversation/conversation";
import { TodoItem } from "@/types/conversation/todo";

import { ComposerPanel } from "@/features/conversation/shared/composer-panel";
import {
  prepare_workspace_attachments,
} from "@/features/conversation/shared/composer-attachments";
import { ConversationErrorBubble } from "@/features/conversation/shared/conversation-error-bubble";
import { is_provider_error } from "@/features/conversation/shared/conversation-error-utils";
import { ConversationFeed } from "@/features/conversation/shared/conversation-feed";
import { goal_continuation_hold_for_permission } from "@/features/conversation/shared/goal-continuation-hold";
import { GoalPanel } from "@/features/conversation/shared/goal-panel";
import { ProviderUnavailableBanner } from "@/features/conversation/shared/provider-unavailable-banner";
import { ScrollToLatestButton } from "@/features/conversation/shared/scroll-to-latest-button";
import { build_timeline_round_ids } from "@/features/conversation/shared/timeline-rounds";
import { useConversationComposerHandlers } from "@/features/conversation/shared/use-conversation-composer-handlers";
import { useConversationHistoryLoader } from "@/features/conversation/shared/use-conversation-history-loader";
import {
  useConversationSnapshotReporter,
  type ConversationSnapshotBuildInput,
} from "@/features/conversation/shared/use-conversation-snapshot-reporter";
import {
  group_messages_by_round,
} from "@/features/conversation/shared/utils";
import { CONVERSATION_TOUR_ANCHORS } from "../room-tour";

export interface DmChatPanelProps {
  current_agent_name?: string | null;
  current_agent_avatar?: string | null;
  current_agent_permission_mode?: string | null;
  session_identity: AgentConversationIdentity | null;
  layout?: "desktop" | "mobile";
  initial_draft?: string | null;
  on_initial_draft_consumed?: () => void;
  on_open_agent_contact?: (agentId: string) => void;
  on_open_workspace_file?: (path: string) => void;
  on_todos_change?: (todos: TodoItem[]) => void;
  on_loading_change?: (isLoading: boolean) => void;
  on_conversation_snapshot_change?: (snapshot: SessionSnapshotPayload) => void;
  on_room_event?: (
    eventType: string,
    data: import("@/types/agent/agent-conversation").RoomEventPayload,
  ) => void;
}

export function DmChatPanel({
  current_agent_name: currentAgentName,
  current_agent_avatar: currentAgentAvatar,
  current_agent_permission_mode: currentAgentPermissionMode,
  session_identity: sessionIdentity,
  layout = "desktop",
  initial_draft: initialDraft = null,
  on_initial_draft_consumed: onInitialDraftConsumed,
  on_open_agent_contact: onOpenAgentContact,
  on_open_workspace_file: onOpenWorkspaceFile,
  on_todos_change: onTodosChange,
  on_loading_change: onLoadingChange,
  on_conversation_snapshot_change: onConversationSnapshotChange,
  on_room_event: onRoomEvent,
}: DmChatPanelProps) {
  const isMobileLayout = layout === "mobile";
  const sessionKey = sessionIdentity?.session_key ?? null;
  const defaultDeliveryPolicy = useDefaultChatDeliveryPolicy();
  const { status: authStatus } = useAuth();
  const currentUserAvatar = authStatus?.avatar ?? null;
  const [goalRefreshSeq, setGoalRefreshSeq] = useState(0);
  const refreshGoalPanel = useCallback(() => {
    setGoalRefreshSeq((value) => value + 1);
  }, []);
  const goalContinuationHold = useMemo(
    () =>
      goal_continuation_hold_for_permission(
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
      console.error("DM conversation error:", err);
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
    auxiliary_block_count: pendingPermissions.length,
    auxiliary_block_key: systemError,
    is_loading: isLoading,
    session_key: sessionKey,
    history_prepend_token: historyPrependToken,
  });
  const prepareDmAttachments = useCallback(async (files: File[]) => {
    const targetAgentId = sessionIdentity?.agent_id;
    if (!targetAgentId) {
      throw new Error("当前会话尚未准备好，暂时无法附加文件。");
    }
    return prepare_workspace_attachments(targetAgentId, files);
  }, [sessionIdentity?.agent_id]);
  const { handle_prepare_attachments: handlePrepareAttachments, handle_send_message: handleSendMessage } =
    useConversationComposerHandlers({
      initial_draft: initialDraft,
      initial_draft_log_label: "DM",
      is_loading: isLoading,
      on_initial_draft_consumed: onInitialDraftConsumed,
      prepare_attachments: prepareDmAttachments,
      scroll_to_bottom: scrollToBottom,
      send_message: sendMessage,
      session_key: sessionKey,
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

  const messageGroups = useMemo(
    () => group_messages_by_round(messages),
    [messages],
  );
  const roundIds = useMemo(
    () => build_timeline_round_ids(messageGroups, liveRoundIds),
    [liveRoundIds, messageGroups],
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

  const handleStop = () => stopGeneration();

  const handleCreateGoal = useCallback(async (objective: string) => {
    if (!sessionKey) {
      throw new Error("当前会话尚未准备好，暂时无法启动 Goal。");
    }
    await create_goal_api({
      session_key: sessionKey,
      objective,
      token_budget: null,
    });
    refreshGoalPanel();
  }, [refreshGoalPanel, sessionKey]);

  return (
    <div className="relative flex h-full min-w-0 flex-1 flex-col overflow-hidden bg-transparent">

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
          bottom_anchor_ref={bottomAnchorRef}
          feed_ref={feedRef}
          scroll_ref={scrollRef}
          current_agent_name={currentAgentName ?? null}
          current_agent_avatar={currentAgentAvatar ?? null}
          workspace_agent_id={sessionIdentity?.agent_id ?? null}
          current_user_avatar={currentUserAvatar}
          is_last_round_pending_permissions={pendingPermissions}
          is_loading={isLoading}
          runtime_phase={runtimePhase}
          live_round_ids={liveRoundIds}
          is_mobile_layout={isMobileLayout}
          message_groups={messageGroups}
          on_open_agent_contact={onOpenAgentContact}
          on_open_workspace_file={onOpenWorkspaceFile}
          on_permission_response={sendPermissionResponse}
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

      <GoalPanel
        activity_key={`${messages.length}:${isLoading ? "loading" : "idle"}:${goalRefreshSeq}`}
        compact={isMobileLayout}
        continuation_hold={goalContinuationHold}
        disabled={!canControlSession}
        is_generating={isLoading}
        session_key={sessionKey}
        scope_label="会话 Goal"
      />

      <ComposerPanel
        allow_send_while_loading
        compact={isMobileLayout}
        default_delivery_policy={defaultDeliveryPolicy}
        input_queue_items={inputQueueItems}
        is_loading={isLoading}
        goal_scope_label="会话 Goal"
        runtime_phase={runtimePhase}
        on_delete_queued_message={deleteInputQueueMessage}
        on_enqueue_message={enqueueInputQueueMessage}
        on_create_goal={sessionKey && canControlSession ? handleCreateGoal : undefined}
        on_guide_queued_message={guideInputQueueMessage}
        on_prepare_attachments={handlePrepareAttachments}
        on_reorder_queue_messages={reorderInputQueueMessages}
        on_send_message={handleSendMessage}
        on_stop={handleStop}
        tour_anchor={CONVERSATION_TOUR_ANCHORS.composer}
      />
    </div>
  );
}
