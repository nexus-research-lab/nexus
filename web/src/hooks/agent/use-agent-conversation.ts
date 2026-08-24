/**
 * INPUT: Agent/Room Conversation identity、共享 transport、Session/runtime 子域与事件回调。
 * OUTPUT: 消息、运行态、可靠性快照和带 ACK 所有权的 Conversation 公共命令。
 * POS: Agent Conversation 的 React composition root。
 */
import {
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";

import { useWorkspaceLiveStore } from "@/store/workspace-live";
import { WORKGRAPH_WORKFLOWS_CHANGED_EVENT } from "@/features/conversation/shared/execution/workgraph-distillation-intent";
import type {
  CommandCatalogData,
  ContextUsageData,
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
import { buildSessionBindMessage } from "./actions/conversation-command-builders";
import { useAgentConversationActions } from "./actions/use-agent-conversation-actions";
import { usePendingRequestAcks } from "./actions/use-pending-request-acks";
import { useRequestAckFailure } from "./actions/use-request-ack-failure";
import { useAgentMessageCollection } from "./message/use-agent-message-collection";
import { useAgentConversationRuntime } from "./runtime/use-agent-conversation-runtime";
import { readAgentSessionMessages } from "./session/conversation-lifecycle";
import { useAgentSessionController } from "./session/controller/use-agent-session-controller";
import { useAgentConversationSocket } from "./transport/use-agent-conversation-socket";
import { useAgentEventDispatcher } from "./transport/use-agent-event-dispatcher";
import { useConversationStreamBuffer } from "./transport/use-conversation-stream-buffer";
import { useConversationReliability } from "./reliability/use-conversation-reliability";
import {
  buildAgentConversationResult,
  resolveAgentConversationConfig,
} from "./agent-conversation-model";

const EMPTY_COMMAND_CATALOG: CommandCatalogData = {
  commands: [],
  status: "cold",
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
  const [contextUsageByAgent, setContextUsageByAgent] = useState<
    Record<string, ContextUsageData>
  >({});
  const reliability = useConversationReliability();
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
    discard_pending_request_ack: discardPendingRequestAck,
    has_pending_request_ack: hasPendingRequestAck,
    reject_pending_request_ack: rejectPendingRequestAck,
    resolve_pending_request_ack: resolvePendingRequestAck,
    track_pending_request_ack: trackPendingRequestAck,
    wait_for_request_ack: waitForRequestAck,
  } = usePendingRequestAcks();

  const {
    acknowledgePermissionRequest,
    applyAgentRoundStatus,
    applyRoundStatus,
    beginAgentRoundStop,
    clearLiveRuntimeState,
    clearOutboundRequest,
    confirmAgentRoundStop,
    pendingAgentSlots,
    pendingPermissions,
    roomAgentExecutionStates,
    reconcileRuntimeStateFromSnapshot,
    readStoppingAgentRoundIds,
    removeRewrittenRound,
    resetRuntimeMachine,
    runtimeSnapshot,
    setPendingAgentSlots,
    setPendingPermissions,
    setRuntimeStatus,
    settleAgentRoundStop,
    stoppingAgentRoundIds,
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
      reliability: reliability.controller,
      setMessages,
      setPendingAgentSlots,
      setPendingPermissions,
    },
  });
  const inputQueueItemsRef = useRef(session.inputQueueItems);
  inputQueueItemsRef.current = session.inputQueueItems;
  const getInputQueueItems = useCallback(
    () => inputQueueItemsRef.current,
    [],
  );
  useEffect(() => {
    setCommandCatalog(EMPTY_COMMAND_CATALOG);
  }, [agentId, session.sessionKey]);
  useEffect(() => {
    setContextUsageByAgent({});
  }, [session.sessionKey]);

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
    activeSessionKeyRef: session.activeSessionKeyRef,
    clearOutboundRequest,
    getInputQueueItems,
    hasPendingRequestAck,
    readSessionMessages: readAgentSessionMessages,
    rejectPendingRequestAck,
    reloadCurrentSession: session.reloadCurrentSession,
    resolvePendingRequestAck,
    reliability: reliability.controller,
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
      chatType,
      conversationId,
      isCurrentRoomEvent,
      isCurrentSessionEvent: session.isCurrentSessionEvent,
      roomId,
      sessionKey: session.sessionKey,
    },
    state: {
      reliability: reliability.controller,
      setCommandCatalog,
      setContextUsageByAgent,
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
  const {
    acquireRequestTransportLease,
    wsState,
    wsSend,
  } = useAgentConversationSocket({
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
    onReconnected: session.reloadCurrentSession,
    reliability: reliability.controller,
  });
  useEffect(() => {
    const refreshCommandCatalog = () => {
      const sessionKey = session.sessionKey?.trim();
      if (!sessionKey) {
        return;
      }
      wsSend(buildSessionBindMessage({
        session_key: sessionKey,
        last_seen_session_seq: sessionSeqCursorRef.current,
        agent_id: agentId,
        room_id: roomId,
        conversation_id: conversationId,
      }));
    };
    window.addEventListener(
      WORKGRAPH_WORKFLOWS_CHANGED_EVENT,
      refreshCommandCatalog,
    );
    return () => window.removeEventListener(
      WORKGRAPH_WORKFLOWS_CHANGED_EVENT,
      refreshCommandCatalog,
    );
  }, [agentId, conversationId, roomId, session.sessionKey, wsSend]);
  const actionContext: AgentConversationActionContext = {
    acknowledgePermissionRequest,
    activeSessionKeyRef: session.activeSessionKeyRef,
    identity,
    messages,
    pendingPermissions,
    reliability: reliability.controller,
    sessionKey: session.sessionKey,
    setMessages,
    setPendingPermissions,
    wsSend,
    wsState,
  };
  const actions = useAgentConversationActions({
    acquireRequestTransportLease,
    actionContext,
    beginAgentRoundStop,
    clearOutboundRequest,
    confirmAgentRoundStop,
    discardPendingRequestAck,
    handleRequestAckTimeout,
    readStoppingAgentRoundIds,
    rejectPendingRequestAck,
    resolvePendingRequestAck,
    settleAgentRoundStop,
    settleChatAckWaitFailure,
    settleRequestAckWaitFailure,
    trackPendingRequestAck,
    trackOutboundRequest,
    waitForRequestAck,
  });

  return buildAgentConversationResult({
    actions,
    commandCatalog,
    contextUsage: agentId ? contextUsageByAgent[agentId] ?? null : null,
    contextUsageByAgent,
    messages,
    reliability: reliability.projectSnapshot(session.sessionKey),
    runtime: {
      pendingAgentSlots,
      pendingPermissions,
      roomAgentExecutionStates,
      snapshot: runtimeSnapshot,
      stoppingAgentRoundIds,
    },
    session,
    wsState,
  });
}
