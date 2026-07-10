import { useCallback, useEffect, useMemo } from "react";

import { prepareRoomConversationAttachments } from "@/features/conversation/shared/composer/attachments/composer-attachments";
import { useConversationComposerHandlers } from "@/features/conversation/shared/composer/use-conversation-composer-handlers";
import { ROOM_GOAL_SCOPE_LABEL } from "@/features/conversation/shared/goal-continuation-hold";
import {
  useConversationSnapshotReporter,
  type ConversationSnapshotBuildInput,
} from "@/features/conversation/shared/use-conversation-snapshot-reporter";
import { useProviderAvailability } from "@/hooks/capability/use-provider-availability";
import { useExtractTodos } from "@/hooks/conversation/use-extract-todos";
import { useDefaultChatDeliveryPolicy } from "@/hooks/settings/use-default-chat-delivery-policy";
import { buildRoomSharedSessionKey } from "@/lib/conversation/session-key";
import { useAuth } from "@/shared/auth/auth-context";
import type { Agent } from "@/types/agent/agent";
import type {
  AgentConversationIdentity,
  RoomEventPayload,
} from "@/types/agent/agent-conversation";
import type { RoomConversationSnapshotPayload } from "@/types/conversation/conversation";
import type { Message } from "@/types/conversation/message";
import type { TodoItem } from "@/types/conversation/todo";

import { useRoomThreadSource } from "../use-room-thread-panel-data";
import { CONVERSATION_TOUR_ANCHORS } from "../../../room-tour";
import type { GroupChatPanelViewModel } from "./group-chat-panel-view";
import type { GroupChatPanelProps } from "./group-chat-panel-types";
import { useGroupChatSession } from "./use-group-chat-session";
import { useRoomGoalComposer } from "./use-room-goal-composer";

export function useGroupChatPanelModel({
  agentId,
  conversationId,
  currentAgentAvatar = null,
  currentAgentName = null,
  initialDraft = null,
  layout = "desktop",
  onConversationSnapshotChange,
  onCreateConversation = () => {},
  onInitialDraftConsumed,
  onLoadingChange,
  onOpenAgentContact,
  onOpenWorkspaceFile,
  onRoomEvent,
  onTodosChange,
  roomHostAgentId = null,
  roomHostAutoReplyEnabled = false,
  roomId = null,
  roomMembers,
}: GroupChatPanelProps): GroupChatPanelViewModel {
  const sessionKey = conversationId
    ? buildRoomSharedSessionKey(conversationId)
    : null;
  const goal = useRoomGoalComposer({
    roomHostAgentId,
    roomMembers,
    sessionKey,
  });
  const refreshGoal = goal.refresh;
  const handleRoomEvent = useCallback(
    (eventType: string, data: RoomEventPayload) => {
      if (eventType.startsWith("goal_")) {
        refreshGoal();
      }
      onRoomEvent?.(eventType, data);
    },
    [onRoomEvent, refreshGoal],
  );
  const identity = useRoomConversationIdentity({
    agentId,
    conversationId,
    roomId,
    sessionKey,
  });
  const session = useGroupChatSession({
    identity,
    onRoomEvent: handleRoomEvent,
    sessionKey,
  });
  const { conversation, history, roundIndexItems, roundScrollRef, scroll, timeline } =
    session;
  const directory = useRoomAgentDirectory(roomMembers);
  const { status: authStatus } = useAuth();
  const currentUserAvatar = authStatus?.avatar ?? null;
  const isMobileLayout = layout === "mobile";

  useGroupConversationObservers({
    conversationId,
    isLoading: conversation.is_loading,
    messages: conversation.messages,
    onConversationSnapshotChange,
    onLoadingChange,
    onTodosChange,
    sessionKey,
  });
  const { hasAvailableProvider, isReady: providerReady } =
    useProviderAvailability();
  const defaultDeliveryPolicy = useDefaultChatDeliveryPolicy();
  const prepareAttachments = useCallback(
    async (files: File[]) => {
      if (!roomId || !conversationId) {
        throw new Error("当前 Room 会话尚未就绪，暂时无法附加文件。");
      }
      return prepareRoomConversationAttachments(roomId, conversationId, files);
    },
    [conversationId, roomId],
  );
  const composer = useConversationComposerHandlers({
    canSendInitialDraft: true,
    initialDraft,
    initialDraftLogLabel: "room",
    isLoading: conversation.is_loading,
    onInitialDraftConsumed,
    prepareAttachments,
    scrollToBottom: scroll.scrollToBottom,
    sendMessage: conversation.send_message,
    sessionKey,
  });
  const stopMessage = conversation.stop_generation;

  useRoomThreadSource({
    agentAvatarMap: directory.avatars,
    agentNameMap: directory.names,
    conversationId,
    currentUserAvatar,
    messageGroups: timeline.message_groups,
    onOpenWorkspaceFile,
    onStopMessage: stopMessage,
    pendingPermissionGroups: timeline.pending_permission_groups,
    pendingSlotGroups: timeline.pending_slot_groups,
    sendPermissionResponse: conversation.send_permission_response,
  });

  return {
    composer: {
      allowSendWhileLoading: true,
      defaultDeliveryPolicy,
      disabled: false,
      enableLoops: true,
      goalCreateDisabledReason: goal.createDisabledReason,
      goalScopeLabel: ROOM_GOAL_SCOPE_LABEL,
      inputQueueItems: conversation.input_queue_items,
      isLoading: conversation.is_loading,
      onCreateGoal: sessionKey ? goal.onCreateGoal : undefined,
      onCreateLoopGoal: sessionKey ? goal.onCreateLoopGoal : undefined,
      onDeleteQueuedMessage: conversation.delete_input_queue_message,
      onEnqueueMessage: conversation.enqueue_input_queue_message,
      onGuideQueuedMessage: conversation.guide_input_queue_message,
      onPrepareAttachments: composer.handlePrepareAttachments,
      onReorderQueueMessages: conversation.reorder_input_queue_messages,
      onSendMessage: composer.handleSendMessage,
      onStop: () => conversation.stop_generation(),
      queueWhenSessionBusy: false,
      roomMembers,
      runtimePhase: conversation.runtime_phase,
      tourAnchor: CONVERSATION_TOUR_ANCHORS.composer,
    },
    feed: {
      isMobileLayout,
      refs: {
        bottomAnchorRef: scroll.bottomAnchorRef,
        feedRef: scroll.feedRef,
        roundScrollRef,
        scrollRef: scroll.scrollRef,
      },
      renderer: {
        agentAvatarMap: directory.avatars,
        agentNameMap: directory.names,
        currentAgentAvatar,
        currentAgentName,
        currentUserAvatar,
        isLastRoundPendingPermissions: conversation.pending_permissions,
        onOpenAgentContact,
        onOpenWorkspaceFile,
        onPermissionResponse: conversation.send_permission_response,
        onStopMessage: stopMessage,
        runtimePhase: conversation.runtime_phase,
      },
      source: {
        liveRoundIds: conversation.live_round_ids,
        messageGroups: timeline.message_groups,
        pendingPermissionGroups: timeline.pending_permission_groups,
        pendingSlotGroups: timeline.pending_slot_groups,
        roundIds: timeline.feed_round_ids,
        roundIndexItems,
      },
    },
    goalLead: {
      agentId: goal.leadAgentId,
      disabled: conversation.is_loading || roomMembers.length === 0,
      onChange: goal.setLeadAgentId,
      roomMembers,
    },
    goalPanel: {
      activityKey: `${conversation.messages.length}:${conversation.is_loading ? "loading" : "idle"}:${goal.refreshSequence}`,
      canControlSession: true,
      isLoading: conversation.is_loading,
      roomHostAgentId,
      roomHostAutoReplyEnabled: Boolean(roomHostAutoReplyEnabled),
      roomMembers,
      sessionKey,
    },
    isMobileLayout,
    navigator: {
      onLoadRoundWindow: conversation.load_round_window,
      onNavigateStart: scroll.pauseFollowLatest,
      roundScrollRef,
      scrollRef: scroll.scrollRef,
      timeline,
    },
    onCreateConversation,
    providerWarningVisible: providerReady && !hasAvailableProvider,
    scrollToLatest: {
      isLoading: conversation.is_loading,
      onClick: () => scroll.scrollToBottom("smooth"),
      visible: scroll.showScrollToBottom,
    },
    sessionKey,
    viewport: {
      error: conversation.error,
      isHistoryLoading: conversation.is_history_loading,
      onScroll: history.handleScroll,
      onTouchEnd: scroll.onTouchEnd,
      onTouchMove: scroll.onTouchMove,
      onTouchStart: scroll.onTouchStart,
      onWheel: scroll.onWheel,
      scrollRef: scroll.scrollRef,
    },
  };
}

