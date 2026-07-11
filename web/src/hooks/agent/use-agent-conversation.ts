import {
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";

import { getAgentWsUrl } from "@/config/options";
import { useAgentStore } from "@/store/agent";
import { useWorkspaceLiveStore } from "@/store/workspace-live";
import type {
  WebSocketMessage,
  WebSocketState,
} from "@/types/system/websocket";
import type {
  RoomEventPayload,
  UseAgentConversationOptions,
  UseAgentConversationReturn,
} from "@/types/agent/agent-conversation";

import type { AgentConversationActionContext } from "./actions/conversation-action-context";
import { useAgentConversationActions } from "./actions/use-agent-conversation-actions";
import { useChatAckFailure } from "./actions/use-chat-ack-failure";
import { usePendingChatAcks } from "./actions/use-pending-chat-acks";
import { useAgentMessageCollection } from "./message/use-agent-message-collection";
import { useAgentConversationRuntime } from "./runtime/use-agent-conversation-runtime";
import { useAgentSessionController } from "./session/controller/use-agent-session-controller";
import { useAgentConversationSocket } from "./transport/use-agent-conversation-socket";
import { useAgentEventDispatcher } from "./transport/use-agent-event-dispatcher";
import { useConversationStreamBuffer } from "./transport/use-conversation-stream-buffer";

export function useAgentConversation(
  options: UseAgentConversationOptions = {},
): UseAgentConversationReturn {
  const wsUrl = options.ws_url || getAgentWsUrl();
  const identity = options.identity ?? null;
  const agentId = identity?.agent_id ?? null;
  const roomId = identity?.room_id ?? null;
  const conversationId = identity?.conversation_id ?? null;
  const chatType = identity?.chat_type ?? "dm";
  const identitySessionKey = identity?.session_key?.trim() || null;
  const onError = options.on_error;
  const onRoomEventCallback = options.on_room_event;
  const applyWorkspaceEvent = useWorkspaceLiveStore(
    (state) => state.apply_event,
  );
  const settleAgentWorkspaceWrites = useWorkspaceLiveStore(
    (state) => state.settle_agent_writes,
  );
  const agentRuntimeStatus = useAgentStore((state) => (
    agentId ? state.agent_runtime_statuses[agentId] : undefined
  ));

  const { messages, setMessages } = useAgentMessageCollection();
  const [error, setError] = useState<string | null>(null);
  const sessionSeqCursorRef = useRef(0);
  const roomSeqCursorRef = useRef(0);
  const wsSendRef = useRef<
    (payload: WebSocketMessage) => {
      disposition: "sent" | "queued" | "dropped";
    }
  >(() => ({ disposition: "dropped" }));
  const wsReconnectRef = useRef<() => void>(() => {});
  const wsStateRef = useRef<WebSocketState>("disconnected");

  const {
    cancel_pending_chat_acks: cancelPendingChatAcks,
    clear_pending_chat_ack: clearPendingChatAck,
    reject_pending_chat_ack: rejectPendingChatAck,
    wait_for_chat_ack: waitForChatAck,
  } = usePendingChatAcks();

  const {
    applyAgentRoundStatus,
    applyRoundStatus,
    clearLiveRuntimeState,
    clearOutboundRequest,
    pendingAgentSlots,
    pendingPermissions,
    reconcileRuntimeStateFromSnapshot,
    removeRewrittenRound,
    resetRuntimeMachine,
    runtimeSnapshot,
    setPendingAgentSlots,
    setPendingPermissions,
    syncSessionStatus,
    trackAssistantMessage,
    trackChatAck,
    trackOutboundRequest,
    updateMessageStatus,
  } = useAgentConversationRuntime({
    agentId,
    chatType,
    clearPendingChatAck,
    setMessages,
    settleAgentWorkspaceWrites,
  });

  const session = useAgentSessionController({
    cancelPendingChatAcks,
    identity,
    identitySessionKey,
    roomSeqCursorRef,
    sessionSeqCursorRef,
    runtime: {
      clearLiveRuntimeState,
      reconcileRuntimeStateFromSnapshot,
      resetRuntimeMachine,
      snapshot: runtimeSnapshot,
    },
    state: {
      messages,
      pendingAgentSlots,
      setError,
      setMessages,
      setPendingAgentSlots,
      setPendingPermissions,
    },
  });

  const isCurrentRoomEvent = useCallback(
    (incomingRoomId?: string | null): boolean => (
      Boolean(incomingRoomId && roomId) && incomingRoomId === roomId
    ),
    [roomId],
  );
  const onRoomEvent = useCallback(
    (eventType: string, data: RoomEventPayload): void => {
      onRoomEventCallback?.(eventType, data);
    },
    [onRoomEventCallback],
  );

  const { handleChatAckTimeout, settleChatAckWaitFailure } = useChatAckFailure({
    clearOutboundRequest,
    rejectPendingChatAck,
    setError,
    setMessages,
    wsReconnectRef,
    wsStateRef,
  });

  const enqueueStreamPayload = useConversationStreamBuffer(setMessages);
  const handleWebsocketMessage = useAgentEventDispatcher({
    callbacks: {
      applyWorkspaceEvent,
      enqueueStreamPayload,
      onBackgroundMessage: session.onBackgroundMessage,
      onRoomEvent,
      settleAgentWorkspaceWrites,
    },
    runtime: {
      applyAgentRoundStatus,
      applyRoundStatus,
      rejectChatAck: rejectPendingChatAck,
      removeRewrittenRound,
      syncSessionStatus,
      trackAssistantMessage,
      trackChatAck,
      updateMessageStatus,
    },
    scope: {
      agentId,
      conversationId,
      isCurrentRoomEvent,
      isCurrentSessionEvent: session.isCurrentSessionEvent,
      roomId,
      sessionKey: session.sessionKey,
    },
    state: {
      setError,
      setInputQueueItems: session.setInputQueueItems,
      setMessages,
      setPendingPermissions,
    },
    transport: {
      reloadCurrentSession: session.reloadCurrentSession,
      roomSeqCursorRef,
      sessionSeqCursorRef,
      wsSendRef,
      wsStateRef,
    },
  });
  const { wsState, wsSend } = useAgentConversationSocket({
    wsUrl,
    agentId,
    roomId,
    conversationId,
    sessionKey: session.sessionKey,
    sessionSeqCursorRef,
    roomSeqCursorRef,
    wsSendRef,
    wsReconnectRef,
    wsStateRef,
    onMessage: handleWebsocketMessage,
    onError,
    setError,
  });

  useEffect(() => {
    if (
      agentId &&
      agentRuntimeStatus?.running_task_count === 0 &&
      agentRuntimeStatus.status !== "running"
    ) {
      settleAgentWorkspaceWrites(agentId);
    }
  }, [agentId, agentRuntimeStatus, settleAgentWorkspaceWrites]);

  const actionContext: AgentConversationActionContext = {
    activeSessionKeyRef: session.activeSessionKeyRef,
    identity,
    messages,
    pendingPermissions,
    sessionKey: session.sessionKey,
    setError,
    setMessages,
    setPendingPermissions,
    wsSend,
    wsState,
  };
  const actions = useAgentConversationActions({
    actionContext,
    clearOutboundRequest,
    handleChatAckTimeout,
    setPendingAgentSlots,
    settleChatAckWaitFailure,
    trackOutboundRequest,
    waitForChatAck,
  });

  return {
    error,
    messages,
    session_key: session.sessionKey,
    ws_state: wsState,
    is_loading: runtimeSnapshot.isLoading,
    live_round_ids: runtimeSnapshot.liveRoundIds,
    is_session_loading: session.isSessionLoading,
    is_history_loading: session.isHistoryLoading,
    has_more_history: session.hasMoreHistory,
    history_prepend_token: session.historyPrependToken,
    runtime_phase: runtimeSnapshot.phase,
    pending_agent_slots: pendingAgentSlots,
    input_queue_items: session.inputQueueItems,
    pending_permissions: pendingPermissions,
    send_message: actions.sendMessage,
    rewrite_last_user_message: actions.rewriteLastMessage,
    enqueue_input_queue_message: actions.enqueueQueueMessage,
    delete_input_queue_message: actions.deleteQueueMessage,
    guide_input_queue_message: actions.guideQueueMessage,
    reorder_input_queue_messages: actions.reorderQueueMessages,
    bind_session_key: session.bindSessionKey,
    start_session: session.startSession,
    load_session: session.loadSession,
    load_older_messages: session.loadOlderMessages,
    load_round_window: session.loadRoundWindow,
    clear_session: session.clearSession,
    reset_session: session.resetSession,
    stop_generation: actions.stopGeneration,
    send_permission_response: actions.sendPermissionResponse,
  };
}

export type {
  UseAgentConversationOptions,
  UseAgentConversationReturn,
} from "@/types/agent/agent-conversation";
