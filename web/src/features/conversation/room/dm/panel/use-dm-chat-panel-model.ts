import { useCallback, useEffect, useMemo, useState } from "react";

import { prepareWorkspaceAttachments } from "@/features/conversation/shared/composer/attachments/composer-attachments";
import { useConversationComposerHandlers } from "@/features/conversation/shared/composer/use-conversation-composer-handlers";
import { goalContinuationHoldForPermission } from "@/features/conversation/shared/goal/goal-continuation-hold";
import { useConversationSession } from "@/features/conversation/shared/session/use-conversation-session";
import {
  useConversationSnapshotReporter,
  type ConversationSnapshotBuildInput,
} from "@/features/conversation/shared/use-conversation-snapshot-reporter";
import { useProviderAvailability } from "@/hooks/capability/use-provider-availability";
import { useConversationTodos } from "@/features/conversation/shared/todos/use-conversation-todos";
import { useDefaultChatDeliveryPolicy } from "@/hooks/settings/use-default-chat-delivery-policy";
import { createGoalApi } from "@/lib/api/goal-api";
import { useAuth } from "@/shared/auth/auth-context";
import type {
  AgentConversationIdentity,
  RoomEventPayload,
} from "@/types/agent/agent-conversation";
import type { SessionSnapshotPayload } from "@/types/conversation/conversation";
import type { Message } from "@/types/conversation/message";
import type { TodoItem } from "@/types/conversation/todo";

import { CONVERSATION_TOUR_ANCHORS } from "@/features/onboarding/tours/conversation-tour";
import type { DmChatPanelProps } from "./dm-chat-panel-types";
import type { DmChatPanelViewModel } from "./dm-chat-panel-view";

