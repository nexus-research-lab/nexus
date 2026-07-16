import {
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";

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
import { usePendingRequestAcks } from "./actions/use-pending-request-acks";
import { useRequestAckFailure } from "./actions/use-request-ack-failure";
import { useAgentMessageCollection } from "./message/use-agent-message-collection";
import { useAgentConversationRuntime } from "./runtime/use-agent-conversation-runtime";
import { useAgentSessionController } from "./session/controller/use-agent-session-controller";
import { useAgentConversationSocket } from "./transport/use-agent-conversation-socket";
import { useAgentEventDispatcher } from "./transport/use-agent-event-dispatcher";
import { useConversationStreamBuffer } from "./transport/use-conversation-stream-buffer";
import {
  buildAgentConversationResult,
  resolveAgentConversationConfig,
} from "./agent-conversation-model";

export function useAgentConversation(
  options: UseAgentConversationOptions = {},
): UseAgentConversationReturn {
  const {
    agentId,
    chatType,
    conversationId,
    identity,
    identitySessionKey,
    onError,
    onRoomEvent: onRoomEventCallback,
    roomId,
    wsUrl,
  } = resolveAgentConversationConfig(options);
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
    cancel_pending_request_acks: cancelPendingRequestAcks,
    reject_pending_request_ack: rejectPendingRequestAck,
    resolve_pending_request_ack: resolvePendingRequestAck,
    wait_for_request_ack: waitForRequestAck,
  } = usePendingRequestAcks();

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
    setRuntimeStatus,
    syncSessionStatus,
    trackAssistantMessage,
    trackChatAck,
    trackOutboundRequest,
    updateMessageStatus,
  } = useAgentConversationRuntime({
    agentId,
    chatType,
    resolvePendingRequestAck,
    setMessages,
    settleAgentWorkspaceWrites,
  });

  const session = useAgentSessionController({
    cancelPendingRequestAcks,
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

  const {
    handleRequestAckTimeout,
    settleChatAckWaitFailure,
    settleRequestAckWaitFailure,
  } = useRequestAckFailure({
    clearOutboundRequest,
    rejectPendingRequestAck,
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
      rejectPendingRequestAck,
      resolvePendingRequestAck,
      removeRewrittenRound,
      setRuntimeStatus,
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
    handleRequestAckTimeout,
    setPendingAgentSlots,
    settleChatAckWaitFailure,
    settleRequestAckWaitFailure,
    trackOutboundRequest,
    waitForRequestAck,
  });

  return buildAgentConversationResult({
    actions,
    error,
    messages,
    runtime: {
      pendingAgentSlots,
      pendingPermissions,
      snapshot: runtimeSnapshot,
    },
    session,
    wsState,
  });
}