function useRoomConversationIdentity({
  agentId,
  conversationId,
  roomId,
  sessionKey,
}: {
  agentId: string | null;
  conversationId: string | null;
  roomId: string | null;
  sessionKey: string | null;
}): AgentConversationIdentity | null {
  return useMemo(() => {
    if (!conversationId || !sessionKey) {
      return null;
    }
    return {
      agent_id: agentId,
      chat_type: "group",
      conversation_id: conversationId,
      room_id: roomId,
      session_key: sessionKey,
    };
  }, [agentId, conversationId, roomId, sessionKey]);
}

function useRoomAgentDirectory(roomMembers: Agent[]): {
  avatars?: Record<string, string | null>;
  names?: Record<string, string>;
} {
  return useMemo(() => {
    if (roomMembers.length === 0) {
      return {};
    }
    return {
      avatars: Object.fromEntries(
        roomMembers.map((member) => [member.agent_id, member.avatar ?? null]),
      ),
      names: Object.fromEntries(
        roomMembers.map((member) => [member.agent_id, member.name]),
      ),
    };
  }, [roomMembers]);
}

function useGroupConversationObservers({
  conversationId,
  isLoading,
  messages,
  onConversationSnapshotChange,
  onLoadingChange,
  onTodosChange,
  sessionKey,
}: {
  conversationId: string | null;
  isLoading: boolean;
  messages: Message[];
  onConversationSnapshotChange?: (
    snapshot: RoomConversationSnapshotPayload,
  ) => void;
  onLoadingChange?: (isLoading: boolean) => void;
  onTodosChange?: (todos: TodoItem[]) => void;
  sessionKey: string | null;
}): void {
  const todos = useExtractTodos(messages, sessionKey);
  useEffect(() => onTodosChange?.(todos), [onTodosChange, todos]);
  useEffect(
    () => onLoadingChange?.(isLoading),
    [isLoading, onLoadingChange],
  );
  useConversationSnapshotReporter({
    build_snapshot: buildRoomSnapshot,
    messages,
    on_snapshot_change: onConversationSnapshotChange,
    scope_key: conversationId,
  });
}

function buildRoomSnapshot(
  input: ConversationSnapshotBuildInput,
): RoomConversationSnapshotPayload {
  return {
    conversation_id: input.scope_key,
    ...(input.should_report_last_activity &&
    input.latest_reply_timestamp !== null
      ? { last_activity_at: input.latest_reply_timestamp }
      : {}),
    session_id: input.last_message.session_id ?? null,
  };
}