export function useDmChatPanelModel({
  currentAgentName = null,
  currentAgentAvatar = null,
  currentAgentPermissionMode = null,
  sessionIdentity,
  layout = "desktop",
  initialDraft = null,
  onInitialDraftConsumed,
  onOpenAgentContact,
  onOpenWorkspaceFile,
  onTodosChange,
  onConversationSnapshotChange,
  onRoomEvent,
}: DmChatPanelProps): DmChatPanelViewModel {
  const sessionKey = sessionIdentity?.session_key ?? null;
  const goal = useDmGoalController({
    agentName: currentAgentName,
    onRoomEvent,
    permissionMode: currentAgentPermissionMode,
    sessionKey,
  });
  const session = useConversationSession({
    chatType: "dm",
    debugName: "DmChatPanel",
    identity: sessionIdentity,
    onRoomEvent: goal.handleConversationEvent,
  });
  const { conversation, history, roundIndexItems, roundScrollRef, scroll, timeline } =
    session;
  const isMobileLayout = layout === "mobile";
  const { status: authStatus } = useAuth();
  const currentUserAvatar = authStatus?.avatar ?? null;
  const { hasAvailableProvider, isReady: providerReady } =
    useProviderAvailability();
  const defaultDeliveryPolicy = useDefaultChatDeliveryPolicy();

  useDmConversationObservers({
    identity: sessionIdentity,
    messages: conversation.messages,
    onConversationSnapshotChange,
    onTodosChange,
    sessionKey,
  });

  const prepareAttachments = useCallback(
    async (files: File[]) => {
      const agentId = sessionIdentity?.agent_id;
      if (!agentId) {
        throw new Error("当前会话尚未准备好，暂时无法附加文件。");
      }
      return prepareWorkspaceAttachments(agentId, files);
    },
    [sessionIdentity?.agent_id],
  );
  const composer = useConversationComposerHandlers({
    initialDraft,
    initialDraftLogLabel: "DM",
    isLoading: conversation.is_loading,
    onInitialDraftConsumed,
    prepareAttachments,
    scrollToBottom: scroll.scrollToBottom,
    sendMessage: conversation.send_message,
    sessionKey,
  });
  const rewriteLastUserMessage = conversation.rewrite_last_user_message;
  const handleEditLastUserMessage = useCallback(
    (messageId: string, content: string): void => {
      void rewriteLastUserMessage(messageId, content);
    },
    [rewriteLastUserMessage],
  );

  return {
    composer: {
      defaultDeliveryPolicy,
      inputQueueItems: conversation.input_queue_items,
      isLoading: conversation.is_loading,
      onCreateGoal: sessionKey ? goal.createGoal : undefined,
      onDeleteQueuedMessage: conversation.delete_input_queue_message,
      onEnqueueMessage: conversation.enqueue_input_queue_message,
      onGuideQueuedMessage: conversation.guide_input_queue_message,
      onPrepareAttachments: composer.handlePrepareAttachments,
      onReorderQueueMessages: conversation.reorder_input_queue_messages,
      onSendMessage: composer.handleSendMessage,
      onStop: conversation.stop_generation,
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
        currentAgentAvatar,
        currentAgentName,
        currentUserAvatar,
        onEditLastUserMessage: handleEditLastUserMessage,
        onOpenAgentContact,
        onOpenWorkspaceFile,
        onPermissionResponse: conversation.send_permission_response,
        workspaceAgentId: sessionIdentity?.agent_id ?? null,
      },
      source: {
        liveRoundIds: conversation.live_round_ids,
        messageGroups: timeline.message_groups,
        pendingPermissions: conversation.pending_permissions,
        roundIds: timeline.feed_round_ids,
        roundIndexItems,
        runtimePhase: conversation.runtime_phase,
      },
    },
    goalPanel: {
      activityKey: `${conversation.messages.length}:${conversation.is_loading ? "loading" : "idle"}:${goal.refreshSequence}`,
      continuationHold: goal.continuationHold,
      disabled: false,
      isGenerating: conversation.is_loading,
      scopeLabel: "会话 Goal",
      sessionKey,
    },
    isMobileLayout,
    navigator: {
      onLoadRoundWindow: conversation.load_round_window,
      onNavigateStart: scroll.pauseFollowLatest,
      roundScrollRef,
      scopeKey: sessionKey,
      scrollRef: scroll.scrollRef,
      timeline,
    },
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

function useDmGoalController({
  agentName,
  onRoomEvent,
  permissionMode,
  sessionKey,
}: {
  agentName: string | null;
  onRoomEvent?: (eventType: string, data: RoomEventPayload) => void;
  permissionMode: string | null;
  sessionKey: string | null;
}) {
  const [refreshSequence, setRefreshSequence] = useState(0);
  const refresh = useCallback(() => {
    setRefreshSequence((value) => value + 1);
  }, []);
  const handleConversationEvent = useCallback(
    (eventType: string, data: RoomEventPayload): void => {
      if (eventType.startsWith("goal_")) {
        refresh();
      }
      onRoomEvent?.(eventType, data);
    },
    [onRoomEvent, refresh],
  );
  const continuationHold = useMemo(
    () => goalContinuationHoldForPermission(agentName, permissionMode),
    [agentName, permissionMode],
  );
  const createGoal = useCallback(
    async (objective: string): Promise<void> => {
      if (!sessionKey) {
        throw new Error("当前会话尚未准备好，暂时无法启动 Goal。");
      }
      await createGoalApi({
        objective,
        session_key: sessionKey,
        token_budget: null,
      });
      refresh();
    },
    [refresh, sessionKey],
  );
  return {
    continuationHold,
    createGoal,
    handleConversationEvent,
    refreshSequence,
  };
}

function useDmConversationObservers({
  identity,
  messages,
  onConversationSnapshotChange,
  onTodosChange,
  sessionKey,
}: {
  identity: AgentConversationIdentity | null;
  messages: Message[];
  onConversationSnapshotChange?: (snapshot: SessionSnapshotPayload) => void;
  onTodosChange?: (todos: TodoItem[]) => void;
  sessionKey: string | null;
}): void {
  const todos = useConversationTodos(messages, sessionKey);
  useEffect(() => onTodosChange?.(todos), [onTodosChange, todos]);
  const buildSnapshot = useCallback(
    (input: ConversationSnapshotBuildInput) =>
      buildDmSnapshot(input, identity),
    [identity],
  );
  useConversationSnapshotReporter({
    build_snapshot: buildSnapshot,
    messages,
    on_snapshot_change: onConversationSnapshotChange,
    scope_key: sessionKey,
  });
}

function buildDmSnapshot(
  input: ConversationSnapshotBuildInput,
  identity: AgentConversationIdentity | null,
): SessionSnapshotPayload {
  return {
    agent_id: identity?.agent_id ?? null,
    conversation_id: identity?.conversation_id ?? null,
    room_id: identity?.room_id ?? null,
    room_session_id: identity?.room_session_id ?? null,
    session_id: input.last_message.session_id ?? null,
    session_key: input.scope_key,
    ...(input.should_report_last_activity &&
    input.latest_reply_timestamp !== null
      ? { last_activity_at: input.latest_reply_timestamp }
      : {}),
  };
}
