import {
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";

import { useWorkspaceLiveStore } from "@/store/workspace-live";
import type {
  CommandCatalogData,
} from "@/types/generated/protocol";
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
  buildCommandCatalogRequest,
} from "./actions/conversation-command-builders";
import {
  buildAgentConversationResult,
  resolveAgentConversationConfig,
} from "./agent-conversation-model";

const EMPTY_COMMAND_CATALOG: CommandCatalogData = {
  commands: [],
  status: "loading",
};

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
  const { messages, setMessages } = useAgentMessageCollection();
  const [commandCatalog, setCommandCatalog] = useState<CommandCatalogData>(
    EMPTY_COMMAND_CATALOG,
  );
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
    acknowledgePermissionRequest,
    applyAgentRoundStatus,
    applyRoundStatus,
    clearLiveRuntimeState,
    clearOutboundRequest,
    pendingAgentSlots,
    pendingPermissions,
    roomAgentExecutionStates,
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
    trackStreamExecution,
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
  useEffect(() => {
    setCommandCatalog(EMPTY_COMMAND_CATALOG);
  }, [agentId, session.sessionKey]);

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

  const {
    enqueueStreamPayload,
    flushStreamPayloads,
    settleLiveMessageSnapshot,
  } = useConversationStreamBuffer(
    setMessages,
    session.activeSessionKeyRef,
  );
  const handleWebsocketMessage = useAgentEventDispatcher({
    callbacks: {
      applyWorkspaceEvent,
      enqueueStreamPayload,
      flushStreamPayloads,
      settleLiveMessageSnapshot,
      onBackgroundMessage: session.onBackgroundMessage,
      onRoomEvent,
      settleAgentWorkspaceWrites,
    },
    runtime: {
      acknowledgePermissionRequest,
      applyAgentRoundStatus,
      applyRoundStatus,
      rejectPendingRequestAck,
      resolvePendingRequestAck,
      removeRewrittenRound,
      setRuntimeStatus,
      syncSessionStatus,
      trackAssistantMessage,
      trackChatAck,
      trackStreamExecution,
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
      setCommandCatalog,
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
  const requestCommandCatalog = useCallback((initializeRuntime: boolean) => {
    if (!session.sessionKey || wsState !== "connected") {
      return;
    }
    wsSend(buildCommandCatalogRequest({
      agent_id: agentId,
      conversation_id: conversationId,
      initialize_runtime: initializeRuntime,
      room_id: roomId,
      session_key: session.sessionKey,
    }));
  }, [
    agentId,
    conversationId,
    roomId,
    session.sessionKey,
    wsSend,
    wsState,
  ]);
  const refreshCommandCatalog = useCallback(() => {
    requestCommandCatalog(true);
  }, [requestCommandCatalog]);
  useEffect(() => {
    if (
      !session.sessionKey ||
      wsState !== "connected" ||
      runtimeSnapshot.phase !== "idle"
    ) {
      return;
    }
    requestCommandCatalog(false);
  }, [
    requestCommandCatalog,
    runtimeSnapshot.phase,
    session.sessionKey,
    wsState,
  ]);

  const actionContext: AgentConversationActionContext = {
    acknowledgePermissionRequest,
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
    commandCatalog,
    error,
    messages,
    refreshCommandCatalog,
    runtime: {
      pendingAgentSlots,
      pendingPermissions,
      roomAgentExecutionStates,
      snapshot: runtimeSnapshot,
    },
    session,
    wsState,
  });
}
